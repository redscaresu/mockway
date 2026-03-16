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

  inbound_rule {
    action = "accept"
    port   = 22
  }
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
	// Second apply must be a no-op — exit code 0 means no changes (idempotency check).
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderRegistry(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_registry_namespace" "ns" {
  name       = "mockway-registry"
  is_public  = false
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderRedis(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_redis_cluster" "redis" {
  name         = "mockway-redis"
  version      = "7.2.4"
  node_type    = "RED1-MICRO"
  user_name    = "default"
  password     = "R3d1sP@ssw0rd!"
  cluster_size = 1
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderRDB(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_rdb_instance" "db" {
  name           = "mockway-db"
  node_type      = "DB-DEV-S"
  engine         = "PostgreSQL-15"
  is_ha_cluster  = false
  disable_backup = true
}

resource "scaleway_rdb_database" "appdb" {
  instance_id = scaleway_rdb_instance.db.id
  name        = "appdb"
}

resource "scaleway_rdb_user" "appuser" {
  instance_id = scaleway_rdb_instance.db.id
  name        = "appuser"
  password    = "S3cr3tP@ssw0rd!"
  is_admin    = false
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderK8s(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_k8s_cluster" "cluster" {
  name                        = "mockway-cluster"
  version                     = "1.31.2"
  cni                         = "cilium"
  delete_additional_resources = false
}

resource "scaleway_k8s_pool" "pool" {
  cluster_id = scaleway_k8s_cluster.cluster.id
  name       = "mockway-pool"
  node_type  = "DEV1-M"
  size       = 1
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderLB(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_lb" "lb" {
  name = "mockway-lb"
  type = "LB-S"
}

resource "scaleway_lb_backend" "backend" {
  lb_id            = scaleway_lb.lb.id
  name             = "mockway-backend"
  forward_protocol = "http"
  forward_port     = 80
}

resource "scaleway_lb_frontend" "frontend" {
  lb_id        = scaleway_lb.lb.id
  backend_id   = scaleway_lb_backend.backend.id
  name         = "mockway-frontend"
  inbound_port = 80
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderIAM(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_iam_application" "app" {
  name = "mockway-app"
}

resource "scaleway_iam_api_key" "key" {
  application_id = scaleway_iam_application.app.id
  description    = "mockway api key"
}

resource "scaleway_iam_policy" "policy" {
  name           = "mockway-policy"
  application_id = scaleway_iam_application.app.id
  rule {
    organization_id      = "00000000-0000-0000-0000-000000000000"
    permission_set_names = ["AllProductsFullAccess"]
  }
}

resource "scaleway_iam_ssh_key" "ssh" {
  name       = "mockway-ssh"
  public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GkZL test@mockway"
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderVPC(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_vpc" "vpc" {
  name = "mockway-vpc"
}

resource "scaleway_vpc_private_network" "pn" {
  name   = "mockway-pn"
  vpc_id = scaleway_vpc.vpc.id
}

resource "scaleway_instance_security_group" "sg" {
  name                    = "mockway-sg"
  inbound_default_policy  = "drop"
  outbound_default_policy = "accept"
  stateful                = true
}

resource "scaleway_instance_server" "web" {
  name              = "mockway-server"
  type              = "DEV1-S"
  image             = "ubuntu_noble"
  security_group_id = scaleway_instance_security_group.sg.id
}

resource "scaleway_instance_private_nic" "nic" {
  server_id          = scaleway_instance_server.web.id
  private_network_id = scaleway_vpc_private_network.pn.id
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderLBACL(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_lb" "lb" {
  name = "mockway-lb"
  type = "LB-S"
}

resource "scaleway_lb_backend" "backend" {
  lb_id            = scaleway_lb.lb.id
  name             = "mockway-backend"
  forward_protocol = "http"
  forward_port     = 80
}

resource "scaleway_lb_frontend" "frontend" {
  lb_id         = scaleway_lb.lb.id
  backend_id    = scaleway_lb_backend.backend.id
  name          = "mockway-frontend"
  inbound_port  = 80
  external_acls = true
}

resource "scaleway_lb_acl" "allow_internal" {
  frontend_id = scaleway_lb_frontend.frontend.id
  name        = "allow-internal"
  index       = 1
  action { type = "allow" }
  match { ip_subnet = ["10.0.0.0/8"] }
}

resource "scaleway_lb_acl" "deny_all" {
  frontend_id = scaleway_lb_frontend.frontend.id
  name        = "deny-all"
  index       = 2
  action { type = "deny" }
  match { ip_subnet = ["0.0.0.0/0"] }
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderLBRoute(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_lb" "lb" {
  name = "mockway-lb"
  type = "LB-S"
}

resource "scaleway_lb_backend" "backend_a" {
  lb_id            = scaleway_lb.lb.id
  name             = "backend-a"
  forward_protocol = "http"
  forward_port     = 80
}

resource "scaleway_lb_backend" "backend_b" {
  lb_id            = scaleway_lb.lb.id
  name             = "backend-b"
  forward_protocol = "http"
  forward_port     = 8080
}

resource "scaleway_lb_frontend" "frontend" {
  lb_id        = scaleway_lb.lb.id
  backend_id   = scaleway_lb_backend.backend_a.id
  name         = "mockway-frontend"
  inbound_port = 80
}

resource "scaleway_lb_route" "route" {
  frontend_id       = scaleway_lb_frontend.frontend.id
  backend_id        = scaleway_lb_backend.backend_b.id
  match_host_header = "api.example.com"
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
	runIaCPlanNoOp(t, tmp, env, bin)
	runIaC(t, tmp, env, bin, "destroy", "-auto-approve", "-input=false", "-no-color")
}

func TestProviderK8sAutoUpgrade(t *testing.T) {
	bin := chooseIaCBinary()
	if bin == "" {
		t.Skip("skipping provider E2E: neither tofu nor terraform found in PATH")
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

resource "scaleway_k8s_cluster" "cluster" {
  name                        = "mockway-cluster"
  version                     = "1.31"
  cni                         = "cilium"
  delete_additional_resources = false

  auto_upgrade {
    enable                        = true
    maintenance_window_start_hour = 4
    maintenance_window_day        = "monday"
  }
}

resource "scaleway_k8s_pool" "pool" {
  cluster_id = scaleway_k8s_cluster.cluster.id
  name       = "mockway-pool"
  node_type  = "DEV1-M"
  size       = 1
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
	runIaCPlanNoOp(t, tmp, env, bin)
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
