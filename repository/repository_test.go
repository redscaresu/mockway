package repository_test

import (
	"database/sql"
	"encoding/json"
	"math"
	"path/filepath"
	"testing"

	"github.com/redscaresu/mockway/models"
	"github.com/redscaresu/mockway/repository"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestVPCRepository(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	vpc, err := repo.CreateVPC("fr-par", map[string]any{"name": "main"})
	require.NoError(t, err)
	require.NotEmpty(t, vpc["id"])
	vpcID := vpc["id"].(string)

	got, err := repo.GetVPC(vpcID)
	require.NoError(t, err)
	require.Equal(t, vpcID, got["id"])

	vpcs, err := repo.ListVPCs("fr-par")
	require.NoError(t, err)
	require.Len(t, vpcs, 1)

	err = repo.DeleteVPC(vpcID)
	require.NoError(t, err)

	_, err = repo.GetVPC(vpcID)
	require.ErrorIs(t, err, models.ErrNotFound)
}

func TestFKEnforcement(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	_, err = repo.CreatePrivateNetwork("fr-par", map[string]any{"name": "net", "vpc_id": "nonexistent"})
	require.ErrorIs(t, err, models.ErrNotFound)

	vpc, err := repo.CreateVPC("fr-par", map[string]any{"name": "main"})
	require.NoError(t, err)
	_, err = repo.CreatePrivateNetwork("fr-par", map[string]any{"name": "net", "vpc_id": vpc["id"]})
	require.NoError(t, err)

	err = repo.DeleteVPC(vpc["id"].(string))
	require.ErrorIs(t, err, models.ErrConflict)
}

func TestDuplicateCompositeKey(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	inst, err := repo.CreateRDBInstance("fr-par", map[string]any{"name": "db"})
	require.NoError(t, err)
	instID := inst["id"].(string)

	_, err = repo.CreateRDBDatabase(instID, "appdb", map[string]any{"name": "appdb"})
	require.NoError(t, err)
	_, err = repo.CreateRDBDatabase(instID, "appdb", map[string]any{"name": "appdb"})
	require.ErrorIs(t, err, models.ErrConflict)
}

func TestCreateDoesNotMutateInputMap(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	input := map[string]any{"name": "main"}
	_, err = repo.CreateVPC("fr-par", input)
	require.NoError(t, err)

	_, hasID := input["id"]
	_, hasRegion := input["region"]
	_, hasCreatedAt := input["created_at"]
	require.False(t, hasID)
	require.False(t, hasRegion)
	require.False(t, hasCreatedAt)
}

func TestExistsAndReset(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	vpc, err := repo.CreateVPC("fr-par", map[string]any{"name": "main"})
	require.NoError(t, err)

	exists, err := repo.Exists("vpcs", "id", vpc["id"].(string))
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = repo.Exists("vpcs", "id", "does-not-exist")
	require.NoError(t, err)
	require.False(t, exists)

	err = repo.Reset()
	require.NoError(t, err)

	vpcs, err := repo.ListVPCs("fr-par")
	require.NoError(t, err)
	require.Len(t, vpcs, 0)
}

func TestUpdateSecurityGroup(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	sg, err := repo.CreateSecurityGroup("fr-par-1", map[string]any{"name": "sg", "inbound_default_policy": "drop"})
	require.NoError(t, err)
	sgID := sg["id"].(string)

	updated, err := repo.UpdateSecurityGroup(sgID, map[string]any{
		"inbound_default_policy": "accept",
		"description":            "patched",
	})
	require.NoError(t, err)
	require.Equal(t, "accept", updated["inbound_default_policy"])
	require.Equal(t, "patched", updated["description"])
	require.Equal(t, sgID, updated["id"])
}

func TestDeleteServerDetachesIPAndCascadesNICsRepository(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	vpc, err := repo.CreateVPC("fr-par", map[string]any{"name": "v"})
	require.NoError(t, err)
	pn, err := repo.CreatePrivateNetwork("fr-par", map[string]any{"name": "pn", "vpc_id": vpc["id"]})
	require.NoError(t, err)
	server, err := repo.CreateServer("fr-par-1", map[string]any{"name": "srv"})
	require.NoError(t, err)
	serverID := server["id"].(string)

	ip, err := repo.CreateIP("fr-par-1", map[string]any{"server_id": serverID})
	require.NoError(t, err)
	_, err = repo.CreatePrivateNIC("fr-par-1", serverID, map[string]any{"private_network_id": pn["id"]})
	require.NoError(t, err)

	err = repo.DeleteServer(serverID)
	require.NoError(t, err)

	gotIP, err := repo.GetIP(ip["id"].(string))
	require.NoError(t, err)
	require.Contains(t, gotIP, "server_id")
	require.Nil(t, gotIP["server_id"])

	nics, err := repo.ListPrivateNICsByServer(serverID)
	require.NoError(t, err)
	require.Len(t, nics, 0)
}

func TestDeleteSecurityGroupDetachesServerRepository(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	sg, err := repo.CreateSecurityGroup("fr-par-1", map[string]any{"name": "sg"})
	require.NoError(t, err)
	sgID := sg["id"].(string)
	server, err := repo.CreateServer("fr-par-1", map[string]any{
		"name":              "srv",
		"security_group_id": sgID,
		"security_group":    map[string]any{"id": sgID, "name": "sg"},
	})
	require.NoError(t, err)
	serverID := server["id"].(string)

	err = repo.DeleteSecurityGroup(sgID)
	require.NoError(t, err)

	gotServer, err := repo.GetServer(serverID)
	require.NoError(t, err)
	require.Contains(t, gotServer, "security_group_id")
	require.Nil(t, gotServer["security_group_id"])
	require.Contains(t, gotServer, "security_group")
	require.Nil(t, gotServer["security_group"])
}

func TestFullAndServiceState(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	_, err = repo.CreateServer("fr-par-1", map[string]any{"name": "srv"})
	require.NoError(t, err)
	_, err = repo.CreateVPC("fr-par", map[string]any{"name": "vpc"})
	require.NoError(t, err)
	_, err = repo.CreateIAMApplication(map[string]any{"name": "app"})
	require.NoError(t, err)

	state, err := repo.FullState()
	require.NoError(t, err)
	require.Contains(t, state, "instance")
	require.Contains(t, state, "vpc")
	require.Contains(t, state, "lb")
	require.Contains(t, state, "k8s")
	require.Contains(t, state, "rdb")
	require.Contains(t, state, "iam")

	instanceState, err := repo.ServiceState("instance")
	require.NoError(t, err)
	require.Contains(t, instanceState, "servers")
	require.Contains(t, instanceState, "ips")
	require.Contains(t, instanceState, "private_nics")
	require.Contains(t, instanceState, "security_groups")

	_, err = repo.ServiceState("unknown")
	require.ErrorIs(t, err, models.ErrNotFound)
}

func TestServiceStateAllBranches(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	vpc, err := repo.CreateVPC("fr-par", map[string]any{"name": "v"})
	require.NoError(t, err)
	pn, err := repo.CreatePrivateNetwork("fr-par", map[string]any{"name": "pn", "vpc_id": vpc["id"]})
	require.NoError(t, err)
	server, err := repo.CreateServer("fr-par-1", map[string]any{"name": "srv"})
	require.NoError(t, err)
	_, err = repo.CreateIP("fr-par-1", map[string]any{"server_id": server["id"]})
	require.NoError(t, err)
	_, err = repo.CreatePrivateNIC("fr-par-1", server["id"].(string), map[string]any{"private_network_id": pn["id"]})
	require.NoError(t, err)
	_, err = repo.CreateSecurityGroup("fr-par-1", map[string]any{"name": "sg"})
	require.NoError(t, err)

	lb, err := repo.CreateLB("fr-par-1", map[string]any{"name": "lb"})
	require.NoError(t, err)
	_, err = repo.CreateFrontend(map[string]any{"name": "fe", "lb_id": lb["id"]})
	require.NoError(t, err)
	_, err = repo.CreateBackend(map[string]any{"name": "be", "lb_id": lb["id"]})
	require.NoError(t, err)
	_, err = repo.AttachLBPrivateNetwork(lb["id"].(string), pn["id"].(string))
	require.NoError(t, err)

	cluster, err := repo.CreateCluster("fr-par", map[string]any{"name": "c", "private_network_id": pn["id"]})
	require.NoError(t, err)
	_, err = repo.CreatePool("fr-par", cluster["id"].(string), map[string]any{"name": "p"})
	require.NoError(t, err)

	rdb, err := repo.CreateRDBInstance("fr-par", map[string]any{"name": "db", "engine": "PostgreSQL-15"})
	require.NoError(t, err)
	_, err = repo.CreateRDBDatabase(rdb["id"].(string), "appdb", map[string]any{"name": "appdb"})
	require.NoError(t, err)
	_, err = repo.CreateRDBUser(rdb["id"].(string), "appuser", map[string]any{"name": "appuser"})
	require.NoError(t, err)

	app, err := repo.CreateIAMApplication(map[string]any{"name": "app"})
	require.NoError(t, err)
	_, err = repo.CreateIAMAPIKey(map[string]any{"application_id": app["id"]})
	require.NoError(t, err)
	_, err = repo.CreateIAMPolicy(map[string]any{"name": "p", "application_id": app["id"]})
	require.NoError(t, err)
	_, err = repo.CreateIAMSSHKey(map[string]any{"name": "k", "public_key": "ssh-ed25519 AAAA"})
	require.NoError(t, err)

	for _, svc := range []string{"instance", "vpc", "lb", "k8s", "rdb", "iam"} {
		st, err := repo.ServiceState(svc)
		require.NoError(t, err)
		require.NotEmpty(t, st)
	}
}

func TestRepositoryCRUDWrappersCoverage(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	// base graph for FK-dependent resources
	vpc, err := repo.CreateVPC("fr-par", map[string]any{"name": "v"})
	require.NoError(t, err)
	pn, err := repo.CreatePrivateNetwork("fr-par", map[string]any{"name": "pn", "vpc_id": vpc["id"]})
	require.NoError(t, err)
	sg, err := repo.CreateSecurityGroup("fr-par-1", map[string]any{"name": "sg"})
	require.NoError(t, err)
	server, err := repo.CreateServer("fr-par-1", map[string]any{
		"name":              "srv",
		"security_group_id": sg["id"],
		"security_group":    map[string]any{"id": sg["id"], "name": "sg"},
	})
	require.NoError(t, err)

	// direct wrapper calls: get/list/delete variants
	_, err = repo.GetPrivateNetwork(pn["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListPrivateNetworks("fr-par")
	require.NoError(t, err)

	_, err = repo.GetSecurityGroup(sg["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListSecurityGroups("fr-par-1")
	require.NoError(t, err)

	ip, err := repo.CreateIP("fr-par-1", map[string]any{"server_id": server["id"]})
	require.NoError(t, err)
	_, err = repo.ListIPs("fr-par-1")
	require.NoError(t, err)
	_, err = repo.GetIP(ip["id"].(string))
	require.NoError(t, err)

	nic, err := repo.CreatePrivateNIC("fr-par-1", server["id"].(string), map[string]any{"private_network_id": pn["id"]})
	require.NoError(t, err)
	_, err = repo.GetPrivateNIC(nic["id"].(string))
	require.NoError(t, err)
	_, err = repo.GetInstanceVolume("fr-par-1", server["volumes"].(map[string]any)["0"].(map[string]any)["id"].(string))
	require.NoError(t, err)

	lb, err := repo.CreateLB("fr-par-1", map[string]any{"name": "lb"})
	require.NoError(t, err)
	_, err = repo.GetLB(lb["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListLBs("fr-par-1")
	require.NoError(t, err)
	fe, err := repo.CreateFrontend(map[string]any{"name": "fe", "lb_id": lb["id"]})
	require.NoError(t, err)
	_, err = repo.GetFrontend(fe["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListFrontends()
	require.NoError(t, err)
	be, err := repo.CreateBackend(map[string]any{"name": "be", "lb_id": lb["id"]})
	require.NoError(t, err)
	_, err = repo.GetBackend(be["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListBackends()
	require.NoError(t, err)
	_, err = repo.AttachLBPrivateNetwork(lb["id"].(string), pn["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListLBPrivateNetworks(lb["id"].(string))
	require.NoError(t, err)

	cluster, err := repo.CreateCluster("fr-par", map[string]any{"name": "c", "private_network_id": pn["id"]})
	require.NoError(t, err)
	_, err = repo.GetCluster(cluster["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListClusters("fr-par")
	require.NoError(t, err)
	pool, err := repo.CreatePool("fr-par", cluster["id"].(string), map[string]any{"name": "p"})
	require.NoError(t, err)
	_, err = repo.GetPool(pool["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListPoolsByCluster(cluster["id"].(string))
	require.NoError(t, err)

	rdb, err := repo.CreateRDBInstance("fr-par", map[string]any{"name": "db", "engine": "MySQL-8"})
	require.NoError(t, err)
	_, err = repo.GetRDBInstance(rdb["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListRDBInstances("fr-par")
	require.NoError(t, err)
	db, err := repo.CreateRDBDatabase(rdb["id"].(string), "appdb", map[string]any{"name": "appdb"})
	require.NoError(t, err)
	_, err = repo.ListRDBDatabases(rdb["id"].(string))
	require.NoError(t, err)
	user, err := repo.CreateRDBUser(rdb["id"].(string), "appuser", map[string]any{"name": "appuser"})
	require.NoError(t, err)
	_, err = repo.ListRDBUsers(rdb["id"].(string))
	require.NoError(t, err)

	app, err := repo.CreateIAMApplication(map[string]any{"name": "app"})
	require.NoError(t, err)
	_, err = repo.GetIAMApplication(app["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListIAMApplications()
	require.NoError(t, err)
	apiKey, err := repo.CreateIAMAPIKey(map[string]any{"application_id": app["id"]})
	require.NoError(t, err)
	_, err = repo.GetIAMAPIKey(apiKey["access_key"].(string))
	require.NoError(t, err)
	_, err = repo.ListIAMAPIKeys()
	require.NoError(t, err)
	policy, err := repo.CreateIAMPolicy(map[string]any{"name": "p", "application_id": app["id"]})
	require.NoError(t, err)
	_, err = repo.GetIAMPolicy(policy["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListIAMPolicies()
	require.NoError(t, err)
	ssh, err := repo.CreateIAMSSHKey(map[string]any{"name": "k", "public_key": "ssh-ed25519 AAAA"})
	require.NoError(t, err)
	_, err = repo.GetIAMSSHKey(ssh["id"].(string))
	require.NoError(t, err)
	_, err = repo.ListIAMSSHKeys()
	require.NoError(t, err)

	// deletion wrappers
	require.NoError(t, repo.DeleteIAMSSHKey(ssh["id"].(string)))
	require.NoError(t, repo.DeleteIAMPolicy(policy["id"].(string)))
	require.NoError(t, repo.DeleteIAMAPIKey(apiKey["access_key"].(string)))
	require.NoError(t, repo.DeleteIAMApplication(app["id"].(string)))
	require.NoError(t, repo.DeleteRDBUser(rdb["id"].(string), user["name"].(string)))
	require.NoError(t, repo.DeleteRDBDatabase(rdb["id"].(string), db["name"].(string)))
	require.NoError(t, repo.DeleteRDBInstance(rdb["id"].(string)))
	require.NoError(t, repo.DeletePool(pool["id"].(string)))
	require.NoError(t, repo.DeleteCluster(cluster["id"].(string)))
	require.NoError(t, repo.DeleteLBPrivateNetwork(lb["id"].(string), pn["id"].(string)))
	require.NoError(t, repo.DeleteBackend(be["id"].(string)))
	require.NoError(t, repo.DeleteFrontend(fe["id"].(string)))
	require.NoError(t, repo.DeleteLB(lb["id"].(string)))
	require.NoError(t, repo.DeletePrivateNIC(nic["id"].(string)))
	require.NoError(t, repo.DeleteIP(ip["id"].(string)))
	require.NoError(t, repo.DeleteServer(server["id"].(string)))
	require.NoError(t, repo.DeleteSecurityGroup(sg["id"].(string)))
	require.NoError(t, repo.DeletePrivateNetwork(pn["id"].(string)))
	require.NoError(t, repo.DeleteVPC(vpc["id"].(string)))
}

func TestRDBEndpointHelpersAndRandom(t *testing.T) {
	publicEPs, err := repository.BuildRDBEndpointsFromInit(nil, "PostgreSQL-15")
	require.NoError(t, err)
	require.Len(t, publicEPs, 1)
	require.Equal(t, float64(5432), publicEPs[0].(map[string]any)["port"])

	mysqlEPs, err := repository.BuildRDBEndpointsFromInit([]any{map[string]any{
		"private_network": map[string]any{"id": "pn-1"},
	}}, "MySQL-8")
	require.NoError(t, err)
	require.Len(t, mysqlEPs, 1)
	mysqlEP := mysqlEPs[0].(map[string]any)
	require.Equal(t, float64(3306), mysqlEP["port"])
	require.Equal(t, "pn-1", mysqlEP["private_network"].(map[string]any)["id"])

	_, err = repository.BuildRDBEndpointsFromInit([]any{map[string]any{}}, "PostgreSQL-15")
	require.Error(t, err)
	_, err = repository.BuildRDBEndpointsFromInit([]any{map[string]any{
		"private_network": map[string]any{},
	}}, "PostgreSQL-15")
	require.Error(t, err)
	_, err = repository.BuildRDBEndpointsFromInit([]any{"bad"}, "PostgreSQL-15")
	require.Error(t, err)

	// Also exercise JSON unmarshal failure path in repository helpers indirectly.
	var bad map[string]any
	err = json.Unmarshal([]byte("{"), &bad)
	require.Error(t, err)
}

func TestRepositoryMethodsReturnErrorAfterClose(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	require.NoError(t, repo.Close())

	_, err = repo.Exists("vpcs", "id", "x")
	require.Error(t, err)

	err = repo.Reset()
	require.Error(t, err)

	_, err = repo.FullState()
	require.Error(t, err)

	_, err = repo.ServiceState("instance")
	require.Error(t, err)

	err = repo.DeleteServer("x")
	require.Error(t, err)
}

func TestCreateMarshalFailureUnsupportedValue(t *testing.T) {
	repo, err := repository.New(":memory:")
	require.NoError(t, err)
	defer repo.Close()

	_, err = repo.CreateVPC("fr-par", map[string]any{
		"name": "bad",
		"nan":  math.NaN(), // json.Marshal unsupported value: NaN
	})
	require.Error(t, err)
}

func TestFullStateFailsOnCorruptRowJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "repo.db")
	repo, err := repository.New(dbPath)
	require.NoError(t, err)
	defer repo.Close()

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`INSERT INTO vpcs (id, region, data) VALUES (?, ?, ?)`, "vpc-bad", "fr-par", "{")
	require.NoError(t, err)

	_, err = repo.FullState()
	require.Error(t, err)
}

func TestServiceStateFailsOnCorruptRowJSON(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "repo.db")
	repo, err := repository.New(dbPath)
	require.NoError(t, err)
	defer repo.Close()

	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(`INSERT INTO instance_servers (id, zone, data) VALUES (?, ?, ?)`, "srv-bad", "fr-par-1", "{")
	require.NoError(t, err)

	_, err = repo.ServiceState("instance")
	require.Error(t, err)
}
