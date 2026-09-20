package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redscaresu/mockway/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doAuthed(t *testing.T, ts *httptest.Server, method, path string, body []byte) (int, string) {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, r)
	require.NoError(t, err)
	req.Header.Set("X-Auth-Token", "test-token")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(out)
}

// TestContract_instance_root_volume_boot_default_false — wire-shape
// regression for the CRITICAL[instance-root-volume-boot-default-false]
// invariant in handlers/instance.go::CreateServer.
//
// The spec gives the volume `boot` flag default: false. Returning true
// made the provider read back a value its schema defaults to false, so
// every plan after an apply proposed `boot = true -> false` and no
// stack with an instance ever converged. The apply always succeeded,
// which is why this survived until a second plan was run.
func TestContract_instance_root_volume_boot_default_false(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{
		"name": "boot-default", "commercial_type": "DEV1-S", "image": "ubuntu_jammy",
	})
	require.Equal(t, http.StatusOK, status)

	inner := server["server"].(map[string]any)
	volumes := inner["volumes"].(map[string]any)
	root := volumes["0"].(map[string]any)
	assert.Equal(t, false, root["boot"],
		"the spec default is false; true makes every plan diff on a resource nobody touched")
}

// TestContract_lb_backend_pool_mirrors_server_ip — wire-shape
// regression for the CRITICAL[lb-backend-pool-mirrors-server-ip]
// invariant in handlers/lb.go::CreateBackend.
//
// A Backend is WRITTEN with `server_ip` and READ BACK as `pool`.
// Echoing the request field is not answering: the provider maps `pool`
// onto `server_ips`, so returning only `server_ip` left that empty and
// every plan proposed re-adding the backend servers.
func TestContract_lb_backend_pool_mirrors_server_ip(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, lbIP := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/ips", map[string]any{})
	status, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/lbs", map[string]any{
		"name": "pool-lb", "type": "LB-S", "ip_ids": []any{lbIP["id"]},
	})
	require.Equal(t, http.StatusOK, status, "lb create: %#v", lb)

	status, backend := testutil.DoCreate(t, ts, "/lb/v1/zones/fr-par-1/backends", map[string]any{
		"lb_id": lb["id"], "name": "web", "forward_protocol": "http",
		"forward_port": 80, "server_ip": []any{"51.15.1.1"},
	})
	require.Equal(t, http.StatusOK, status, "backend create: %#v", backend)
	assert.Equal(t, []any{"51.15.1.1"}, backend["pool"], "create response must carry pool")

	// The read the provider actually refreshes from.
	status, fetched := testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/backends/"+backend["id"].(string))
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []any{"51.15.1.1"}, fetched["pool"], "GET must carry pool, not only server_ip")

	// An update that changes the servers has to move pool with it, or
	// the change reads back as never having happened.
	body, err := json.Marshal(map[string]any{
		"lb_id": lb["id"], "name": "web", "forward_protocol": "http",
		"forward_port": 80, "server_ip": []any{"51.15.2.2"},
	})
	require.NoError(t, err)
	code, _ := doAuthed(t, ts, http.MethodPut,
		"/lb/v1/zones/fr-par-1/backends/"+backend["id"].(string), body)
	require.Equal(t, http.StatusOK, code)

	_, updated := testutil.DoGet(t, ts, "/lb/v1/zones/fr-par-1/backends/"+backend["id"].(string))
	assert.Equal(t, []any{"51.15.2.2"}, updated["pool"])
}

// TestContract_instance_user_data_round_trips — wire-shape regression
// for the CRITICAL[instance-user-data-round-trips] invariant in
// handlers/instance.go::SetServerUserData.
//
// mockway used to discard user_data on write and return an empty value
// on read, so the provider proposed re-adding cloud-init on every plan
// and the stack never converged. Worse for what this mock is FOR: a
// scenario whose point is "the instance serves a page" was validated
// against a mock that threw the startup script away.
func TestContract_instance_user_data_round_trips(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{
		"name": "ud", "commercial_type": "DEV1-S", "image": "ubuntu_jammy",
	})
	serverID := server["server"].(map[string]any)["id"].(string)
	base := "/instance/v1/zones/fr-par-1/servers/" + serverID + "/user_data"

	const script = "#cloud-config\nruncmd:\n  - echo hello\n"
	code, _ := doAuthed(t, ts, http.MethodPatch, base+"/cloud-init", []byte(script))
	require.Equal(t, http.StatusNoContent, code)

	code, value := doAuthed(t, ts, http.MethodGet, base+"/cloud-init", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, script, value, "what goes in must come back out, byte for byte")

	status, listed := testutil.DoGet(t, ts, base)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []any{"cloud-init"}, listed["user_data"],
		"the keys written must be the keys listed")

	// Removing cloud-init from the config deletes the key.
	code, _ = doAuthed(t, ts, http.MethodDelete, base+"/cloud-init", nil)
	require.Equal(t, http.StatusNoContent, code)
	_, afterDelete := testutil.DoGet(t, ts, base)
	assert.Empty(t, afterDelete["user_data"])

	// Idempotent: a key already gone is the desired state. Terraform
	// retries teardowns, and 404ing the second attempt fails a destroy
	// whose first attempt had already succeeded.
	code, _ = doAuthed(t, ts, http.MethodDelete, base+"/cloud-init", nil)
	assert.Equal(t, http.StatusNoContent, code, "deleting an absent key must not 404")

	// Reset must take user_data with it. The server rows go with FK
	// checks disabled, so the cascade does not fire -- without an
	// explicit entry in Reset's table list one scenario's cloud-init
	// survives into the next.
	code, _ = doAuthed(t, ts, http.MethodPatch, base+"/cloud-init", []byte(script))
	require.Equal(t, http.StatusNoContent, code)
	code, _ = doAuthed(t, ts, http.MethodPost, "/mock/reset", nil)
	require.Equal(t, http.StatusNoContent, code, "reset: %d", code)
	status, _ = testutil.DoGet(t, ts, base)
	assert.Equal(t, http.StatusNotFound, status, "the server itself is gone after a reset")

	// A key on a server that does not exist is a 404, not an empty stub.
	status, _ = testutil.DoGet(t,
		ts, "/instance/v1/zones/fr-par-1/servers/00000000-0000-0000-0000-000000000000/user_data/cloud-init")
	assert.Equal(t, http.StatusNotFound, status)
}

// TestContract_instance_v2_detach_pni — wire-shape regression for the
// CRITICAL[instance-v2-detach-pni] invariant in
// handlers/instance.go::DetachPrivateNetworkInterfaceV2.
//
// Provider 2.83.0 tears a private network attachment down through this
// route, naming the interface in `private_network_interface_id`.
// 2.81.0 used DELETE on the interface, which always refuses. Without
// this route the mock answers 501 and every compute teardown fails --
// the same "mock is behind the provider" failure, one version later.
//
// The field name was read off the wire from provider 2.83.0; there is
// no published v2alpha1 spec.
func TestContract_instance_v2_detach_pni(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()
	serverID := createServerForNIC(t, ts)
	pnID := createPrivateNetworkForNIC(t, ts)

	status, nic := testutil.DoCreate(t, ts, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces",
		map[string]any{"server_id": serverID, "private_network_id": pnID})
	require.Equal(t, http.StatusOK, status, "nic create: %#v", nic)
	nicID := nic["id"].(string)

	detach := "/instance/v2alpha1/zones/fr-par-1/servers/" + serverID + "/detach-private-network-interface"

	// The id is REQUIRED. A first version of this handler defaulted to
	// detaching every interface on the server when it was absent, which
	// is a mock permissive enough to accept a request naming nothing.
	body, err := json.Marshal(map[string]any{})
	require.NoError(t, err)
	code, _ := doAuthed(t, ts, http.MethodPost, detach, body)
	assert.Equal(t, http.StatusBadRequest, code, "a detach naming nothing must not succeed")

	// An interface this server does not own is a 404, not a silent
	// success: accepting it tells the client its mistake worked.
	body, err = json.Marshal(map[string]any{
		"private_network_interface_id": "00000000-0000-0000-0000-000000000000",
	})
	require.NoError(t, err)
	code, _ = doAuthed(t, ts, http.MethodPost, detach, body)
	assert.Equal(t, http.StatusNotFound, code)

	// The real shape.
	body, err = json.Marshal(map[string]any{"private_network_interface_id": nicID})
	require.NoError(t, err)
	code, payload := doAuthed(t, ts, http.MethodPost, detach, body)
	require.Equal(t, http.StatusOK, code)

	// The SDK decodes this into a Server -- not a no-content result and
	// not a {"server": ...} envelope. v1 GetServer decodes into a
	// GetServerResponse that HAS a Server field; this v2alpha1 method
	// decodes straight into Server, so the body is the bare object.
	// 204 is tolerated by provider 2.83.0 today, which is exactly how a
	// mock ends up behind its consumer.
	var detached map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &detached))
	assert.Equal(t, serverID, detached["id"], "the response is the updated Server, unwrapped")
	assert.Nil(t, detached["server"], "v2alpha1 does not wrap")

	// v2alpha1's Server is NOT v1's, and returning the stored v1 object
	// made the provider reject the body outright:
	//
	//	cannot unmarshal object into Go struct field .volumes of type
	//	[]*instance.ServerVolume
	//
	// v1 keys volumes by position in an object; v2alpha1 takes an array.
	assert.IsType(t, []any{}, detached["volumes"], "v2alpha1 volumes is an ARRAY, v1's is an object")
	assert.IsType(t, []any{}, detached["private_network_interfaces"])
	assert.NotNil(t, detached["server_type"], "v2alpha1 says server_type, not commercial_type")
	assert.Empty(t, detached["private_network_interfaces"], "the detached interface is gone from the server")

	status, _ = testutil.DoGet(t, ts, "/instance/v2alpha1/zones/fr-par-1/private-network-interfaces/"+nicID)
	assert.Equal(t, http.StatusNotFound, status, "the interface is gone after a detach")
}
