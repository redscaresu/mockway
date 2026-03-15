# Mockway — Stateful Scaleway API Mock

A stateful mock of the Scaleway cloud API. Think LocalStack, but for Scaleway. Single Go binary, SQLite state, path-based routing on a single port. Built for [InfraFactory](https://github.com/redscaresu/scaleway_infra_factory) and standalone Terraform/OpenTofu offline testing.

**Binary**: `mockway --port 8080`

**Flags**:
- `--port` — HTTP port (default: `8080`)
- `--db` — SQLite path (default: `:memory:` ephemeral). Use `--db ./mockway.db` for state that survives restarts.
- `--echo` — catch-all echo server; every request returns `{"ok":true}` and logs method + path. Use to discover which endpoints a Terraform config calls before writing handlers.

**`:memory:` is not true in-memory**: creates a real temp SQLite file via `os.CreateTemp`, deleted on clean exit. Use `--db` if you need persistence or want to inspect state with `sqlite3`.

**OpenTofu and Terraform**: both supported — identical HTTP calls to `SCW_API_URL`.

## Architecture

- Single HTTP server, path-based routing via **chi**
- SQLite with `PRAGMA foreign_keys = ON` for referential integrity
- Auth: `X-Auth-Token` header required on all Scaleway routes (any non-empty value accepted)
- Admin endpoints under `/mock/` (no auth)

**SQLite connection strategy**: `db.SetMaxOpenConns(1)` is mandatory. Without it, `:memory:` gives each connection its own isolated database — breaking state sharing and FK enforcement. `PRAGMA foreign_keys = ON` is also per-connection, so a single connection is required for it to apply globally.

**The Terraform provider SDK is the contract**: Mockway must return responses in the exact shape the provider expects. When fixing a bug, ask: "what does the provider send, and what does it expect to read back on the next GET?" Wrong shapes cause silent drift (phantom plan changes on second apply) or provider panics.

**Double-apply idempotency check**: run `terraform apply` twice with no config changes. First apply catches missing endpoints. Second apply must be a no-op — any planned changes indicate a GET response shape mismatch (drift).

## Project Structure

```
mockway/
├── cmd/mockway/main.go           # Entrypoint — flag parsing, DI wiring, server start
├── handlers/
│   ├── handlers.go               # Application struct, NewApplication(), RegisterRoutes()
│   ├── instance.go / vpc.go / lb.go / k8s.go / rdb.go / redis.go
│   ├── registry.go / iam.go / ipam.go / domain.go / marketplace.go
│   ├── admin.go                  # /mock/reset, /mock/snapshot, /mock/restore, /mock/state
│   ├── unimplemented.go          # 501 catch-all
│   └── handlers_test.go          # Integration tests (HTTP round-trips)
├── models/models.go              # ErrNotFound, ErrConflict
├── repository/repository.go      # SQLite state management, schema, CRUD
├── testutil/testutil.go          # NewTestServer, DoCreate, DoGet, DoList, DoDelete
├── specs/                        # Downloaded Scaleway OpenAPI YAML specs
└── examples/
    ├── working/                  # Configs that apply+destroy cleanly
    └── misconfigured/            # Configs that fail deliberately (FK violations)
```

**Key pattern**: DI via `Application` struct. Handlers are thin — delegate to repository. Repository returns domain errors (`ErrNotFound`, `ErrConflict`); handlers map to HTTP status codes.

## Services in Scope

| Service | Path Prefix | Notes |
|---------|-------------|-------|
| Instance | `/instance/v1/zones/{zone}/` | servers, ips, security_groups, private_nics; server actions; products/servers catalog; user_data stubs; volume GET/DELETE |
| VPC | `/vpc/v1/` and `/vpc/v2/` | vpcs, private-networks (v1+v2 same handlers) |
| Load Balancer | `/lb/v1/zones/{zone}/` | lbs, frontends, backends, lb private-networks (attach/detach) |
| Kubernetes | `/k8s/v1/regions/{region}/` | clusters, pools; versions list; kubeconfig GET; nodes list |
| RDB | `/rdb/v1/regions/{region}/` | instances, databases, users; upgrade, certificate, ACLs, privileges, settings |
| Redis | `/redis/v1/zones/{zone}/` | clusters; certificate GET |
| Registry | `/registry/v1/regions/{region}/` | namespaces (PATCH supported) |
| IAM | `/iam/v1alpha1/` | applications, api-keys, policies, ssh-keys, users, groups, group-members |
| IPAM | `/ipam/v1/regions/{region}/` | ips (list-only stub) |
| Domain | `/domain/v2beta1/` | dns-zones, records (stubs) |
| Block | `/block/v1alpha1/zones/{zone}/` | volumes (full CRUD); snapshots |
| Marketplace | `/marketplace/v2/` | local-images (image label → UUID resolution) |
| Account (legacy) | `/account/v2alpha1/` | ssh-keys alias → IAM ssh-keys table |

**Naming**: Scaleway uses hyphens in URL paths (`/private-networks/`) but underscores in JSON keys (`"private_network_id"`).

**IAM**: organisation-scoped — no `{zone}` or `{region}` in path. Account routes are a pure alias to the same `iam_ssh_keys` table.

## SQLite Schema

Per-type tables with a JSON `data` blob for full resource data, plus extracted FK columns for integrity. See `repository/repository.go` for the full schema. Notable FK relationships:

- `private_networks.vpc_id → vpcs(id)`
- `instance_servers.security_group_id → instance_security_groups(id) ON DELETE SET NULL`
- `instance_ips.server_id → instance_servers(id) ON DELETE SET NULL`
- `instance_private_nics.server_id → instance_servers(id) ON DELETE CASCADE`
- `instance_private_nics.private_network_id → private_networks(id)`
- `k8s_pools.cluster_id → k8s_clusters(id) ON DELETE CASCADE`
- `rdb_databases.instance_id → rdb_instances(id) ON DELETE CASCADE`
- `rdb_users.instance_id → rdb_instances(id) ON DELETE CASCADE`
- `iam_api_keys.application_id → iam_applications(id)`
- `iam_policies.application_id → iam_applications(id)`
- `iam_group_members.user_id → iam_users(id)`

**Migration system**: `migrate()` runs versioned DDL via create-new/copy/drop/rename to handle existing file DBs (`--db`). `CREATE TABLE IF NOT EXISTS` is a no-op on existing tables — schema changes require migrations.

## Server-Generated Fields on Create

| Resource | Fields Injected |
|----------|-----------------|
| All UUID-based | `"id": "<uuid>"` |
| Instance servers | `"state": "stopped"`, `"creation_date"`, `"modification_date"` (RFC3339) |
| Instance IPs | `"address": "51.15.<random>.x"` |
| Load balancers | `"status": "ready"`, `"ip": [{"id":"<uuid>","ip_address":"51.15.x.x","lb_id":"<id>"}]` |
| K8s clusters/pools | `"status": "ready"`, `"created_at"`, `"updated_at"` |
| RDB instances | `"status": "ready"`, `"endpoints": [{"id":"<uuid>","load_balancer":{},"private_network":null,"ip":"51.15.x.x","port":5432}]` |
| IPAM IPs | `"address": "10.x.x.x/32"` (CIDR — provider uses `expandIPNet()`) |
| Block volumes | `"status": "available"`, `"specs": {"perf_iops": 5000}` (15000 for `sbs_15k`) |
| IAM API keys | `"access_key": "SCW<17 random chars>"`, `"secret_key": "<uuid>"` (stripped on Get/List) |
| K8s pools | `"autohealing": false`, `"autoscaling": false` |

**Server state lifecycle**: servers start as `"stopped"`. The provider sends a `poweron` action after create → `SetServerState("running")`. Handle all actions in `ServerAction`: `poweron`/`reboot` → `"running"`, `poweroff`/`stop_in_place` → `"stopped"`/`"stopped_in_place"`, `terminate` → delete.

**RDB endpoint shape**: the provider classifies endpoint type by checking `endpoint.LoadBalancer != nil`. Always return `"load_balancer": {}` (non-nil) and `"private_network": null` for public endpoints. Include `"id"` on every endpoint object.

**LB backend `on_marked_down_action`**: store and return the API wire format `"on_marked_down_action_none"`. The provider's expand/flatten converts schema `"none"` ↔ wire format around API calls.

**RDB/Redis certificates**: the SDK's `File.Content` is `[]byte` — JSON requires base64-encoded data. Return `{"content": base64.StdEncoding.EncodeToString([]byte(pem))}`. Do NOT return raw PEM.

## Referential Integrity

- **On create**: FK violations → **404 Not Found** (`"referenced resource not found"`)
- **On delete**: dependents exist → **409 Conflict** (`"cannot delete: dependents exist"`)
- **Duplicate composite key**: RDB databases/users with same `(instance_id, name)` → **409 Conflict**
- **`POST /mock/reset`**: disable FK checks, delete all rows, re-enable

SQLite enforces most FKs natively. For FKs inside the JSON blob (e.g. RDB `init_endpoints[].private_network.id`), validate programmatically in the handler before inserting.

**IAM FK rules**:
- API key: `application_id` or `user_id` required. `application_id` must reference existing application → 404.
- Policy: `application_id` optional, must reference existing application if provided → 404.
- Delete application: reject if API keys or policies still reference it → 409.
- Group members: `user_id` must reference existing user → 404.

**Error helper selection**:
- `writeCreateError(w, err)` — Create handlers only. Maps `ErrNotFound` → 404 `"referenced resource not found"`.
- `writeDomainError(w, err)` — Get/List/Delete handlers. Maps `ErrNotFound` → 404 `"resource not found"`.

Wrong choice silently breaks provider error handling — the provider uses the message text.

## API Response Format

| Operation | Status | Body |
|-----------|--------|------|
| Create | 200 | Resource JSON |
| Get | 200 | Resource JSON |
| List | 200 | `{"<plural_key>": [...], "total_count": N}` |
| Delete (most) | 204 | Empty |
| Delete (RDB instance, Redis cluster, Registry namespace) | 200 | Deleted resource JSON |
| `POST /mock/reset` | 204 | Empty |
| `GET /mock/state` | 200 | Full state JSON |

| Condition | Status | Body |
|-----------|--------|------|
| Missing `X-Auth-Token` | 401 | `{"message": "missing or empty X-Auth-Token", "type": "denied_authentication"}` |
| Resource not found | 404 | `{"message": "resource not found", "type": "not_found"}` |
| Referenced resource not found (create) | 404 | `{"message": "referenced resource not found", "type": "not_found"}` |
| Dependents exist (delete) | 409 | `{"message": "cannot delete: dependents exist", "type": "conflict"}` |

### Response Wrapping

**Instance API only**: wraps single-object Create/Get in a key (e.g. `{"server": {...}}`).
**All other APIs**: flat object at top level (e.g. `{"id": "...", "name": "..."}`).
**List**: always `{"<plural_key>": [...], "total_count": N}` across all APIs.

| Resource | Singular key | Plural key |
|----------|-------------|-----------|
| Instance Server | `"server"` | `"servers"` |
| Instance IP | `"ip"` | `"ips"` |
| Instance Security Group | `"security_group"` | `"security_groups"` |
| Instance Private NIC | `"private_nic"` | `"private_nics"` |
| VPC | _(flat)_ | `"vpcs"` |
| Private Network | _(flat)_ | `"private_networks"` |
| Load Balancer | _(flat)_ | `"lbs"` |
| LB Frontend | _(flat)_ | `"frontends"` |
| LB Backend | _(flat)_ | `"backends"` |
| LB Private Network | _(flat)_ | `"private_network"` **(singular — Scaleway quirk)** |
| K8s Cluster | _(flat)_ | `"clusters"` |
| K8s Pool | _(flat)_ | `"pools"` |
| RDB Instance | _(flat)_ | `"instances"` |
| RDB Database | _(flat)_ | `"databases"` |
| RDB User | _(flat)_ | `"users"` |
| Redis Cluster | _(flat)_ | `"clusters"` |
| Registry Namespace | _(flat)_ | `"namespaces"` |
| IAM Application | _(flat)_ | `"applications"` |
| IAM API Key | _(flat)_ | `"api_keys"` |
| IAM Policy | _(flat)_ | `"policies"` |
| IAM SSH Key | _(flat)_ | `"ssh_keys"` |

**Checklist when adding an endpoint**:
1. Instance API? Wrap Create/Get in singular key; use `writeList` with plural key for List.
2. Any other API? Return flat. Use `writeList` with plural key for List.
3. Security group GET: call `splitSecurityGroupRules` — provider expects `"inbound_rule"` and `"outbound_rule"` arrays, not a flat `"rules"` array.
4. Pagination: always return all results (no paging needed at mock scale).

## Admin State API

- `POST /mock/reset` — wipe all state
- `POST /mock/snapshot` — save state to `<db_path>.snapshot`
- `POST /mock/restore` — restore from snapshot
- `GET /mock/state` — full resource graph (versioned contract — InfraFactory depends on this shape)
- `GET /mock/state/{service}` — single service (`instance`, `vpc`, `lb`, `k8s`, `rdb`, `iam`; unknown → 404)

## Key Resource Relationships

```
VPC → Private Network → Instance Private NIC → Instance Server
                      → RDB Instance (private endpoint)
                      → LB Private Network attachment

Instance Server → Instance IP, Private NIC, Security Group

Load Balancer → LB IP (inline in LB JSON), Frontend, Backend, Private Network attachment

K8s Cluster → Node Pool, Private Network (optional)

IAM Application → API Key, Policy
IAM Group → Group Members → IAM Users
```

## Testing

**Tools**: stdlib `testing` + `net/http/httptest` + `testify`. No other test deps.

**Test helpers** (`testutil/testutil.go`):
```go
NewTestServer(t)                                          // httptest.Server + in-memory SQLite
DoCreate(t, ts, path, body) (int, map[string]any)
DoGet(t, ts, path) (int, map[string]any)
DoList(t, ts, path) (int, map[string]any)
DoDelete(t, ts, path) int
ResetState / SnapshotState / RestoreState / GetState
```

**Repository tests** (`repository/repository_test.go`): test CRUD, FK enforcement on create and delete, JSON round-trips, reset, name-based keys (RDB databases/users).

**Handler tests** (`handlers/handlers_test.go`): full HTTP round-trips — lifecycle (Create→Get→List→Delete→404), FK rejection (404 on bad FK, 409 on parent with children), admin reset and state structure, cross-service flows.

**What NOT to test**: handler isolation, mocked repository interfaces, Scaleway field validation (not Mockway's job).

**E2E tests** (`e2e/provider_smoke_test.go`, build tag `provider_e2e`): spin up real mockway, write HCL inline, run `terraform apply` via `os/exec`. The key assertion is the **double-apply no-op check** — second apply must exit 0 with `-detailed-exitcode`. Exit 2 = drift bug in a GET handler.

```bash
go test ./...                          # full unit + integration suite
go test -tags provider_e2e ./e2e -v   # provider integration (needs terraform/tofu in PATH)
```

**Common drift causes**:
- Missing `status` field on GET — provider polls for `"ready"`/`"running"` on refresh
- Nested object vs flat returned (or vice versa)
- Array field returned as `null` instead of `[]`
- Field present on create response but absent on GET

## Safe Workflow

1. Add/adjust repository logic (`repository/repository.go`)
2. Wire handlers and error mapping (`handlers/*.go`)
3. Add tests (`handlers/handlers_test.go`)
4. `go test ./...`
5. Verify `/mock/state` shape unchanged

## Discovering Gaps

```bash
# Use --echo to see every endpoint a Terraform config calls
mockway --echo --port 8080 &
export SCW_API_URL=http://localhost:8080 SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX \
  SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000 \
  SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
terraform apply   # grep mockway logs for every path called

# Cross-reference routes against downloaded OpenAPI specs
python3 scripts/spec_diff.py          # high-priority gaps only
python3 scripts/spec_diff.py --all    # every unimplemented spec operation
```

**Fix workflow**: identify gap → check `specs/` for shape → add repo method → register route + handler → add test → re-run `spec_diff.py`.

## Misconfigured Examples

Every example in `examples/misconfigured/` must be **verified by running it** — not just by code reasoning. The failure must come from mockway's FK enforcement (404 or 409), not from provider-side validation before the API call.

**Realistic patterns**: prefer `.name` used where `.id` is expected (provider doesn't always validate UUID format), stale UUIDs from destroyed workspaces (valid format, non-existent resource), or `terraform_remote_state` references pointing at torn-down infrastructure. Avoid bare hardcoded UUIDs with no plausible story.

**Ordering examples** (`vpc_deleted_before_private_network`, `private_network_deleted_before_nic`): use `terraform state rm <child>` before `terraform destroy -target <parent>`. This simulates the child being managed in a separate workspace — Terraform then sends a bare DELETE to mockway without first removing the dependent, triggering the 409.

**Quick harness**:
```bash
go build -o /tmp/mockway ./cmd/mockway
/tmp/mockway --port 18080 &

export SCW_API_URL=http://localhost:18080 SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX \
  SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000 \
  SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000 \
  SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000 \
  SCW_DEFAULT_REGION=fr-par SCW_DEFAULT_ZONE=fr-par-1

cd examples/misconfigured/<name>
terraform init && terraform apply -auto-approve
# Expected: error from mockway (404 or 409)

# For cross_state_orphan and ordering examples, see comments in platform/main.tf
```

## Known Limitations

- **IAM rules stub**: `ListIAMRules` always returns empty — Mockway doesn't model individual rules.
- **IPAM/Domain**: list-only stubs, not exercised by full provider e2e.
- **Block**: delegates volume GET/DELETE to instance volume handlers.
- **No auth validation**: any non-empty `X-Auth-Token` accepted.
- **No S3/Object Storage**.

## Distribution

- Docker: `ghcr.io/redscaresu/mockway:latest`
- Homebrew: `brew install redscaresu/tap/mockway`
- Binary: `go install github.com/redscaresu/mockway/cmd/mockway@latest`
