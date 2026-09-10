package handlers_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/redscaresu/mockway/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// TestContract_nic_delete_v2alpha1_always_refuses pins the defect that
// makes provider 2.81.0 unable to destroy a private NIC at all.
//
// Measured against real Scaleway on 2026-09-10, same NIC, same server,
// seconds apart:
//
//	DELETE /instance/v2alpha1/.../private-network-interfaces/{id}  -> 412
//	DELETE /instance/v1/zones/{z}/servers/{s}/private_nics/{id}    -> 204
//
// Power state is NOT the variable. It was assumed to be for two days --
// because the manual recovery stopped the server first and then used
// `scw`, which calls v1 -- and that assumption produced two refuted
// ADRs and a teardown fix that did nothing. The endpoint was the whole
// difference.
//
// Both halves are asserted. v2alpha1 refusing alone would also pass
// against a mock that refuses everywhere, which is what mockway did for
// a few hours today and it broke every Layer 2 teardown.
func TestContract_nic_delete_v2alpha1_always_refuses(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, pn := testutil.DoCreate(t, ts, "/vpc/v2/regions/fr-par/private-networks",
		map[string]any{"name": "pn-contract"})
	require.Equal(t, http.StatusOK, status)
	pnID, _ := pn["id"].(string)

	status, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "srv-contract", "commercial_type": "DEV1-S", "image": "ubuntu_jammy"})
	require.Equal(t, http.StatusOK, status, "server create: %#v", server)
	srv, _ := server["server"].(map[string]any)
	require.NotNil(t, srv)
	serverID, _ := srv["id"].(string)

	newNIC := func() string {
		st, nic := testutil.DoCreate(t, ts,
			"/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics",
			map[string]any{"private_network_id": pnID})
		require.Contains(t, []int{http.StatusOK, http.StatusCreated}, st, "nic create: %#v", nic)
		inner, _ := nic["private_nic"].(map[string]any)
		require.NotNil(t, inner)
		id, _ := inner["id"].(string)
		require.NotEmpty(t, id)
		return id
	}

	// v2alpha1: refused, and refused with the prose the provider prints.
	nicID := newNIC()
	code, body := deleteRaw(t, ts.URL, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces/"+nicID)
	require.Equal(t, http.StatusPreconditionFailed, code, "body: %s", body)
	assert.Contains(t, body, "Can't delete a private network interface attached to a server")

	// v1, same NIC, server still RUNNING: allowed. Without this half the
	// mock would refuse everywhere and no teardown could ever succeed.
	code, body = deleteRaw(t, ts.URL,
		"/instance/v1/zones/fr-par-1/servers/"+serverID+"/private_nics/"+nicID)
	require.Equal(t, http.StatusNoContent, code, "body: %s", body)
}
