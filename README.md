# mockway

Local mock of the Scaleway API for offline OpenTofu and Terraform testing.

Mockway runs as a single Go binary, tracks resource state in SQLite, and exposes Scaleway-like API routes on one port. State is kept in-memory by default — each run starts clean, which is ideal for test cycles. Use `--db ./mockway.db` if you need state to survive restarts.

> **This project is in early development.** Only a subset of Scaleway services have been tested against the real Terraform provider. Other services have handler code but will likely need further work to pass a full `terraform apply` + `terraform destroy` cycle. See [Tested Services](#tested-services) and [Untested Services](#untested-services) below.

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

## Tested Services

The following services have been tested through a full `terraform apply` + `terraform destroy` cycle against the real Scaleway Terraform provider:

- **Instance** (`/instance/v1/zones/{zone}/`) — servers, IPs, security groups (with rules), private NICs, volumes, server actions, user_data stubs, products catalog
- **IAM** (`/iam/v1alpha1/`) — applications, API keys, policies, SSH keys
- **Marketplace** (`/marketplace/v2/`) — image label → zone-specific UUID resolution

The specific Terraform resources verified end-to-end:

- `scaleway_account_ssh_key`
- `scaleway_iam_application`
- `scaleway_iam_api_key`
- `scaleway_iam_policy`
- `scaleway_instance_ip`
- `scaleway_instance_security_group` (with inbound rules)
- `scaleway_instance_server` (with image label, security group, reserved IP, cloud-init user_data)

## Untested Services

The following services have CRUD handler code and pass integration tests at the HTTP level, but have **not** been tested against the real Terraform provider. Based on the experience with Instance (which required 10 fixes for response shapes, FK cascades, and provider quirks), these will almost certainly need further development to work with `terraform apply`:

- **VPC** (`/vpc/v1/regions/{region}/`) — VPCs, private networks
- **Load Balancer** (`/lb/v1/zones/{zone}/`) — LBs, frontends, backends, private network attachments
- **Kubernetes** (`/k8s/v1/regions/{region}/`) — clusters, pools
- **RDB** (`/rdb/v1/regions/{region}/`) — managed database instances, databases, users
- **Account** (`/account/v2alpha1/`) — SSH keys (legacy alias → IAM)

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

- **Not a full Scaleway API mock.** Only CRUD operations are implemented. Update/patch operations are limited (security groups only). Many API features (snapshots, placement groups, DNS, object storage/S3, block storage, serverless, etc.) are not implemented.
- **No field validation.** Mockway accepts whatever JSON you send and stores it. It does not validate `commercial_type`, `node_type`, required fields, or value constraints beyond foreign key references.
- **No pagination.** All list endpoints return all results in a single page. `page`/`per_page` query parameters are ignored.
- **No S3 / Object Storage.** S3-compatible endpoints are not implemented. Scaleway's Object Storage uses the S3 protocol (AWS SigV4 auth, XML responses) which is a different problem from the JSON REST API.
- **IAM rules are a stub.** `GET /iam/v1alpha1/rules` always returns an empty list regardless of policy.
- **User data is discarded.** `PATCH /servers/{id}/user_data/{key}` accepts the body but does not store it. `GET /servers/{id}/user_data` always returns an empty list.
- **Unimplemented routes return 501.** Any route not explicitly handled returns `501 Not Implemented` with a log line — useful for discovering which endpoints your Terraform config needs.

## Admin API

```
POST /mock/reset          — wipe all state
GET  /mock/state          — full resource graph as JSON
GET  /mock/state/{service} — single service (instance, vpc, lb, k8s, rdb, iam)
```

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

Key packages:

- `cmd/mockway` — binary entrypoint
- `handlers` — HTTP routes and error mapping
- `repository` — SQLite schema + CRUD/state logic
- `models` — domain errors
- `testutil` — shared integration test helpers
