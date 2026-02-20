# mockway
Stateful local mock of the Scaleway API for offline OpenTofu and Terraform testing.

Mockway runs as a single Go binary, persists resource state in SQLite, and exposes Scaleway-like API routes on one port.

## Features
- Single-port HTTP API with path-based service routing
- Stateful resource lifecycle (create/get/list/delete)
- SQLite-backed state (`:memory:` by default, file DB optional)
- Foreign-key integrity checks (404 on bad references, 409 on dependent deletes)
- Admin inspection/reset API under `/mock/*`
- Marketplace image label resolution (e.g., `ubuntu_noble` → UUID)
- Catch-all 501 handler logs unimplemented routes for easy discovery
- Echo mode for provider path discovery

## Install
```bash
go install github.com/redscaresu/mockway/cmd/mockway@latest
```

## Run
Stateful mock mode:
```bash
mockway --port 8080 --db :memory:
```

File-backed DB:
```bash
mockway --port 8080 --db ./mockway.db
```

## Echo Smoke Mode
Use this mode to discover exactly which routes the Scaleway provider calls.

```bash
mockway --port 8080 --echo
```

Echo mode logs request method/path/headers and replies with:
```json
{"ok": true}
```

Recommended provider env vars for local testing:
```bash
export SCW_API_URL=http://localhost:8080
export SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX
export SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000
export SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
```

Then run either:
```bash
tofu plan
```
or:
```bash
terraform plan
```

Typical workflow for either CLI:
```bash
# OpenTofu
tofu init
tofu plan

# Terraform
terraform init
terraform plan
```

## Auth
- Scaleway routes require `X-Auth-Token` with any non-empty value.
- Admin routes (`/mock/*`) do not require auth.

Missing auth response:
```json
{"message":"missing or empty X-Auth-Token","type":"denied_authentication"}
```

## Services and Routes
Implemented services:
- Instance (`/instance/v1/zones/{zone}`) — servers, IPs, security groups, private NICs, products catalog
- VPC (`/vpc/v1/regions/{region}`) — VPCs, private networks
- Load Balancer (`/lb/v1/zones/{zone}`) — LBs, frontends, backends, private network attachments
- Kubernetes (`/k8s/v1/regions/{region}`) — clusters, pools
- RDB (`/rdb/v1/regions/{region}`) — instances, databases, users
- IAM (`/iam/v1alpha1/`) — applications, API keys, policies, SSH keys
- Marketplace (`/marketplace/v2/`) — local image label resolution (e.g., `ubuntu_noble` → image UUID)
- Account (`/account/v2alpha1/`) — SSH keys (legacy alias → IAM)

Each resource supports Create/Get/List/Delete, except:
- RDB databases/users: Create/List/Delete (no Get)
- LB private-network attachment: Attach/List/Detach
- Security groups: also Patch (update), PUT/GET rules
- IAM rules: list only (stub, returns empty)
- Marketplace: list and get local images (static catalog)
- Instance products/servers: list only (static catalog)

## Response Conventions
Success:
- Create/Get/List: `200`
- Delete: `204`
- `POST /mock/reset`: `204`

List payload shape:
```json
{"<plural_key>":[...],"total_count":N}
```

Error types:
- `404 not_found` with `resource not found` (missing target on get/delete)
- `404 not_found` with `referenced resource not found` (bad FK on create)
- `409 conflict` with `cannot delete: dependents exist`
- `409 conflict` with `resource already exists`

## Admin API
```text
POST /mock/reset
GET  /mock/state
GET  /mock/state/{service}
```

`{service}` supports: `instance`, `vpc`, `lb`, `k8s`, `rdb`, `iam`.

## Quick Example
```bash
# Create VPC
curl -s -X POST \
  -H 'X-Auth-Token: test' \
  -H 'Content-Type: application/json' \
  http://localhost:8080/vpc/v1/regions/fr-par/vpcs \
  -d '{"name":"main"}'

# Inspect full state
curl -s http://localhost:8080/mock/state | jq .
```

## Development
```bash
go test ./...
```

Key packages:
- `cmd/mockway` - binary entrypoint
- `handlers` - HTTP routes and error mapping
- `repository` - SQLite schema + CRUD/state logic
- `models` - domain errors
- `testutil` - shared integration test helpers
