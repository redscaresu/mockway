# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
