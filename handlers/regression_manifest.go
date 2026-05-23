// Package handlers — regression-seed manifest.
//
// LandedServices is the tracked list of service-level ids that have a
// fully-implemented handler set. requireHandlerImplemented(t, id, ...)
// checks against this list to decide whether a regression test
// proceeds with real assertions or skips with a structured TODO
// message until the corresponding handler lands.
//
// Mirrored from fakeaws/handlers/regression_manifest.go per the S52-T1
// retrofit (slice-52-plan.md). Mockway's handler set is already fully
// landed; the helper exists so that future "standing pattern" regression
// tests can be pre-seeded for not-yet-landed services without spurious
// failures.
package handlers

import (
	"slices"
	"testing"
)

// LandedServices lists the service-level ids whose handlers are fully
// implemented in mockway today. Each id is a single lowercase token
// matching the `<service>[_<subresource>].go` filename convention in
// handlers/. Adding a service to this list is the last step of any
// future per-bundle PR (handler + tests + examples + coverage_matrix
// entry + manifest flip, all together).
//
// As of S52-T1, the twelve mockway service prefixes are all landed
// (see README "Provider Compatibility Matrix").
var LandedServices = []string{
	"block",
	"domain",
	"iam",
	"instance",
	"ipam",
	"k8s",
	"lb",
	"marketplace",
	"rdb",
	"redis",
	"registry",
	"vpc",
}

// requireHandlerImplemented is the manifest-gated skip helper. Tests
// for not-yet-landed services call this — the test calls t.Skipf with
// a structured TODO marker so CI can grep+count outstanding work
// without silent green-lights.
//
// Bare t.Skip() is forbidden; this helper is the *only* sanctioned skip
// path. The audit (TestRegressionSeedAuditNoVacuousPasses) parses
// test bodies via go/ast and fails CI if any test func contains both
// requireHandlerImplemented(...) AND a passing assert./require./t.Errorf
// call — that combination is the vacuous-pass smell.
//
// id is one of the LandedServices values; slice + pattern are
// human-readable labels for CI grep counting.
func requireHandlerImplemented(t *testing.T, id, slice, pattern string) {
	t.Helper()
	if slices.Contains(LandedServices, id) {
		return
	}
	t.Skipf("TODO(slice=%s,service=%s,pattern=%s) regression awaits handler — flip handlers/regression_manifest.go::LandedServices when the service lands",
		slice, id, pattern)
}

// RequireHandlerImplementedForTest is the public re-export for the
// handlers_test package's regression suite. Internal handler tests
// can use the package-private requireHandlerImplemented directly.
func RequireHandlerImplementedForTest(t *testing.T, id, slice, pattern string) {
	t.Helper()
	requireHandlerImplemented(t, id, slice, pattern)
}
