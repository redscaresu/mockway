//go:build provider_e2e

package e2e_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/redscaresu/mockway/testutil"
	"github.com/stretchr/testify/require"
)

func TestProviderApplyDestroySmoke(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E smoke test: neither tofu nor terraform found in PATH")
	}

	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	tmp := t.TempDir()
	tf := `
terraform {
  required_providers {
    scaleway = {
      source  = "scaleway/scaleway"
      version = "~> 2.50"
    }
  }
}

provider "scaleway" {}

resource "scaleway_instance_security_group" "sg" {
  name                    = "mockway-sg"
  inbound_default_policy  = "drop"
  outbound_default_policy = "accept"
}

resource "scaleway_instance_server" "srv" {
  name              = "mockway-srv"
  type              = "DEV1-S"
  image             = "ubuntu_noble"
  security_group_id = scaleway_instance_security_group.sg.id
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "main.tf"), []byte(strings.TrimSpace(tf)+"\n"), 0o600))

	env := append(os.Environ(),
		"SCW_API_URL="+ts.URL,
		"SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX",
		"SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000",
		"SCW_DEFAULT_ZONE=fr-par-1",
		"SCW_DEFAULT_REGION=fr-par",
		"TF_IN_AUTOMATION=1",
	)

	runIaC(t, tmp, env, bin, "init", "-input=false", "-no-color", "-reconfigure")
	runIaC(t, tmp, env, bin, "apply", "-auto-approve", "-input=false", "-no-color")
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
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
