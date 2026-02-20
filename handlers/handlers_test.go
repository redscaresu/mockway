package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/redscaresu/mockway/testutil"
)

func TestAuthRequiredOnScalewayRoutes(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/vpc/v1/regions/fr-par/vpcs", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestInstanceServerLifecycle(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "commercial_type": "DEV1-S"},
	)
	require.Equal(t, 200, status)
	serverID := body["id"].(string)
	require.NotEmpty(t, serverID)

	status, body = testutil.DoGet(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID,
	)
	require.Equal(t, 200, status)
	require.Equal(t, "web-1", body["name"])

	status, body = testutil.DoList(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
	)
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), body["total_count"])

	status = testutil.DoDelete(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID,
	)
	require.Equal(t, 204, status)

	status, _ = testutil.DoGet(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+serverID,
	)
	require.Equal(t, 404, status)
}

func TestCrossServiceFlow(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts,
		"/vpc/v1/regions/fr-par/vpcs",
		map[string]any{"name": "main"},
	)
	_, pn := testutil.DoCreate(t, ts,
		"/vpc/v1/regions/fr-par/private-networks",
		map[string]any{"name": "app-net", "vpc_id": vpc["id"]},
	)
	_, srv := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers",
		map[string]any{"name": "web-1", "commercial_type": "DEV1-S"},
	)
	_, nic := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/"+srv["id"].(string)+"/private_nics",
		map[string]any{"private_network_id": pn["id"]},
	)

	state := testutil.GetState(t, ts)
	instance := state["instance"].(map[string]any)
	nics := instance["private_nics"].([]any)
	require.Len(t, nics, 1)
	require.Equal(t, nic["id"], nics[0].(map[string]any)["id"])
}

func TestFKRejectionHTTP(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoCreate(t, ts,
		"/instance/v1/zones/fr-par-1/servers/nonexistent/private_nics",
		map[string]any{"private_network_id": "also-nonexistent"},
	)
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "referenced resource not found", body["message"])

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks",
		map[string]any{"name": "pn", "vpc_id": vpc["id"]})
	status = testutil.DoDelete(t, ts, "/vpc/v1/regions/fr-par/vpcs/"+vpc["id"].(string))
	require.Equal(t, 409, status)
}

func TestDeleteConflictForMultipleDependencies(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})
	_, server := testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "s"})
	testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers/"+server["id"].(string)+"/private_nics", map[string]any{
		"private_network_id": pn["id"],
	})
	status := testutil.DoDelete(t, ts, "/instance/v1/zones/fr-par-1/servers/"+server["id"].(string))
	require.Equal(t, 409, status)

	_, cluster := testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters", map[string]any{"name": "k"})
	testutil.DoCreate(t, ts, "/k8s/v1/regions/fr-par/clusters/"+cluster["id"].(string)+"/pools", map[string]any{"name": "p"})
	status = testutil.DoDelete(t, ts, "/k8s/v1/regions/fr-par/clusters/"+cluster["id"].(string))
	require.Equal(t, 409, status)

	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{"name": "db"})
	testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances/"+inst["id"].(string)+"/databases", map[string]any{"name": "appdb"})
	status = testutil.DoDelete(t, ts, "/rdb/v1/regions/fr-par/instances/"+inst["id"].(string))
	require.Equal(t, 409, status)
}

func TestUnknownServiceState404(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	status, body := testutil.DoGet(t, ts, "/mock/state/unknown")
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "unknown service", body["message"])
}

func TestRDBInitEndpointsValidationAndEnginePort(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/private-networks", map[string]any{
		"name": "pn", "vpc_id": vpc["id"],
	})

	status, body := testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":   "mysql-db",
		"engine": "MySQL-8",
		"init_endpoints": []any{
			map[string]any{
				"private_network": map[string]any{"id": pn["id"]},
			},
		},
	})
	require.Equal(t, 200, status)
	endpoints := body["endpoints"].([]any)
	require.Len(t, endpoints, 1)
	ep := endpoints[0].(map[string]any)
	require.Equal(t, float64(3306), ep["port"])
	require.Equal(t, pn["id"], ep["private_network"].(map[string]any)["id"])

	status, body = testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":   "bad-pn-db",
		"engine": "PostgreSQL-15",
		"init_endpoints": []any{
			map[string]any{
				"private_network": map[string]any{"id": "non-existent-pn"},
			},
		},
	})
	require.Equal(t, 404, status)
	require.Equal(t, "not_found", body["type"])
	require.Equal(t, "referenced resource not found", body["message"])

	status, body = testutil.DoCreate(t, ts, "/rdb/v1/regions/fr-par/instances", map[string]any{
		"name":   "public-mysql",
		"engine": "MySQL-8",
	})
	require.Equal(t, 200, status)
	endpoints = body["endpoints"].([]any)
	require.Len(t, endpoints, 1)
	ep = endpoints[0].(map[string]any)
	require.Equal(t, float64(3306), ep["port"])
}

func TestAdminResetAndStateShape(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	testutil.DoCreate(t, ts, "/vpc/v1/regions/fr-par/vpcs", map[string]any{"name": "v"})
	testutil.ResetState(t, ts)

	state := testutil.GetState(t, ts)
	require.Contains(t, state, "instance")
	require.Contains(t, state, "vpc")
	require.Contains(t, state, "lb")
	require.Contains(t, state, "k8s")
	require.Contains(t, state, "rdb")

	instance := state["instance"].(map[string]any)
	require.Len(t, instance["servers"].([]any), 0)
	vpc := state["vpc"].(map[string]any)
	require.Len(t, vpc["vpcs"].([]any), 0)
}

func TestServiceStateSuccessPath(t *testing.T) {
	ts, cleanup := testutil.NewTestServer(t)
	defer cleanup()

	testutil.DoCreate(t, ts, "/instance/v1/zones/fr-par-1/servers", map[string]any{"name": "s"})
	status, body := testutil.DoGet(t, ts, "/mock/state/instance")
	require.Equal(t, 200, status)
	require.Contains(t, body, "servers")
	require.Contains(t, body, "ips")
	require.Contains(t, body, "private_nics")
	require.Contains(t, body, "security_groups")
}

func TestResourceLifecyclesTableDriven(t *testing.T) {
	type lifecycleCase struct {
		name       string
		setup      func(t *testing.T, ts *httptest.Server, ctx map[string]string)
		createPath string
		listPath   string
		getPath    string
		deletePath string
		listKey    string
		body       map[string]any
		idField    string
	}

	cases := []lifecycleCase{
		{
			name:       "InstanceIPs",
			setup:      setupServer,
			createPath: "/instance/v1/zones/{zone}/ips",
			listPath:   "/instance/v1/zones/{zone}/ips",
			getPath:    "/instance/v1/zones/{zone}/ips/{id}",
			deletePath: "/instance/v1/zones/{zone}/ips/{id}",
			listKey:    "ips",
			body:       map[string]any{"server_id": "{server_id}"},
			idField:    "id",
		},
		{
			name:       "SecurityGroups",
			createPath: "/instance/v1/zones/{zone}/security_groups",
			listPath:   "/instance/v1/zones/{zone}/security_groups",
			getPath:    "/instance/v1/zones/{zone}/security_groups/{id}",
			deletePath: "/instance/v1/zones/{zone}/security_groups/{id}",
			listKey:    "security_groups",
			body:       map[string]any{"name": "sg-1"},
			idField:    "id",
		},
		{
			name:       "PrivateNICs",
			setup:      setupServerAndPrivateNetwork,
			createPath: "/instance/v1/zones/{zone}/servers/{server_id}/private_nics",
			listPath:   "/instance/v1/zones/{zone}/servers/{server_id}/private_nics",
			getPath:    "/instance/v1/zones/{zone}/servers/{server_id}/private_nics/{id}",
			deletePath: "/instance/v1/zones/{zone}/servers/{server_id}/private_nics/{id}",
			listKey:    "private_nics",
			body:       map[string]any{"private_network_id": "{pn_id}"},
			idField:    "id",
		},
		{
			name:       "VPCGetListDelete",
			createPath: "/vpc/v1/regions/{region}/vpcs",
			listPath:   "/vpc/v1/regions/{region}/vpcs",
			getPath:    "/vpc/v1/regions/{region}/vpcs/{id}",
			deletePath: "/vpc/v1/regions/{region}/vpcs/{id}",
			listKey:    "vpcs",
			body:       map[string]any{"name": "main"},
			idField:    "id",
		},
		{
			name:       "PrivateNetworkGetListDelete",
			setup:      setupVPC,
			createPath: "/vpc/v1/regions/{region}/private-networks",
			listPath:   "/vpc/v1/regions/{region}/private-networks",
			getPath:    "/vpc/v1/regions/{region}/private-networks/{id}",
			deletePath: "/vpc/v1/regions/{region}/private-networks/{id}",
			listKey:    "private_networks",
			body:       map[string]any{"name": "pn", "vpc_id": "{vpc_id}"},
			idField:    "id",
		},
		{
			name:       "LoadBalancers",
			createPath: "/lb/v1/zones/{zone}/lbs",
			listPath:   "/lb/v1/zones/{zone}/lbs",
			getPath:    "/lb/v1/zones/{zone}/lbs/{id}",
			deletePath: "/lb/v1/zones/{zone}/lbs/{id}",
			listKey:    "lbs",
			body:       map[string]any{"name": "lb"},
			idField:    "id",
		},
		{
			name:       "Frontends",
			setup:      setupLB,
			createPath: "/lb/v1/zones/{zone}/frontends",
			listPath:   "/lb/v1/zones/{zone}/frontends",
			getPath:    "/lb/v1/zones/{zone}/frontends/{id}",
			deletePath: "/lb/v1/zones/{zone}/frontends/{id}",
			listKey:    "frontends",
			body:       map[string]any{"name": "http", "lb_id": "{lb_id}"},
			idField:    "id",
		},
		{
			name:       "Backends",
			setup:      setupLB,
			createPath: "/lb/v1/zones/{zone}/backends",
			listPath:   "/lb/v1/zones/{zone}/backends",
			getPath:    "/lb/v1/zones/{zone}/backends/{id}",
			deletePath: "/lb/v1/zones/{zone}/backends/{id}",
			listKey:    "backends",
			body:       map[string]any{"name": "be", "lb_id": "{lb_id}"},
			idField:    "id",
		},
		{
			name:       "K8sClusters",
			createPath: "/k8s/v1/regions/{region}/clusters",
			listPath:   "/k8s/v1/regions/{region}/clusters",
			getPath:    "/k8s/v1/regions/{region}/clusters/{id}",
			deletePath: "/k8s/v1/regions/{region}/clusters/{id}",
			listKey:    "clusters",
			body:       map[string]any{"name": "k"},
			idField:    "id",
		},
		{
			name:       "K8sPools",
			setup:      setupCluster,
			createPath: "/k8s/v1/regions/{region}/clusters/{cluster_id}/pools",
			listPath:   "/k8s/v1/regions/{region}/clusters/{cluster_id}/pools",
			getPath:    "/k8s/v1/regions/{region}/pools/{id}",
			deletePath: "/k8s/v1/regions/{region}/pools/{id}",
			listKey:    "pools",
			body:       map[string]any{"name": "pool"},
			idField:    "id",
		},
		{
			name:       "RDBInstances",
			createPath: "/rdb/v1/regions/{region}/instances",
			listPath:   "/rdb/v1/regions/{region}/instances",
			getPath:    "/rdb/v1/regions/{region}/instances/{id}",
			deletePath: "/rdb/v1/regions/{region}/instances/{id}",
			listKey:    "instances",
			body:       map[string]any{"name": "rdb", "engine": "PostgreSQL-15"},
			idField:    "id",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ts, cleanup := testutil.NewTestServer(t)
			defer cleanup()

			ctx := map[string]string{"zone": "fr-par-1", "region": "fr-par"}
			if tt.setup != nil {
				tt.setup(t, ts, ctx)
			}

			status, body := testutil.DoCreate(t, ts, pathWithCtx(tt.createPath, ctx), bodyWithCtx(tt.body, ctx))
			require.Equal(t, 200, status)
			id := body[tt.idField].(string)
			require.NotEmpty(t, id)
			ctx["id"] = id

			status, body = testutil.DoGet(t, ts, pathWithCtx(tt.getPath, ctx))
			require.Equal(t, 200, status)
			require.Equal(t, id, body[tt.idField])

			status, body = testutil.DoList(t, ts, pathWithCtx(tt.listPath, ctx))
			require.Equal(t, 200, status)
			require.Equal(t, float64(1), body["total_count"])
			require.Len(t, body[tt.listKey].([]any), 1)

			status = testutil.DoDelete(t, ts, pathWithCtx(tt.deletePath, ctx))
			require.Equal(t, 204, status)

			status, _ = testutil.DoGet(t, ts, pathWithCtx(tt.getPath, ctx))
			require.Equal(t, 404, status)
		})
	}
}

func TestResourceLifecyclesWithoutGet(t *testing.T) {
	type noGetCase struct {
		name         string
		setup        func(t *testing.T, ts *httptest.Server, ctx map[string]string)
		createPath   string
		listPath     string
		deletePath   string
		listKey      string
		body         map[string]any
		deleteIDFrom string
	}

	cases := []noGetCase{
		{
			name:         "LBPrivateNetworkAttachment",
			setup:        setupLBAndPrivateNetwork,
			createPath:   "/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks",
			listPath:     "/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks",
			deletePath:   "/lb/v1/zones/{zone}/lbs/{lb_id}/private-networks/{delete_id}",
			listKey:      "private_networks",
			body:         map[string]any{"private_network_id": "{pn_id}"},
			deleteIDFrom: "private_network_id",
		},
		{
			name:         "RDBDatabases",
			setup:        setupRDBInstance,
			createPath:   "/rdb/v1/regions/{region}/instances/{instance_id}/databases",
			listPath:     "/rdb/v1/regions/{region}/instances/{instance_id}/databases",
			deletePath:   "/rdb/v1/regions/{region}/instances/{instance_id}/databases/{delete_id}",
			listKey:      "databases",
			body:         map[string]any{"name": "appdb"},
			deleteIDFrom: "name",
		},
		{
			name:         "RDBUsers",
			setup:        setupRDBInstance,
			createPath:   "/rdb/v1/regions/{region}/instances/{instance_id}/users",
			listPath:     "/rdb/v1/regions/{region}/instances/{instance_id}/users",
			deletePath:   "/rdb/v1/regions/{region}/instances/{instance_id}/users/{delete_id}",
			listKey:      "users",
			body:         map[string]any{"name": "appuser"},
			deleteIDFrom: "name",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ts, cleanup := testutil.NewTestServer(t)
			defer cleanup()

			ctx := map[string]string{"zone": "fr-par-1", "region": "fr-par"}
			if tt.setup != nil {
				tt.setup(t, ts, ctx)
			}

			status, body := testutil.DoCreate(t, ts, pathWithCtx(tt.createPath, ctx), bodyWithCtx(tt.body, ctx))
			require.Equal(t, 200, status)
			deleteID := body[tt.deleteIDFrom].(string)
			ctx["delete_id"] = deleteID

			status, body = testutil.DoList(t, ts, pathWithCtx(tt.listPath, ctx))
			require.Equal(t, 200, status)
			require.Equal(t, float64(1), body["total_count"])
			require.Len(t, body[tt.listKey].([]any), 1)

			status = testutil.DoDelete(t, ts, pathWithCtx(tt.deletePath, ctx))
			require.Equal(t, 204, status)

			status, body = testutil.DoList(t, ts, pathWithCtx(tt.listPath, ctx))
			require.Equal(t, 200, status)
			require.Equal(t, float64(0), body["total_count"])
		})
	}
}

func pathWithCtx(path string, ctx map[string]string) string {
	out := path
	for k, v := range ctx {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

func bodyWithCtx(body map[string]any, ctx map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range body {
		switch s := v.(type) {
		case string:
			out[k] = pathWithCtx(s, ctx)
		default:
			out[k] = v
		}
	}
	return out
}

func setupVPC(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, vpc := testutil.DoCreate(t, ts, "/vpc/v1/regions/"+ctx["region"]+"/vpcs", map[string]any{"name": "vpc"})
	ctx["vpc_id"] = vpc["id"].(string)
}

func setupServer(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, srv := testutil.DoCreate(t, ts, "/instance/v1/zones/"+ctx["zone"]+"/servers", map[string]any{"name": "srv"})
	ctx["server_id"] = srv["id"].(string)
}

func setupLB(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, lb := testutil.DoCreate(t, ts, "/lb/v1/zones/"+ctx["zone"]+"/lbs", map[string]any{"name": "lb"})
	ctx["lb_id"] = lb["id"].(string)
}

func setupCluster(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, cluster := testutil.DoCreate(t, ts, "/k8s/v1/regions/"+ctx["region"]+"/clusters", map[string]any{"name": "cluster"})
	ctx["cluster_id"] = cluster["id"].(string)
}

func setupRDBInstance(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	_, inst := testutil.DoCreate(t, ts, "/rdb/v1/regions/"+ctx["region"]+"/instances", map[string]any{"name": "rdb", "engine": "PostgreSQL-15"})
	ctx["instance_id"] = inst["id"].(string)
}

func setupServerAndPrivateNetwork(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	setupVPC(t, ts, ctx)
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/"+ctx["region"]+"/private-networks", map[string]any{
		"name": "pn", "vpc_id": ctx["vpc_id"],
	})
	ctx["pn_id"] = pn["id"].(string)
	setupServer(t, ts, ctx)
}

func setupLBAndPrivateNetwork(t *testing.T, ts *httptest.Server, ctx map[string]string) {
	t.Helper()
	setupLB(t, ts, ctx)
	setupVPC(t, ts, ctx)
	_, pn := testutil.DoCreate(t, ts, "/vpc/v1/regions/"+ctx["region"]+"/private-networks", map[string]any{
		"name": "pn", "vpc_id": ctx["vpc_id"],
	})
	ctx["pn_id"] = pn["id"].(string)
}
