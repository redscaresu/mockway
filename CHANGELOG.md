# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
