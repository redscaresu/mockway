# BACKLOG

## Next up (work in this order)

1. **MW-28** — Fix `vpc_gateway_network` `enable_masquerade` drift

Provider-compatibility work for mockway.

Legend: `todo` | `in_progress` | `blocked` | `done`

## The core bug pattern

The security-group drift fix is the template for this class of bug: the Terraform provider sends data in one shape on create, mockway stores it, then returns it in a different shape on GET — causing perpetual plan drift. The reliable detection method is the **double-apply idempotency check**.

**Detection steps per service:**
1. Write a minimal working Terraform config for the service
2. `terraform apply` — catches missing endpoints (501), wrong response shapes (provider errors), FK violations (404/409)
3. `terraform apply` again with no config changes — catches drift (non-empty plan = GET response shape doesn't match what the provider stored on create)
4. `terraform destroy` — catches missing delete endpoints and cascade failures
5. Each bug found → fix the handler response shape + add a Go regression test asserting the correct shape

**What counts as a valid fix:** The provider sends a specific JSON body; mockway stores it and must return it in the shape the provider expects on the next GET. Fixes must not mask real FK violations or bypass the provider's expected semantics. The Terraform provider SDK is the contract.

---

## Tickets

| id | title | priority | status | deps |
|---|---|---|---|---|
| MW-12 | Update existing `TestProviderApplyDestroySmoke`: add second apply no-op check (`-detailed-exitcode`) and add `inbound_rule` block to the security group HCL to exercise the drift fix | P1 | done | — |
| MW-11 | Expand `e2e/provider_smoke_test.go`: add `TestProvider*` per untested service (IAM full, LB, K8s, RDB, Redis, Registry) each with double-apply idempotency check — second apply must produce a no-op plan | P1 | done | MW-12 |
| MW-8 | Fix response shape bugs surfaced by MW-11; add Go regression test per bug found | P1 | done | MW-11 |
| MW-2 | Working HCL example: `examples/working/iam_full` (application + api-key + policy + ssh-key) | P2 | done | MW-11 |
| MW-3 | Working HCL example: `examples/working/load_balancer` (LB + frontend + backend) | P2 | done | MW-11 |
| MW-4 | Working HCL example: `examples/working/kubernetes_cluster` (cluster + node pool) | P2 | done | MW-11 |
| MW-5 | Working HCL example: `examples/working/rdb_instance` (instance + database + user + ACL) | P2 | done | MW-11 |
| MW-6 | Working HCL example: `examples/working/redis_cluster` | P3 | done | MW-11 |
| MW-7 | Working HCL example: `examples/working/registry_namespace` | P3 | done | MW-11 |
| MW-13 | Replace README "Untested Services" section with a compatibility matrix updated as services pass MW-11 | P2 | done | MW-11 |
| MW-14 | Investigate whether security group rules need server-assigned IDs — real Scaleway API assigns an `id` to each rule; mockway stores rules without IDs; if provider uses rule IDs to match state this is a latent drift bug | P2 | done | MW-12 |
| MW-17 | Implement missing UPDATE endpoints surfaced by scripted spec diff — 12 high-priority missing PATCH/PUT handlers that the Terraform provider calls when a resource attribute changes in-place (see MW-17 detail below) | P1 | done | MW-16 |
| MW-16 | Spec audit: compare every mockway-implemented endpoint against the downloaded Scaleway OpenAPI specs in `specs/`; for each endpoint check response shape, cascade behaviour on DELETE, required fields, and correctness of FK semantics | P1 | done | MW-15 |
| MW-15 | Systematic FK audit: map every parent-child reference across all services, write one negative e2e test per relationship (bad UUID → assert 404), fix every handler gap found | P1 | done | MW-11 |
| MW-9 | Misconfigured examples: FK violations for LB, K8s, RDB | P3 | done | MW-3, MW-4, MW-5 |
| MW-18 | K8s cluster upgrade/set-type/GetVersion, pool upgrade (provider calls these on version change) — K8s ACLs still todo | P2 | done | — |
| MW-19 | Marketplace: proper label→UUID map for all image labels (current stub always returns the same UUID regardless of label) | P2 | done | — |
| MW-20 | VPC gateway networks and routes (`scaleway_vpc_gateway_network`, `scaleway_vpc_route`, `scaleway_vpc_public_gateway`) | P2 | done | — |
| MW-21 | Domain DNS zone CRUD (CreateDNSZone, UpdateDNSZone, DeleteDNSZone) | P3 | done | — |
| MW-25 | Extract null-safe deep-merge helper (`patchMerge`) and use it in all ~30 Update functions — eliminates the class of bugs where providers send `null` for optional fields and nested-object patches wipe stored sub-fields | P1 | done | — |
| MW-26 | Add `examples/updates/` CI suite — auto-discovers update scenarios (apply v1 → apply v2 → no-op plan check → destroy); catches update-field-name divergences and null-overwrite bugs before they reach prod | P1 | done | MW-25 |
| MW-27 | Document proxy-capture workflow in AGENTS.md — when implementing or debugging a resource, run tofu against the real Scaleway API through the logging proxy to capture exact PATCH/POST bodies before implementing; prevents create-vs-update field name divergences like `disable_backup` vs `is_backup_schedule_disabled` | P2 | done | — |
| MW-23 | Delete redundant individual `TestProvider*` e2e tests — all 12 are now covered by `TestExamplesWorkingIdempotency`; keeping them doubles e2e runtime for no added coverage | P2 | done | — |
| MW-24 | Simplify CI retry loop — was working around parallel provider-cache race; race fixed by removing `t.Parallel()` from `TestExamplesWorkingIdempotency` | P2 | done | — |
| MW-22 | Instance standalone volumes (`scaleway_instance_volume`) — POST/GET/LIST/PATCH/DELETE `/volumes` | P2 | done | — |
| MW-10 | Document `--echo` mode in README | P3 | done | — |
| MW-1 | Shell script idempotency harness over `examples/` dirs (manual debugging aid only — MW-11 covers CI) | P3 | done | MW-2 |
| MW-28 | Fix `vpc_gateway_network` `enable_masquerade` drift — proxy-capture the real API response to find the correct field shape | P1 | todo | — |
| MW-29 | Fix `scaleway_lb_ip` + `scaleway_lb` `ip_id` drift — explicit LB IP causes `ip_id` mismatch on second plan | P1 | done | — |
| MW-30 | K8s version resolution on auto-upgrade toggle — not a bug; matches real Scaleway API behavior (minor version required with auto-upgrade, patch version without) | P2 | wontfix | — |
| MW-31 | Full e2e coverage: add missing working examples + update examples for all supported resources (see detail below) | P1 | done | — |

## MW-31 Full E2E Coverage

**Goal**: every supported resource has a working example (apply → no-op plan → destroy) and every resource with an Update handler has an update example (apply v1 → no-op → apply v2 → no-op → destroy). Any bug found → fix the handler + add a regression test.

**Missing working examples (4 resources, skip gateway network until MW-28 is fixed):**

| Resource | Example name | Notes |
|----------|-------------|-------|
| `scaleway_lb_certificate` | `lb_with_certificate` | Needs LB + frontend + cert |
| `scaleway_rdb_acl` | `rdb_with_acl` | Needs RDB instance + ACL rules |
| `scaleway_iam_user` | `iam_user_group` | User + group + group membership |
| `scaleway_iam_group` | (same as above) | Covered by `iam_user_group` |

**Missing update examples (prioritised by risk — nested objects most likely to have patchMerge bugs):**

| Resource | Example name | What to change v1→v2 |
|----------|-------------|---------------------|
| `scaleway_rdb_instance` | `update_rdb_instance` | `backup_schedule_disabled`, `name` |
| `scaleway_k8s_cluster` | `update_k8s_cluster` | `name`, `tags` (avoid version/auto_upgrade — MW-30) |
| `scaleway_vpc` | `update_vpc` | `name`, `tags` |
| `scaleway_vpc_private_network` | `update_private_network` | `name`, `tags` |
| `scaleway_iam_application` | `update_iam_application` | `name`, `description` |
| `scaleway_iam_policy` | `update_iam_policy` | `name`, `description` |
| `scaleway_iam_ssh_key` | `update_iam_ssh_key` | `name` |
| `scaleway_redis_cluster` | `update_redis_cluster` | `name`, `tags` |
| `scaleway_registry_namespace` | `update_registry_namespace` | `description`, `is_public` |
| `scaleway_instance_security_group` | `update_security_group` | `name`, `inbound_default_policy` |
| `scaleway_instance_ip` | `update_instance_ip` | `tags` |
| `scaleway_block_volume` | `update_block_volume` | `name`, `tags` |
| `scaleway_domain_zone` | `update_domain_zone` | `project_id` |
| `scaleway_vpc_route` | `update_vpc_route` | `description`, `tags` |
| `scaleway_lb` | `update_lb` | `name`, `description` |

**Execution**: for each example, write the HCL, run the idempotency check, fix any bugs found, add a handler regression test per bug. Each example auto-registers in CI via `TestExamplesWorkingIdempotency` / `TestExamplesUpdatesIdempotency`.

---

## MW-17 Missing UPDATE Endpoints

Identified by scripted spec diff (all 361 spec operations cross-referenced against 150 registered routes). 242 total gaps; filtered to those the Terraform provider calls during normal apply/destroy.

**High priority — provider calls these on in-place resource updates (config change → terraform apply):**

| Endpoint | Needed for |
|----------|-----------|
| `PATCH /instance/v1/zones/{zone}/servers/{server_id}` | Renaming server, changing type, updating security group |
| `PATCH /instance/v1/zones/{zone}/ips/{ip}` | Reassigning IP to a different server |
| `GET /instance/v1/zones/{zone}/servers/{server_id}/user_data/{key}` | Provider reads specific user-data key on refresh |
| `PATCH /vpc/v2/regions/{region}/vpcs/{vpc_id}` | Renaming VPC |
| `PATCH /vpc/v2/regions/{region}/private-networks/{private_network_id}` | Renaming private network |
| `PATCH /lb/v1/zones/{zone}/ips/{ip_id}` | Updating LB IP attributes |
| `PATCH /iam/v1alpha1/applications/{application_id}` | Updating IAM application name/description |
| `PATCH /iam/v1alpha1/api-keys/{access_key}` | Updating API key description/expiry |
| `PATCH /iam/v1alpha1/policies/{policy_id}` | Updating policy name/application binding |
| `PATCH /iam/v1alpha1/ssh-keys/{ssh_key_id}` | Updating SSH key name |
| `PUT /iam/v1alpha1/rules` | Replacing all rules for a policy (used on policy update) |
| `GET /k8s/v1/regions/{region}/nodes/{node_id}` | Provider reads node IPs after pool creation |

**Medium priority — needed for less common workflows:**
- `PATCH /instance/.../private_nics/{id}` — update NIC
- `POST /rdb/.../instances/{id}/endpoints` + `DELETE /rdb/.../endpoints/{id}` — add/remove private network endpoint to RDB instance
- `POST /k8s/.../clusters/{id}/upgrade`, `POST /k8s/.../pools/{id}/upgrade` — Kubernetes version upgrades

**Detection method**: `python3 scripts/spec_diff.py` (script to be added) — re-run after adding any new handler to catch regressions.

**Implementation pattern for PATCH UPDATE handlers**: read current record → merge patch fields → store → return updated record. Same as existing `UpdateCluster`, `UpdatePool`, `UpdateRDBInstance` patterns. For IAM resources the `updated_at` timestamp should be bumped.

---

## MW-16 Spec Audit Results

Fixes applied from audit of 9 Scaleway OpenAPI specs against all mockway handlers:

**DELETE response codes** (fixed): The Scaleway spec requires `200 + body` for DELETE on RDB instances, Redis clusters, and Registry namespaces (resources that may take time to delete asynchronously). Mockway was returning `204 no body`. Fixed in `handlers/rdb.go`, `handlers/redis.go`, `handlers/registry.go`.

**LB UpdateLB method** (fixed): The spec specifies `PUT /lbs/{lb_id}` but mockway had `PATCH`. Fixed in `handlers/handlers.go` and updated tests.

**LB detach private network** (fixed): The spec provides `POST /lbs/{lb_id}/detach-private-network` (body: `{private_network_id}`). Mockway only had `DELETE /lbs/{lb_id}/private-networks/{pn_id}`. Added `DetachLBPrivateNetwork` handler in `handlers/lb.go`.

**CASCADE behaviour confirmed correct**: RDB databases/users and K8s pools cascade-delete per spec language. No changes required.

**Confirmed correct per spec**: Instance server DELETE 204, Security group DELETE 204, VPC/PN DELETE 204, LB DELETE 204, Frontend/Backend DELETE 204, IAM DELETE 204.

---

## MW-14 Investigation Result

**Security group rules do NOT need server-assigned IDs for idempotency.** The Scaleway Terraform provider compares rules by content (action, protocol, port, direction) rather than by ID. The `TestProviderApplyDestroySmoke` test with an `inbound_rule` block produces a no-op second plan, confirming the current behavior is correct. The real API does assign IDs to rules, but the provider ignores them for state matching. No changes required.

## Bug fixes from MW-11

**IAM policy rules drift** (fixed): The provider sends `rules` inline in `POST /iam/v1alpha1/policies` body, then reads them back via `GET /iam/v1alpha1/rules?policy_id=xxx`. The `ListIAMRules` stub always returned empty — fixed by storing inline rules in the `iam_rules` table on policy create and returning them via the rules endpoint.

**RDB `disable_backup` drift** (fixed): The provider sends `disable_backup: true` as a flat field; the real API stores it as `backup_schedule.disabled`. Mockway now translates `disable_backup` → `backup_schedule.disabled` on create so GET returns the shape the provider reads.

**Execution order for a fresh context**: start with MW-12 (update the existing smoke test — familiar ground, small change, confirms the toolchain works). Then MW-11. For each service, write the HCL inline in `e2e/provider_smoke_test.go`, run `go test -tags provider_e2e ./e2e -v -run TestProvider<Service>`, observe failures, fix the response shape in the handler, add a Go regression test in `handlers_test.go`, repeat until the double-apply is a no-op. Then move to the next service. MW-2..MW-7 (the HCL examples) are written after the service passes MW-11 — they become the human-readable documentation of the working shape.

**Why MW-11 before MW-2..MW-7**: the Go e2e tests catch what the provider *actually* does; the Go unit tests only catch what we *think* it does. Writing examples before the service is verified just documents a broken config.
