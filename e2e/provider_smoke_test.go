//go:build provider_e2e

package e2e_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/redscaresu/mockway/testutil"
	"github.com/stretchr/testify/require"
)

// TestExamplesWorkingIdempotency auto-discovers every directory under
// examples/working/ and runs apply → plan (no-op check) → destroy.
// Adding a new working example automatically adds it to the idempotency gate.
func TestExamplesWorkingIdempotency(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping examples idempotency: neither tofu nor terraform found in PATH")
	}

	repoRoot := filepath.Dir(mustAbs(t, "."))
	examplesDir := filepath.Join(repoRoot, "examples", "working")
	entries, err := os.ReadDir(examplesDir)
	require.NoError(t, err)

	env := append(os.Environ(),
		"SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX",
		"SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_ZONE=fr-par-1",
		"SCW_DEFAULT_REGION=fr-par",
		"TF_IN_AUTOMATION=1",
	)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		srcDir := filepath.Join(examplesDir, name)
		t.Run(name, func(t *testing.T) {
			ts, cleanup := testutil.NewTestServer(t)
			defer cleanup()

			tmp := t.TempDir()
			require.NoError(t, copyDir(srcDir, tmp))

			testEnv := append(env, "SCW_API_URL="+ts.URL)
			runIaC(t, tmp, testEnv, bin, "init", "-input=false", "-no-color", "-reconfigure")
			runIaC(t, tmp, testEnv, bin, "apply", "-auto-approve", "-input=false", "-no-color")
			runIaCPlanNoOp(t, tmp, testEnv, bin)
			runIaC(t, tmp, testEnv, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
		})
	}
}

// TestExamplesUpdatesIdempotency auto-discovers every directory under
// examples/updates/ and runs apply v1 → no-op plan → apply v2 → no-op plan → destroy.
// Each update example must contain main.tf plus v1.tfvars and v2.tfvars.
func TestExamplesUpdatesIdempotency(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping updates idempotency: neither tofu nor terraform found in PATH")
	}

	repoRoot := filepath.Dir(mustAbs(t, "."))
	updatesDir := filepath.Join(repoRoot, "examples", "updates")
	entries, err := os.ReadDir(updatesDir)
	require.NoError(t, err)

	env := append(os.Environ(),
		"SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX",
		"SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_ZONE=fr-par-1",
		"SCW_DEFAULT_REGION=fr-par",
		"TF_IN_AUTOMATION=1",
	)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		srcDir := filepath.Join(updatesDir, name)
		t.Run(name, func(t *testing.T) {
			ts, cleanup := testutil.NewTestServer(t)
			defer cleanup()

			tmp := t.TempDir()
			require.NoError(t, copyDir(srcDir, tmp))

			testEnv := append(env, "SCW_API_URL="+ts.URL)
			v1Vars := filepath.Join(tmp, "v1.tfvars")
			v2Vars := filepath.Join(tmp, "v2.tfvars")

			runIaC(t, tmp, testEnv, bin, "init", "-input=false", "-no-color", "-reconfigure")

			// Apply v1 and verify idempotent.
			runIaC(t, tmp, testEnv, bin, "apply", "-auto-approve", "-input=false", "-no-color", "-var-file="+v1Vars)
			runIaCPlanNoOpWithVars(t, tmp, testEnv, bin, v1Vars)

			// Apply v2 and verify idempotent.
			runIaC(t, tmp, testEnv, bin, "apply", "-auto-approve", "-input=false", "-no-color", "-var-file="+v2Vars)
			runIaCPlanNoOpWithVars(t, tmp, testEnv, bin, v2Vars)

			// Destroy.
			runIaC(t, tmp, testEnv, bin, "destroy", "-auto-approve", "-input=false", "-no-color", "-var-file="+v2Vars)
		})
	}
}

// runIaCPlanNoOpWithVars runs "plan -detailed-exitcode" with a var-file and
// fails the test if the plan is non-empty.
func runIaCPlanNoOpWithVars(t *testing.T, workdir string, env []string, bin, varFile string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "plan", "-detailed-exitcode", "-input=false", "-no-color", "-var-file="+varFile)
	cmd.Dir = workdir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s plan timed out\n%s", bin, out.String())
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Fatalf("plan after apply is not idempotent: drift detected\n%s", out.String())
		}
		t.Fatalf("%s plan -detailed-exitcode failed: %v\n%s", bin, err, out.String())
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	return abs
}

// copyDir copies all files from src into dst (non-recursive: working examples are flat).
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func chooseIaCBinary() string {
	if _, err := exec.LookPath("tofu"); err == nil {
		return "tofu"
	}
	if _, err := exec.LookPath("terraform"); err == nil {
		return "terraform"
	}
	return ""
}

func runIaC(t *testing.T, workdir string, env []string, bin string, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workdir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s %v timed out\n%s", bin, args, out.String())
	}
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", bin, args, err, out.String())
	}
}

// runIaCPlanNoOp runs "plan -detailed-exitcode" and fails the test if the plan
// is non-empty (exit code 2 = drift) or errors (exit code 1). Exit code 0
// means no changes — the expected result for an idempotent second apply.
func runIaCPlanNoOp(t *testing.T, workdir string, env []string, bin string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "plan", "-detailed-exitcode", "-input=false", "-no-color")
	cmd.Dir = workdir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s plan timed out\n%s", bin, out.String())
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Fatalf("second apply is not idempotent: plan detected drift\n%s", out.String())
		}
		t.Fatalf("%s plan -detailed-exitcode failed: %v\n%s", bin, err, out.String())
	}
}
