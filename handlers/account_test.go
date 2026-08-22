package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redscaresu/mockway/models"
	"github.com/redscaresu/mockway/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContract_account_project_create_returns_id — wire-shape regression
// for the CRITICAL[account-project-create-returns-id] invariant in
// handlers/account.go::CreateAccountProject.
//
// The Scaleway provider reads `id` straight into state for
// scaleway_account_project, and every child resource interpolates
// project_id from it. An empty id breaks the whole graph at plan time on
// the following run rather than at create, which is a much harder
// failure to diagnose.
func TestContract_account_project_create_returns_id(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, project := testutil.DoCreate(t, ts, "/account/v3/projects", map[string]any{
		"name":        "infrafactory-run",
		"description": "layer 3 canary",
	})
	require.Equal(t, 200, status)

	id, ok := project["id"].(string)
	require.True(t, ok, "project response must carry a string id, got %#v", project["id"])
	require.NotEmpty(t, id)
	assert.Equal(t, "infrafactory-run", project["name"])
	assert.NotEmpty(t, project["organization_id"])
	assert.NotEmpty(t, project["created_at"])

	// Round-trips: the provider re-reads by id on the next plan.
	status, fetched := testutil.DoGet(t, ts, "/account/v3/projects/"+id)
	require.Equal(t, 200, status)
	assert.Equal(t, id, fetched["id"])
}

// TestContract_account_project_delete_nonempty — wire-shape regression
// for the CRITICAL[account-project-delete-nonempty] invariant in
// handlers/account.go::DeleteAccountProject.
//
// Real Scaleway refuses to delete a project that still owns resources.
// Mockway must refuse too: otherwise it teaches the generator a destroy
// ordering that works against the mock and fails against the real API,
// leaving billable orphans behind. Layer 2 exists to catch exactly this
// divergence in seconds rather than at real teardown.
func TestContract_account_project_delete_nonempty(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, project := testutil.DoCreate(t, ts, "/account/v3/projects", map[string]any{"name": "with-children"})
	require.Equal(t, 200, status)
	projectID := project["id"].(string)

	status, vpc := testutil.DoCreate(t, ts, "/vpc/v2/regions/fr-par/vpcs", map[string]any{
		"name":       "child-vpc",
		"project_id": projectID,
	})
	require.Equal(t, 200, status)
	vpcID := vpc["id"].(string)

	// Deleting the populated project must be refused, and must say what
	// is blocking it -- the blocker list is the whole point.
	status, body := doDeleteJSON(t, ts, "/account/v3/projects/"+projectID)
	require.Equal(t, 409, status)
	assert.Equal(t, "precondition_failed", body["type"])
	assert.Equal(t, projectID, body["project_id"])

	blockers, ok := body["blockers"].([]any)
	require.True(t, ok, "409 body must enumerate blockers, got %#v", body["blockers"])
	require.Len(t, blockers, 1)
	assert.Contains(t, blockers[0], vpcID, "blocker should name the resource id so an operator can act on it")
	assert.Contains(t, blockers[0], "vpcs")

	// Once the child is gone the project deletes cleanly. This is the
	// ordering the generator must learn.
	require.Equal(t, 204, testutil.DoDelete(t, ts, "/vpc/v2/regions/fr-par/vpcs/"+vpcID))
	require.Equal(t, 204, testutil.DoDelete(t, ts, "/account/v3/projects/"+projectID))

	status, _ = testutil.DoGet(t, ts, "/account/v3/projects/"+projectID)
	assert.Equal(t, 404, status)
}

// The default project must exist from boot. Scenarios that never declare
// a scaleway_account_project still send project_id=00000000-... on every
// create; without the seed they would reference a project that does not
// exist, which would regress the entire existing scenario suite.
func TestAccountProjectDefaultSeededAtBoot(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, list := testutil.DoList(t, ts, "/account/v3/projects")
	require.Equal(t, 200, status)

	projects, ok := list["projects"].([]any)
	require.True(t, ok)
	require.Len(t, projects, 1)

	def := projects[0].(map[string]any)
	assert.Equal(t, models.DefaultProjectID, def["id"])
	assert.Equal(t, "default", def["name"])
}

func TestAccountProjectUpdateAndNotFound(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, project := testutil.DoCreate(t, ts, "/account/v3/projects", map[string]any{"name": "before"})
	require.Equal(t, 200, status)
	id := project["id"].(string)

	status, updated := doPatch(t, ts, "/account/v3/projects/"+id, map[string]any{"name": "after"})
	require.Equal(t, 200, status)
	assert.Equal(t, "after", updated["name"])
	assert.Equal(t, id, updated["id"], "id must be immutable")

	status, _ = testutil.DoGet(t, ts, "/account/v3/projects/does-not-exist")
	assert.Equal(t, 404, status)
	assert.Equal(t, 404, testutil.DoDelete(t, ts, "/account/v3/projects/does-not-exist"))
}

// doDeleteJSON is DoDelete plus the response body. The existing helper
// returns only the status code, but the whole contract here is what the
// 409 body says.
func doDeleteJSON(t *testing.T, ts *httptest.Server, path string) (int, map[string]any) {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+path, nil)
	require.NoError(t, err)
	req.Header.Set("X-Auth-Token", "test-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	out := map[string]any{}
	if resp.StatusCode != http.StatusNoContent {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	}
	return resp.StatusCode, out
}
