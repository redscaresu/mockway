# mockway

Local mock of the Scaleway API for offline OpenTofu and Terraform testing.

Mockway runs as a single Go binary, tracks resource state in SQLite, and exposes Scaleway-like API routes on one port. State is kept in-memory by default — each run starts clean, which is ideal for test cycles. Use `--db ./mockway.db` if you need state to survive restarts.

> **This project is in early development.** The services in the compatibility matrix below have been verified against the real Scaleway Terraform provider with a full `apply → plan (no-op) → destroy` cycle. Other services may have handler code but have not been tested.

## Install

```bash
go install github.com/redscaresu/mockway/cmd/mockway@latest
```

## Run

```bash
mockway --port 8080
```

File-backed DB (state persists across restarts):

```bash
mockway --port 8080 --db ./mockway.db
```

Default is `:memory:` — state resets on exit.

### Echo mode

```bash
mockway --echo --port 8080
```

Echo mode replaces the real mock with a catch-all handler that returns `{"ok":true}` for every request and logs the method, path, and headers to stdout. Use it to discover which API endpoints a Terraform config calls before writing handlers:

```bash
mockway --echo --port 8080 &
export SCW_API_URL=http://localhost:8080
terraform apply   # runs against echo; watch stdout for the paths you need
```

Then grep the log output for the paths, implement them in mockway, and switch back to normal mode.

## Usage with OpenTofu / Terraform

Point the Scaleway provider at Mockway:

```bash
export SCW_API_URL=http://localhost:8080
export SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX
export SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000
export SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
```

Then run `tofu plan && tofu apply` or `terraform plan && terraform apply` as normal.

## Provider Compatibility Matrix

Each row reflects a verified `apply → plan (no-op) → destroy` cycle against the real `scaleway/scaleway` Terraform provider (≥ 2.50). "No-op plan" means the second `plan -detailed-exitcode` exits 0 — no drift.

| Service | API prefix | Terraform resources | Status | Example |
|---------|-----------|---------------------|--------|---------|
| Instance | `/instance/v1/zones/{zone}/` | `scaleway_instance_server`, `scaleway_instance_security_group` (with inbound rules), `scaleway_instance_ip`, `scaleway_instance_private_nic`, `scaleway_instance_volume` | ✅ verified | [`examples/working/basic_instance`](examples/working/basic_instance), [`examples/working/instance_volume`](examples/working/instance_volume) |
| IAM | `/iam/v1alpha1/` | `scaleway_iam_application`, `scaleway_iam_api_key`, `scaleway_iam_policy` (with rules), `scaleway_iam_ssh_key` | ✅ verified | [`examples/working/iam_full`](examples/working/iam_full) |
| Load Balancer | `/lb/v1/zones/{zone}/` | `scaleway_lb`, `scaleway_lb_backend`, `scaleway_lb_frontend`, `scaleway_lb_acl`, `scaleway_lb_route` | ✅ verified | [`examples/working/load_balancer`](examples/working/load_balancer), [`examples/working/lb_with_acl`](examples/working/lb_with_acl), [`examples/working/lb_with_route`](examples/working/lb_with_route) |
| Kubernetes | `/k8s/v1/regions/{region}/` | `scaleway_k8s_cluster` (with auto-upgrade, version upgrade), `scaleway_k8s_pool` | ✅ verified | [`examples/working/kubernetes_cluster`](examples/working/kubernetes_cluster), [`examples/working/k8s_with_auto_upgrade`](examples/working/k8s_with_auto_upgrade) |
| RDB | `/rdb/v1/regions/{region}/` | `scaleway_rdb_instance`, `scaleway_rdb_database`, `scaleway_rdb_user` | ✅ verified | [`examples/working/rdb_instance`](examples/working/rdb_instance) |
| Redis | `/redis/v1/zones/{zone}/` | `scaleway_redis_cluster` | ✅ verified | [`examples/working/redis_cluster`](examples/working/redis_cluster) |
| Registry | `/registry/v1/regions/{region}/` | `scaleway_registry_namespace` | ✅ verified | [`examples/working/registry_namespace`](examples/working/registry_namespace) |
| Marketplace | `/marketplace/v2/` | (image label resolution — used by Instance) | ✅ verified | — |
| VPC | `/vpc/v1/`, `/vpc/v2/` | `scaleway_vpc`, `scaleway_vpc_private_network`, `scaleway_vpc_route` | ⚠️ handler only | [`examples/working/vpc_and_private_network`](examples/working/vpc_and_private_network) |
| VPC GW | `/vpc-gw/v2/` | `scaleway_vpc_public_gateway`, `scaleway_vpc_gateway_network` | ⚠️ handler only | — |
| Account (legacy) | `/account/v2alpha1/` | `scaleway_account_ssh_key` | ✅ verified | — |
| IPAM | `/ipam/v1/` | list stub | ⚠️ stub | — |
| Domain | `/domain/v2beta1/` | `scaleway_domain_zone`, `scaleway_domain_record` | ⚠️ handler only | — |
| Block | `/block/v1alpha1/` | `scaleway_block_volume`, block snapshots | ✅ verified | — |

## What mockway catches

`terraform validate` checks syntax. `terraform plan` checks the dependency graph. Neither calls the API, so neither can catch mistakes that only surface when the provider actually makes HTTP requests. mockway fills that gap by enforcing the same FK constraints as the real Scaleway API during a local apply.

Three categories of mistake are consistently missed by `validate`, `plan`, and `mock_provider`:

**1. Wrong reference** — a reference that resolves to the wrong value at apply time. `validate` and `plan` see a valid string; mockway returns 404 when the ID doesn't match any stored resource.

Common forms:
- `.name` used where `.id` is required — both are strings, both pass type checks, the name is just not a UUID (`scaleway_rdb_instance.db.name` instead of `.id`)
- Wrong resource referenced — autocomplete picks the parent when the child was intended (`scaleway_lb.lb.id` instead of `scaleway_lb_backend.backend.id` — both are UUIDs)
- Missing resource entirely — a child resource is defined but its parent was never added to the config

Examples: [`misconfigured/app_stack_db_ref`](examples/misconfigured/app_stack_db_ref), [`misconfigured/lb_missing_backend`](examples/misconfigured/lb_missing_backend), [`misconfigured/nic_with_missing_private_network`](examples/misconfigured/nic_with_missing_private_network), [`misconfigured/security_group_name_not_id`](examples/misconfigured/security_group_name_not_id)

**2. Wrong destroy order** — a parent resource is deleted while children still hold FK references to it. A full `terraform destroy` is safe because Terraform reads the dependency graph and destroys in the right order. The failure only appears with `-target` destroys or multi-workspace teardowns where Terraform can't see the full graph.

Example: [`misconfigured/vpc_deleted_before_private_network`](examples/misconfigured/vpc_deleted_before_private_network)

**3. Cross-state orphan** — one Terraform state file owns a shared resource (e.g. an IAM application); another state file creates children that reference it by ID. When the first workspace is destroyed, its `terraform plan` shows a clean single-resource deletion — it has no visibility into the other state file. mockway holds state for both workspaces on the same port and returns 409 when the parent is deleted while children still exist.

Example: [`misconfigured/cross_state_orphan`](examples/misconfigured/cross_state_orphan)

---

## Features

- Single-port HTTP API with path-based service routing
- Stateful resource lifecycle (create, get, list, delete)
- SQLite-backed state (`:memory:` by default, file DB optional)
- Foreign-key integrity (404 on bad references, 409 on dependent deletes)
- Cascade semantics matching real Scaleway (IP detaches on server delete, NICs cascade-delete)
- Admin API under `/mock/*` for state inspection and reset
- Catch-all 501 handler logs unimplemented routes for easy discovery
- Auth: `X-Auth-Token` required on Scaleway routes (any non-empty value accepted)

## Known Limitations

- **No field validation.** Mockway accepts whatever JSON you send and stores it. It does not validate `commercial_type`, `node_type`, required fields, or value constraints beyond foreign key references.
- **No pagination.** All list endpoints return all results in a single page. `page`/`per_page` query parameters are ignored.
- **No S3 / Object Storage.** S3-compatible endpoints are not implemented. Scaleway's Object Storage uses the S3 protocol (AWS SigV4 auth, XML responses).
- **IAM rules are policy-scoped.** `GET /iam/v1alpha1/rules?policy_id=<id>` returns rules stored during policy create. `GET /iam/v1alpha1/rules` without a `policy_id` always returns an empty list.
- **User data is discarded.** `PATCH /servers/{id}/user_data/{key}` accepts the body but does not store it. `GET /servers/{id}/user_data` always returns an empty list.
- **Unimplemented routes return 501.** Any route not explicitly handled returns `501 Not Implemented` with a log line — useful for discovering which endpoints your Terraform config needs.
- **VPC gateway network `enable_masquerade` drift.** `scaleway_vpc_gateway_network` with `enable_masquerade = true` causes a perpetual plan diff — the GET response shape doesn't match what the provider expects. Needs proxy-capture investigation against the real API.

## Not Implemented

`spec_diff.py` tracks all gaps against downloaded Scaleway OpenAPI specs. Run `python3 scripts/spec_diff.py --all` to see the current list. As of this writing there are **155 unimplemented endpoints** across 8 services. The table below documents what is and isn't covered per Terraform resource, so you know before you run.

### Instance

| Terraform resource | Status | Gap |
|---|---|---|
| `scaleway_instance_server` | ✅ full CRUD + actions | — |
| `scaleway_instance_ip` | ✅ full CRUD | — |
| `scaleway_instance_security_group` | ✅ full CRUD + rules | — |
| `scaleway_instance_private_nic` | ✅ full CRUD | — |
| `scaleway_instance_volume` | ✅ full CRUD | — |
| `scaleway_instance_image` | ❌ not implemented | `POST/GET/PATCH/DELETE /images` |
| `scaleway_instance_snapshot` | ❌ not implemented | `POST/GET/PATCH/DELETE /snapshots` |
| `scaleway_instance_placement_group` | ❌ not implemented | `POST/GET/PATCH/DELETE /placement_groups` |

Hot-plug operations (`attach-volume`, `detach-volume`) are also not implemented — standalone volumes can be created and destroyed but not dynamically attached to running servers.

### Kubernetes

| Terraform resource | Status | Gap |
|---|---|---|
| `scaleway_k8s_cluster` | ✅ CRUD + upgrade + set-type | K8s API server ACL rules (`/acls`) not implemented |
| `scaleway_k8s_pool` | ✅ CRUD + upgrade | pool labels/taints (`set-labels`, `set-taints`) not implemented |

### VPC

| Terraform resource | Status | Gap |
|---|---|---|
| `scaleway_vpc` | ✅ full CRUD | — |
| `scaleway_vpc_private_network` | ✅ full CRUD | — |
| `scaleway_vpc_route` | ✅ full CRUD | — |
| `scaleway_vpc_public_gateway` | ✅ full CRUD | — |
| `scaleway_vpc_gateway_network` | ✅ full CRUD | — |
| `scaleway_vpc_acl` | ❌ not implemented | `GET/PUT /vpc/v2/vpcs/{id}/acl-rules` |

### Redis

| Terraform resource | Status | Gap |
|---|---|---|
| `scaleway_redis_cluster` | ✅ full CRUD | — |
| `scaleway_redis_cluster` with ACLs | ❌ partial | `POST/GET/DELETE /redis/v1/acls` not implemented |

### RDB

| Terraform resource | Status | Gap |
|---|---|---|
| `scaleway_rdb_instance` | ✅ full CRUD | — |
| `scaleway_rdb_database` | ✅ full CRUD | — |
| `scaleway_rdb_user` | ✅ full CRUD | — |
| `scaleway_rdb_acl` | ✅ full CRUD | — |
| `scaleway_rdb_read_replica` | ✅ CRUD | — |

### Load Balancer

| Terraform resource | Status | Gap |
|---|---|---|
| `scaleway_lb` | ✅ full CRUD | — |
| `scaleway_lb_backend` | ✅ full CRUD | — |
| `scaleway_lb_frontend` | ✅ full CRUD | — |
| `scaleway_lb_acl` | ✅ full CRUD | — |
| `scaleway_lb_route` | ✅ full CRUD | — |
| `scaleway_lb_certificate` | ✅ full CRUD | — |
| LB subscribers / alerting | ❌ not implemented | `POST/GET /subscribers`, `subscribe/unsubscribe` |

### IAM

| Terraform resource | Status | Gap |
|---|---|---|
| `scaleway_iam_application` | ✅ full CRUD | — |
| `scaleway_iam_api_key` | ✅ full CRUD | — |
| `scaleway_iam_policy` | ✅ full CRUD + rules | — |
| `scaleway_iam_ssh_key` | ✅ full CRUD | — |
| `scaleway_iam_user` | ✅ full CRUD | — |
| `scaleway_iam_group` | ✅ full CRUD + members | — |
| IAM organizations / SAML / MFA / JWTs / quotas / logs | ❌ not implemented | Not exposed as Terraform resources |

### Registry, Domain, Block, IPAM, Marketplace

| Service | Status | Gap |
|---|---|---|
| `scaleway_registry_namespace` | ✅ full CRUD | — |
| Registry images / tags | ❌ not implemented | Not Terraform-managed resources |
| Domain DNS (`scaleway_domain_zone`, `scaleway_domain_record`) | ✅ full CRUD | Zone create/update/delete + record patch/list |
| Block storage (`scaleway_block_volume`) | ✅ full CRUD | — |
| Block snapshots | ✅ full CRUD | — |
| IPAM (`scaleway_ipam_ip`) | ⚠️ stub | List stub only |
| Marketplace | ✅ local-image resolution | `/images`, `/versions`, `/categories` catalog not implemented (not needed for Terraform) |
| Object Storage / S3 | ❌ not implemented | Requires S3 protocol (AWS SigV4) — out of scope |
| Serverless / Containers / Functions | ❌ not implemented | — |

## Admin API

```
POST /mock/reset          — wipe all state
GET  /mock/state          — full resource graph as JSON
GET  /mock/state/{service} — single service (instance, vpc, lb, k8s, rdb, iam)
```

## Examples

The [`examples/`](examples/) directory contains self-contained Terraform configs you can run against mockway to see it in action. It includes working configs that apply and destroy cleanly, and deliberately misconfigured configs that show the kinds of mistakes mockway catches — mistakes that `terraform validate` and `terraform plan` both miss.

See [examples/README.md](examples/README.md) for step-by-step instructions.

## Practical Example

[hardened-scaleway-openclaw](https://github.com/redscaresu/hardened-scaleway-openclaw) is a real Terraform config that provisions a hardened Scaleway instance with IAM credentials, a security group, and cloud-init. It uses mockway for offline testing via `make test-apply`.

The setup involves three things:

**1. Install mockway**

The Makefile auto-installs if missing:

```makefile
test-apply:
	@command -v mockway >/dev/null 2>&1 || go install github.com/redscaresu/mockway/cmd/mockway@latest
	./scripts/test-with-mock.sh
```

**2. Start mockway and configure the environment**

The test script starts mockway on a random port, overrides the S3 backend with a local backend (so no remote state bucket is needed), and sets dummy credentials:

```bash
# Start mockway
mockway -port "$PORT" &

# Point the Scaleway provider at mockway
export SCW_API_URL="http://localhost:$PORT"
export SCW_ACCESS_KEY="SCWXXXXXXXXXXXXXXXXX"
export SCW_SECRET_KEY="00000000-0000-0000-0000-000000000000"
export SCW_DEFAULT_PROJECT_ID="00000000-0000-0000-0000-000000000000"
export SCW_DEFAULT_ORGANIZATION_ID="00000000-0000-0000-0000-000000000000"

# Override the S3 backend with local state
cat > "$TF_DIR/backend_override.tf" <<EOF
terraform {
  backend "local" {
    path = "$TF_TEMP_DIR/test.tfstate"
  }
}
EOF
```

**3. Run the full cycle**

```bash
terraform init -input=false -reconfigure
terraform apply  -auto-approve -input=false -var 'tailscale_auth_key=dummy' ...
terraform destroy -auto-approve -input=false -var 'tailscale_auth_key=dummy' ...
```

This creates and destroys 8 real Scaleway resources (SSH key, IAM application, API key, policy, reserved IP, security group with firewall rules, and a server with cloud-init) entirely offline against mockway. No Scaleway account needed, no API costs, runs in seconds.

See [scripts/test-with-mock.sh](https://github.com/redscaresu/hardened-scaleway-openclaw/blob/main/scripts/test-with-mock.sh) for the full script including port selection, health checks, and cleanup.

## Development

```bash
go test ./...
```

To manually verify the working examples end-to-end (apply → no-op plan → destroy):

```bash
./scripts/test-examples.sh                  # all examples/working/* dirs
./scripts/test-examples.sh load_balancer    # specific dir by name
```

To verify the misconfigured examples fail with the expected error (404/409) rather than a provider panic:

```bash
./scripts/test-misconfigured.sh                        # all examples/misconfigured/* dirs
./scripts/test-misconfigured.sh lb_acl_missing_frontend  # specific example
```

Both scripts start mockway on a random free port, reset state between runs, and report pass/fail per directory. These are manual debugging aids; the authoritative CI test is `go test -tags provider_e2e ./e2e`.

Key packages:

- `cmd/mockway` — binary entrypoint
- `handlers` — HTTP routes and error mapping
- `repository` — SQLite schema + CRUD/state logic
- `models` — domain errors
- `testutil` — shared integration test helpers
