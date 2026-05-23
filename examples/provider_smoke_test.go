// Package examples — auto-discovered provider smoke harness for mockway.
//
// Per the S53 plan, every example dir under examples/{working,misconfigured,
// updates}/ is auto-discovered here and run through the per-tree contract:
//
//   working/      apply → plan -detailed-exitcode (no diff) → destroy
//   misconfigured/ apply MUST fail (output must contain a documented
//                  Scaleway-style error indicator: 404 / 409 / conflict /
//                  not_found — same heuristic as scripts/test-misconfigured.sh)
//   updates/      apply -var-file=v1.tfvars → plan no-op
//                  → apply -var-file=v2.tfvars → plan no-op → destroy
//
// Adding a directory to ANY of the three trees auto-registers — no
// per-example test ticket. Each subdirectory is its own t.Run sub-test.
//
// Idempotency ratchet:
//
//   Working-dir failures listed in examples/known_broken.yaml are treated
//   as *expected* drift and do not fail the test. If a dir on the list
//   stops drifting, the test fails ("congratulations, remove this entry").
//   This implements the ratchet-only-tighten pattern: known_broken.yaml
//   can only shrink, never grow without explicit code-review.
//
// Gating:
//
//   - The smoke loop is gated by MOCKWAY_ENABLE_E2E=1 because it shells
//     out to `tofu` and spawns a mockway binary per example. Without the
//     env var, the tests t.Skip with a clear message.
//   - Each example gets its own freshly-spawned mockway on a kernel-
//     assigned port (so there is no cross-example state leakage even if
//     /mock/reset misbehaves). The mockway binary is built once into
//     a per-package temp dir.
//
// This package is `examples_test` so the auto-discovery walks the repo
// via runtime.Caller (mirror of fakeaws/examples/provider_smoke_test.go).
package examples_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	gateEnvVar     = "MOCKWAY_ENABLE_E2E"
	defaultMockURL = "http://127.0.0.1:8080"
)

// knownBrokenFile is the YAML file at examples/known_broken.yaml that
// allowlists currently-drifting working examples.
const knownBrokenFile = "known_broken.yaml"

// knownBroken models examples/known_broken.yaml.
type knownBroken struct {
	Entries []knownBrokenEntry `yaml:"entries"`
}

type knownBrokenEntry struct {
	Dir     string `yaml:"dir"`
	Symptom string `yaml:"symptom"`
	Ticket  string `yaml:"ticket"`
}

// scwEnv is the standard fake-credential env every example needs to point
// the Scaleway provider at the local mockway instance. Callers append
// SCW_API_URL=<spawned mockway URL> for the per-example run.
func scwEnv() []string {
	return []string{
		"SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX",
		"SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_REGION=fr-par",
		"SCW_DEFAULT_ZONE=fr-par-1",
		"TF_IN_AUTOMATION=1",
	}
}

// ----- top-level tests -----

// TestProviderSmokeWorking walks examples/working/<svc>/ and runs
// `tofu init && tofu apply && tofu plan -detailed-exitcode && tofu destroy`.
// plan-after-apply MUST be no-op (exit 0 = "no diff") unless the dir is
// listed in known_broken.yaml.
func TestProviderSmokeWorking(t *testing.T) {
	requireE2EGate(t)
	requireTofu(t)
	root := repoRoot(t)
	mockwayBin := buildMockway(t, root)
	broken := loadKnownBroken(t, root)
	dir := filepath.Join(root, "examples", "working")
	walkExamplesAndRun(t, dir, func(t *testing.T, exampleDir string) {
		runWorkingExample(t, mockwayBin, exampleDir, broken)
	})
}

// TestProviderSmokeMisconfigured walks examples/misconfigured/<svc>/.
// `tofu apply` MUST fail. The failure output MUST contain a Scaleway-
// style error indicator (404 / 409 / conflict / not_found) — mirroring
// the heuristic in scripts/test-misconfigured.sh. Multi-workspace dirs
// (those that contain subdirs instead of main.tf) are skipped: the
// dedicated shell script test-misconfigured.sh covers them.
func TestProviderSmokeMisconfigured(t *testing.T) {
	requireE2EGate(t)
	requireTofu(t)
	root := repoRoot(t)
	mockwayBin := buildMockway(t, root)
	dir := filepath.Join(root, "examples", "misconfigured")
	walkExamplesAndRun(t, dir, func(t *testing.T, exampleDir string) {
		// Multi-workspace examples have sub-dirs (vpc/pn, app/platform)
		// and no top-level main.tf. The single-workspace contract here
		// doesn't fit them — the dedicated shell script
		// scripts/test-misconfigured.sh covers them.
		if _, err := os.Stat(filepath.Join(exampleDir, "main.tf")); err != nil {
			t.Skipf("multi-workspace example (no top-level main.tf) — covered by scripts/test-misconfigured.sh")
		}
		runMisconfiguredExample(t, mockwayBin, exampleDir)
	})
}

// TestProviderSmokeUpdates walks examples/updates/<svc>/, applies v1,
// asserts plan is clean, applies v2, asserts plan is clean, destroys.
// Each updates/ directory MUST contain v1.tfvars + v2.tfvars + main.tf.
func TestProviderSmokeUpdates(t *testing.T) {
	requireE2EGate(t)
	requireTofu(t)
	root := repoRoot(t)
	mockwayBin := buildMockway(t, root)
	dir := filepath.Join(root, "examples", "updates")
	walkExamplesAndRun(t, dir, func(t *testing.T, exampleDir string) {
		runUpdatesExample(t, mockwayBin, exampleDir)
	})
}

// ----- discovery -----

func walkExamplesAndRun(t *testing.T, parent string, run func(t *testing.T, dir string)) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Logf("skipping %s — directory does not exist (no examples in this tree yet)", parent)
			return
		}
		t.Fatalf("read %s: %v", parent, err)
	}
	any := false
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		any = true
		dir := filepath.Join(parent, ent.Name())
		t.Run(ent.Name(), func(t *testing.T) {
			run(t, dir)
		})
	}
	if !any {
		t.Logf("no example subdirectories under %s", parent)
	}
}

// ----- per-tree contracts -----

func runWorkingExample(t *testing.T, mockwayBin, dir string, broken *knownBroken) {
	t.Helper()
	tmp := copyExampleToTemp(t, dir)
	mockURL, stop := startMockway(t, mockwayBin)
	defer stop()
	rewriteLocalhostPort(t, tmp, mockURL)
	env := exampleEnv(mockURL)

	relDir := relativeToExamples(t, dir)
	expectDrift := broken.has(relDir)

	tofu(t, tmp, env, "init", "-input=false", "-no-color", "-reconfigure")
	tofu(t, tmp, env, "apply", "-auto-approve", "-input=false", "-no-color")

	planExit := tofuPlanExit(t, tmp, env, nil)
	switch planExit {
	case 0:
		if expectDrift {
			t.Fatalf("known_broken entry for %s is no longer broken — congratulations, remove this entry from %s",
				relDir, knownBrokenFile)
		}
	case 2:
		if !expectDrift {
			t.Fatalf("second apply is not idempotent: plan detected drift for %s\n"+
				"(if this drift is being deferred for a fix, add to %s with a ticket id)",
				relDir, knownBrokenFile)
		}
		t.Logf("known_broken: %s drifts on second plan (allowlisted, see %s)", relDir, knownBrokenFile)
	default:
		t.Fatalf("tofu plan -detailed-exitcode returned unexpected exit %d", planExit)
	}

	tofu(t, tmp, env, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func runMisconfiguredExample(t *testing.T, mockwayBin, dir string) {
	t.Helper()
	tmp := copyExampleToTemp(t, dir)
	mockURL, stop := startMockway(t, mockwayBin)
	defer stop()
	rewriteLocalhostPort(t, tmp, mockURL)
	env := exampleEnv(mockURL)

	tofu(t, tmp, env, "init", "-input=false", "-no-color", "-reconfigure")
	out, err := tofuExpectingFailure(t, tmp, env, "apply", "-auto-approve", "-input=false", "-no-color")
	if err == nil {
		t.Fatalf("misconfigured example: tofu apply UNEXPECTEDLY succeeded — expected a 404/409/conflict failure")
	}
	// Mirror the heuristic in scripts/test-misconfigured.sh — accept
	// any of: 404, 409, conflict, not_found, or a generic "Error:" line
	// (data-source lookups can fail provider-side before they reach the
	// API, e.g. iam_group_nonexistent_user prints
	// `Error: no user found with the email address ...`). The script's
	// final OR-fallback is the literal token "error" — we keep parity.
	low := strings.ToLower(out)
	hasErrorIndicator := strings.Contains(low, "404") ||
		strings.Contains(low, "409") ||
		strings.Contains(low, "conflict") ||
		strings.Contains(low, "not_found") ||
		strings.Contains(low, "not found") ||
		strings.Contains(low, "error")
	if !hasErrorIndicator {
		t.Fatalf("misconfigured example: tofu apply failed but output has no recognisable error indicator (404/409/conflict/not_found/error)\noutput:\n%s", out)
	}
}

func runUpdatesExample(t *testing.T, mockwayBin, dir string) {
	t.Helper()
	v1Name := "v1.tfvars"
	v2Name := "v2.tfvars"
	for _, p := range []string{v1Name, v2Name} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Fatalf("updates example missing %s: %v", p, err)
		}
	}

	tmp := copyExampleToTemp(t, dir)
	mockURL, stop := startMockway(t, mockwayBin)
	defer stop()
	rewriteLocalhostPort(t, tmp, mockURL)
	env := exampleEnv(mockURL)

	v1 := filepath.Join(tmp, v1Name)
	v2 := filepath.Join(tmp, v2Name)

	tofu(t, tmp, env, "init", "-input=false", "-no-color", "-reconfigure")

	tofu(t, tmp, env, "apply", "-auto-approve", "-input=false", "-no-color", "-var-file="+v1)
	planExit := tofuPlanExit(t, tmp, env, []string{"-var-file=" + v1})
	if planExit == 2 {
		t.Fatalf("v1 plan reports drift (not idempotent) for updates example")
	}
	if planExit != 0 {
		t.Fatalf("v1 plan returned unexpected exit %d", planExit)
	}

	tofu(t, tmp, env, "apply", "-auto-approve", "-input=false", "-no-color", "-var-file="+v2)
	planExit = tofuPlanExit(t, tmp, env, []string{"-var-file=" + v2})
	if planExit == 2 {
		t.Fatalf("v2 plan reports drift (not idempotent) for updates example")
	}
	if planExit != 0 {
		t.Fatalf("v2 plan returned unexpected exit %d", planExit)
	}

	tofu(t, tmp, env, "destroy", "-auto-approve", "-input=false", "-no-color", "-var-file="+v2)
}

// ----- tofu wrappers -----

// tofu runs `tofu <args...>` in workdir with env and fails the test on error.
func tofu(t *testing.T, workdir string, env []string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tofu", args...)
	cmd.Dir = workdir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("tofu %v timed out\n%s", args, out)
	}
	if err != nil {
		t.Fatalf("tofu %v: %v\n%s", args, err, out)
	}
}

// tofuExpectingFailure runs `tofu <args...>` and returns (output, error)
// without failing the test if the command fails — callers assert on the
// failure themselves (used for misconfigured/).
func tofuExpectingFailure(t *testing.T, workdir string, env []string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tofu", args...)
	cmd.Dir = workdir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("tofu %v timed out\n%s", args, out)
	}
	return string(out), err
}

// tofuPlanExit runs `tofu plan -detailed-exitcode` and returns the exit
// code (0 = no diff, 1 = error, 2 = drift). extra is appended to args.
func tofuPlanExit(t *testing.T, workdir string, env []string, extra []string) int {
	t.Helper()
	args := append([]string{"plan", "-detailed-exitcode", "-input=false", "-no-color"}, extra...)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tofu", args...)
	cmd.Dir = workdir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("tofu plan timed out\n%s", out)
	}
	if err == nil {
		return 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("tofu plan: %v\n%s", err, out)
	}
	code := exitErr.ExitCode()
	if code == 1 {
		t.Fatalf("tofu plan failed (exit 1):\n%s", out)
	}
	return code
}

// ----- mockway lifecycle -----

var (
	mockwayBuildOnce sync.Once
	mockwayBuildPath string
	mockwayBuildErr  error
)

// buildMockway builds the mockway binary into a per-process temp file
// the first time it is called, then returns the cached path. Returning
// the path (instead of t.TempDir-scoping) lets every t.Run reuse the
// same binary, which is important since the build is the slowest step.
func buildMockway(t *testing.T, root string) string {
	t.Helper()
	mockwayBuildOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "mockway-e2e-*")
		if err != nil {
			mockwayBuildErr = fmt.Errorf("mkdir temp: %w", err)
			return
		}
		bin := filepath.Join(tmpDir, "mockway")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/mockway")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			mockwayBuildErr = fmt.Errorf("go build ./cmd/mockway: %w\n%s", err, out)
			return
		}
		mockwayBuildPath = bin
	})
	if mockwayBuildErr != nil {
		t.Fatalf("build mockway: %v", mockwayBuildErr)
	}
	return mockwayBuildPath
}

// startMockway spawns a fresh mockway binary on a free port and waits
// for /mock/state to respond. It returns the base URL and a stop func
// the caller must defer.
func startMockway(t *testing.T, bin string) (string, func()) {
	t.Helper()
	port := pickFreePort(t)
	cmd := exec.Command(bin, "--port", fmt.Sprintf("%d", port))
	// Mockway's chi middleware Logger writes to stderr — capture into
	// a discard writer so test output stays clean. Surface stderr only
	// on a failed wait.
	var logBuf strings.Builder
	cmd.Stdout = &logBuf
	cmd.Stderr = &logBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start mockway: %v", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForMockReady(url, 15*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("mockway did not become ready at %s: %v\nmockway log:\n%s", url, err, logBuf.String())
	}

	stop := func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	return url, stop
}

// pickFreePort asks the kernel for a free TCP port.
func pickFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen :0: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// waitForMockReady polls GET <url>/mock/state until it returns 200 or
// the deadline expires.
func waitForMockReady(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/mock/state")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out after %s", timeout)
}

// ----- copy + rewrite helpers -----

// copyExampleToTemp copies the example directory into a t.TempDir so
// state files (terraform.tfstate, .terraform/) don't pollute the repo.
// Subdirectories are copied recursively (cross_state_orphan etc).
func copyExampleToTemp(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copy %s → %s: %v", src, dst, err)
	}
	return dst
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		// Skip pre-existing state from manual runs.
		base := filepath.Base(path)
		if base == "terraform.tfstate" || base == "terraform.tfstate.backup" || strings.HasPrefix(base, ".terraform") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
}

// rewriteLocalhostPort rewrites every `localhost:8080` or
// `127.0.0.1:8080` occurrence under tmp to point at the mockway URL the
// test just spawned. Mirrors scripts/test-examples.sh's pattern. The
// rewrite is defensive — most providers.tf only reference :8080 in
// comments — but keeps multi-workspace remote_state paths consistent if
// any are ever added.
func rewriteLocalhostPort(t *testing.T, tmp, mockURL string) {
	t.Helper()
	host := strings.TrimPrefix(strings.TrimPrefix(mockURL, "http://"), "https://")
	err := filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".tf" && ext != ".tfvars" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := strings.ReplaceAll(string(data), "localhost:8080", host)
		updated = strings.ReplaceAll(updated, "127.0.0.1:8080", host)
		if updated == string(data) {
			return nil
		}
		return os.WriteFile(path, []byte(updated), info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("rewrite localhost:8080 → %s under %s: %v", host, tmp, err)
	}
}

// exampleEnv builds the env slice for a single tofu invocation: inherits
// the parent process env (so PATH locates tofu + go), overlays the SCW
// fake credentials, and points SCW_API_URL at the spawned mockway.
func exampleEnv(mockURL string) []string {
	env := append([]string(nil), os.Environ()...)
	env = append(env, scwEnv()...)
	env = append(env, "SCW_API_URL="+mockURL)
	return env
}

// ----- known_broken helpers -----

func loadKnownBroken(t *testing.T, root string) *knownBroken {
	t.Helper()
	path := filepath.Join(root, "examples", knownBrokenFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &knownBroken{}
		}
		t.Fatalf("read %s: %v", path, err)
	}
	var kb knownBroken
	if err := yaml.Unmarshal(data, &kb); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	// Normalise — entries are written with forward slashes.
	for i := range kb.Entries {
		kb.Entries[i].Dir = filepath.ToSlash(kb.Entries[i].Dir)
	}
	return &kb
}

func (kb *knownBroken) has(relDir string) bool {
	if kb == nil {
		return false
	}
	for _, e := range kb.Entries {
		if e.Dir == relDir {
			return true
		}
	}
	return false
}

// relativeToExamples returns the dir's path relative to examples/, using
// forward slashes so it matches known_broken.yaml entries verbatim.
func relativeToExamples(t *testing.T, dir string) string {
	t.Helper()
	root := repoRoot(t)
	rel, err := filepath.Rel(filepath.Join(root, "examples"), dir)
	if err != nil {
		t.Fatalf("rel %s: %v", dir, err)
	}
	return filepath.ToSlash(rel)
}

// ----- generic helpers -----

func requireE2EGate(t *testing.T) {
	t.Helper()
	if os.Getenv(gateEnvVar) != "1" {
		t.Skipf("set %s=1 to run example smoke tests (requires tofu + ability to spawn mockway from ./cmd/mockway; default mock URL %s)",
			gateEnvVar, defaultMockURL)
	}
}

func requireTofu(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skipf("tofu not on PATH: %v", err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate mockway repo root from %s", file)
	return ""
}
