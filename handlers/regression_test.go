package handlers_test

// Package handlers regression catalogue.
//
// These tests encode misconfiguration patterns documented under
// examples/misconfigured/. Each public TestRegression... function starts with
// the manifest-gated helper, then delegates to an assertion helper so the
// no-vacuous-pass audit can distinguish landed tests from pre-seeded stubs.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redscaresu/mockway/handlers"
	"github.com/redscaresu/mockway/testutil"
	"github.com/stretchr/testify/require"
)

func requireHandlerImplemented(t *testing.T, id, slice, pattern string) {
	t.Helper()
	handlers.RequireHandlerImplementedForTest(t, id, slice, pattern)
}

// Cross-state orphan rejection.
//
// Pattern: one workspace owns a shared IAM application while another creates
// API keys or policies against it. Destroying the owner first must 409 because
// mockway sees global API state, not just the current Terraform state file.
func TestRegressionCrossStateOrphanRejection(t *testing.T) {
	requireHandlerImplemented(t, "iam", "M75", "cross-state orphan")
	checkCrossStateOrphanRejection(t)
}

// Security-group name-vs-id confusion.
//
// Pattern: Terraform accepts both .name and .id as strings, but Scaleway expects
// security_group_id to be the UUID. Passing the human name must fail as an FK
// miss instead of silently creating a server with the wrong firewall posture.
func TestRegressionSecurityGroupNameNotID(t *testing.T) {
	requireHandlerImplemented(t, "instance", "M75", "security group name not id")
	checkSecurityGroupNameNotID(t)
}

// Wrong-collection LB backend reference.
//
// Pattern: a frontend's backend_id is populated from scaleway_lb.lb.id instead
// of scaleway_lb_backend.backend.id. Both are UUID strings, so only the API can
// reject the wrong collection.
func TestRegressionWrongCollectionLBBackendReference(t *testing.T) {
	requireHandlerImplemented(t, "lb", "M75", "wrong collection lb backend")
	checkWrongCollectionLBBackendReference(t)
}

// Cross-parent LB route rejection.
//
// Pattern: routes bind a frontend and backend that must belong to the same LB.
// A UUID from another LB is syntactically valid but semantically wrong and must
// be rejected before storing cross-LB state.
func TestRegressionCrossParentLBRouteReference(t *testing.T) {
	requireHandlerImplemented(t, "lb", "M75", "cross parent lb route")
	checkCrossParentLBRouteReference(t)
}

// VPC delete ordering.
//
// Pattern: a VPC workspace can be destroyed while private-network workspaces
// still exist. Parent deletion must return 409 so Terraform exposes the missing
// destroy ordering instead of orphaning child state.
func TestRegressionVPCDeleteWithPrivateNetwork(t *testing.T) {
	requireHandlerImplemented(t, "vpc", "M75", "vpc delete with private network")
	checkVPCDeleteWithPrivateNetwork(t)
}

// Block snapshot missing volume.
//
// Pattern: a stale hard-coded UUID in volume_id cannot be verified by plan. The
// create call must reject with the create-path 404 message because the referenced
// block volume does not exist.
func TestRegressionBlockSnapshotMissingVolume(t *testing.T) {
	requireHandlerImplemented(t, "block", "M75", "block snapshot missing volume")
	checkBlockSnapshotMissingVolume(t)
}

// IAM group missing user.
//
// Pattern: group membership may be fed by a user data-source lookup. If that
// user is absent, membership creation must 404 rather than creating an empty or
// partially-populated group.
func TestRegressionIAMGroupMissingUser(t *testing.T) {
	requireHandlerImplemented(t, "iam", "M75", "iam group missing user")
	checkIAMGroupMissingUser(t)
}

// K8s pool missing cluster.
//
// Pattern: cluster_id can accidentally receive a cluster name. The pool endpoint
// must validate the parent cluster ID from the nested path and return 404 when
// no such cluster exists.
func TestRegressionK8sPoolMissingCluster(t *testing.T) {
	requireHandlerImplemented(t, "k8s", "M75", "k8s pool missing cluster")
	checkK8sPoolMissingCluster(t)
}

// LB ACL missing frontend.
//
// Pattern: ACL frontend_id is easy to fill with the LB's UUID. ACL creation is
// nested under a frontend path and must reject a missing or wrong-collection
// frontend with 404.
func TestRegressionLBACLMissingFrontend(t *testing.T) {
	requireHandlerImplemented(t, "lb", "M75", "lb acl missing frontend")
	checkLBACLMissingFrontend(t)
}

// Private NIC missing private network.
//
// Pattern: a server can be valid while the requested private network attachment
// points to stale external state. NIC creation must validate both the server and
// private_network_id FK before writing state.
func TestRegressionPrivateNICMissingPrivateNetwork(t *testing.T) {
	requireHandlerImplemented(t, "instance", "M75", "private nic missing pn")
	checkPrivateNICMissingPrivateNetwork(t)
}

// Nested private NIC ownership.
//
// Pattern: /servers/{server_id}/private_nics/{nic_id} must prove the NIC belongs
// to the server in the URL, not merely that the NIC exists globally.
func TestRegressionNestedPrivateNICOwnership(t *testing.T) {
	requireHandlerImplemented(t, "instance", "M75", "nested private nic ownership")
	checkNestedPrivateNICOwnership(t)
}

// Destroy idempotency on missing resources.
//
// Pattern: retrying a DELETE after remote state is already gone should return a
// clean 404 domain error, not a 500 or malformed response that masks the original
// destroy-order problem.
func TestRegressionDestroyIdempotencyMissingDelete(t *testing.T) {
	requireHandlerImplemented(t, "block", "M75", "destroy idempotency missing delete")
	checkDestroyIdempotencyMissingDelete(t)
}

// Marketplace unknown labels stay unknown.
//
// Pattern: image-name typos must not auto-resolve to deterministic UUIDs. The
// provider relies on an empty local-images list to surface mistakes like
// ubuntu_noble misspellings before a server create proceeds.
func TestRegressionMarketplaceUnknownLabelEmpty(t *testing.T) {
	requireHandlerImplemented(t, "marketplace", "M75", "marketplace unknown label")
	checkMarketplaceUnknownLabelEmpty(t)
}

func checkCrossStateOrphanRejection(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	_, app := createIAMApplication(t, ts)
	appID := app["id"].(string)
	assertCreateStatus(t, ts, "/iam/v1alpha1/api-keys", map[string]any{"application_id": appID}, http.StatusOK)
	assertCreateStatus(t, ts, "/iam/v1alpha1/policies", map[string]any{"name": "p", "application_id": appID}, http.StatusOK)
	assertDeleteStatus(t, testutil.DoDelete(t, ts, "/iam/v1alpha1/applications/"+appID), http.StatusConflict)
}

func checkSecurityGroupNameNotID(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	assertCreateStatus(t, ts, "/instance/v1/zones/fr-par-1/security_groups", map[string]any{"name": "example-sg"}, http.StatusOK)
	status, body := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "web", "commercial_type": "DEV1-S", "image": "ubuntu_noble", "security_group_id": "example-sg"})
	assertStatusWithMessage(t, status, body, http.StatusNotFound, "referenced resource not found")
}

func checkWrongCollectionLBBackendReference(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	lbID := createLB(t, ts)
	assertCreateStatus(t, ts, "/lb/v1/zones/fr-par-1/backends", backendBody(lbID, "backend"), http.StatusOK)
	status, body := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/frontends", frontendBody(lbID, lbID, "frontend"))
	assertStatusWithMessage(t, status, body, http.StatusNotFound, "referenced resource not found")
}

func checkCrossParentLBRouteReference(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	lbA := createLB(t, ts)
	backendA := createBackend(t, ts, lbA, "backend-a")
	frontendA := createFrontend(t, ts, lbA, backendA, "frontend-a")
	lbB := createLB(t, ts)
	backendB := createBackend(t, ts, lbB, "backend-b")
	status, _ := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/routes", map[string]any{"frontend_id": frontendA, "backend_id": backendB})
	assertStatusCode(t, status, http.StatusBadRequest)
}

func checkVPCDeleteWithPrivateNetwork(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "vpc"})
	vpcID := vpc["id"].(string)
	assertCreateStatus(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{"name": "pn", "vpc_id": vpcID}, http.StatusOK)
	assertDeleteStatus(t, testutil.DoDelete(t, ts, "/vpc/v1/regions/fr-par/vpcs/"+vpcID), http.StatusConflict)
}

func checkBlockSnapshotMissingVolume(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	status, body := testutil.DoCreate(t, ts, "/block/v1alpha1/zones/fr-par-1/snapshots", map[string]any{"name": "broken", "volume_id": missingUUID()})
	assertStatusWithMessage(t, status, body, http.StatusNotFound, "referenced resource not found")
}

func checkIAMGroupMissingUser(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	_, group := testutil.DoCreate(t, ts, "/iam/v1alpha1/groups", map[string]any{"name": "team"})
	status, body := testutil.DoCreate(t, ts, "/iam/v1alpha1/groups/"+group["id"].(string)+"/add-member", map[string]any{"user_id": missingUUID()})
	assertStatusWithMessage(t, status, body, http.StatusNotFound, "referenced resource not found")
}

func checkK8sPoolMissingCluster(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	status, body := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters/my-cluster/pools", map[string]any{"name": "pool", "node_type": "DEV1-M", "size": 1})
	assertStatusWithMessage(t, status, body, http.StatusNotFound, "referenced resource not found")
}

func checkLBACLMissingFrontend(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	lbID := createLB(t, ts)
	status, body := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/frontends/"+lbID+"/acls", map[string]any{"name": "acl", "index": 1})
	assertStatusWithMessage(t, status, body, http.StatusNotFound, "referenced resource not found")
}

func checkPrivateNICMissingPrivateNetwork(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	serverID := createServer(t, ts, "web")
	status, body := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics", map[string]any{"private_network_id": missingUUID()})
	assertStatusWithMessage(t, status, body, http.StatusNotFound, "referenced resource not found")
}

func checkNestedPrivateNICOwnership(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	serverA := createServer(t, ts, "a")
	serverB := createServer(t, ts, "b")
	pnID := createPrivateNetwork(t, ts)
	_, nic := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverA+"/private_nics", map[string]any{"private_network_id": pnID})
	status, _ := testutil.DoGet(t, ts, "/instance/v1/zones/fr-par-1/servers/"+serverB+"/private_nics/"+unwrap(nic, "private_nic")["id"].(string))
	assertGetStatus(t, status, http.StatusNotFound)
}

func checkDestroyIdempotencyMissingDelete(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	assertDeleteStatus(t, testutil.DoDelete(t, ts, "/block/v1alpha1/zones/fr-par-1/snapshots/"+missingUUID()), http.StatusNotFound)
}

func checkMarketplaceUnknownLabelEmpty(t *testing.T) {
	ts, cleanup := regressionServer(t)
	defer cleanup()
	status, body := testutil.DoGet(t, ts, "/marketplace/v2/local-images?image_label=ubuntu_noble_typo&zone=fr-par-1&type=instance_sbs")
	assertStatusCode(t, status, http.StatusOK)
	assertFloat(t, body["total_count"], 0)
	assertLen(t, body["local_images"].([]any), 0)
}

func regressionServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	return testutil.NewTestServer(t)
}

func createIAMApplication(t *testing.T, ts *httptest.Server) (int, map[string]any) {
	t.Helper()
	return testutil.DoCreate(t, ts, "/iam/v1alpha1/applications", map[string]any{"name": "app"})
}

func createLB(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	status, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{"name": "lb", "type": "LB-S"})
	assertStatusCode(t, status, http.StatusOK)
	return lb["id"].(string)
}

func createBackend(t *testing.T, ts *httptest.Server, lbID, name string) string {
	t.Helper()
	status, body := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/backends", backendBody(lbID, name))
	assertStatusCode(t, status, http.StatusOK)
	return body["id"].(string)
}

func createFrontend(t *testing.T, ts *httptest.Server, lbID, backendID, name string) string {
	t.Helper()
	status, body := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/frontends", frontendBody(lbID, backendID, name))
	assertStatusCode(t, status, http.StatusOK)
	return body["id"].(string)
}

func createServer(t *testing.T, ts *httptest.Server, name string) string {
	t.Helper()
	status, body := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": name, "commercial_type": "DEV1-S", "image": "ubuntu_noble"})
	assertStatusCode(t, status, http.StatusOK)
	return unwrap(body, "server")["id"].(string)
}

func createPrivateNetwork(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	vpcStatus, vpcBody := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "vpc-for-pn"})
	assertStatusCode(t, vpcStatus, http.StatusOK)
	vpcID := vpcBody["id"].(string)
	status, body := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{"name": "pn", "vpc_id": vpcID})
	assertStatusCode(t, status, http.StatusOK)
	return body["id"].(string)
}

func backendBody(lbID, name string) map[string]any {
	return map[string]any{"lb_id": lbID, "name": name, "forward_protocol": "http", "forward_port": 80}
}

func frontendBody(lbID, backendID, name string) map[string]any {
	return map[string]any{"lb_id": lbID, "backend_id": backendID, "name": name, "inbound_port": 80}
}

func unwrap(body map[string]any, key string) map[string]any {
	return body[key].(map[string]any)
}

func missingUUID() string {
	return "aabbccdd-1234-1234-1234-aabbccddeeff"
}

func assertCreateStatus(t *testing.T, ts *httptest.Server, path string, body map[string]any, want int) {
	t.Helper()
	status, _ := testutil.DoCreate(t, ts, path, body)
	assertStatusCode(t, status, want)
}

func assertGetStatus(t *testing.T, status, want int) {
	t.Helper()
	assertStatusCode(t, status, want)
}

func assertDeleteStatus(t *testing.T, status, want int) {
	t.Helper()
	assertStatusCode(t, status, want)
}

func assertStatusCode(t *testing.T, got, want int) {
	t.Helper()
	require.Equal(t, want, got, "status")
}

func assertStatusWithMessage(t *testing.T, status int, body map[string]any, want int, message string) {
	t.Helper()
	assertStatusCode(t, status, want)
	require.Equal(t, message, body["message"], "message")
}

func assertFloat(t *testing.T, got any, want float64) {
	t.Helper()
	require.Equal(t, want, got, "float")
}

func assertLen(t *testing.T, got []any, want int) {
	t.Helper()
	require.Len(t, got, want, "len")
}
