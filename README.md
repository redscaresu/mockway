# mockway

[![ci](https://github.com/redscaresu/mockway/actions/workflows/ci.yml/badge.svg)](https://github.com/redscaresu/mockway/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go 1.24+](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)](go.mod)

Local mock of the Scaleway API for offline OpenTofu and Terraform testing.

Mockway runs as a single Go binary, tracks resource state in SQLite, and exposes Scaleway-like API routes on one port. State is kept in-memory by default — each run starts clean, which is ideal for test cycles. Use `--db ./mockway.db` if you need state to survive restarts.

> **This project is in early development.** The services in the compatibility matrix below have been verified against the real Scaleway Terraform provider with a full `apply → plan (no-op) → destroy` cycle. Other services may have handler code but have not been tested.

## Architecture

```
              +------------------+
              |  Terraform /     |   SCW_API_URL=http://localhost:8080
              |  OpenTofu apply  |
              +--------+---------+
                       |
                       v
              +-----------------------------+
              |   mockway HTTP server       |
              |   chi router, port :8080    |
              +-----+-----------------+-----+
                    |                 |
        +-----------+--------+ +------+---------------+
        | Scaleway routes    | | Admin routes         |
        |  /instance/v1/...  | |  /mock/state         |
        |  /lb/v1/...        | |  /mock/reset         |
        |  /rdb/v1/...       | |  /mock/snapshot      |
        |  /vpc/v1/...       | |  /mock/restore       |
        |  /k8s/v1/...       | |                      |
        |  /iam/v1alpha1/... | | (no auth)            |
        |  /redis/v1/...     | +----------------------+
        |  /registry/v1/...  |          |
        |  /domain/v2beta1   |          |
        |  /ipam/v1/...      |          |
        |  /block/v1alpha1   |          |
        |  ...               |          |
        +---------+----------+          |
                  |                     |
                  v                     v
              +---------------------------------+
              |   SQLite repository             |
              |   - PRAGMA foreign_keys = ON    |
              |   - SetMaxOpenConns(1)          |
              |   - .snapshot file on demand    |
              +---------------------------------+

   Auth header  : X-Auth-Token (any non-empty value accepted)
   Resource ids : UUIDv4
   Timestamps   : RFC3339
   Drift contract: apply -> plan (no-op exit 0) -> destroy
```

## Consumer

[`infrafactory`](https://github.com/redscaresu/infrafactory) drives mockway as Layer-2 mock-deploy in a deterministic generate → validate → apply → destroy loop. Its cross-repo e2e helpers (`internal/e2e/helpers.go::StartMockway`) start mockway from this source tree on a free port for every test, so any mockway change you ship is exercised by infrafactory's Scaleway training scenarios automatically.

### One-shot demo (with sibling repos)

If you've cloned the four-repo layout, the easiest way to see mockway
in action is via [`infrafactory`](https://github.com/redscaresu/infrafactory)'s
`make up`:

```bash
cd ~/dev && for r in infrafactory fakeaws fakegcp fakegenesys mockway; do git clone https://github.com/redscaresu/$r.git; done
cd infrafactory && make up
./bin/infrafactory run scenarios/training/web-app-paris.yaml --config infrafactory.yaml   # drives mockway end-to-end
make down
```

That brings up mockway on `:8080`, exercises a Scaleway scenario
through `tofu apply → test → destroy`, and tears everything down.

## Install

```bash
go install github.com/redscaresu/mockway/cmd/mockway@latest
```

### Docker

Pre-built multi-arch images are published to GitHub Container Registry on every push to `main`:

```bash
docker run --rm -p 8080:8080 ghcr.io/redscaresu/mockway:latest --port 8080
```

The Dockerfile in the repo root produces a `~15MB` static image (multi-stage build from `golang:1.25-alpine`).

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

### Driving real terraform/tofu against the mock

The repo ships `make demo-*` targets that wire up the env + drive a real
`scaleway/scaleway` provider through a full lifecycle against mockway.
Useful for blog demos and manual exploration.

```bash
make build              # one-time
make demo-apply         # boots mockway + init + apply + plan-no-op (lb_with_ip)
make demo-apply EXAMPLE=load_balancer
make demo-shell         # bash subshell with env set + cd'd to example
make demo-help          # full target list + available examples
make demo-down          # kill mockway + clean temp files
```

The `plan -detailed-exitcode == 0` check at the end of `demo-apply` is the
correctness oracle — drift in any wire-shape detail (case-sensitive JSON
keys, exact status codes, default fields) surfaces here.

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

## Testing examples

The canonical entry point for end-to-end example coverage is `go test ./e2e/...`:

```bash
# Run every example end-to-end (apply → plan-no-op → destroy)
MOCKWAY_ENABLE_E2E=1 go test ./e2e/...

# Run one specific example, with verbose output
MOCKWAY_ENABLE_E2E=1 go test ./e2e/... -v -run TestProviderSmokeWorking/<dir>

# Filter to a single sub-tree
MOCKWAY_ENABLE_E2E=1 go test ./e2e/... -run TestProviderSmokeMisconfigured
```

The harness builds the `mockway` binary once and spawns a fresh
instance on a kernel-assigned port per example, so dirs can't
cross-contaminate state. The shell scripts under `scripts/` are
supplementary manual debugging aids — the in-test path above is
authoritative.

The same in-test pattern is canonical across all four sibling fakes
([fakegcp](https://github.com/redscaresu/fakegcp),
[fakeaws](https://github.com/redscaresu/fakeaws),
[fakegenesys](https://github.com/redscaresu/fakegenesys)) — `go test`
against the smoke harness works identically in each.

## API compatibility

The point of mockway is to be wire-shape compatible with the real `scaleway/scaleway` provider — every byte the provider sends or expects to receive must match what real Scaleway would do, or the provider detects "drift" and the apply loop fails. Three guardrails enforce this; they're identical across [`mockway`](https://github.com/redscaresu/mockway) (Scaleway), [`fakegcp`](https://github.com/redscaresu/fakegcp) (GCP), [`fakeaws`](https://github.com/redscaresu/fakeaws) (AWS), and [`fakegenesys`](https://github.com/redscaresu/fakegenesys) (Genesys Cloud CCaaS).

### 1. Three example trees, auto-discovered

Every directory under `examples/` is an executable contract against a real Terraform/OpenTofu provider:

| Tree | Contract |
|---|---|
| `examples/working/<svc>/` | `apply → plan -detailed-exitcode 0 → destroy` — second plan MUST be a no-op |
| `examples/misconfigured/<svc>/` | `apply` MUST fail with a documented error indicator (404 / 409 / conflict / not_found) |
| `examples/updates/<svc>/` | `apply -var-file=v1.tfvars → plan no-op → apply -var-file=v2.tfvars → plan no-op → destroy` |

`e2e/provider_smoke_test.go` walks the three trees with `runtime.Caller` and registers each subdirectory as its own `t.Run` sub-test. Adding a directory adds a test — no per-example test wiring. Each sub-test boots a fresh mockway on a kernel-assigned port so there's no cross-example state leakage.

The **idempotency gate** (`plan -detailed-exitcode 0`) is the strongest compatibility signal: if mockway returns a single field with the wrong case, type, or default, the provider sees drift on the second plan and the test fails. The `lb` (`ip_ids` array vs deprecated string) and `redis` (default port in Set + Update) fixes were both caught by this gate.

### 2. `examples/known_broken.yaml` — ratchet-only allowlist

Working examples whose idempotency gate is currently expected to fail are listed in `examples/known_broken.yaml`, each entry pointing at a tracking ticket in [`infrafactory/BACKLOG.md`](https://github.com/redscaresu/infrafactory/blob/main/BACKLOG.md). The smoke harness skips the drift assertion for allowlisted dirs but still runs `apply + destroy`.

The list is **ratchet-only-tighten**: if an allowlisted dir starts passing idempotency, the test fails with `"congratulations, remove this entry"`. Compatibility coverage can only grow, never silently regress.

### 3. Cross-repo e2e from infrafactory

[`infrafactory`](https://github.com/redscaresu/infrafactory) builds mockway from this source tree on a free port for every gated Scaleway e2e test (`TestE2E_Scaleway*` + `TestE2E_FullStackParis` in `internal/e2e/`, gated by `INFRAFACTORY_ENABLE_E2E=1`). Those tests drive scenarios end-to-end through infrafactory's harness (plan → mock-apply → topology derivation → destroy), so a compatibility regression surfaces in two places: the local `MOCKWAY_ENABLE_E2E=1 go test ./e2e/...` and the upstream infrafactory CI.

### Adding coverage for a new resource

1. Add an `examples/working/<svc>/` directory with `providers.tf` + `main.tf`.
2. Run `MOCKWAY_ENABLE_E2E=1 go test ./e2e/...` — auto-discovery picks it up.
3. If it drifts: either fix the handler, or (if the fix is non-trivial) add a `known_broken.yaml` entry pointing at a new BACKLOG ticket.
4. Mirror with `examples/misconfigured/<svc>/` (FK / validation paths) and `examples/updates/<svc>/` (update paths) as the service warrants.
5. Add a `TestE2E_Scaleway<Svc>` in infrafactory's `internal/e2e/scaleway_services_test.go` so the cross-repo gate covers the scenario flow too.
6. Append the service id to `LandedServices` in `handlers/regression_manifest.go`. This trips infrafactory's `TestCrossRepoParity_EveryLandedServiceHasScenario` (in its `internal/e2e/cross_repo_parity_test.go`) until either (a) a `scenarios/training/<svc>-paris.yaml` is added on the infrafactory side AND a `cloudParityMap["mockway"]["<svc>"]` entry pointing at it lands in the same PR, or (b) the service is added to that test's `exempt` map with a written reason (current exemptions: `ipam` — exercised transitively by instance/lb/vpc scenarios, no standalone resource type; `marketplace` — read-only image catalog exercised by every instance scenario). The parity test runs in infrafactory CI on every push, so landing here without the upstream change will break the badge — coordinate the two PRs.

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

- **No field validation.** Mockway does not validate required fields, `commercial_type`, `node_type`, or value constraints. This is deliberate — the Terraform provider SDK validates required fields before sending the API call, so these errors never reach the API in real usage. Mockway focuses on catching the bugs that `terraform validate` and `terraform plan` miss: FK references, dependency ordering, attachment constraints, and response shape correctness.
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

## Documentation

- [`AGENTS.md`](AGENTS.md) — fresh-agent entry point: layout, conventions, where to look for Scaleway wire shapes.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — PR contract, quality gates, pre-commit hook setup.
- [`CHANGELOG.md`](CHANGELOG.md) — Keep a Changelog format.
- [`infrafactory's auto-learning loop`](https://github.com/redscaresu/infrafactory/blob/main/docs/auto-learning-loop.md) — deep-dive on how infrafactory turns mockway failures (and its own LLM-generated HCL mistakes) into durable pitfalls. The S75 Scaleway phase-3 rule retirement (`scaleway_instance_private_nic` requirement, replaced by a `source: fix` learned pitfall) is the closest worked example for Scaleway-side learning; the doc itself uses an aws-subnet example for breadth.
