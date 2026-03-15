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
| `working/basic_instance` | Server with a security group; `security_group_id` uses a resource reference, not a hard-coded UUID |
| `working/vpc_and_private_network` | Full dependency chain: VPC → private network → server → private NIC; all FK parents created before their children |

### misconfigured

Configs with real mistakes that `terraform validate` and `terraform plan` do not catch. Each one only fails when `terraform apply` (or a targeted destroy) calls the API.

| Example | The mistake | What mockway returns |
|---|---|---|
| `misconfigured/hardcoded_security_group_id` | `security_group_id` is a literal UUID copied from another environment — it does not exist | `404 referenced resource not found` at server create |
| `misconfigured/nic_with_missing_private_network` | `private_network_id` comes from a variable with a stale or wrong UUID — the private network was never created | `404 referenced resource not found` at NIC create, after the server has already been created |
| `misconfigured/vpc_deleted_before_private_network` | VPC is destroyed while a private network still references it — run `terraform destroy -target scaleway_vpc.vpc` to trigger it | `409 cannot delete: dependents exist` |

---

## Why `terraform validate` and `terraform plan` miss these

`terraform validate` checks syntax and type constraints. It cannot know whether a UUID exists in a remote API.

`terraform plan` shows what will be created or destroyed. It does not call create/delete endpoints, so FK violations never surface.

`terraform test` with a provider mock could catch some cases, but only if the mock is configured to enforce the same constraints as the real API — which requires the same level of work as mockway itself.

mockway runs the full apply/destroy cycle locally, with no cloud credentials, and enforces the same resource dependency rules as Scaleway. Broken configs fail fast, before CI runs and before any real infrastructure is touched.
