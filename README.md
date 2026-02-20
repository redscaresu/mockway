# mockway

Stateful local mock of the Scaleway API for offline OpenTofu and Terraform testing.

Mockway runs as a single Go binary, persists resource state in SQLite, and exposes Scaleway-like API routes on one port.

> **Status: Work in progress.** Mockway implements CRUD for the most common Scaleway resources but does not cover the full API surface. See [What's Supported](#whats-supported) and [Known Limitations](#known-limitations) below.

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

## Usage with OpenTofu / Terraform

Point the Scaleway provider at Mockway:

```bash
export SCW_API_URL=http://localhost:8080
export SCW_ACCESS_KEY=SCWXXXXXXXXXXXXXXXXX
export SCW_SECRET_KEY=00000000-0000-0000-0000-000000000000
export SCW_DEFAULT_PROJECT_ID=00000000-0000-0000-0000-000000000000
```

Then run `tofu plan && tofu apply` or `terraform plan && terraform apply` as normal.

## What's Supported

### Implemented Services

| Service | Path Prefix | Resources | Operations |
|---------|-------------|-----------|------------|
| Instance | `/instance/v1/zones/{zone}/` | servers, IPs, security groups, private NICs, volumes | CRUD + server actions, user_data stubs, products catalog |
| VPC | `/vpc/v1/regions/{region}/` | VPCs, private networks | CRUD |
| Load Balancer | `/lb/v1/zones/{zone}/` | LBs, frontends, backends, private network attachments | CRUD |
| Kubernetes | `/k8s/v1/regions/{region}/` | clusters, pools | CRUD |
| RDB | `/rdb/v1/regions/{region}/` | instances, databases, users | CRUD (databases/users: no individual Get) |
| IAM | `/iam/v1alpha1/` | applications, API keys, policies, SSH keys | CRUD + rules list stub |
| Marketplace | `/marketplace/v2/` | local images | List + Get (image label → UUID resolution) |
| Account | `/account/v2alpha1/` | SSH keys | Legacy alias → IAM SSH keys |

### Features

- Single-port HTTP API with path-based service routing
- Stateful resource lifecycle (create, get, list, delete)
- SQLite-backed state (`:memory:` by default, file DB optional)
- Foreign-key integrity (404 on bad references, 409 on dependent deletes)
- Cascade semantics matching real Scaleway (IP detaches on server delete, NICs cascade-delete)
- Admin API under `/mock/*` for state inspection and reset
- Marketplace image label resolution (e.g., `ubuntu_noble` → zone-specific UUID)
- Instance products/servers catalog (provider uses this for client-side validation)
- Server action endpoint (poweroff/terminate — returns completed task)
- Catch-all 501 handler logs unimplemented routes for easy discovery
- Auth: `X-Auth-Token` required on Scaleway routes (any non-empty value accepted)

### Verified End-to-End

The following resource combination has been tested through a full `terraform apply` + `terraform destroy` cycle:

- `random_id`
- `scaleway_account_ssh_key`
- `scaleway_iam_application`
- `scaleway_iam_api_key`
- `scaleway_iam_policy`
- `scaleway_instance_ip`
- `scaleway_instance_security_group` (with inbound rules)
- `scaleway_instance_server` (with image label, security group, reserved IP, cloud-init user_data)

## Known Limitations

- **Not a full Scaleway API mock.** Only CRUD operations are implemented. Update/patch operations are limited (security groups only). Many API features (snapshots, placement groups, DNS, object storage/S3, block storage, serverless, etc.) are not implemented.
- **No field validation.** Mockway accepts whatever JSON you send and stores it. It does not validate `commercial_type`, `node_type`, required fields, or value constraints beyond foreign key references.
- **No pagination.** All list endpoints return all results in a single page. `page`/`per_page` query parameters are ignored.
- **No S3 / Object Storage.** S3-compatible endpoints are not implemented.
- **IAM rules are a stub.** `GET /iam/v1alpha1/rules` always returns an empty list regardless of policy.
- **User data is discarded.** `PATCH /servers/{id}/user_data/{key}` accepts the body but does not store it. `GET /servers/{id}/user_data` always returns an empty list.
- **No state persistence by default.** Using `:memory:` (the default), state is lost on exit. Use `--db ./mockway.db` for persistence.
- **Unimplemented routes return 501.** Any route not explicitly handled returns `501 Not Implemented` with a log line — useful for discovering which endpoints your Terraform config needs.

## Admin API

```
POST /mock/reset          — wipe all state
GET  /mock/state          — full resource graph as JSON
GET  /mock/state/{service} — single service (instance, vpc, lb, k8s, rdb, iam)
```

## Quick Example

```bash
# Start mockway
mockway --port 8080 &

# Create a VPC
curl -s -X POST \
  -H 'X-Auth-Token: test' \
  -H 'Content-Type: application/json' \
  http://localhost:8080/vpc/v1/regions/fr-par/vpcs \
  -d '{"name":"main"}'

# Inspect full state
curl -s http://localhost:8080/mock/state | jq .

# Reset all state
curl -s -X POST http://localhost:8080/mock/reset
```

## Development

```bash
go test ./...
```

Key packages:

- `cmd/mockway` — binary entrypoint
- `handlers` — HTTP routes and error mapping
- `repository` — SQLite schema + CRUD/state logic
- `models` — domain errors
- `testutil` — shared integration test helpers
