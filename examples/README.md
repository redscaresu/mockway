# mockway examples

mockway is a local Scaleway API mock that catches infrastructure mistakes at apply time — mistakes that `terraform validate`, `terraform plan`, and `terraform test` all let through.

The broken configs in this directory are valid Terraform. They pass validation. They produce a clean plan. The errors only surface when the provider actually calls the API and mockway enforces the same FK constraints as real Scaleway.

---

## Prerequisites

- Go 1.21+
- Terraform or OpenTofu

---

## Step 1 — Install mockway

```bash
go install github.com/redscaresu/mockway/cmd/mockway@latest
```

---

## Step 2 — Start mockway

Open a dedicated terminal and leave it running:

```bash
mockway --port 8080
```

mockway logs every API call to stdout. Unimplemented endpoints print as:

```
UNIMPLEMENTED: POST /some/v1/endpoint
```

This is useful when onboarding a new Terraform module — run apply and let the logs tell you which endpoints need adding to mockway.

To confirm mockway is ready:

```bash
curl -s http://localhost:8080/mock/state
```

---

## Step 3 — Export environment variables

These redirect the Scaleway provider to mockway. The credentials are fake — mockway accepts any non-empty token.

```bash
export SCW_API_URL=http://localhost:8080
export SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX
export SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000
export SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
export SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000
export SCW_DEFAULT_REGION=fr-par
export SCW_DEFAULT_ZONE=fr-par-1
```

---

## Step 4 — Run an example

Each example is a self-contained directory. `cd` into it and run the usual Terraform workflow:

```bash
cd working/basic_instance

terraform init
terraform apply -auto-approve
terraform destroy -auto-approve
```

The failure path examples will error during apply (or a targeted destroy). The comments at the top of each `main.tf` show the exact error and how to fix it.

---

## Step 5 — Reset state between runs

mockway holds state in memory. After a failed apply, partial resources may remain. Reset without restarting:

```bash
curl -s -X POST http://localhost:8080/mock/reset
```

Inspect current state at any time:

```bash
curl -s http://localhost:8080/mock/state | jq .
```

---

## Examples

### working

Configs that apply and destroy correctly. These show the right way to express Scaleway resource dependencies so that mockway's FK constraints — and the real API's — are satisfied.

| Example | What it demonstrates |
|---|---|
| `working/basic_instance` | Server with a security group; `security_group_id` uses a resource reference |
| `working/vpc_and_private_network` | VPC → private network → server → private NIC dependency chain |
| `working/vpc_route` | VPC with a custom route pointing at a private network |
| `working/iam_full` | IAM application, API key, policy (with rules), and SSH key |
| `working/load_balancer` | LB → backend → frontend; each resource references its parent |
| `working/lb_with_ip` | LB with explicit `scaleway_lb_ip` allocation via `ip_id` |
| `working/lb_with_acl` | LB with frontend ACL rules |
| `working/lb_with_route` | LB with route rules |
| `working/lb_with_certificate` | LB with Let's Encrypt certificate attached to frontend |
| `working/lb_private_network` | LB attached to a VPC private network |
| `working/kubernetes_cluster` | K8s cluster and node pool |
| `working/k8s_with_auto_upgrade` | K8s cluster with auto-upgrade and maintenance window |
| `working/rdb_instance` | RDB instance, database, and user; `disable_backup` translation |
| `working/rdb_read_replica` | RDB instance with a read replica |
| `working/rdb_with_acl` | RDB instance with ACL rules (stateful — persisted across reads) |
| `working/redis_cluster` | Redis cluster |
| `working/registry_namespace` | Container registry namespace |
| `working/block_volume` | Block storage volume |
| `working/block_snapshot` | Block volume with a snapshot |
| `working/instance_ip` | Standalone reserved IP |
| `working/instance_volume` | Standalone instance volume |
| `working/domain_zone` | DNS zone with an A record |

### updates

Update scenarios that verify in-place resource modifications work correctly. Each directory contains `main.tf` (with variables), `v1.tfvars` (initial state), and `v2.tfvars` (updated state). The test cycle is: apply v1 → verify no-op plan → apply v2 → verify no-op plan → destroy.

**How to run an update example manually:**

```bash
cd updates/rename_server

terraform init
terraform apply -auto-approve -var-file=v1.tfvars    # create with v1 values
terraform plan -detailed-exitcode -var-file=v1.tfvars # should be no-op (exit 0)
terraform apply -auto-approve -var-file=v2.tfvars     # update to v2 values
terraform plan -detailed-exitcode -var-file=v2.tfvars # should be no-op (exit 0)
terraform destroy -auto-approve -var-file=v2.tfvars   # clean up
```

Or use the automated script:

```bash
./scripts/test-updates.sh                    # all update examples
./scripts/test-updates.sh rename_server      # specific example
```

| Example | What it changes v1 → v2 |
|---|---|
| `updates/rename_server` | Instance server name |
| `updates/update_lb` | LB name and description |
| `updates/update_lb_backend` | LB backend name and `health_check_max_retries` |
| `updates/update_k8s_cluster` | K8s cluster name and tags |
| `updates/update_rdb_instance` | RDB instance name and `disable_backup` toggle |
| `updates/update_redis_cluster` | Redis cluster name and tags |
| `updates/update_vpc` | VPC name |
| `updates/update_private_network` | Private network name |
| `updates/update_vpc_route` | VPC route description |
| `updates/update_iam_application` | IAM application name and description |
| `updates/update_iam_policy` | IAM policy name and description |
| `updates/update_iam_ssh_key` | SSH key name |
| `updates/update_security_group` | Security group name and inbound default policy |
| `updates/update_instance_ip` | Instance IP tags |
| `updates/update_block_volume` | Block volume name and tags |
| `updates/update_registry_namespace` | Registry namespace description and `is_public` toggle |

### misconfigured

Configs with real mistakes that `terraform validate` and `terraform plan` do not catch. Each fails when `terraform apply` (or a targeted destroy) calls the API.

Three categories of failure are represented:

**Wrong reference** — a Terraform reference that resolves to the wrong value at apply time.

| Example | The mistake | What mockway returns |
|---|---|---|
| `misconfigured/security_group_name_not_id` | `security_group_id = sg.name` — uses the name string where the UUID `.id` is required; both are valid strings | `404` at server create |
| `misconfigured/nic_with_missing_private_network` | `private_network_id` is a stale UUID — the private network was never created | `404` at NIC create, after the server already applied |
| `misconfigured/lb_missing_backend` | `backend_id` points at `scaleway_lb.lb.id` instead of `scaleway_lb_backend.backend.id` — wrong resource, both are UUIDs | `404` at frontend create |
| `misconfigured/lb_acl_missing_frontend` | `frontend_id` points at `scaleway_lb.lb.id` instead of `scaleway_lb_frontend.fe.id` — both are UUIDs | `404` at ACL create |
| `misconfigured/k8s_pool_missing_cluster` | `cluster_id` is a literal UUID; no cluster with that ID was created | `404` at pool create |
| `misconfigured/app_stack_db_ref` | 12-resource stack (IAM + Instance + LB + RDB); `scaleway_rdb_database` uses `.name` instead of `.id` for `instance_id` — 11 resources apply before the failure | `404` at database create |

**Wrong destroy order** — a parent resource is deleted while children still hold references to it.

| Example | The mistake | What mockway returns |
|---|---|---|
| `misconfigured/vpc_deleted_before_private_network` | Two workspaces: `vpc/` creates a VPC, `pn/` creates a private network referencing it. Destroying `vpc/` while `pn/` is still applied fails. | `409 cannot delete: dependents exist` |

**Cross-state orphan** — a resource in one Terraform state file references a resource in another; standard tooling cannot see across state files.

| Example | The mistake | What mockway returns |
|---|---|---|
| `misconfigured/cross_state_orphan` | `platform/` owns a shared IAM application; `app/` creates an API key and policy that reference it via variable. Destroying `platform/` while `app/` is still applied fails. | `409 cannot delete: dependents exist` |

The cross-state example is two steps — see `misconfigured/cross_state_orphan/platform/main.tf` for the full reproduction instructions.

---

## Why `terraform validate` and `terraform plan` miss these

`terraform validate` checks syntax and type constraints. It cannot know whether a UUID exists in a remote API.

`terraform plan` shows what will be created or destroyed. It does not call create/delete endpoints, so FK violations never surface.

`terraform test` with a provider mock could catch some cases, but only if the mock is configured to enforce the same constraints as the real API — which requires the same level of work as mockway itself.

mockway runs the full apply/destroy cycle locally, with no cloud credentials, and enforces the same resource dependency rules as Scaleway. Broken configs fail fast, before CI runs and before any real infrastructure is touched.
