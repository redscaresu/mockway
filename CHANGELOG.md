# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed (2026-09-10)
- **Deleting a private network interface now requires its server to be powered off**, as real
  Scaleway does (`412`, `Can't delete a private network interface attached to a server`). Both
  delete routes enforce it — v1 `/servers/{id}/private_nics/{nic}` and v2alpha1
  `/private-network-interfaces/{id}` — because a precondition honoured on one route and not the
  other is a mock that disagrees with itself depending on which API the provider calls.

  This is the expensive kind of infidelity: **the mock was more permissive than reality**, so it
  did not merely miss a bug, it *certified a wrong fix*. `tofu destroy` removes a NIC before its
  server (reverse dependency order), so a running instance makes the whole teardown fail. On
  2026-09-09 a fix for exactly that failure was verified here, mockway answered `7 added, 7
  destroyed`, and it shipped on that evidence — real Scaleway then refused the destroy exactly as
  before, twice, each time leaving billable infrastructure that needed a human with cloud
  credentials to unpick.

  Pinned by `TestContract_nic_delete_requires_stopped_server`, which asserts **both** directions:
  refused while running, and accepted once stopped. A test with only the first half would pass
  against a mock that refuses unconditionally — which would break every teardown instead of fixing
  one.

### Fixed (2026-09-09)
- **A private network with no `vpc_id` now lands in the default VPC**, matching Scaleway. Real
  Scaleway gives every project a default VPC per region and places the network there when the
  request names none; mockway required one, so the empty string reached the foreign-key check as a
  missing reference and the mock **rejected configuration Scaleway accepts** — a private network
  with just a name being the common shape. The provider surfaced it as `resource  with ID  is not
  found`, whose two empty gaps are what a FK failure on `""` looks like by the time it reaches HCL.
  Found by an infrafactory run whose generated HCL was *byte-identical* to a stack that had deployed
  to real Scaleway the day before: it failed twice and the loop declared itself stuck. The generator
  was right and the mock was wrong — the inversion of the usual case. The default VPC is created on
  demand and reused per region rather than seeded, so no test has to know about it; an explicit
  `vpc_id` still wins and is still foreign-key checked.

### Added (M73 + M75 + M77 + M82 + M85, 2026-05-28)
- **M75 — Regression patterns catalogue** at `handlers/regression_test.go` (13 `TestRegression*` functions). mockway had `regression_audit_test.go` + `regression_manifest.go` scaffolding for ~6 months but ZERO patterns — audit passed vacuously. Patterns ported from fakeaws's S43-T10 catalogue and adapted to Scaleway's surface: cross-state-orphan rejection (iam api-keys), VPC→private-network FK, LB→ACL→frontend chain, K8s node-pool→cluster, RDB read-replica→primary, registry-namespace uniqueness, nested-private-NIC ownership check, marketplace unknown-label behavior, etc.
- **M85 — `TestRegressionSeedAuditHasPatterns`** added — meta-guard asserts pattern count ≥ `min(len(LandedServices), 8)`. Prevents the M75-class "audit scaffolding ships with zero patterns" recurrence.
- **M73 — README badges** (CI / License / Go-version) under the `# mockway` heading for parity.
- **M82 — Dependabot** at `.github/dependabot.yml` for gomod + github-actions.

### Changed (M77)
- **Go 1.24.2 → 1.25.0** (go.mod + toolchain) and **modernc.org/sqlite 1.46.1 → 1.50.0** — coordinated cross-repo dep alignment so fakeaws/fakegcp/mockway share the same shared-dep versions.

### Added (earlier)
- README "API Compatibility" section documenting the wire-shape contract + the `examples/working/<svc>` smoke harness (`apply → plan -detailed-exitcode 0 → destroy`) every handler is validated against (mockway@001cca7). Parity with the equivalent sections in fakeaws + fakegcp.
- 280+ handler tests covering Scaleway Compute (Instance: servers, IPs, NICs, security groups, volumes), Networking (VPC, Private Network, Public Gateway), Load Balancer (LB + Frontend + Backend + LB IP + LB Private Network with multi-backend support), Database (RDB Instance/User/Database/ACL/Privilege/Certificate/Endpoint, PostgreSQL + MySQL), Kubernetes (Cluster + Node Pool), IAM (Application/API Key/Policy/Rule), Container Registry, Redis Cluster, and Block Storage.
- 22 working terraform examples under `examples/working/` + 10 misconfigured + 16 updates examples for integration testing.
- Admin endpoints (`/mock/state`, `/mock/reset`, `/mock/snapshot`, `/mock/restore`) for state inspection and lifecycle control.
- FK enforcement at the database layer with cascade/restrict semantics matching real Scaleway behavior.
- `scripts/test-examples.sh` + `scripts/test-misconfigured.sh` idempotency harness.
- `scripts/spec_diff.py` to surface gaps between mockway's coverage and what terraform-provider-scaleway actually calls.

### Security
- `gitleaks` pre-commit hook installable via `make install-hooks`; `.gitleaks.toml` config shipped.
- `SECURITY.md` with private vulnerability reporting via GitHub Security Advisories.
- Apache-2.0 LICENSE (re-licensed from MIT 2026-05-23 for project-family parity).
