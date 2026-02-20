# Mockway — Stateful Scaleway API Mock

## What This Is

A stateful mock of the Scaleway cloud API. Think LocalStack, but for Scaleway. Single Go binary, SQLite state, path-based routing on a single port. Built to be used by [InfraFactory](https://github.com/redscaresu/scaleway_infra_factory) but also useful standalone for anyone testing Scaleway OpenTofu/Terraform code offline.

**Binary**: `mockway --port 8080`

**Flags**:
- `--port` — HTTP port (default: `8080`)
- `--db` — SQLite database path (default: `:memory:` — ephemeral, like LocalStack). Use `--db ./mockway.db` for file-based debugging with `sqlite3` CLI.

**OpenTofu and Terraform**: Both are fully supported. Mockway is an HTTP API mock — it doesn't know or care which client is calling it. Both OpenTofu and Terraform use the same `scaleway/scaleway` provider, which makes identical HTTP calls to `SCW_API_URL`. Point either tool at Mockway and it works the same way.

**SQLite connection strategy**: `database/sql` pools multiple connections by default. With `:memory:`, each connection gets its own isolated database — breaking state sharing and FK enforcement. Fix: call `db.SetMaxOpenConns(1)` to force a single connection. This also ensures `PRAGMA foreign_keys = ON` (which is per-connection) stays active for all queries. For file-based `--db`, a single connection is also acceptable at Mockway's expected concurrency.

## Architecture

- Single HTTP server on one port (default 8080)
- Path-based routing matching Scaleway's real API structure
- SQLite database for stateful, queryable state (ephemeral by default, file-backed with `--db`)
- Auth: `X-Auth-Token` header is **required** on all Scaleway API routes (not admin `/mock/*` routes). Accept any non-empty value — no real validation. Return `401 Unauthorized` with `{"message": "missing or empty X-Auth-Token", "type": "denied_authentication"}` if the header is missing or empty.
- Admin endpoints under `/mock/` for state inspection and reset

## Services in Scope (v1)

6 services + 1 legacy alias, 19 resource types + 1 catalog endpoint. Most have 4 operations (Create, Get, Delete, List); security groups also have Patch (update), PUT rules (bulk set), and GET rules (list); IAM has an additional rules list endpoint; Instance has a products/servers catalog endpoint. ~85 handler methods + 3 admin endpoints + 1 catch-all (UnimplementedHandler). No S3 in v1.

| Service | Path Prefix | Resource Types |
|---------|-------------|----------------|
| Instance | `/instance/v1/zones/{zone}/` | servers, ips, security_groups, private_nics |
| VPC | `/vpc/v1/regions/{region}/` | vpcs, private-networks |
| Load Balancer | `/lb/v1/zones/{zone}/` | lbs, frontends, backends, private-networks |
| Kubernetes | `/k8s/v1/regions/{region}/` | clusters, pools |
| RDB | `/rdb/v1/regions/{region}/` | instances, databases, users |
| IAM | `/iam/v1alpha1/` | applications, api-keys, policies, ssh-keys |
| Account (legacy) | `/account/v2alpha1/` | ssh-keys (alias → IAM ssh-keys state) |

**Naming convention**: Scaleway uses **hyphens in URL paths** (`/private-networks/`, `/api-keys/`, `/ssh-keys/`) but **underscores in JSON keys** (`"private_network_id"`). This is Scaleway's actual API style — follow it exactly. The resource type names in the table above match URL path segments. In code and JSON, always use underscores.

Each resource type needs: Create, Get (by ID), Delete, and List. Exceptions: RDB databases and users have Create, List, Delete only (no individual Get). Security groups also need Patch (update), PUT `/security_groups/{sg_id}/rules` (bulk-set rules), and GET `/security_groups/{sg_id}/rules` (list rules with `?page=` pagination) — the provider uses all three after creation. IAM has an additional `/rules` list endpoint filtered by `policy_id`. Instance has a `/products/servers` catalog endpoint — the provider queries this to validate the `commercial_type` (e.g., `DEV1-S`) before creating a server. Response shape is `{"servers": {"DEV1-S": {...}, ...}}` — a map keyed by commercial type. Each entry must include `monthly_price`, `hourly_price`, `ncpus`, `ram`, `arch`, `volumes_constraint` (with `min_size` and `max_size`), and `per_volume_constraint` (with `l_ssd` sub-object containing `min_size` and `max_size`). Why these fields matter: the provider reads `volumes_constraint.max_size` to validate total local volume size, and reads `per_volume_constraint.l_ssd` to determine that the server type supports local SSD volumes. Both are required — see Pending Fixes if `per_volume_constraint` is not yet implemented.

**IAM note**: The IAM API is organisation-scoped — no `{zone}` or `{region}` path parameter. All IAM resources use `/iam/v1alpha1/` as their prefix.

**Account legacy shim**: The Scaleway provider has two SSH key resources: `scaleway_iam_ssh_key` (current, uses `/iam/v1alpha1/ssh-keys`) and `scaleway_account_ssh_key` (deprecated, uses `/account/v2alpha1/ssh-keys`). Existing configs may use either. Mockway supports both paths as aliases to the **same underlying `iam_ssh_keys` table**. Account routes delegate to the same repository methods as IAM SSH key routes — one canonical storage, two entry points.

**Full route patterns** (needed if hand-writing routes — codegen generates these automatically):

| Method | Route | Operation |
|--------|-------|-----------|
| GET | `/instance/v1/zones/{zone}/products/servers` | List available server types (static catalog) |
| POST/GET | `/instance/v1/zones/{zone}/servers` | Create/List servers |
| GET/DELETE | `/instance/v1/zones/{zone}/servers/{server_id}` | Get/Delete server |
| POST/GET | `/instance/v1/zones/{zone}/ips` | Create/List IPs |
| GET/DELETE | `/instance/v1/zones/{zone}/ips/{ip_id}` | Get/Delete IP |
| POST/GET | `/instance/v1/zones/{zone}/security_groups` | Create/List security groups |
| GET/PATCH/DELETE | `/instance/v1/zones/{zone}/security_groups/{sg_id}` | Get/Update/Delete security group |
| PUT/GET | `/instance/v1/zones/{zone}/security_groups/{sg_id}/rules` | Set/List security group rules |
| POST/GET | `/instance/v1/zones/{zone}/servers/{server_id}/private_nics` | Create/List private NICs |
| GET/DELETE | `/instance/v1/zones/{zone}/servers/{server_id}/private_nics/{nic_id}` | Get/Delete private NIC |
| POST/GET | `/vpc/v1/regions/{region}/vpcs` | Create/List VPCs |
| GET/DELETE | `/vpc/v1/regions/{region}/vpcs/{vpc_id}` | Get/Delete VPC |
| POST/GET | `/vpc/v1/regions/{region}/private-networks` | Create/List private networks |
| GET/DELETE | `/vpc/v1/regions/{region}/private-networks/{pn_id}` | Get/Delete private network |
| POST/GET | `/lb/v1/zones/{zone}/lbs` | Create/List LBs |
| GET/DELETE | `/lb/v1/zones/{zone}/lbs/{lb_id}` | Get/Delete LB |
| POST/GET | `/lb/v1/zones/{zone}/frontends` | Create/List frontends |
| GET/DELETE | `/lb/v1/zones/{zone}/frontends/{frontend_id}` | Get/Delete frontend |
| POST/GET | `/lb/v1/zones/{zone}/backends` | Create/List backends |
| GET/DELETE | `/lb/v1/zones/{zone}/backends/{backend_id}` | Get/Delete backend |
| POST/GET | `/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks` | Attach/List LB private networks |
| DELETE | `/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks/{pn_id}` | Detach LB private network |
| POST/GET | `/k8s/v1/regions/{region}/clusters` | Create/List clusters |
| GET/DELETE | `/k8s/v1/regions/{region}/clusters/{cluster_id}` | Get/Delete cluster |
| POST/GET | `/k8s/v1/regions/{region}/clusters/{cluster_id}/pools` | Create/List pools |
| GET/DELETE | `/k8s/v1/regions/{region}/pools/{pool_id}` | Get/Delete pool |
| POST/GET | `/rdb/v1/regions/{region}/instances` | Create/List RDB instances |
| GET/DELETE | `/rdb/v1/regions/{region}/instances/{instance_id}` | Get/Delete RDB instance |
| POST/GET | `/rdb/v1/regions/{region}/instances/{instance_id}/databases` | Create/List databases |
| DELETE | `/rdb/v1/regions/{region}/instances/{instance_id}/databases/{db_name}` | Delete database |
| POST/GET | `/rdb/v1/regions/{region}/instances/{instance_id}/users` | Create/List users |
| DELETE | `/rdb/v1/regions/{region}/instances/{instance_id}/users/{user_name}` | Delete user |
| POST/GET | `/iam/v1alpha1/applications` | Create/List IAM applications |
| GET/DELETE | `/iam/v1alpha1/applications/{application_id}` | Get/Delete IAM application |
| POST/GET | `/iam/v1alpha1/api-keys` | Create/List API keys |
| GET/DELETE | `/iam/v1alpha1/api-keys/{access_key}` | Get/Delete API key |
| POST/GET | `/iam/v1alpha1/policies` | Create/List IAM policies |
| GET/DELETE | `/iam/v1alpha1/policies/{policy_id}` | Get/Delete IAM policy |
| GET | `/iam/v1alpha1/rules` | List IAM rules (filtered by `policy_id` query param) |
| POST/GET | `/iam/v1alpha1/ssh-keys` | Create/List SSH keys |
| GET/DELETE | `/iam/v1alpha1/ssh-keys/{ssh_key_id}` | Get/Delete SSH key |
| POST/GET | `/account/v2alpha1/ssh-keys` | Create/List SSH keys (legacy alias → IAM ssh-keys) |
| GET/DELETE | `/account/v2alpha1/ssh-keys/{ssh_key_id}` | Get/Delete SSH key (legacy alias → IAM ssh-keys) |

Note: These routes are based on the Scaleway API structure. The echo server smoke test (Build Order step 1) will empirically confirm which exact paths the provider hits. Adjust if the provider uses different paths.

## HTTP Router

**chi** — lightweight, stdlib-compatible, best-supported oapi-codegen target.

## OpenAPI Codegen Pipeline

Scaleway publishes OpenAPI 3.1 YAML specs per service. Use **oapi-codegen** to generate Go types, chi server interfaces, and router per service.

**Specs** (download to `specs/` directory):

| Service | Spec URL |
|---------|----------|
| Instance | `https://developers.scaleway.com/static/scaleway.instance.v1.Api.yml` |
| VPC | `https://developers.scaleway.com/static/scaleway.vpc.v1.Api.yml` |
| Load Balancer | `https://developers.scaleway.com/static/scaleway.lb.v1.Api.yml` |
| Kubernetes | `https://developers.scaleway.com/static/scaleway.k8s.v1.Api.yml` |
| RDB | `https://developers.scaleway.com/static/scaleway.rdb.v1.Api.yml` |
| IAM | `https://developers.scaleway.com/static/scaleway.iam.v1alpha1.Api.yml` |

**Pipeline**:
1. Download specs → `specs/` directory
2. Pre-process specs (strip/convert `x-one-of` extensions)
3. Run oapi-codegen per service:
   ```bash
   oapi-codegen -package instance -generate types    specs/scaleway.instance.v1.Api.yml > generated/instance/types.go
   oapi-codegen -package instance -generate chi-server specs/scaleway.instance.v1.Api.yml > generated/instance/server.go
   ```
4. Implement the interface methods with SQLite-backed stateful logic
5. When Scaleway updates their API → re-download specs → re-run generator → compiler shows unimplemented methods

Use `go generate` directives or a `Makefile` target to automate steps 1-3.

**Codegen strategy**: Try oapi-codegen first. Scaleway uses a custom `x-one-of` extension in their OpenAPI specs — pre-process the YAML to strip or convert these to standard `oneOf` before running oapi-codegen. If codegen produces usable types + interfaces, use them. If `x-one-of` or other spec issues block codegen, fall back to hand-written Go types and chi routes based on the specs as reference documentation. Document which approach was used and why.

## Project Structure

Follows the layout from [redscaresu/simpleAPI](https://github.com/redscaresu/simpleAPI): top-level packages, `handlers/` with DI via `Application` struct, `repository/` for data access, `models/` for shared types.

```
mockway/
├── cmd/
│   └── mockway/
│       └── main.go               # Entrypoint — flag parsing, DI wiring, server start
├── handlers/
│   ├── handlers.go               # Application struct, NewApplication(), RegisterRoutes()
│   ├── instance.go               # Instance API handlers (servers, ips, security_groups, private_nics)
│   ├── vpc.go                    # VPC API handlers (vpcs, private_networks)
│   ├── lb.go                     # Load Balancer handlers (lbs, frontends, backends, private_networks)
│   ├── k8s.go                    # Kubernetes handlers (clusters, pools)
│   ├── rdb.go                    # RDB handlers (instances, databases, users)
│   ├── iam.go                    # IAM handlers (applications, api_keys, policies, ssh_keys)
│   ├── admin.go                  # /mock/reset, /mock/state handlers
│   └── handlers_test.go          # Integration tests (HTTP round-trips)
├── models/
│   └── models.go                 # Shared types: domain errors (ErrNotFound, ErrConflict), resource types
├── repository/
│   ├── repository.go             # SQLite state management, schema init, CRUD, reset
│   └── repository_test.go        # Unit tests (repository layer)
├── testutil/
│   └── testutil.go               # Shared test helpers (NewTestServer, DoCreate, etc.)
├── specs/                         # Downloaded Scaleway OpenAPI YAML specs
├── generated/                     # oapi-codegen output (only if codegen succeeds — omit if hand-written)
├── go.mod                         # module github.com/redscaresu/mockway
├── go.sum
├── goreleaser.yml
├── Dockerfile
├── AGENTS.md
└── README.md
```

**Key pattern** (from simpleAPI): dependency injection via `Application` struct:

```go
// cmd/mockway/main.go
func run() error {
    port := flag.Int("port", 8080, "HTTP port")
    dbPath := flag.String("db", ":memory:", "SQLite database path")
    flag.Parse()

    repo := repository.New(*dbPath)
    defer repo.Close()

    app := handlers.NewApplication(repo)

    r := chi.NewRouter()
    r.Use(middleware.Logger)
    app.RegisterRoutes(r)

    // Catch-all for unimplemented routes — returns 501 and logs the
    // method + path so a single `tofu apply` / `terraform apply` reveals every missing
    // endpoint at once instead of failing on the first one.
    r.NotFound(handlers.UnimplementedHandler)
    r.MethodNotAllowed(handlers.UnimplementedHandler)

    return http.ListenAndServe(fmt.Sprintf(":%d", *port), r)
}

// handlers/handlers.go
type Application struct {
    repo *repository.Repository
}

func NewApplication(repo *repository.Repository) *Application {
    return &Application{repo: repo}
}

func (app *Application) RegisterRoutes(r chi.Router) {
    // Admin routes — no auth required
    r.Post("/mock/reset", app.ResetState)
    r.Get("/mock/state", app.GetState)
    r.Get("/mock/state/{service}", app.GetServiceState)

    // Scaleway API routes — require X-Auth-Token
    r.Group(func(r chi.Router) {
        r.Use(app.requireAuthToken)
        r.Route("/instance/v1/zones/{zone}", func(r chi.Router) {
            r.Get("/products/servers", app.ListProductsServers)

            r.Post("/servers", app.CreateServer)
            r.Get("/servers", app.ListServers)
            r.Get("/servers/{server_id}", app.GetServer)
            r.Delete("/servers/{server_id}", app.DeleteServer)

            r.Post("/ips", app.CreateIP)
            r.Get("/ips", app.ListIPs)
            r.Get("/ips/{ip_id}", app.GetIP)
            r.Delete("/ips/{ip_id}", app.DeleteIP)

            r.Post("/security_groups", app.CreateSecurityGroup)
            r.Get("/security_groups", app.ListSecurityGroups)
            r.Get("/security_groups/{sg_id}", app.GetSecurityGroup)
            r.Patch("/security_groups/{sg_id}", app.UpdateSecurityGroup)
            r.Put("/security_groups/{sg_id}/rules", app.SetSecurityGroupRules)
            r.Get("/security_groups/{sg_id}/rules", app.GetSecurityGroupRules)
            r.Delete("/security_groups/{sg_id}", app.DeleteSecurityGroup)

            r.Post("/servers/{server_id}/private_nics", app.CreatePrivateNIC)
            r.Get("/servers/{server_id}/private_nics", app.ListPrivateNICs)
            r.Get("/servers/{server_id}/private_nics/{nic_id}", app.GetPrivateNIC)
            r.Delete("/servers/{server_id}/private_nics/{nic_id}", app.DeletePrivateNIC)
        })

        r.Route("/vpc/v1/regions/{region}", func(r chi.Router) {
            r.Post("/vpcs", app.CreateVPC)
            r.Get("/vpcs", app.ListVPCs)
            r.Get("/vpcs/{vpc_id}", app.GetVPC)
            r.Delete("/vpcs/{vpc_id}", app.DeleteVPC)

            r.Post("/private-networks", app.CreatePrivateNetwork)
            r.Get("/private-networks", app.ListPrivateNetworks)
            r.Get("/private-networks/{pn_id}", app.GetPrivateNetwork)
            r.Delete("/private-networks/{pn_id}", app.DeletePrivateNetwork)
        })

        r.Route("/lb/v1/zones/{zone}", func(r chi.Router) {
            r.Post("/lbs", app.CreateLB)
            r.Get("/lbs", app.ListLBs)
            r.Get("/lbs/{lb_id}", app.GetLB)
            r.Delete("/lbs/{lb_id}", app.DeleteLB)

            r.Post("/frontends", app.CreateFrontend)
            r.Get("/frontends", app.ListFrontends)
            r.Get("/frontends/{frontend_id}", app.GetFrontend)
            r.Delete("/frontends/{frontend_id}", app.DeleteFrontend)

            r.Post("/backends", app.CreateBackend)
            r.Get("/backends", app.ListBackends)
            r.Get("/backends/{backend_id}", app.GetBackend)
            r.Delete("/backends/{backend_id}", app.DeleteBackend)

            r.Post("/lbs/{lb_id}/private-networks", app.AttachLBPrivateNetwork)
            r.Get("/lbs/{lb_id}/private-networks", app.ListLBPrivateNetworks)
            r.Delete("/lbs/{lb_id}/private-networks/{pn_id}", app.DeleteLBPrivateNetwork)
        })

        r.Route("/k8s/v1/regions/{region}", func(r chi.Router) {
            r.Post("/clusters", app.CreateCluster)
            r.Get("/clusters", app.ListClusters)
            r.Get("/clusters/{cluster_id}", app.GetCluster)
            r.Delete("/clusters/{cluster_id}", app.DeleteCluster)

            r.Post("/clusters/{cluster_id}/pools", app.CreatePool)
            r.Get("/clusters/{cluster_id}/pools", app.ListPools)
            r.Get("/pools/{pool_id}", app.GetPool)
            r.Delete("/pools/{pool_id}", app.DeletePool)
        })

        r.Route("/rdb/v1/regions/{region}", func(r chi.Router) {
            r.Post("/instances", app.CreateRDBInstance)
            r.Get("/instances", app.ListRDBInstances)
            r.Get("/instances/{instance_id}", app.GetRDBInstance)
            r.Delete("/instances/{instance_id}", app.DeleteRDBInstance)

            r.Post("/instances/{instance_id}/databases", app.CreateRDBDatabase)
            r.Get("/instances/{instance_id}/databases", app.ListRDBDatabases)
            r.Delete("/instances/{instance_id}/databases/{db_name}", app.DeleteRDBDatabase)

            r.Post("/instances/{instance_id}/users", app.CreateRDBUser)
            r.Get("/instances/{instance_id}/users", app.ListRDBUsers)
            r.Delete("/instances/{instance_id}/users/{user_name}", app.DeleteRDBUser)
        })

        r.Route("/iam/v1alpha1", func(r chi.Router) {
            r.Post("/applications", app.CreateIAMApplication)
            r.Get("/applications", app.ListIAMApplications)
            r.Get("/applications/{application_id}", app.GetIAMApplication)
            r.Delete("/applications/{application_id}", app.DeleteIAMApplication)

            r.Post("/api-keys", app.CreateIAMAPIKey)
            r.Get("/api-keys", app.ListIAMAPIKeys)
            r.Get("/api-keys/{access_key}", app.GetIAMAPIKey)
            r.Delete("/api-keys/{access_key}", app.DeleteIAMAPIKey)

            r.Post("/policies", app.CreateIAMPolicy)
            r.Get("/policies", app.ListIAMPolicies)
            r.Get("/policies/{policy_id}", app.GetIAMPolicy)
            r.Delete("/policies/{policy_id}", app.DeleteIAMPolicy)

            r.Get("/rules", app.ListIAMRules)

            r.Post("/ssh-keys", app.CreateIAMSSHKey)
            r.Get("/ssh-keys", app.ListIAMSSHKeys)
            r.Get("/ssh-keys/{ssh_key_id}", app.GetIAMSSHKey)
            r.Delete("/ssh-keys/{ssh_key_id}", app.DeleteIAMSSHKey)
        })

        // Account (legacy alias — same handlers as IAM SSH keys)
        r.Route("/account/v2alpha1", func(r chi.Router) {
            r.Post("/ssh-keys", app.CreateIAMSSHKey)
            r.Get("/ssh-keys", app.ListIAMSSHKeys)
            r.Get("/ssh-keys/{ssh_key_id}", app.GetIAMSSHKey)
            r.Delete("/ssh-keys/{ssh_key_id}", app.DeleteIAMSSHKey)
        })
    })
}
```

## SQLite Schema

Per-type tables with JSON blob for full resource data, extracted FK columns for integrity. Run `PRAGMA foreign_keys = ON` immediately after opening the connection (before creating tables). This is safe because `SetMaxOpenConns(1)` ensures all queries go through the same connection (see SQLite connection strategy above).

```sql
-- VPC
CREATE TABLE vpcs (
    id TEXT PRIMARY KEY,
    region TEXT NOT NULL,
    data JSON NOT NULL
);

CREATE TABLE private_networks (
    id TEXT PRIMARY KEY,
    vpc_id TEXT NOT NULL REFERENCES vpcs(id),
    region TEXT NOT NULL,
    data JSON NOT NULL
);

-- Instance (security_groups first — referenced by servers)
CREATE TABLE instance_security_groups (
    id TEXT PRIMARY KEY,
    zone TEXT NOT NULL,
    data JSON NOT NULL
);

CREATE TABLE instance_servers (
    id TEXT PRIMARY KEY,
    zone TEXT NOT NULL,
    security_group_id TEXT REFERENCES instance_security_groups(id),
    data JSON NOT NULL
);

CREATE TABLE instance_ips (
    id TEXT PRIMARY KEY,
    server_id TEXT REFERENCES instance_servers(id),
    zone TEXT NOT NULL,
    data JSON NOT NULL
);

CREATE TABLE instance_private_nics (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES instance_servers(id),
    private_network_id TEXT NOT NULL REFERENCES private_networks(id),
    zone TEXT NOT NULL,
    data JSON NOT NULL
);

-- Load Balancer
CREATE TABLE lbs (
    id TEXT PRIMARY KEY,
    zone TEXT NOT NULL,
    data JSON NOT NULL
);

CREATE TABLE lb_frontends (
    id TEXT PRIMARY KEY,
    lb_id TEXT NOT NULL REFERENCES lbs(id),
    data JSON NOT NULL
);

CREATE TABLE lb_backends (
    id TEXT PRIMARY KEY,
    lb_id TEXT NOT NULL REFERENCES lbs(id),
    data JSON NOT NULL
);

CREATE TABLE lb_private_networks (
    lb_id TEXT NOT NULL REFERENCES lbs(id),
    private_network_id TEXT NOT NULL REFERENCES private_networks(id),
    data JSON NOT NULL,
    PRIMARY KEY (lb_id, private_network_id)
);

-- Kubernetes
CREATE TABLE k8s_clusters (
    id TEXT PRIMARY KEY,
    region TEXT NOT NULL,
    private_network_id TEXT REFERENCES private_networks(id),
    data JSON NOT NULL
);

CREATE TABLE k8s_pools (
    id TEXT PRIMARY KEY,
    cluster_id TEXT NOT NULL REFERENCES k8s_clusters(id),
    region TEXT NOT NULL,
    data JSON NOT NULL
);

-- RDB
CREATE TABLE rdb_instances (
    id TEXT PRIMARY KEY,
    region TEXT NOT NULL,
    data JSON NOT NULL
);

CREATE TABLE rdb_databases (
    instance_id TEXT NOT NULL REFERENCES rdb_instances(id),
    name TEXT NOT NULL,
    data JSON NOT NULL,
    PRIMARY KEY (instance_id, name)
);

CREATE TABLE rdb_users (
    instance_id TEXT NOT NULL REFERENCES rdb_instances(id),
    name TEXT NOT NULL,
    data JSON NOT NULL,
    PRIMARY KEY (instance_id, name)
);

-- IAM (organisation-scoped — no zone/region column)
CREATE TABLE iam_applications (
    id TEXT PRIMARY KEY,
    data JSON NOT NULL
);

CREATE TABLE iam_api_keys (
    access_key TEXT PRIMARY KEY,
    application_id TEXT REFERENCES iam_applications(id),
    data JSON NOT NULL
);

CREATE TABLE iam_policies (
    id TEXT PRIMARY KEY,
    application_id TEXT REFERENCES iam_applications(id),
    data JSON NOT NULL
);

CREATE TABLE iam_ssh_keys (
    id TEXT PRIMARY KEY,
    data JSON NOT NULL
);
```

The JSON `data` column holds the full API response shape — flexible, no migrations when Scaleway adds optional fields.

**Data flow for Create**:
1. Parse request body JSON
2. Generate UUID for `id` (for UUID-based resources), use the provided `name` (for RDB databases/users), or generate `access_key` (for IAM API keys — `SCW` + 17 random chars)
3. Inject `id` + server-generated fields into the JSON (see table below)
4. Extract FK fields (e.g., `vpc_id`, `server_id`) from JSON into dedicated columns for SQLite FK enforcement
5. Store the full enriched JSON in the `data` column
6. Return the response body (200 OK) — **Instance API**: wrap in a key (e.g., `{"server": {...}}`); **all other APIs**: return the flat object directly (see Response Wrapping below)

**Server-generated fields injected on create**:

| Resource Type | Fields Injected |
|--------------|-----------------|
| All (UUID-based) | `"id": "<uuid>"` |
| Instance servers | `"state": "running"`, `"creation_date": "<RFC3339>"`, `"modification_date": "<RFC3339>"` |
| Instance IPs | `"address": "51.15.<random>.x"` (fake public IP) |
| VPCs | `"created_at": "<RFC3339>"`, `"updated_at": "<RFC3339>"` |
| Private networks | `"created_at": "<RFC3339>"`, `"updated_at": "<RFC3339>"` |
| Load balancers | `"status": "ready"`, `"created_at": "<RFC3339>"`, `"ip": [{"id": "<uuid>", "ip_address": "51.15.<random>.x", "lb_id": "<lb_id>"}]` |
| LB frontends/backends | (no extra fields beyond `id`) |
| K8s clusters | `"status": "ready"`, `"created_at": "<RFC3339>"`, `"updated_at": "<RFC3339>"` |
| K8s pools | `"status": "ready"`, `"created_at": "<RFC3339>"`, `"updated_at": "<RFC3339>"` |
| RDB instances | `"status": "ready"`, `"created_at": "<RFC3339>"`, `"endpoints": [...]` (see RDB endpoint transformation below) |
| RDB databases/users | (no extra fields beyond what's in the request — identified by name, not UUID) |
| IAM applications | `"created_at": "<RFC3339>"`, `"updated_at": "<RFC3339>"` |
| IAM API keys | `"access_key": "SCW<random 17 chars>"`, `"secret_key": "<uuid>"` (returned on create only — omit from Get/List), `"created_at": "<RFC3339>"`, `"updated_at": "<RFC3339>"` |
| IAM policies | `"created_at": "<RFC3339>"`, `"updated_at": "<RFC3339>"` |
| IAM SSH keys | `"created_at": "<RFC3339>"`, `"updated_at": "<RFC3339>"`, `"fingerprint": "256 SHA256:<random>"` |

Timestamp format: RFC 3339 (`time.Now().UTC().Format(time.RFC3339)`). Use the same value for both `created_at` and `updated_at` on create. Fake IPs: generate deterministically or randomly — consistency doesn't matter for mock testing, just uniqueness.

**RDB endpoint transformation**:

The Scaleway RDB API accepts `init_endpoints` on create and returns `endpoints` in the response. Mockway transforms as follows:

Port is derived from the `engine` field in the create request: `5432` for PostgreSQL, `3306` for MySQL. If engine is unrecognised, default to `5432`.

1. **With `init_endpoints`** (private network): Request contains `"init_endpoints": [{"private_network": {"id": "<pn_id>"}}]`. Handler validates `<pn_id>` exists in `private_networks` table (404 if not). Stored/returned as:
   ```json
   "endpoints": [{"ip": "10.0.<random>.x", "port": 5432, "private_network": {"id": "<pn_id>"}}]
   ```
   The handler generates a fake private IP (`10.x.x.x` range) and sets the port based on engine.

2. **Without `init_endpoints`** (public endpoint): No `init_endpoints` in request body. Stored/returned as:
   ```json
   "endpoints": [{"ip": "51.15.<random>.x", "port": 5432}]
   ```
   The handler generates a fake public IP. No `private_network` key in the endpoint object.

In both cases, `init_endpoints` is **not** stored — it's consumed on create and transformed into `endpoints` in the stored `data` JSON.

**Data flow for Get/List**:
1. Query the `data` column from SQLite
2. For Get: **Instance API** — wrap in a key (e.g., `{"server": {...}}`); **all other APIs** — return the flat object directly
3. For List: wrap in a plural key with `total_count` (all APIs — see Response Wrapping below)

**Exception**: IAM API keys. The `secret_key` is stored in the `data` blob (so create can return it) but must be **stripped** before returning from Get and List. Parse the JSON, delete the `"secret_key"` key, return the rest. All other fields (including `user_id` or `application_id`) are returned unchanged. This is the only Get/List transformation in Mockway.

## Referential Integrity

Mockway must enforce the same referential integrity as the real Scaleway API:

- **On create**: validate that referenced resources exist. Creating a `private_nic` with a non-existent `server_id` → **404 Not Found**. Creating an RDB instance with an `init_endpoints[].private_network.id` that doesn't exist → **404 Not Found**.
- **On duplicate create**: UUID-based resources generate a new UUID on each create, so collisions are impossible. For composite-key resources (RDB databases/users), creating with the same `(instance_id, name)` that already exists → **409 Conflict** with `{"message": "resource already exists", "type": "conflict"}`.
- **On delete**: reject if dependents still exist. Delete a VPC when private networks are still attached → **409 Conflict**. Delete a private network when NICs are attached → **409 Conflict**.
- **`POST /mock/reset`**: wipes all state. Disable FK checks for reset (`PRAGMA foreign_keys = OFF`, delete all rows, `PRAGMA foreign_keys = ON`). This avoids needing FK-ordered truncation.

SQLite enforces FKs natively (`PRAGMA foreign_keys = ON`). Use this for most integrity checks. For cases where the FK is inside the JSON `data` blob, validate programmatically in the handler before inserting:

- **RDB instance create**: if the request body contains `"init_endpoints": [{"private_network": {"id": "<pn_id>"}}]`, the handler must query the `private_networks` table to verify `<pn_id>` exists before inserting. Return 404 if not found. This is handler-level validation because `rdb_instances` has no explicit `private_network_id` column — the reference lives only inside the JSON data blob.

**IAM-specific FK rules**:
- **API key create**: either `application_id` or `user_id` must be provided (mutually exclusive). If `application_id` is provided, it must reference an existing IAM application → 404 if not found. If `user_id` is provided, accept any non-empty value (Mockway doesn't mock users). If neither is provided → 400 Bad Request. The PK is `access_key` (server-generated `SCW` + 17 random chars), not a UUID. The `secret_key` is stored in the `data` blob but stripped on Get/List (see Data flow exception above). **`user_id` persistence**: `user_id` is stored only in the JSON `data` blob — no dedicated column, no FK enforcement (Mockway has no users table). When provided, it appears in Get, List, and admin state responses. The `application_id` column in `iam_api_keys` is set to `NULL` when the API key belongs to a user instead of an application.
- **Policy create**: `application_id` is optional. If provided, must reference an existing IAM application → 404 if not found.
- **Delete application**: reject if API keys or policies still reference it → 409 Conflict.
- **SSH keys**: standalone — no FK to any other resource. Accessible via both `/iam/v1alpha1/ssh-keys` and `/account/v2alpha1/ssh-keys` (same underlying state).

## Key Resource Relationships

```
VPC
 └── Private Network
      ├── Instance Private NIC → Instance Server
      ├── RDB Instance (private endpoint)
      └── LB Private Network attachment

Instance Server
 ├── Instance IP (public, optional)
 ├── Instance Private NIC → Private Network
 └── Instance Security Group

Load Balancer
 ├── LB IP (public, auto-generated on LB create — stored inline in LB data JSON, no separate table)
 ├── LB Frontend (inbound port)
 ├── LB Backend (forward port, server IPs)
 └── LB Private Network attachment

K8s Cluster
 ├── K8s Node Pool
 └── Private Network (optional)

IAM Application
 ├── IAM API Key (access_key is the PK, not UUID; application_id optional)
 └── IAM Policy (optional application_id FK)

IAM SSH Key (standalone — no parent dependency)
 └── Also accessible via Account legacy routes (same state)
```

## Admin State API

These endpoints are NOT part of the Scaleway API — they're Mockway-specific for testing and inspection.

**Endpoints**:
- `POST /mock/reset` — wipe all state (disable FKs, delete all rows, re-enable FKs)
- `GET /mock/state` — full resource graph as JSON
- `GET /mock/state/{service}` — single service state (e.g., `/mock/state/instance`). Valid service names: `instance`, `vpc`, `lb`, `k8s`, `rdb`, `iam`. Unknown service → `404 Not Found` with `{"message": "unknown service", "type": "not_found"}`.

### `GET /mock/state` Response Schema

This schema is a **versioned contract** — InfraFactory depends on this exact structure. Changes must be backwards-compatible.

```json
{
  "instance": {
    "servers": [
      {"id": "uuid", "zone": "fr-par-1", "name": "web-1", "commercial_type": "DEV1-S",
       "public_ip": null, "private_nics": ["nic-uuid-1"]}
    ],
    "ips": [
      {"id": "uuid", "address": "51.15.x.x", "server_id": null}
    ],
    "private_nics": [
      {"id": "nic-uuid-1", "server_id": "uuid", "private_network_id": "pn-uuid"}
    ],
    "security_groups": [
      {"id": "uuid", "zone": "fr-par-1", "inbound_default_policy": "drop"}
    ]
  },
  "vpc": {
    "vpcs": [
      {"id": "uuid", "region": "fr-par", "name": "main"}
    ],
    "private_networks": [
      {"id": "pn-uuid", "vpc_id": "uuid", "region": "fr-par", "name": "app-network"}
    ]
  },
  "lb": {
    "lbs": [
      {"id": "uuid", "zone": "fr-par-1", "name": "web-lb",
       "ip": [{"id": "uuid", "ip_address": "51.15.x.x", "lb_id": "uuid"}]}
    ],
    "frontends": [
      {"id": "uuid", "lb_id": "uuid", "name": "http", "inbound_port": 80,
       "backend_id": "be-uuid"}
    ],
    "backends": [
      {"id": "be-uuid", "lb_id": "uuid", "name": "web-servers",
       "forward_port": 80, "server_ips": ["10.0.0.1", "10.0.0.2"]}
    ],
    "private_networks": [
      {"lb_id": "uuid", "private_network_id": "pn-uuid"}
    ]
  },
  "k8s": {
    "clusters": [
      {"id": "uuid", "region": "fr-par", "name": "kapsule-1", "status": "ready",
       "private_network_id": "pn-uuid"}
    ],
    "pools": [
      {"id": "uuid", "cluster_id": "uuid", "name": "default",
       "node_type": "DEV1-M", "size": 3}
    ]
  },
  "rdb": {
    "instances": [
      {"id": "uuid", "region": "fr-par", "name": "app-db", "engine": "PostgreSQL-15",
       "node_type": "DB-DEV-S",
       "endpoints": [
         {"ip": "10.0.0.5", "port": 5432, "private_network": {"id": "pn-uuid"}}
       ]}
    ],
    "databases": [
      {"instance_id": "uuid", "name": "appdb"}
    ],
    "users": [
      {"instance_id": "uuid", "name": "appuser"}
    ]
  },
  "iam": {
    "applications": [
      {"id": "uuid", "name": "my-app", "description": "CI/CD application"}
    ],
    "api_keys": [
      {"access_key": "SCWxxxxxxxxxxxxxxxxx", "application_id": "uuid",
       "description": "deploy key"}
    ],
    "policies": [
      {"id": "uuid", "name": "full-access", "application_id": "uuid"}
    ],
    "ssh_keys": [
      {"id": "uuid", "name": "my-laptop",
       "public_key": "ssh-ed25519 AAAA..."}
    ]
  }
}
```

This mirrors Scaleway API response shapes. Empty arrays for resource types with no instances (never omit a key — always return the full structure with empty arrays).

**Account legacy shim and admin state**: There is no separate `"account"` section. SSH keys created via `/account/v2alpha1/ssh-keys` are stored in and returned from the `"iam"."ssh_keys"` array. The Account routes are purely a routing alias — the admin state has one canonical shape. `/mock/state/account` is **not** a valid service name (returns 404).

## Build Order

**Start with the echo server smoke test** before writing real handlers:

1. Create a minimal HTTP server on port 8080
2. Mount a catch-all handler that logs the request method, path, and headers
3. Point the Scaleway OpenTofu/Terraform provider at it (`SCW_API_URL=http://localhost:8080`) with fake credentials:
   ```
   SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX
   SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000
   SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
   ```
4. Run `tofu plan` (or `terraform plan`) for a simple Scaleway config and observe which API paths the provider hits
5. This proves `SCW_API_URL` routes all services through a single URL before writing real handlers

Then build incrementally:
1. Repository + schema init + admin endpoints (`/mock/reset`, `/mock/state`) — needed by every integration test
2. VPC + Private Networks (foundation — most other resources reference these)
3. Instance (servers, IPs, security groups, private NICs)
4. RDB (instances, databases, users)
5. Load Balancer (LBs, frontends, backends, private networks)
6. Kubernetes (clusters, pools)
7. IAM (applications, API keys, policies, SSH keys) + Account legacy SSH key alias — no dependencies on other services, can be built in any order after step 1

## Catch-All for Unimplemented Routes

Register `NotFound` and `MethodNotAllowed` handlers on the chi router that return **501 Not Implemented** and log the method + path. This turns every `tofu apply` / `terraform apply` run into a discovery tool — instead of failing on the first missing endpoint and stopping, the provider hits all endpoints it needs and Mockway logs every unimplemented one in a single run.

```go
// handlers/handlers.go
func UnimplementedHandler(w http.ResponseWriter, r *http.Request) {
    log.Printf("UNIMPLEMENTED: %s %s", r.Method, r.URL.String())
    writeJSON(w, http.StatusNotImplemented, map[string]any{
        "message": fmt.Sprintf("not implemented: %s %s", r.Method, r.URL.Path),
        "type":    "not_implemented",
    })
}
```

**How to use**: run `tofu apply` / `terraform apply` against Mockway, then grep the logs for `UNIMPLEMENTED`. Each line is a missing endpoint that needs a handler. Implement them all in one pass instead of the discover-one-fix-one-repeat cycle.

**When to remove**: once all provider-used endpoints are implemented and `tofu apply` / `terraform apply` succeeds cleanly for all resource types, the catch-all becomes a safety net — keep it in place but it should never fire during normal operation.

## Testing

Use stdlib `testing` + `net/http/httptest` + `github.com/stretchr/testify` for assertions. No other external test dependencies.

### Test Helper (`testutil/testutil.go`)

Shared helper that eliminates boilerplate across all tests:

```go
// NewTestServer creates an httptest.Server backed by in-memory SQLite.
// Returns the server and a cleanup function.
//
// Usage:
//   ts, cleanup := testutil.NewTestServer(t)
//   defer cleanup()
func NewTestServer(t *testing.T) (*httptest.Server, func())

// HTTP helpers — make requests and return parsed response + status code.
// All helpers call t.Fatal on transport errors (not on 4xx/5xx — those are valid test outcomes).
// All helpers automatically add the X-Auth-Token header (any non-empty value)
// so test code doesn't need to set auth manually.
func DoCreate(t *testing.T, ts *httptest.Server, path string, body any) (int, map[string]any)
func DoGet(t *testing.T, ts *httptest.Server, path string) (int, map[string]any)
func DoList(t *testing.T, ts *httptest.Server, path string) (int, map[string]any)
func DoDelete(t *testing.T, ts *httptest.Server, path string) int

// Admin helpers — these hit /mock/* routes which do NOT require auth.
func ResetState(t *testing.T, ts *httptest.Server)
func GetState(t *testing.T, ts *httptest.Server) map[string]any
```

### Unit Tests (`repository/repository_test.go`)

Test the repository layer directly — no HTTP, no router. Fastest feedback loop.

**What to test**:
- CRUD per table: create a resource, get it back, list returns it, delete removes it
- FK enforcement on create: insert with non-existent FK → returns `models.ErrNotFound`
- FK enforcement on delete: delete parent with children → returns `models.ErrConflict`
- JSON round-trip: store a JSON blob, retrieve it, verify it's identical
- Reset: create resources, call reset, verify all tables are empty
- Name-based keys: RDB databases/users identified by `(instance_id, name)`, not UUID

**Pattern**:
```go
func TestVPCRepository(t *testing.T) {
    repo := repository.New(":memory:")
    defer repo.Close()

    // Create
    vpc, err := repo.CreateVPC("fr-par", json.RawMessage(`{"name":"main"}`))
    require.NoError(t, err)
    require.NotEmpty(t, vpc.ID)

    // Get
    got, err := repo.GetVPC(vpc.ID)
    require.NoError(t, err)
    require.Equal(t, vpc.ID, got.ID)

    // List
    vpcs, err := repo.ListVPCs("fr-par")
    require.NoError(t, err)
    require.Len(t, vpcs, 1)

    // Delete
    err = repo.DeleteVPC(vpc.ID)
    require.NoError(t, err)

    // Get after delete → ErrNotFound
    _, err = repo.GetVPC(vpc.ID)
    require.ErrorIs(t, err, models.ErrNotFound)
}

func TestFKEnforcement(t *testing.T) {
    repo := repository.New(":memory:")
    defer repo.Close()

    // Create private network without VPC → ErrNotFound
    _, err := repo.CreatePrivateNetwork("nonexistent-vpc", "fr-par", json.RawMessage(`{}`))
    require.ErrorIs(t, err, models.ErrNotFound)

    // Create VPC + PN, then delete VPC → ErrConflict
    vpc, _ := repo.CreateVPC("fr-par", json.RawMessage(`{"name":"main"}`))
    repo.CreatePrivateNetwork(vpc.ID, "fr-par", json.RawMessage(`{"name":"net"}`))
    err = repo.DeleteVPC(vpc.ID)
    require.ErrorIs(t, err, models.ErrConflict)
}
```

Note: `require` is from `github.com/stretchr/testify`. Use it for assertions — clear failure messages, stops test on first failure.

### Integration Tests (`handlers/handlers_test.go`)

Full HTTP round-trips against `httptest.NewServer`. Tests the contract: correct status codes, response shapes, Content-Type headers.

**What to test**:
- Lifecycle per resource type: Create (200) → Get (200) → List (200, correct `total_count`) → Delete (204) → Get (404)
- FK rejection via HTTP: create private_nic with non-existent server → 404
- FK rejection via HTTP: delete VPC with attached private networks → 409
- Admin reset: create resources → `POST /mock/reset` (204) → `GET /mock/state` returns all empty arrays
- Admin state structure: create resources → `GET /mock/state` → verify JSON matches the versioned contract schema
- Cross-service flow: create VPC → create PN → create server → create private NIC → `GET /mock/state` → verify all relationships
- IAM application lifecycle: Create → Get → List → Delete
- IAM API key lifecycle: create application → create API key with `application_id` → Get → List → Delete. Also: create API key with `user_id` (no application) → verify works
- IAM API key FK: create API key with non-existent `application_id` → 404. Delete application with attached API keys → 409
- IAM policy lifecycle: Create (with and without `application_id`) → Get → List → Delete
- IAM SSH key lifecycle via `/iam/v1alpha1/ssh-keys`: Create → Get → List → Delete
- Account SSH key compatibility: create via `/account/v2alpha1/ssh-keys` → Get via `/iam/v1alpha1/ssh-keys/{id}` (cross-path). Create via IAM → Get via Account (reverse cross-path). List via either path returns same results
- IAM admin state: create IAM resources → `GET /mock/state/iam` → verify all 4 resource type arrays present

**Pattern**:
```go
func TestInstanceServerLifecycle(t *testing.T) {
    ts, cleanup := testutil.NewTestServer(t)
    defer cleanup()

    // Create — Instance API wraps response in "server" key
    status, body := testutil.DoCreate(t, ts,
        "/instance/v1/zones/fr-par-1/servers",
        map[string]any{"name": "web-1", "commercial_type": "DEV1-S"},
    )
    require.Equal(t, 200, status)
    server := body["server"].(map[string]any)
    serverID := server["id"].(string)
    require.NotEmpty(t, serverID)

    // Get — also wrapped
    status, body = testutil.DoGet(t, ts,
        "/instance/v1/zones/fr-par-1/servers/"+serverID,
    )
    require.Equal(t, 200, status)
    server = body["server"].(map[string]any)
    require.Equal(t, "web-1", server["name"])

    // List
    status, body = testutil.DoList(t, ts,
        "/instance/v1/zones/fr-par-1/servers",
    )
    require.Equal(t, 200, status)
    require.Equal(t, float64(1), body["total_count"])

    // Delete
    status = testutil.DoDelete(t, ts,
        "/instance/v1/zones/fr-par-1/servers/"+serverID,
    )
    require.Equal(t, 204, status)

    // Get after delete → 404
    status, _ = testutil.DoGet(t, ts,
        "/instance/v1/zones/fr-par-1/servers/"+serverID,
    )
    require.Equal(t, 404, status)
}

func TestCrossServiceFlow(t *testing.T) {
    ts, cleanup := testutil.NewTestServer(t)
    defer cleanup()

    // VPC → Private Network → Server → Private NIC
    _, vpc := testutil.DoCreate(t, ts,
        "/vpc/v1/regions/fr-par/vpcs",
        map[string]any{"name": "main"},
    )
    _, pn := testutil.DoCreate(t, ts,
        "/vpc/v1/regions/fr-par/private-networks",
        map[string]any{"name": "app-net", "vpc_id": vpc["id"]},
    )
    _, srvBody := testutil.DoCreate(t, ts,
        "/instance/v1/zones/fr-par-1/servers",
        map[string]any{"name": "web-1", "commercial_type": "DEV1-S"},
    )
    srv := srvBody["server"].(map[string]any)
    _, nicBody := testutil.DoCreate(t, ts,
        "/instance/v1/zones/fr-par-1/servers/"+srv["id"].(string)+"/private_nics",
        map[string]any{"private_network_id": pn["id"]},
    )
    nic := nicBody["private_nic"].(map[string]any)

    // Verify via admin state
    state := testutil.GetState(t, ts)
    instance := state["instance"].(map[string]any)
    nics := instance["private_nics"].([]any)
    require.Len(t, nics, 1)
    require.Equal(t, nic["id"], nics[0].(map[string]any)["id"])
}

func TestFKRejectionHTTP(t *testing.T) {
    ts, cleanup := testutil.NewTestServer(t)
    defer cleanup()

    // Create NIC with non-existent server → 404
    status, body := testutil.DoCreate(t, ts,
        "/instance/v1/zones/fr-par-1/servers/nonexistent/private_nics",
        map[string]any{"private_network_id": "also-nonexistent"},
    )
    require.Equal(t, 404, status)
    require.Equal(t, "not_found", body["type"])

    // Create VPC + PN, delete VPC → 409
    _, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
    testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks",
        map[string]any{"name": "pn", "vpc_id": vpc["id"]})
    status = testutil.DoDelete(t, ts, "/vpc/v1/regions/fr-par/vpcs/"+vpc["id"].(string))
    require.Equal(t, 409, status)
}
```

### Response Shape Conformance Tests (`handlers/handlers_test.go`)

Table-driven tests that validate every endpoint returns the correct response structure. These prevent regressions where Mockway's response format drifts from the real Scaleway API (e.g., missing wrapper keys, wrong list key names).

**Why**: The Scaleway provider deserializes responses into typed structs. If the JSON shape is wrong (e.g., flat `{"id": "..."}` instead of wrapped `{"server": {"id": "..."}}`), the provider panics with a nil dereference. These tests catch that class of bug at the integration level.

**`TestCreateGetResponseWrapping`** — validates Create and Get responses have the correct wrapper structure:

```go
func TestCreateGetResponseWrapping(t *testing.T) {
    type wrapCase struct {
        name       string
        setup      func(t *testing.T, ts *httptest.Server, ctx map[string]string)
        createPath string
        getPath    string
        body       map[string]any
        wrapKey    string // e.g. "server" for Instance, "" for flat
        idField    string // field inside the resource object that holds the ID
    }

    cases := []wrapCase{
        // Instance API — wrapped in a key
        {name: "Server",        createPath: "/instance/v1/zones/{zone}/servers",         getPath: "/instance/v1/zones/{zone}/servers/{id}",         body: map[string]any{"name": "s"}, wrapKey: "server",         idField: "id"},
        {name: "IP",            setup: setupServer, createPath: "/instance/v1/zones/{zone}/ips", getPath: "/instance/v1/zones/{zone}/ips/{id}",     body: map[string]any{"server_id": "{server_id}"}, wrapKey: "ip", idField: "id"},
        {name: "SecurityGroup", createPath: "/instance/v1/zones/{zone}/security_groups", getPath: "/instance/v1/zones/{zone}/security_groups/{id}", body: map[string]any{"name": "sg"}, wrapKey: "security_group", idField: "id"},
        {name: "PrivateNIC",    setup: setupServerAndPN, createPath: "/instance/v1/zones/{zone}/servers/{server_id}/private_nics", getPath: "/instance/v1/zones/{zone}/servers/{server_id}/private_nics/{id}", body: map[string]any{"private_network_id": "{pn_id}"}, wrapKey: "private_nic", idField: "id"},

        // All other APIs — flat (no wrapper)
        {name: "VPC",            createPath: "/vpc/v1/regions/{region}/vpcs",             getPath: "/vpc/v1/regions/{region}/vpcs/{id}",             body: map[string]any{"name": "v"}, wrapKey: "", idField: "id"},
        {name: "PrivateNetwork", setup: setupVPC, createPath: "/vpc/v1/regions/{region}/private-networks", getPath: "/vpc/v1/regions/{region}/private-networks/{id}", body: map[string]any{"name": "pn", "vpc_id": "{vpc_id}"}, wrapKey: "", idField: "id"},
        {name: "LB",             createPath: "/lb/v1/zones/{zone}/lbs",                  getPath: "/lb/v1/zones/{zone}/lbs/{id}",                   body: map[string]any{"name": "lb"}, wrapKey: "", idField: "id"},
        {name: "Cluster",        createPath: "/k8s/v1/regions/{region}/clusters",         getPath: "/k8s/v1/regions/{region}/clusters/{id}",         body: map[string]any{"name": "k"}, wrapKey: "", idField: "id"},
        {name: "RDBInstance",    createPath: "/rdb/v1/regions/{region}/instances",         getPath: "/rdb/v1/regions/{region}/instances/{id}",        body: map[string]any{"name": "db"}, wrapKey: "", idField: "id"},
        {name: "IAMApplication", createPath: "/iam/v1alpha1/applications",                getPath: "/iam/v1alpha1/applications/{id}",                body: map[string]any{"name": "app"}, wrapKey: "", idField: "id"},
        // ... remaining flat endpoints
    }

    for _, tt := range cases {
        t.Run(tt.name, func(t *testing.T) {
            ts, cleanup := testutil.NewTestServer(t)
            defer cleanup()

            ctx := map[string]string{"zone": "fr-par-1", "region": "fr-par"}
            if tt.setup != nil {
                tt.setup(t, ts, ctx)
            }

            // Create
            status, raw := testutil.DoCreate(t, ts, pathWithCtx(tt.createPath, ctx), bodyWithCtx(tt.body, ctx))
            require.Equal(t, 200, status)

            if tt.wrapKey != "" {
                // Wrapped: top-level must have wrapKey, resource inside it
                inner, ok := raw[tt.wrapKey].(map[string]any)
                require.True(t, ok, "expected wrapper key %q", tt.wrapKey)
                require.NotEmpty(t, inner[tt.idField])
                ctx["id"] = inner[tt.idField].(string)
            } else {
                // Flat: top-level has id directly
                require.NotEmpty(t, raw[tt.idField])
                ctx["id"] = raw[tt.idField].(string)
            }

            // Get
            status, raw = testutil.DoGet(t, ts, pathWithCtx(tt.getPath, ctx))
            require.Equal(t, 200, status)

            if tt.wrapKey != "" {
                inner, ok := raw[tt.wrapKey].(map[string]any)
                require.True(t, ok, "expected wrapper key %q", tt.wrapKey)
                require.Equal(t, ctx["id"], inner[tt.idField])
            } else {
                require.Equal(t, ctx["id"], raw[tt.idField])
            }
        })
    }
}
```

**`TestListResponseKeys`** — validates every List endpoint returns the correct plural key and `total_count`:

```go
func TestListResponseKeys(t *testing.T) {
    type listCase struct {
        name    string
        setup   func(t *testing.T, ts *httptest.Server, ctx map[string]string)
        path    string
        listKey string // expected key for the array
    }

    cases := []listCase{
        {name: "Servers",           path: "/instance/v1/zones/{zone}/servers",           listKey: "servers"},
        {name: "IPs",               path: "/instance/v1/zones/{zone}/ips",               listKey: "ips"},
        {name: "SecurityGroups",    path: "/instance/v1/zones/{zone}/security_groups",   listKey: "security_groups"},
        {name: "VPCs",              path: "/vpc/v1/regions/{region}/vpcs",               listKey: "vpcs"},
        {name: "PrivateNetworks",   path: "/vpc/v1/regions/{region}/private-networks",   listKey: "private_networks"},
        {name: "LBs",               path: "/lb/v1/zones/{zone}/lbs",                    listKey: "lbs"},
        {name: "Frontends",         path: "/lb/v1/zones/{zone}/frontends",              listKey: "frontends"},
        {name: "Backends",          path: "/lb/v1/zones/{zone}/backends",               listKey: "backends"},
        {name: "LBPrivateNetworks", setup: setupLB, path: "/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks", listKey: "private_network"},
        {name: "Clusters",          path: "/k8s/v1/regions/{region}/clusters",           listKey: "clusters"},
        {name: "IAMApplications",   path: "/iam/v1alpha1/applications",                  listKey: "applications"},
        {name: "IAMAPIKeys",        path: "/iam/v1alpha1/api-keys",                      listKey: "api_keys"},
        {name: "IAMPolicies",       path: "/iam/v1alpha1/policies",                      listKey: "policies"},
        {name: "IAMSSHKeys",        path: "/iam/v1alpha1/ssh-keys",                      listKey: "ssh_keys"},
        // ... remaining list endpoints (pools, rdb databases/users require parent setup)
    }

    for _, tt := range cases {
        t.Run(tt.name, func(t *testing.T) {
            ts, cleanup := testutil.NewTestServer(t)
            defer cleanup()

            ctx := map[string]string{"zone": "fr-par-1", "region": "fr-par"}
            if tt.setup != nil {
                tt.setup(t, ts, ctx)
            }

            status, body := testutil.DoList(t, ts, pathWithCtx(tt.path, ctx))
            require.Equal(t, 200, status)

            _, hasKey := body[tt.listKey]
            require.True(t, hasKey, "expected list key %q in response", tt.listKey)
            require.IsType(t, []any{}, body[tt.listKey])

            _, hasCount := body["total_count"]
            require.True(t, hasCount, "expected total_count in response")
        })
    }
}
```

**What these tests catch**:
- Missing wrapper key on Instance Create/Get → test fails with `"expected wrapper key \"server\""`
- Accidental wrapper on non-Instance Create/Get → test fails because `raw["id"]` is nil
- Wrong list key name (e.g., `"private_networks"` instead of `"private_network"`) → test fails with `"expected list key \"private_network\""`
- Missing `total_count` on any List response

These tests use the same `pathWithCtx`, `bodyWithCtx`, and setup helpers as the existing table-driven lifecycle tests.

### What NOT to test

- **Handler methods in isolation**: handlers are thin wrappers — integration tests cover them fully.
- **Mock interfaces for the repository**: use real SQLite in-memory — it's fast enough and catches real bugs.
- **Scaleway API field validation**: Mockway doesn't validate request fields beyond FK references. Don't test for "invalid commercial_type" — that's not Mockway's job.

### Test file placement

Test files live alongside their source (Go convention):

```
handlers/
├── admin.go
├── handlers.go
├── iam.go
├── instance.go
├── k8s.go
├── lb.go
├── rdb.go
├── vpc.go
└── handlers_test.go            # Integration tests (HTTP round-trips)
repository/
├── repository.go
└── repository_test.go          # Unit tests (repository layer)
testutil/
└── testutil.go                 # Shared test helpers (NewTestServer, DoCreate, etc.)
```

## API Response Format

Scaleway APIs return responses wrapped in a consistent format. Follow these patterns:

**HTTP Status Codes**:

| Operation | Success Code | Body |
|-----------|-------------|------|
| Create | `200 OK` | Created resource JSON (Instance: wrapped in key; others: flat — see Response Wrapping) |
| Get | `200 OK` | Resource JSON (Instance: wrapped in key; others: flat — see Response Wrapping) |
| List | `200 OK` | `{"<plural_key>": [...], "total_count": N}` (e.g., `{"servers": [...], "total_count": 2}`) |
| Delete | `204 No Content` | Empty |
| `POST /mock/reset` | `204 No Content` | Empty |
| `GET /mock/state` | `200 OK` | Full state JSON |
| `GET /mock/state/{service}` | `200 OK` | Single service state JSON (valid: `instance`, `vpc`, `lb`, `k8s`, `rdb`, `iam`; unknown → 404) |

**Error codes**:

| Condition | Status | Body |
|-----------|--------|------|
| Missing/empty `X-Auth-Token` | `401 Unauthorized` | `{"message": "missing or empty X-Auth-Token", "type": "denied_authentication"}` |
| Resource not found (Get/Delete) | `404 Not Found` | `{"message": "resource not found", "type": "not_found"}` |
| Referenced resource not found (Create) | `404 Not Found` | `{"message": "referenced resource not found", "type": "not_found"}` |
| Dependents exist (Delete) | `409 Conflict` | `{"message": "cannot delete: dependents exist", "type": "conflict"}` |
| Duplicate composite key (Create) | `409 Conflict` | `{"message": "resource already exists", "type": "conflict"}` |

### Response Wrapping

The Scaleway API has **two different patterns** for Create/Get responses depending on the service:

1. **Instance API only**: wraps single-object responses in a key — e.g., `{"server": {"id": "...", ...}}`
2. **All other APIs** (VPC, LB, K8s, RDB, IAM): returns the object directly at the top level — e.g., `{"id": "...", "name": "...", ...}`

**List responses always use a wrapper** with a plural key and `total_count`, across all APIs.

The Scaleway provider dereferences the wrapper key and will panic if it's missing. Mockway **must** match the real API's wrapping behavior exactly.

**Wrapping reference**:

| Resource Type | Singular Key (Create/Get) | Plural Key (List) |
|---|---|---|
| Instance Server | `"server"` | `"servers"` |
| Instance IP | `"ip"` | `"ips"` |
| Instance Security Group | `"security_group"` | `"security_groups"` |
| Instance Private NIC | `"private_nic"` | `"private_nics"` |
| VPC | _(flat — no wrapper)_ | `"vpcs"` |
| Private Network | _(flat)_ | `"private_networks"` |
| Load Balancer | _(flat)_ | `"lbs"` |
| LB Frontend | _(flat)_ | `"frontends"` |
| LB Backend | _(flat)_ | `"backends"` |
| LB Private Network | _(flat)_ | `"private_network"` **(singular!)** |
| K8s Cluster | _(flat)_ | `"clusters"` |
| K8s Pool | _(flat)_ | `"pools"` |
| RDB Instance | _(flat)_ | `"instances"` |
| RDB Database | _(flat)_ | `"databases"` |
| RDB User | _(flat)_ | `"users"` |
| IAM Application | _(flat)_ | `"applications"` |
| IAM API Key | _(flat)_ | `"api_keys"` |
| IAM Policy | _(flat)_ | `"policies"` |
| IAM SSH Key | _(flat)_ | `"ssh_keys"` |

**Instance API example** (Create/Get):
```json
{"server": {"id": "uuid", "name": "web-1", "state": "running", ...}}
```

**All other APIs example** (Create/Get):
```json
{"id": "uuid", "name": "main", "region": "fr-par", ...}
```

**List example** (all APIs):
```json
{"servers": [...], "total_count": 2}
```

**LB Private Network list quirk**: uses the **singular** key `"private_network"` (not `"private_networks"`) for the array field. This is a Scaleway API inconsistency — follow it exactly.

**Implementation note**: the `writeJSON` helper returns flat objects. For Instance resources, handlers must wrap before calling `writeJSON`:
```go
// Instance handlers — wrap in key
writeJSON(w, http.StatusOK, map[string]any{"server": out})

// All other handlers — return flat
writeJSON(w, http.StatusOK, out)
```

**Checklist for adding new endpoints** (prevents flat-vs-wrapped bugs):

1. Check the wrapping reference table above — is this an Instance API resource?
2. If **Instance**: wrap Create/Get responses in the singular key (e.g., `{"server": out}`). Sub-resource endpoints (e.g., `/rules`) return their own shape, not the parent wrapper.
3. If **any other API**: return the object flat (`out` directly).
4. For **List** responses on any API: always use `writeList(w, "<key>", items)` with the exact plural key from the table.
5. In tests: Instance Create/Get responses must be unwrapped (via `unwrapInstanceResource` or direct key access like `body["server"].(map[string]any)`). Non-Instance responses are accessed flat (`body["id"]`).
6. For **static catalog endpoints** (e.g., `/products/servers`): include every field the provider reads. Missing nested fields default to zero values in Go, which causes silent validation failures (e.g., `volumes_constraint.max_size` omitted → `max_size=0` → "must be between 10 GB and 0 B"). Check the provider source for which fields it dereferences.
7. When in doubt: check how the real Scaleway provider deserializes the response — it uses typed Go structs with `json:"server"` tags. If the JSON shape doesn't match the struct, the provider panics.

**Pagination**: v1 ignores `page`/`per_page` query parameters — always return all results in a single page. The OpenTofu/Terraform provider handles this correctly for small datasets (InfraFactory scenarios have ~10-20 resources).

Use UUIDs for all resource IDs (generate with `github.com/google/uuid`), except RDB databases/users (identified by name) and IAM API keys (identified by server-generated `access_key`).

## Pending Fixes

1. **Products/servers `per_volume_constraint`**: `volumes_constraint` (with `min_size` and `max_size`) is already implemented. What's still missing: each server type entry also needs a `per_volume_constraint` field with an `l_ssd` sub-object containing `min_size` and `max_size`. Without this, the provider can't determine that the server type supports local SSD volumes — the root volume gets typed as block SSD, the local volume total becomes 0, and validation fails against `volumes_constraint.min_size`. Example for DEV1-S:
   ```json
   "per_volume_constraint": {
     "l_ssd": {"min_size": 1000000000, "max_size": 20000000000}
   }
   ```
   Tests required:
   - Each server type entry contains `per_volume_constraint` with `l_ssd` sub-object
   - `l_ssd` has both `min_size` and `max_size`

## Known Limitations

- **IAM rules stub**: `ListIAMRules` always returns an empty list regardless of `policy_id` — Mockway doesn't model individual rules. If the provider sends an invalid `policy_id`, it still gets 200 instead of 404. Acceptable for v1.

- **No state persistence across runs**: `tofu plan` / `terraform plan` against Mockway always shows all resources as "to be created" because Mockway starts with empty state (`:memory:` default). This is expected — each run is a clean environment. Use `--db ./mockway.db` for file-backed persistence if needed between runs.
- **Plan does not strictly require Mockway**: For all-new resources (no existing state, no data sources), the Scaleway provider computes the plan locally without making API calls. `tofu plan` / `terraform plan` works with just dummy env vars (`SCW_API_URL=http://localhost:1`) and no running server. However, Mockway supports both plan and apply — point `SCW_API_URL` at Mockway for the full workflow (`tofu plan` → `tofu apply` / `terraform plan` → `terraform apply`).

## Distribution

GoReleaser → Go binaries + Docker image + Homebrew tap.

- Docker: `ghcr.io/redscaresu/mockway:latest`
- Homebrew: `brew install redscaresu/tap/mockway`
- Binary: `go install github.com/redscaresu/mockway/cmd/mockway@latest`

## Code Guidelines

- All Go, use stdlib where possible
- Top-level packages (`handlers/`, `repository/`, `models/`) — no `internal/`, following [simpleAPI](https://github.com/redscaresu/simpleAPI) layout
- DI via `Application` struct: repository injected into handlers via `NewApplication(repo)`
- Routes registered via `app.RegisterRoutes(r)` on chi router
- Handlers are thin — delegate to repository for state management
- Repository methods return domain errors (`ErrNotFound`, `ErrConflict` from `models/`), handlers map to HTTP status codes
- Use `database/sql` with `modernc.org/sqlite` (pure Go — no CGo, simpler cross-compilation via GoReleaser)
- `repository.New()` must call `db.SetMaxOpenConns(1)` and `PRAGMA foreign_keys = ON` immediately after `sql.Open()` (see SQLite connection strategy above)
- Generate UUIDs for all resource IDs on create (except RDB databases/users — identified by name, and IAM API keys — identified by server-generated `access_key`)
- Return the created resource in the response body (matching Scaleway's behavior)
