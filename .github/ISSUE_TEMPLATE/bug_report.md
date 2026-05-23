---
name: Bug report
about: Mock behavior diverges from real Scaleway, or mockway crashes.
title: '[bug] '
labels: bug
assignees: ''
---

## Summary

<!-- One sentence describing what's wrong. -->

## Steps to reproduce

```bash
# example
./mockway --port 8080 &
curl -X POST http://localhost:8080/instance/v1/zones/fr-par-1/servers ...
```

## Expected behavior

<!-- What did real Scaleway return? Real-Scaleway raw HTTP response if you have one. -->

## Actual behavior

<!-- What did mockway return? Raw HTTP response. -->

## Environment

- mockway commit: <!-- `git rev-parse --short HEAD` -->
- OS / arch: <!-- macOS arm64 / Linux amd64 -->
- Go version: <!-- `go version` -->
- terraform-provider-scaleway version (if applicable):

## Type of issue

- [ ] Crash / panic in mockway
- [ ] Fidelity gap: mockway accepts a request that real Scaleway rejects
- [ ] Fidelity gap: mockway rejects a request that real Scaleway accepts
- [ ] Wrong response shape (mockway returns different fields than real Scaleway)
- [ ] FK enforcement bug (delete should 409 but mockway 204s, or vice versa)
