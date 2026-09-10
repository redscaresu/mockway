package handlers_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/redscaresu/mockway/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deleteRaw is local because testutil.DoDelete returns only the status,
// and the MESSAGE is half of what this contract pins -- the provider
// surfaces it verbatim into the destroy output an operator then reads.
func deleteRaw(t *testing.T, base, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+path, nil)
	require.NoError(t, err)
	req.Header.Set("X-Auth-Token", "test")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// TestContract_nic_delete_requires_stopped_server pins the precondition
// real Scaleway enforces and mockway did not.
//
// The gap was not harmless. On 2026-09-09 a fix for this exact teardown
// failure was verified against mockway, which answered "7 added, 7
// destroyed", and shipped on that evidence. Real Scaleway refused the
// destroy exactly as before. A mock more permissive than reality does
// not merely miss bugs -- it CERTIFIES wrong fixes, because a green run
// reads as proof.
//
// `tofu destroy` deletes the NIC before the server (reverse dependency
// order), so this one behaviour decides whether a compute stack tears
// itself down or needs a human with cloud credentials.
func TestContract_nic_delete_requires_stopped_server(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, pn := testutil.DoCreate(t, ts, "/vpc/v2/regions/fr-par/private-networks",
		map[string]any{"name": "pn-contract"})
	require.Equal(t, http.StatusOK, status)
	pnID, _ := pn["id"].(string)
	require.NotEmpty(t, pnID)

	status, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "srv-contract", "commercial_type": "DEV1-S", "image": "ubuntu_jammy"})
	require.Equal(t, http.StatusOK, status, "server create: %#v", server)
	srv, _ := server["server"].(map[string]any)
	require.NotNil(t, srv, "server create must answer {\"server\": {...}}: %#v", server)
	serverID, _ := srv["id"].(string)
	require.NotEmpty(t, serverID)

	status, nic := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics",
		map[string]any{"private_network_id": pnID})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, status, "nic create: %#v", nic)
	inner, _ := nic["private_nic"].(map[string]any)
	require.NotNil(t, inner, "nic create must answer {\"private_nic\": {...}}: %#v", nic)
	nicID, _ := inner["id"].(string)
	require.NotEmpty(t, nicID)

	// Power the server on: this is the state a destroy actually meets.
	status, _ = testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/action",
		map[string]any{"action": "poweron"})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated, http.StatusAccepted}, status)

	nicPath := "/instance/v1/zones/fr-par-1/servers/" + serverID + "/private_nics/" + nicID
	code, body := deleteRaw(t, ts.URL, nicPath)
	require.Equal(t, http.StatusPreconditionFailed, code, "body: %s", body)
	assert.Contains(t, body, "Can't delete a private network interface attached to a server")

	// Stopped: the same delete succeeds. Without this half the test would
	// also pass against a mock that refuses unconditionally, which would
	// break every teardown instead of fixing one.
	status, _ = testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/action",
		map[string]any{"action": "poweroff"})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated, http.StatusAccepted}, status)

	code, body = deleteRaw(t, ts.URL, nicPath)
	require.Equal(t, http.StatusNoContent, code, "body: %s", body)
}
