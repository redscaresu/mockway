package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redscaresu/mockway/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createPrivateNetworkForNIC returns a real private network id.
//
// An invented uuid will not do: mockway enforces the foreign key, so a
// create against a network that does not exist answers 404 "referenced
// resource not found" -- which is the mock being more faithful than the
// test was.
func createPrivateNetworkForNIC(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	status, vpc := testutil.DoCreate(t, ts, "/vpc/v2/regions/fr-par/vpcs", map[string]any{
		"name": "if-nic-test",
	})
	require.Equal(t, http.StatusOK, status, "vpc create: %#v", vpc)

	status, pn := testutil.DoCreate(t, ts, "/vpc/v2/regions/fr-par/private-networks", map[string]any{
		"name":   "if-nic-test",
		"vpc_id": vpc["id"],
	})
	require.Equal(t, http.StatusOK, status, "private network create: %#v", pn)
	id, _ := pn["id"].(string)
	require.NotEmpty(t, id)
	return id
}

// createServerForNIC returns a server id the v2alpha1 interface routes can
// attach to. The endpoint checks the server exists, so every test here
// needs a real one rather than a made-up uuid.
func createServerForNIC(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	status, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{
		"name":                "if-nic-test",
		"commercial_type":     "DEV1-S",
		"image":               "ubuntu_jammy",
		"dynamic_ip_required": true,
	})
	require.Equal(t, http.StatusOK, status, "server create: %#v", server)
	inner, _ := server["server"].(map[string]any)
	require.NotNil(t, inner, "server create response must carry a server: %#v", server)
	id, _ := inner["id"].(string)
	require.NotEmpty(t, id)
	return id
}

// TestContract_instance_v2_pni_create — wire-shape regression for the
// CRITICAL[instance-v2-pni-create] invariant in
// handlers/instance.go::CreatePrivateNetworkInterfaceV2.
//
// The Scaleway provider creates scaleway_instance_private_nic through
// this endpoint, not through the v1 /servers/{id}/private_nics route.
// While it returned 501, every Scaleway compute scenario failed at Layer
// 2 — and infrafactory runs Layer 3 only when the mock apply succeeded,
// so one missing verb on one path blocked real-cloud coverage entirely.
func TestContract_instance_v2_pni_create(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()
	serverID := createServerForNIC(t, ts)
	pnID := createPrivateNetworkForNIC(t, ts)

	status, nic := testutil.DoCreate(t, ts, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces",
		map[string]any{
			"private_network_id": pnID,
			"server_id":          serverID,
		})

	require.Equal(t, http.StatusOK, status, "create must not 501: %#v", nic)
	assert.NotEmpty(t, nic["id"])
	assert.Equal(t, serverID, nic["server_id"])
	assert.Equal(t, pnID, nic["private_network_id"])

	// The v2 waiter polls until status leaves {attaching, detaching,
	// syncing}. Anything else here spins until the provider times out.
	assert.Equal(t, "available", nic["status"])

	// Both API versions return a MAC. mockway returned neither before
	// this slice; the real one observed 2026-08-31 was 02:00:00:14:f9:63.
	assert.NotEmpty(t, nic["mac_address"])
}

// TestContract_instance_v2_pni_unwrapped — wire-shape regression for the
// CRITICAL[instance-v2-pni-unwrapped] invariant.
//
// v1 wraps in `private_nic` and the v2 LISTING wraps in
// `private_network_interfaces`, which makes a singular
// `private_network_interface` envelope the natural guess. It is wrong:
// the SDK does `var resp PrivateNetworkInterface; s.client.Do(req,
// &resp)`, so the object is the whole body.
//
// The cost of getting this wrong is why it is pinned. An envelope parses
// as an empty object, so the failure surfaces on the provider's NEXT call
// as "field PrivateNetworkInterfaceID cannot be empty in request" — a
// message that points at a request rather than at this response.
func TestContract_instance_v2_pni_unwrapped(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()
	serverID := createServerForNIC(t, ts)
	pnID := createPrivateNetworkForNIC(t, ts)

	_, nic := testutil.DoCreate(t, ts, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces",
		map[string]any{
			"private_network_id": pnID,
			"server_id":          serverID,
		})

	// The id is at the top level, not under an envelope key.
	require.NotEmpty(t, nic["id"], "response must be the object itself: %#v", nic)
	assert.Nil(t, nic["private_network_interface"], "an envelope here is what breaks the provider")

	// Get is unwrapped too, and is what the waiter polls.
	status, fetched := testutil.DoGet(t, ts,
		"/instance/v2alpha1/zones/fr-par-1/private-network-interfaces/"+nic["id"].(string))
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, nic["id"], fetched["id"])
	assert.Equal(t, "available", fetched["status"])
}

// One record, two API versions. The provider creates through v2 while
// infrafactory's own checks and the v1 route read `state` — writing only
// one spelling would make the same interface look healthy through one
// version and unknown through the other.
func TestPrivateNICCarriesBothStateAndStatus(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()
	serverID := createServerForNIC(t, ts)
	pnID := createPrivateNetworkForNIC(t, ts)

	_, nic := testutil.DoCreate(t, ts, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces",
		map[string]any{
			"private_network_id": pnID,
			"server_id":          serverID,
		})

	assert.Equal(t, "available", nic["status"], "v2alpha1 spells it status")
	assert.Equal(t, "available", nic["state"], "v1 spells it state")

	// And it is the same record: the v1 listing sees what v2 created.
	status, listed := testutil.DoGet(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics")
	require.Equal(t, http.StatusOK, status)
	items, _ := listed["private_nics"].([]any)
	require.Len(t, items, 1, "two stores would let a server report different interfaces per version")
}

// Creating an interface on a server that does not exist would leave a
// record nothing can reach. The v1 listing route makes the same check.
func TestCreatePrivateNetworkInterfaceV2RefusesAnUnknownServer(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()
	pnID := createPrivateNetworkForNIC(t, ts)

	status, body := testutil.DoCreate(t, ts, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces",
		map[string]any{
			"private_network_id": pnID,
			"server_id":          "00000000-0000-0000-0000-000000000000",
		})

	require.Equal(t, http.StatusNotFound, status)
	// A create-path error: the server is being REFERENCED here, and the
	// private-network foreign key answers the same way. Two spellings for
	// the same missing server would be uncodeable against.
	assert.Equal(t, "server", body["resource"])
	assert.Equal(t, "00000000-0000-0000-0000-000000000000", body["resource_id"])
	assert.Contains(t, body["message"], "referenced server")
}

func TestCreatePrivateNetworkInterfaceV2RequiresAServerID(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, _ := testutil.DoCreate(t, ts, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces",
		map[string]any{"private_network_id": "fr-par/11111111-1111-1111-1111-111111111111"})

	assert.Equal(t, http.StatusBadRequest, status)
}

// The provider destroys through this route. Without it a scenario could
// be applied and never torn down, and against a mock the next run
// inherits it.
func TestDeletePrivateNetworkInterfaceV2RemovesIt(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()
	serverID := createServerForNIC(t, ts)
	pnID := createPrivateNetworkForNIC(t, ts)

	_, nic := testutil.DoCreate(t, ts, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces",
		map[string]any{
			"private_network_id": pnID,
			"server_id":          serverID,
		})
	id := nic["id"].(string)

	assert.Equal(t, http.StatusNoContent, testutil.DoDelete(t, ts,
		"/instance/v2alpha1/zones/fr-par-1/private-network-interfaces/"+id))

	status, _ := testutil.DoGet(t, ts,
		"/instance/v2alpha1/zones/fr-par-1/private-network-interfaces/"+id)
	assert.Equal(t, http.StatusNotFound, status, "a destroyed interface must not come back")
}
