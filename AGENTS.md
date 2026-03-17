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
│   ├── block.go / registry.go / iam.go / ipam.go / domain.go / marketplace.go
│   ├── admin.go                  # /mock/reset, /mock/snapshot, /mock/restore, /mock/state
│   ├── unimplemented.go          # 501 catch-all
│   └── handlers_test.go          # Integration tests (HTTP round-trips)
├── models/models.go              # ErrNotFound, ErrConflict
├── repository/repository.go      # SQLite state management, schema, CRUD
├── testutil/testutil.go          # NewTestServer, DoCreate, DoGet, DoList, DoDelete
├── specs/                        # Downloaded Scaleway OpenAPI YAML specs
└── examples/
    ├── working/                  # Configs verified idempotent (apply→plan→destroy)
    ├── updates/                  # Update scenarios (apply v1→plan→apply v2→plan→destroy)
    └── misconfigured/            # Configs that fail deliberately (FK violations)
```

**Key pattern**: DI via `Application` struct. Handlers are thin — delegate to repository. Repository returns domain errors (`ErrNotFound`, `ErrConflict`); handlers map to HTTP status codes.

## Services in Scope

| Service | Path Prefix | Notes |
|---------|-------------|-------|
| Instance | `/instance/v1/zones/{zone}/` | servers, ips, security_groups, private_nics; server actions; products/servers catalog; user_data stubs; standalone volumes (`instance_volumes` table) |
| VPC | `/vpc/v1/` and `/vpc/v2/` | vpcs, private-networks, routes (v1+v2 same handlers for vpcs/pns) |
| VPC GW | `/vpc-gw/v2/zones/{zone}/` | public gateways, gateway-networks |
| Load Balancer | `/lb/v1/zones/{zone}/` | lbs, frontends, backends, ACLs, routes, lb private-networks (attach/detach) |
| Kubernetes | `/k8s/v1/regions/{region}/` | clusters, pools; versions list + GET by name; kubeconfig GET; nodes list; cluster/pool upgrade; set-type |
| RDB | `/rdb/v1/regions/{region}/` | instances, databases, users, read-replicas; upgrade, certificate, ACLs (stateful), privileges, settings |
| Redis | `/redis/v1/zones/{zone}/` | clusters; certificate GET |
| Registry | `/registry/v1/regions/{region}/` | namespaces (PATCH supported) |
| IAM | `/iam/v1alpha1/` | applications, api-keys, policies, ssh-keys, users, groups, group-members |
| IPAM | `/ipam/v1/regions/{region}/` | ips (list-only stub) |
| Domain | `/domain/v2beta1/` | dns-zones (full CRUD), records (patch + list) |
| Block | `/block/v1alpha1/zones/{zone}/` | volumes (full CRUD); snapshots (full CRUD) |
| Marketplace | `/marketplace/v2/` | local-images (image label → UUID resolution; dynamic labels persisted in `marketplace_labels` table) |
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
- `block_snapshots.volume_id → block_volumes(id)`
- `iam_api_keys.application_id → iam_applications(id)`
- `iam_policies.application_id → iam_applications(id)`
- `iam_group_members.user_id → iam_users(id)`
- `vpc_routes.vpc_id → vpcs(id)`
- `vpc_gateway_networks.gateway_id → vpc_public_gateways(id)`
- `vpc_gateway_networks.private_network_id → private_networks(id)`
- `rdb_acls.instance_id → rdb_instances(id) ON DELETE CASCADE`

**Stateless tables** (no FK):
- `dns_zones` — keyed by `dns_zone` (e.g. `app.example.com`), indexed by `domain`
- `marketplace_labels` — persists dynamic image labels for GET resolution across restarts

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
| Block snapshots | `"parent_volume": {"id": "<uuid>"}`, `"status": "available"` — provider sends flat `volume_id` on create but reads `snapshot.ParentVolume.ID` on GET; store as nested object |
| Instance volumes (standalone) | `"state": "available"`, `"creation_date"`, `"modification_date"`, `"server": null`, `"volume_type": "l_ssd"`, `"tags": []` |
| IAM API keys | `"access_key": "SCW<17 random chars>"`, `"secret_key": "<uuid>"` (stripped on Get/List) |
| K8s pools | `"autohealing": false`, `"autoscaling": false` |
| VPC routes | `"type": "custom"`, `"is_read_only": false`, `"tags": []` |
| VPC public gateways | `"status": "running"`, `"bastion_enabled": false`, `"enable_smtp": false`, `"tags": []` |
| VPC gateway networks | `"status": "ready"`, `"enable_masquerade": false` |
| DNS zones | `"status": "active"`, `"ns"`, `"ns_default"`, `"ns_master": []` |
| LB certificates | `"status": "ready"`, `"fingerprint"`, `"not_valid_before"`, `"not_valid_after"`, `"subject_alternative_name": []`, embedded `"lb"` object |
| LB (with `ip_id`) | Uses existing LB IP data for inline `"ip"` array instead of generating new; returns 404 if `ip_id` not found |

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
| VPC Route | _(flat)_ | `"routes"` |
| VPC Public Gateway | _(flat)_ | `"gateways"` |
| VPC Gateway Network | _(flat)_ | `"gateway_networks"` |
| LB Certificate | _(flat)_ | `"certificates"` |
| DNS Zone | _(flat)_ | `"dns_zones"` |

**Checklist when adding an endpoint**:
1. Instance API? Wrap Create/Get in singular key; use `writeList` with plural key for List.
2. Any other API? Return flat. Use `writeList` with plural key for List.
3. Security group GET: call `splitSecurityGroupRules` — provider expects `"inbound_rule"` and `"outbound_rule"` arrays, not a flat `"rules"` array.
4. Pagination: always return all results (no paging needed at mock scale).
5. **Check for create-vs-read field name divergence**: does the provider send `foo_id` on create but read back a nested `foo.id` on GET? If so, the repository must translate (e.g. block snapshot `volume_id` → `parent_volume.id`, RDB `disable_backup` → `backup_schedule.disabled`). Check the provider's `Read` function to confirm field names.

## Admin State API

- `POST /mock/reset` — wipe all state
- `POST /mock/snapshot` — save state to `<db_path>.snapshot`
- `POST /mock/restore` — restore from snapshot
- `GET /mock/state` — full resource graph (versioned contract — InfraFactory depends on this shape)
- `GET /mock/state/{service}` — single service (`instance`, `vpc`, `lb`, `k8s`, `rdb`, `iam`, `redis`, `registry`, `block`, `ipam`, `domain`; unknown → 404)

## Key Resource Relationships

```
VPC → Private Network → Instance Private NIC → Instance Server
                      → RDB Instance (private endpoint)
                      → LB Private Network attachment
                      → VPC Gateway Network
VPC → VPC Route

VPC Public Gateway → VPC Gateway Network → Private Network

Instance Server → Instance IP, Private NIC, Security Group

Load Balancer → LB IP (inline in LB JSON or linked via ip_id), Frontend, Backend,
                Private Network attachment, Certificate

K8s Cluster → Node Pool, Private Network (optional)

Block Volume → Block Snapshot

RDB Instance → Database, User, Read Replica, ACL rules

DNS Zone → Domain Records (cascade delete)

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
DoPatch(t, ts, path, body) (int, map[string]any)
DoPut(t, ts, path, body) (int, map[string]any)
DoDelete(t, ts, path) int
ResetState / SnapshotState / RestoreState / GetState
```

**Repository tests** (`repository/repository_test.go`): test CRUD, FK enforcement on create and delete, JSON round-trips, reset, name-based keys (RDB databases/users).

**Handler tests** (`handlers/handlers_test.go`): full HTTP round-trips — lifecycle (Create→Get→List→Delete→404), FK rejection (404 on bad FK, 409 on parent with children), admin reset and state structure, cross-service flows.

**What NOT to test**: handler isolation, mocked repository interfaces, Scaleway field validation (not Mockway's job).

**E2E tests** (`e2e/provider_smoke_test.go`, build tag `provider_e2e`):
- `TestExamplesWorkingIdempotency` auto-discovers every directory under `examples/working/` and runs apply → plan-no-op → destroy. **Adding a new `examples/working/` directory automatically adds it to the idempotency gate — no separate test to write.**
- `TestExamplesUpdatesIdempotency` auto-discovers every directory under `examples/updates/` and runs apply v1 → plan-no-op → apply v2 → plan-no-op → destroy. Each update example must have `main.tf`, `v1.tfvars`, and `v2.tfvars`. **Adding a new `examples/updates/` directory automatically adds it to the update-idempotency gate.**

```bash
go test ./...                                    # unit + integration
go test -tags provider_e2e ./e2e -v             # all e2e (needs terraform/tofu in PATH)
go test -tags provider_e2e ./e2e -run TestExamplesWorkingIdempotency -v   # create idempotency
go test -tags provider_e2e ./e2e -run TestExamplesUpdatesIdempotency -v   # update idempotency
```

**Common drift causes**:
- Missing `status` field on GET — provider polls for `"ready"`/`"running"` on refresh
- Create-vs-read field name divergence — provider sends flat `foo_id` on create but reads nested `foo.id` on GET; repository must translate (e.g. block snapshot `volume_id` → `parent_volume.id`, RDB `disable_backup` → `backup_schedule.disabled`)
- Create-vs-update field name divergence — provider uses a *different* field name in PATCH than in POST; e.g. RDB sends `disable_backup` on POST but `is_backup_schedule_disabled` on PATCH. **Detection**: capture the actual PATCH body (see Proxy-Capture below) — do not assume the PATCH field names match the POST field names.
- Array field returned as `null` instead of `[]`
- Field present on create response but absent on GET
- Security group rules: provider v2.70+ filters `Editable == false`; `SetSecurityGroupRules` injects `"editable": true` and UUID `"id"` on every rule
- Update null-overwrite — provider sends `null` for optional fields and nested objects it doesn't intend to clear; naive merge overwrites stored non-null values. Use `patchMerge` (see Update Handler Pattern below).

## Safe Workflow

1. Add/adjust repository logic (`repository/repository.go`)
2. Wire handlers and error mapping (`handlers/*.go`)
3. Add a handler test (`handlers/handlers_test.go`)
4. Add a working example in `examples/working/` — automatically create-idempotency-tested
5. Add an update example in `examples/updates/` — automatically update-idempotency-tested
6. `go test ./...`
7. `go test -tags provider_e2e ./e2e -v` (or `./scripts/test-examples.sh` + `./scripts/test-updates.sh`)
8. Verify `/mock/state` shape unchanged if adding a new resource type

**Idempotency is the hardest correctness property**: a passing `apply` does not mean the handler is correct. Run the no-op plan check. If it exits 2 (drift), the GET response shape does not round-trip through the provider. See "Common drift causes" above.

## Common Bug Patterns (from code review)

These recurring patterns have been found across multiple review cycles. Check for them when adding or modifying handlers:

**1. Wrong error helper on create paths**: All Create/Attach/Set handlers must use `writeCreateError(w, err)`, not `writeDomainError(w, err)`. The difference: `writeCreateError` returns `"referenced resource not found"` on 404 (FK violation on create), while `writeDomainError` returns `"resource not found"` (the resource itself is missing). Wrong choice breaks provider error handling.

**2. SQL column desync on update**: Every `Update*` function that writes to a table with extracted SQL columns (FK refs, indexed fields like `region`, `vpc_id`, `cluster_id`) must sync those columns explicitly. Using `updateJSONByID` alone only writes the JSON blob — the SQL columns stay stale. This breaks `listJSON` filtering, FK enforcement, and cascade behavior. See the SQL column sync rule below.

**3. Payload field name variations**: The Scaleway provider changes field names between versions (e.g. `user_ids` → `user_id`, `enable` → `enabled`). Handlers should accept both forms to avoid silently dropping data. A common symptom: a handler reads `body["user_ids"]` but the provider sends `body["user_id"]`, causing the handler to treat the request as empty and delete all existing data.

**4. Truncating multi-item lists**: When processing arrays from request bodies (e.g. `init_endpoints`, `rules`), process all items — not just `list[0]`. Truncating drops user-provided state and skips FK validation for later entries.

**5. Response encoding mismatches**: Fields typed as `[]byte` in the Scaleway Go SDK (like certificate `content`) must be base64-encoded in JSON responses. Raw PEM/binary strings cause decode failures. Check: `GetRDBCertificate`, `GetRedisCertificate`, and any new certificate endpoint.

**6. Reset must include all tables**: When adding a new table to `init()`, also add it to `Reset()`. Missing tables leak state across `/mock/reset` calls and cause nondeterministic test behavior.

## Update Handler Pattern

All `Update*` repository functions use the `patchMerge` helper for null-safe deep-merge:

```go
func patchMerge(current, patch map[string]any, skip ...string) map[string]any
```

**Semantics**:
- Keys listed in `skip` are never overwritten (typically `"id"`).
- Top-level `null` values in `patch` are ignored — the Scaleway SDK sends `null` for optional fields it doesn't intend to clear.
- Nested `map[string]any` values are deep-merged one level: null sub-fields in the patch are skipped so they don't wipe stored sub-field values.

**Usage pattern** in every Update function:
```go
next := patchMerge(current, patch, "id")
next["updated_at"] = nowRFC3339()
// ... any per-resource post-merge normalisation ...
```

**Per-resource normalisations after merge**: some resources need field translations or SQL column sync applied after the `patchMerge` call:
- `UpdateCluster`: normalize `auto_upgrade.enable` → `auto_upgrade.enabled` (provider uses `enable` in PATCH, provider reads `enabled` on GET)
- `UpdateRDBInstance`: translate both `disable_backup` and `is_backup_schedule_disabled` → `backup_schedule.disabled`
- `UpdateServer`: reconcile `security_group` / `security_group_id` FK consistency + SQL column sync
- `UpdateIP`: normalize `server` → `server_id` field name + SQL column sync
- `UpdateBlockVolume`: recompute `specs.perf_iops` when volume type changes
- `UpdateVPC`: sync `region` SQL column
- `UpdatePrivateNetwork`: sync `vpc_id` and `region` SQL columns
- `UpdateVPCRoute`: sync `vpc_id` and `region` SQL columns
- `UpdateVPCGatewayNetwork`: sync `gateway_id` and `private_network_id` SQL columns
- `UpdateIAMAPIKey`: sync `application_id` SQL column
- `UpdateIAMPolicy`: sync `application_id` SQL column

**SQL column sync rule**: if a table has indexed/FK SQL columns extracted from the JSON blob (e.g. `vpc_id`, `region`, `gateway_id`), the Update function must write those columns explicitly with a custom `UPDATE ... SET data = ?, col = ?` query instead of using `updateJSONByID`. Failing to sync causes stale list/filter results and bypassed FK validation.

**When adding a new Update handler**: use `patchMerge` + post-merge normalisations. Never use the naive `for k, v := range patch { next[k] = v }` loop.

## Proxy-Capture Workflow

Use this to discover exact request bodies before implementing a handler, or to debug drift on an existing one. Run tofu through a logging proxy against a real mockway instance:

```python
# /tmp/log_proxy.py — logs PATCH/POST/PUT bodies, proxies all requests to TARGET
import http.server, urllib.request, sys, json
TARGET = f"http://127.0.0.1:{sys.argv[1]}"
class Proxy(http.server.BaseHTTPRequestHandler):
    def do_request(self):
        body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        if self.command in ('PATCH', 'POST', 'PUT') and body:
            print(f">>> {self.command} {self.path}")
            try: print(json.dumps(json.loads(body), indent=2))
            except: print(body.decode())
            sys.stdout.flush()
        req = urllib.request.Request(TARGET + self.path, data=body or None, method=self.command,
            headers={k:v for k,v in self.headers.items()})
        try:
            resp = urllib.request.urlopen(req)
            data = resp.read()
            self.send_response(resp.status)
            for k,v in resp.headers.items():
                if k.lower() not in ('transfer-encoding',): self.send_header(k, v)
            self.end_headers(); self.wfile.write(data)
        except urllib.error.HTTPError as e:
            data = e.read(); self.send_response(e.code); self.end_headers(); self.wfile.write(data)
    do_GET = do_POST = do_PATCH = do_DELETE = do_PUT = do_request
    def log_message(self, *a): pass
http.server.HTTPServer(('127.0.0.1', int(sys.argv[2])), Proxy).serve_forever()
```

```bash
/tmp/mockway-test -port 9000 &
python3 /tmp/log_proxy.py 9000 9001 &>/tmp/proxy.log &
export SCW_API_URL=http://127.0.0.1:9001
# run tofu apply, then inspect /tmp/proxy.log for exact PATCH bodies
```

**Key use**: before implementing an Update handler, capture the real PATCH body to check for field name divergences (e.g. `disable_backup` vs `is_backup_schedule_disabled`). Do not assume PATCH field names match POST field names.

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

**Fix workflow**: identify gap → check `specs/` for shape → add repo method → register route + handler → add test → add example → run idempotency suite.

## Misconfigured Examples

Every example in `examples/misconfigured/` must be **verified by running it** — not just by code reasoning. The failure must come from mockway's FK enforcement (404 or 409), not from provider-side validation before the API call.

**Realistic patterns**: prefer `.name` used where `.id` is expected (provider doesn't always validate UUID format), stale UUIDs from destroyed workspaces (valid format, non-existent resource), or `terraform_remote_state` references pointing at torn-down infrastructure. Avoid bare hardcoded UUIDs with no plausible story.

**Ordering examples** (`vpc_deleted_before_private_network`, `private_network_deleted_before_nic`): use `terraform state rm <child>` before `terraform destroy -target <parent>`. This simulates the child being managed in a separate workspace — Terraform then sends a bare DELETE to mockway without first removing the dependent, triggering the 409.

**Quick harness**:
```bash
go build -o /tmp/mockway ./cmd/mockway && /tmp/mockway --port 18080 &

export SCW_API_URL=http://localhost:18080 SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX \
  SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000 \
  SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000 \
  SCW_DEFAULT_ORGANIZATION_ID=00000000-0000-0000-0000-000000000000 \
  SCW_DEFAULT_REGION=fr-par SCW_DEFAULT_ZONE=fr-par-1

cd examples/misconfigured/<name>
terraform init && terraform apply -auto-approve
# Expected: error from mockway (404 or 409)
```

## Known Limitations

- **VPC gateway network `enable_masquerade` drift** (MW-28): `scaleway_vpc_gateway_network` with `enable_masquerade = true` causes perpetual plan diff. Needs proxy-capture investigation against the real Scaleway API.
- **IAM user/group**: not yet e2e verified — provider v2.70 changed field names (`username` required, `user_ids` → `user_id`).
- **IPAM**: list-only stub, not exercised by full provider e2e.
- **Block snapshots**: `parent_volume` is stored as `{"id": "<uuid>"}` only; name/type fields are absent. Sufficient for provider idempotency but not full API fidelity.
- **No auth validation**: any non-empty `X-Auth-Token` accepted.
- **No S3/Object Storage**.

## Distribution

- Docker: `ghcr.io/redscaresu/mockway:latest`
- Homebrew: `brew install redscaresu/tap/mockway`
- Binary: `go install github.com/redscaresu/mockway/cmd/mockway@latest`
