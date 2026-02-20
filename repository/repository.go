package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redscaresu/mockway/models"
	_ "modernc.org/sqlite"
)

type Repository struct {
	db *sql.DB
}

type colVal struct {
	name string
	val  any
}

func New(path string) (*Repository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	r := &Repository{db: db}
	if err := r.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

func (r *Repository) Close() error {
	return r.db.Close()
}

func (r *Repository) init() error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS vpcs (
			id TEXT PRIMARY KEY,
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS private_networks (
			id TEXT PRIMARY KEY,
			vpc_id TEXT NOT NULL REFERENCES vpcs(id),
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS instance_security_groups (
			id TEXT PRIMARY KEY,
			zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS instance_servers (
			id TEXT PRIMARY KEY,
			zone TEXT NOT NULL,
			security_group_id TEXT REFERENCES instance_security_groups(id),
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS instance_ips (
			id TEXT PRIMARY KEY,
			server_id TEXT REFERENCES instance_servers(id),
			zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS instance_private_nics (
			id TEXT PRIMARY KEY,
			server_id TEXT NOT NULL REFERENCES instance_servers(id),
			private_network_id TEXT NOT NULL REFERENCES private_networks(id),
			zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lbs (
			id TEXT PRIMARY KEY,
			zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lb_frontends (
			id TEXT PRIMARY KEY,
			lb_id TEXT NOT NULL REFERENCES lbs(id),
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lb_backends (
			id TEXT PRIMARY KEY,
			lb_id TEXT NOT NULL REFERENCES lbs(id),
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lb_private_networks (
			lb_id TEXT NOT NULL REFERENCES lbs(id),
			private_network_id TEXT NOT NULL REFERENCES private_networks(id),
			data JSON NOT NULL,
			PRIMARY KEY (lb_id, private_network_id)
		)`,
		`CREATE TABLE IF NOT EXISTS k8s_clusters (
			id TEXT PRIMARY KEY,
			region TEXT NOT NULL,
			private_network_id TEXT REFERENCES private_networks(id),
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS k8s_pools (
			id TEXT PRIMARY KEY,
			cluster_id TEXT NOT NULL REFERENCES k8s_clusters(id),
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_instances (
			id TEXT PRIMARY KEY,
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_databases (
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id),
			name TEXT NOT NULL,
			data JSON NOT NULL,
			PRIMARY KEY (instance_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_users (
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id),
			name TEXT NOT NULL,
			data JSON NOT NULL,
			PRIMARY KEY (instance_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS iam_applications (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS iam_api_keys (
			access_key TEXT PRIMARY KEY,
			application_id TEXT REFERENCES iam_applications(id),
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS iam_policies (
			id TEXT PRIMARY KEY,
			application_id TEXT REFERENCES iam_applications(id),
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS iam_ssh_keys (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := r.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}

	return nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func newID() string {
	return uuid.NewString()
}

func (r *Repository) Exists(table, idColumn, id string) (bool, error) {
	q := fmt.Sprintf("SELECT 1 FROM %s WHERE %s = ? LIMIT 1", table, idColumn)
	var one int
	err := r.db.QueryRow(q, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) Reset() error {
	tables := []string{
		"lb_private_networks",
		"lb_frontends",
		"lb_backends",
		"lbs",
		"instance_private_nics",
		"instance_ips",
		"instance_servers",
		"instance_security_groups",
		"k8s_pools",
		"k8s_clusters",
		"rdb_databases",
		"rdb_users",
		"rdb_instances",
		"iam_api_keys",
		"iam_policies",
		"iam_ssh_keys",
		"iam_applications",
		"private_networks",
		"vpcs",
	}

	if _, err := r.db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() {
		_, _ = r.db.Exec(`PRAGMA foreign_keys = ON`)
	}()

	for _, t := range tables {
		if _, err := r.db.Exec(fmt.Sprintf("DELETE FROM %s", t)); err != nil {
			return err
		}
	}
	return nil
}

func marshalData(data map[string]any) ([]byte, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func unmarshalData(raw []byte) (map[string]any, error) {
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func mapInsertSQLError(err error) error {
	if err == nil {
		return nil
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "FOREIGN KEY constraint failed"):
		return models.ErrNotFound
	case strings.Contains(s, "UNIQUE constraint failed"):
		return models.ErrConflict
	default:
		return err
	}
}

func mapDeleteSQLError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		return models.ErrConflict
	}
	return err
}

func (r *Repository) insertJSON(table string, cols []colVal, data map[string]any) error {
	b, err := marshalData(data)
	if err != nil {
		return err
	}

	allCols := make([]string, 0, len(cols)+1)
	args := make([]any, 0, len(cols)+1)
	placeholders := make([]string, 0, len(cols)+1)
	for _, c := range cols {
		allCols = append(allCols, c.name)
		args = append(args, c.val)
		placeholders = append(placeholders, "?")
	}
	allCols = append(allCols, "data")
	args = append(args, b)
	placeholders = append(placeholders, "?")

	q := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(allCols, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err = r.db.Exec(q, args...)
	return mapInsertSQLError(err)
}

func (r *Repository) getJSONByID(table, idColumn, id string) (map[string]any, error) {
	q := fmt.Sprintf("SELECT data FROM %s WHERE %s = ?", table, idColumn)
	var raw []byte
	err := r.db.QueryRow(q, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return unmarshalData(raw)
}

func (r *Repository) listJSON(table, whereCol, whereVal string) ([]map[string]any, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if whereCol == "" {
		q := fmt.Sprintf("SELECT data FROM %s", table)
		rows, err = r.db.Query(q)
	} else {
		q := fmt.Sprintf("SELECT data FROM %s WHERE %s = ?", table, whereCol)
		rows, err = r.db.Query(q, whereVal)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		v, err := unmarshalData(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) deleteBy(table, whereClause string, args ...any) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE %s", table, whereClause)
	res, err := r.db.Exec(q, args...)
	if err != nil {
		return mapDeleteSQLError(err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) updateJSONByID(table, idColumn, id string, data map[string]any) error {
	b, err := marshalData(data)
	if err != nil {
		return err
	}
	q := fmt.Sprintf("UPDATE %s SET data = ? WHERE %s = ?", table, idColumn)
	res, err := r.db.Exec(q, b, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (r *Repository) createSimple(table, scopeCol, scopeVal string, data map[string]any, extra ...colVal) (map[string]any, error) {
	data = cloneMap(data)
	id := newID()
	data["id"] = id

	cols := []colVal{{name: "id", val: id}, {name: scopeCol, val: scopeVal}}
	cols = append(cols, extra...)
	if err := r.insertJSON(table, cols, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) CreateVPC(region string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	data["region"] = region
	return r.createSimple("vpcs", "region", region, data)
}

func (r *Repository) GetVPC(id string) (map[string]any, error) {
	return r.getJSONByID("vpcs", "id", id)
}
func (r *Repository) ListVPCs(region string) ([]map[string]any, error) {
	return r.listJSON("vpcs", "region", region)
}
func (r *Repository) DeleteVPC(id string) error { return r.deleteBy("vpcs", "id = ?", id) }

func (r *Repository) CreatePrivateNetwork(region string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	data["region"] = region
	vpcID, _ := data["vpc_id"].(string)
	return r.createSimple("private_networks", "region", region, data, colVal{name: "vpc_id", val: vpcID})
}

func (r *Repository) GetPrivateNetwork(id string) (map[string]any, error) {
	return r.getJSONByID("private_networks", "id", id)
}
func (r *Repository) ListPrivateNetworks(region string) ([]map[string]any, error) {
	return r.listJSON("private_networks", "region", region)
}
func (r *Repository) DeletePrivateNetwork(id string) error {
	return r.deleteBy("private_networks", "id = ?", id)
}

func (r *Repository) CreateSecurityGroup(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	data["zone"] = zone
	return r.createSimple("instance_security_groups", "zone", zone, data)
}
func (r *Repository) GetSecurityGroup(id string) (map[string]any, error) {
	return r.getJSONByID("instance_security_groups", "id", id)
}
func (r *Repository) ListSecurityGroups(zone string) ([]map[string]any, error) {
	return r.listJSON("instance_security_groups", "zone", zone)
}
func (r *Repository) DeleteSecurityGroup(id string) error {
	return r.deleteBy("instance_security_groups", "id = ?", id)
}

func (r *Repository) UpdateSecurityGroup(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("instance_security_groups", "id", id)
	if err != nil {
		return nil, err
	}
	next := cloneMap(current)
	for k, v := range patch {
		if k == "id" {
			continue
		}
		next[k] = v
	}
	if err := r.updateJSONByID("instance_security_groups", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) CreateServer(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["zone"] = zone
	data["state"] = "running"
	data["creation_date"] = now
	data["modification_date"] = now
	sgID, _ := data["security_group_id"].(string)
	var extras []colVal
	if sgID != "" {
		extras = append(extras, colVal{name: "security_group_id", val: sgID})
	}
	return r.createSimple("instance_servers", "zone", zone, data, extras...)
}
func (r *Repository) GetServer(id string) (map[string]any, error) {
	return r.getJSONByID("instance_servers", "id", id)
}
func (r *Repository) ListServers(zone string) ([]map[string]any, error) {
	return r.listJSON("instance_servers", "zone", zone)
}
func (r *Repository) DeleteServer(id string) error {
	return r.deleteBy("instance_servers", "id = ?", id)
}

func (r *Repository) CreateIP(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	data["zone"] = zone
	data["address"] = fakePublicIP()
	serverID, _ := data["server_id"].(string)
	var extras []colVal
	if serverID != "" {
		extras = append(extras, colVal{name: "server_id", val: serverID})
	}
	return r.createSimple("instance_ips", "zone", zone, data, extras...)
}
func (r *Repository) GetIP(id string) (map[string]any, error) {
	return r.getJSONByID("instance_ips", "id", id)
}
func (r *Repository) ListIPs(zone string) ([]map[string]any, error) {
	return r.listJSON("instance_ips", "zone", zone)
}
func (r *Repository) DeleteIP(id string) error { return r.deleteBy("instance_ips", "id = ?", id) }

func (r *Repository) CreatePrivateNIC(zone, serverID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	data["server_id"] = serverID
	data["zone"] = zone
	pnID, _ := data["private_network_id"].(string)
	return r.createSimple(
		"instance_private_nics",
		"zone",
		zone,
		data,
		colVal{name: "server_id", val: serverID},
		colVal{name: "private_network_id", val: pnID},
	)
}
func (r *Repository) GetPrivateNIC(id string) (map[string]any, error) {
	return r.getJSONByID("instance_private_nics", "id", id)
}
func (r *Repository) ListPrivateNICsByServer(serverID string) ([]map[string]any, error) {
	return r.listJSON("instance_private_nics", "server_id", serverID)
}
func (r *Repository) DeletePrivateNIC(id string) error {
	return r.deleteBy("instance_private_nics", "id = ?", id)
}

func (r *Repository) CreateLB(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["zone"] = zone
	data["status"] = "ready"
	data["created_at"] = now
	id := newID()
	data["id"] = id
	data["ip"] = []any{map[string]any{
		"id":         newID(),
		"ip_address": fakePublicIP(),
		"lb_id":      id,
	}}
	if err := r.insertJSON("lbs", []colVal{{name: "id", val: id}, {name: "zone", val: zone}}, data); err != nil {
		return nil, err
	}
	return data, nil
}
func (r *Repository) GetLB(id string) (map[string]any, error) { return r.getJSONByID("lbs", "id", id) }
func (r *Repository) ListLBs(zone string) ([]map[string]any, error) {
	return r.listJSON("lbs", "zone", zone)
}
func (r *Repository) DeleteLB(id string) error { return r.deleteBy("lbs", "id = ?", id) }

func (r *Repository) CreateFrontend(data map[string]any) (map[string]any, error) {
	lbID, _ := data["lb_id"].(string)
	return r.createSimple("lb_frontends", "lb_id", lbID, data)
}
func (r *Repository) GetFrontend(id string) (map[string]any, error) {
	return r.getJSONByID("lb_frontends", "id", id)
}
func (r *Repository) ListFrontends() ([]map[string]any, error) {
	return r.listJSON("lb_frontends", "", "")
}
func (r *Repository) DeleteFrontend(id string) error { return r.deleteBy("lb_frontends", "id = ?", id) }

func (r *Repository) CreateBackend(data map[string]any) (map[string]any, error) {
	lbID, _ := data["lb_id"].(string)
	return r.createSimple("lb_backends", "lb_id", lbID, data)
}
func (r *Repository) GetBackend(id string) (map[string]any, error) {
	return r.getJSONByID("lb_backends", "id", id)
}
func (r *Repository) ListBackends() ([]map[string]any, error) {
	return r.listJSON("lb_backends", "", "")
}
func (r *Repository) DeleteBackend(id string) error { return r.deleteBy("lb_backends", "id = ?", id) }

func (r *Repository) AttachLBPrivateNetwork(lbID, privateNetworkID string) (map[string]any, error) {
	data := map[string]any{"lb_id": lbID, "private_network_id": privateNetworkID}
	if err := r.insertJSON("lb_private_networks", []colVal{{name: "lb_id", val: lbID}, {name: "private_network_id", val: privateNetworkID}}, data); err != nil {
		return nil, err
	}
	return data, nil
}
func (r *Repository) ListLBPrivateNetworks(lbID string) ([]map[string]any, error) {
	return r.listJSON("lb_private_networks", "lb_id", lbID)
}
func (r *Repository) DeleteLBPrivateNetwork(lbID, privateNetworkID string) error {
	return r.deleteBy("lb_private_networks", "lb_id = ? AND private_network_id = ?", lbID, privateNetworkID)
}

func (r *Repository) CreateCluster(region string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["region"] = region
	data["status"] = "ready"
	data["created_at"] = now
	data["updated_at"] = now
	pnID, _ := data["private_network_id"].(string)
	var extras []colVal
	if pnID != "" {
		extras = append(extras, colVal{name: "private_network_id", val: pnID})
	}
	return r.createSimple("k8s_clusters", "region", region, data, extras...)
}
func (r *Repository) GetCluster(id string) (map[string]any, error) {
	return r.getJSONByID("k8s_clusters", "id", id)
}
func (r *Repository) ListClusters(region string) ([]map[string]any, error) {
	return r.listJSON("k8s_clusters", "region", region)
}
func (r *Repository) DeleteCluster(id string) error { return r.deleteBy("k8s_clusters", "id = ?", id) }

func (r *Repository) CreatePool(region, clusterID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["region"] = region
	data["cluster_id"] = clusterID
	data["status"] = "ready"
	data["created_at"] = now
	data["updated_at"] = now
	return r.createSimple("k8s_pools", "region", region, data, colVal{name: "cluster_id", val: clusterID})
}
func (r *Repository) GetPool(id string) (map[string]any, error) {
	return r.getJSONByID("k8s_pools", "id", id)
}
func (r *Repository) ListPoolsByCluster(clusterID string) ([]map[string]any, error) {
	return r.listJSON("k8s_pools", "cluster_id", clusterID)
}
func (r *Repository) DeletePool(id string) error { return r.deleteBy("k8s_pools", "id = ?", id) }

func (r *Repository) CreateRDBInstance(region string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["region"] = region
	data["status"] = "ready"
	data["created_at"] = now
	port := rdbPortFromEngine(data["engine"])
	if _, ok := data["endpoints"]; !ok {
		data["endpoints"] = []any{map[string]any{"ip": fakePublicIP(), "port": port}}
	}
	return r.createSimple("rdb_instances", "region", region, data)
}
func (r *Repository) GetRDBInstance(id string) (map[string]any, error) {
	return r.getJSONByID("rdb_instances", "id", id)
}
func (r *Repository) ListRDBInstances(region string) ([]map[string]any, error) {
	return r.listJSON("rdb_instances", "region", region)
}
func (r *Repository) DeleteRDBInstance(id string) error {
	return r.deleteBy("rdb_instances", "id = ?", id)
}

func (r *Repository) CreateRDBDatabase(instanceID, name string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	data["instance_id"] = instanceID
	data["name"] = name
	if err := r.insertJSON("rdb_databases", []colVal{{name: "instance_id", val: instanceID}, {name: "name", val: name}}, data); err != nil {
		return nil, err
	}
	return data, nil
}
func (r *Repository) ListRDBDatabases(instanceID string) ([]map[string]any, error) {
	return r.listJSON("rdb_databases", "instance_id", instanceID)
}
func (r *Repository) DeleteRDBDatabase(instanceID, name string) error {
	return r.deleteBy("rdb_databases", "instance_id = ? AND name = ?", instanceID, name)
}

func (r *Repository) CreateRDBUser(instanceID, name string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	data["instance_id"] = instanceID
	data["name"] = name
	if err := r.insertJSON("rdb_users", []colVal{{name: "instance_id", val: instanceID}, {name: "name", val: name}}, data); err != nil {
		return nil, err
	}
	return data, nil
}
func (r *Repository) ListRDBUsers(instanceID string) ([]map[string]any, error) {
	return r.listJSON("rdb_users", "instance_id", instanceID)
}
func (r *Repository) DeleteRDBUser(instanceID, name string) error {
	return r.deleteBy("rdb_users", "instance_id = ? AND name = ?", instanceID, name)
}

func (r *Repository) CreateIAMApplication(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	id := newID()
	data["id"] = id
	if err := r.insertJSON("iam_applications", []colVal{{name: "id", val: id}}, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) GetIAMApplication(id string) (map[string]any, error) {
	return r.getJSONByID("iam_applications", "id", id)
}

func (r *Repository) ListIAMApplications() ([]map[string]any, error) {
	return r.listJSON("iam_applications", "", "")
}

func (r *Repository) DeleteIAMApplication(id string) error {
	return r.deleteBy("iam_applications", "id = ?", id)
}

func (r *Repository) CreateIAMAPIKey(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	accessKey := "SCW" + randomAlphaNum(17)
	data["access_key"] = accessKey
	data["secret_key"] = newID()
	data["created_at"] = now
	data["updated_at"] = now

	var appID any
	if v, ok := data["application_id"].(string); ok && strings.TrimSpace(v) != "" {
		appID = v
	} else {
		appID = nil
	}

	if err := r.insertJSON("iam_api_keys", []colVal{{name: "access_key", val: accessKey}, {name: "application_id", val: appID}}, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) GetIAMAPIKey(accessKey string) (map[string]any, error) {
	out, err := r.getJSONByID("iam_api_keys", "access_key", accessKey)
	if err != nil {
		return nil, err
	}
	delete(out, "secret_key")
	return out, nil
}

func (r *Repository) ListIAMAPIKeys() ([]map[string]any, error) {
	items, err := r.listJSON("iam_api_keys", "", "")
	if err != nil {
		return nil, err
	}
	for i := range items {
		delete(items[i], "secret_key")
	}
	return items, nil
}

func (r *Repository) DeleteIAMAPIKey(accessKey string) error {
	return r.deleteBy("iam_api_keys", "access_key = ?", accessKey)
}

func (r *Repository) CreateIAMPolicy(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now

	policyID := newID()
	data["id"] = policyID
	var appID any
	if v, ok := data["application_id"].(string); ok && strings.TrimSpace(v) != "" {
		appID = v
	} else {
		appID = nil
	}
	if err := r.insertJSON("iam_policies", []colVal{{name: "id", val: policyID}, {name: "application_id", val: appID}}, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) GetIAMPolicy(id string) (map[string]any, error) {
	return r.getJSONByID("iam_policies", "id", id)
}

func (r *Repository) ListIAMPolicies() ([]map[string]any, error) {
	return r.listJSON("iam_policies", "", "")
}

func (r *Repository) DeleteIAMPolicy(id string) error {
	return r.deleteBy("iam_policies", "id = ?", id)
}

func (r *Repository) CreateIAMSSHKey(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	data["fingerprint"] = "256 SHA256:" + randomAlphaNum(32)
	id := newID()
	data["id"] = id
	if err := r.insertJSON("iam_ssh_keys", []colVal{{name: "id", val: id}}, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) GetIAMSSHKey(id string) (map[string]any, error) {
	return r.getJSONByID("iam_ssh_keys", "id", id)
}

func (r *Repository) ListIAMSSHKeys() ([]map[string]any, error) {
	return r.listJSON("iam_ssh_keys", "", "")
}

func (r *Repository) DeleteIAMSSHKey(id string) error {
	return r.deleteBy("iam_ssh_keys", "id = ?", id)
}

func (r *Repository) FullState() (map[string]any, error) {
	servers, err := r.listJSON("instance_servers", "", "")
	if err != nil {
		return nil, err
	}
	ips, err := r.listJSON("instance_ips", "", "")
	if err != nil {
		return nil, err
	}
	nics, err := r.listJSON("instance_private_nics", "", "")
	if err != nil {
		return nil, err
	}
	sgs, err := r.listJSON("instance_security_groups", "", "")
	if err != nil {
		return nil, err
	}
	vpcs, err := r.listJSON("vpcs", "", "")
	if err != nil {
		return nil, err
	}
	pns, err := r.listJSON("private_networks", "", "")
	if err != nil {
		return nil, err
	}
	lbs, err := r.listJSON("lbs", "", "")
	if err != nil {
		return nil, err
	}
	frontends, err := r.listJSON("lb_frontends", "", "")
	if err != nil {
		return nil, err
	}
	backends, err := r.listJSON("lb_backends", "", "")
	if err != nil {
		return nil, err
	}
	lbPNs, err := r.listJSON("lb_private_networks", "", "")
	if err != nil {
		return nil, err
	}
	clusters, err := r.listJSON("k8s_clusters", "", "")
	if err != nil {
		return nil, err
	}
	pools, err := r.listJSON("k8s_pools", "", "")
	if err != nil {
		return nil, err
	}
	rdbInstances, err := r.listJSON("rdb_instances", "", "")
	if err != nil {
		return nil, err
	}
	rdbDatabases, err := r.listJSON("rdb_databases", "", "")
	if err != nil {
		return nil, err
	}
	rdbUsers, err := r.listJSON("rdb_users", "", "")
	if err != nil {
		return nil, err
	}
	iamApplications, err := r.listJSON("iam_applications", "", "")
	if err != nil {
		return nil, err
	}
	iamAPIKeys, err := r.ListIAMAPIKeys()
	if err != nil {
		return nil, err
	}
	iamPolicies, err := r.listJSON("iam_policies", "", "")
	if err != nil {
		return nil, err
	}
	iamSSHKeys, err := r.listJSON("iam_ssh_keys", "", "")
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"instance": map[string]any{
			"servers":         servers,
			"ips":             ips,
			"private_nics":    nics,
			"security_groups": sgs,
		},
		"vpc": map[string]any{
			"vpcs":             vpcs,
			"private_networks": pns,
		},
		"lb": map[string]any{
			"lbs":              lbs,
			"frontends":        frontends,
			"backends":         backends,
			"private_networks": lbPNs,
		},
		"k8s": map[string]any{
			"clusters": clusters,
			"pools":    pools,
		},
		"rdb": map[string]any{
			"instances": rdbInstances,
			"databases": rdbDatabases,
			"users":     rdbUsers,
		},
		"iam": map[string]any{
			"applications": iamApplications,
			"api_keys":     iamAPIKeys,
			"policies":     iamPolicies,
			"ssh_keys":     iamSSHKeys,
		},
	}, nil
}

func (r *Repository) ServiceState(service string) (map[string]any, error) {
	switch service {
	case "instance":
		servers, err := r.listJSON("instance_servers", "", "")
		if err != nil {
			return nil, err
		}
		ips, err := r.listJSON("instance_ips", "", "")
		if err != nil {
			return nil, err
		}
		nics, err := r.listJSON("instance_private_nics", "", "")
		if err != nil {
			return nil, err
		}
		sgs, err := r.listJSON("instance_security_groups", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"servers":         servers,
			"ips":             ips,
			"private_nics":    nics,
			"security_groups": sgs,
		}, nil
	case "vpc":
		vpcs, err := r.listJSON("vpcs", "", "")
		if err != nil {
			return nil, err
		}
		pns, err := r.listJSON("private_networks", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"vpcs":             vpcs,
			"private_networks": pns,
		}, nil
	case "lb":
		lbs, err := r.listJSON("lbs", "", "")
		if err != nil {
			return nil, err
		}
		frontends, err := r.listJSON("lb_frontends", "", "")
		if err != nil {
			return nil, err
		}
		backends, err := r.listJSON("lb_backends", "", "")
		if err != nil {
			return nil, err
		}
		lbPNs, err := r.listJSON("lb_private_networks", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"lbs":              lbs,
			"frontends":        frontends,
			"backends":         backends,
			"private_networks": lbPNs,
		}, nil
	case "k8s":
		clusters, err := r.listJSON("k8s_clusters", "", "")
		if err != nil {
			return nil, err
		}
		pools, err := r.listJSON("k8s_pools", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"clusters": clusters,
			"pools":    pools,
		}, nil
	case "rdb":
		instances, err := r.listJSON("rdb_instances", "", "")
		if err != nil {
			return nil, err
		}
		databases, err := r.listJSON("rdb_databases", "", "")
		if err != nil {
			return nil, err
		}
		users, err := r.listJSON("rdb_users", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"instances": instances,
			"databases": databases,
			"users":     users,
		}, nil
	case "iam":
		applications, err := r.listJSON("iam_applications", "", "")
		if err != nil {
			return nil, err
		}
		apiKeys, err := r.ListIAMAPIKeys()
		if err != nil {
			return nil, err
		}
		policies, err := r.listJSON("iam_policies", "", "")
		if err != nil {
			return nil, err
		}
		sshKeys, err := r.listJSON("iam_ssh_keys", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"applications": applications,
			"api_keys":     apiKeys,
			"policies":     policies,
			"ssh_keys":     sshKeys,
		}, nil
	default:
		return nil, models.ErrNotFound
	}
}

func fakePublicIP() string {
	p := strings.ReplaceAll(newID(), "-", "")
	return fmt.Sprintf("51.15.%d.%d", int(p[0])%254+1, int(p[1])%254+1)
}

func fakePrivateIP() string {
	p := strings.ReplaceAll(newID(), "-", "")
	return fmt.Sprintf("10.%d.%d.%d", int(p[0])%254+1, int(p[1])%254+1, int(p[2])%254+1)
}

func BuildRDBEndpointsFromInit(initEndpoints any, engine any) ([]any, error) {
	port := rdbPortFromEngine(engine)
	list, ok := initEndpoints.([]any)
	if !ok || len(list) == 0 {
		return []any{map[string]any{"ip": fakePublicIP(), "port": port}}, nil
	}
	first, ok := list[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid init_endpoints")
	}
	pn, ok := first["private_network"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid init_endpoints.private_network")
	}
	pnID, _ := pn["id"].(string)
	if pnID == "" {
		return nil, fmt.Errorf("missing init_endpoints.private_network.id")
	}
	return []any{map[string]any{
		"ip":              fakePrivateIP(),
		"port":            port,
		"private_network": map[string]any{"id": pnID},
	}}, nil
}

func rdbPortFromEngine(engine any) float64 {
	s, _ := engine.(string)
	if strings.Contains(strings.ToLower(s), "mysql") {
		return float64(3306)
	}
	return float64(5432)
}

func randomAlphaNum(n int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var b strings.Builder
	b.Grow(n)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return strings.Repeat("A", n)
		}
		b.WriteByte(alphabet[v.Int64()])
	}
	return b.String()
}
