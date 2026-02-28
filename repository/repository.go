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
			security_group_id TEXT REFERENCES instance_security_groups(id) ON DELETE SET NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS instance_ips (
			id TEXT PRIMARY KEY,
			server_id TEXT REFERENCES instance_servers(id) ON DELETE SET NULL,
			zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS instance_private_nics (
			id TEXT PRIMARY KEY,
			server_id TEXT NOT NULL REFERENCES instance_servers(id) ON DELETE CASCADE,
			private_network_id TEXT NOT NULL REFERENCES private_networks(id),
			zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lb_ips (
			id TEXT PRIMARY KEY,
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
		`CREATE TABLE IF NOT EXISTS rdb_privileges (
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id),
			user_name TEXT NOT NULL,
			database_name TEXT NOT NULL,
			data JSON NOT NULL,
			PRIMARY KEY (instance_id, user_name, database_name)
		)`,
		`CREATE TABLE IF NOT EXISTS domain_records (
			id TEXT PRIMARY KEY,
			dns_zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS redis_clusters (
			id TEXT PRIMARY KEY,
			zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS registry_namespaces (
			id TEXT PRIMARY KEY,
			region TEXT NOT NULL,
			data JSON NOT NULL
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
		"lb_ips",
		"instance_private_nics",
		"instance_ips",
		"instance_servers",
		"instance_security_groups",
		"k8s_pools",
		"k8s_clusters",
		"rdb_privileges",
		"rdb_databases",
		"rdb_users",
		"rdb_instances",
		"redis_clusters",
		"registry_namespaces",
		"iam_api_keys",
		"iam_policies",
		"iam_ssh_keys",
		"iam_applications",
		"domain_records",
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
	// Build subnets as proper objects (the SDK expects vpc.Subnet structs).
	subnet := "172.16.0.0/22"
	if v4, ok := data["ipv4_subnet"].(map[string]any); ok {
		if s, ok := v4["subnet"].(string); ok {
			subnet = s
		}
	} else if existing, ok := data["subnets"].([]any); ok && len(existing) > 0 {
		if s, ok := existing[0].(string); ok {
			subnet = s
		}
	}
	data["subnets"] = []any{map[string]any{
		"id":         newID(),
		"subnet":     subnet,
		"created_at": now,
		"updated_at": now,
	}}
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
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.detachSecurityGroupFromServersTx(tx, id); err != nil {
		return err
	}

	res, err := tx.Exec(`DELETE FROM instance_security_groups WHERE id = ?`, id)
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
	return tx.Commit()
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

func (r *Repository) SetSecurityGroupRules(id string, rules any) (map[string]any, error) {
	current, err := r.getJSONByID("instance_security_groups", "id", id)
	if err != nil {
		return nil, err
	}
	next := cloneMap(current)
	next["rules"] = rules
	if err := r.updateJSONByID("instance_security_groups", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) GetSecurityGroupRules(id string) ([]any, error) {
	current, err := r.getJSONByID("instance_security_groups", "id", id)
	if err != nil {
		return nil, err
	}
	rules, ok := current["rules"].([]any)
	if !ok {
		return []any{}, nil
	}
	return rules, nil
}

func (r *Repository) CreateServer(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["zone"] = zone
	data["state"] = "running"
	data["creation_date"] = now
	data["modification_date"] = now
	var resolvedIPs []any
	if rawIPs, ok := data["public_ips"].([]any); ok {
		for _, raw := range rawIPs {
			if ipID, ok := raw.(string); ok && ipID != "" {
				if ipRec, err := r.GetIP(ipID); err == nil {
					resolvedIPs = append(resolvedIPs, map[string]any{
						"id":      ipID,
						"address": ipRec["address"],
						"dynamic": false,
					})
				}
			}
		}
	}
	if len(resolvedIPs) > 0 {
		data["public_ips"] = resolvedIPs
		data["public_ip"] = resolvedIPs[0]
	} else {
		data["public_ips"] = []any{}
		data["public_ip"] = nil
	}
	serverName, _ := data["name"].(string)
	serverName = strings.TrimSpace(serverName)
	if serverName == "" {
		serverName = "server"
	}
	data["volumes"] = map[string]any{
		"0": map[string]any{
			"id":          newID(),
			"name":        fmt.Sprintf("%s-vol-0", serverName),
			"size":        20000000000,
			"volume_type": "l_ssd",
			"state":       "available",
			"boot":        true,
			"zone":        zone,
		},
	}
	sgID, _ := data["security_group_id"].(string)
	if _, ok := data["security_group"].(map[string]any); !ok {
		// Provider dereferences SecurityGroup.ID without nil check (server.go:693).
		sgObjID := sgID
		if sgObjID == "" {
			sgObjID = newID()
		}
		data["security_group"] = map[string]any{"id": sgObjID, "name": "default"}
	}
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
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.detachIPsFromServerTx(tx, id); err != nil {
		return err
	}

	// Keep behavior consistent on older DB files created before FK CASCADE migration.
	if _, err := tx.Exec(`DELETE FROM instance_private_nics WHERE server_id = ?`, id); err != nil {
		return mapDeleteSQLError(err)
	}

	res, err := tx.Exec(`DELETE FROM instance_servers WHERE id = ?`, id)
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
	return tx.Commit()
}

func (r *Repository) GetInstanceVolume(zone, volumeID string) (map[string]any, error) {
	servers, err := r.listJSON("instance_servers", "zone", zone)
	if err != nil {
		return nil, err
	}
	for _, server := range servers {
		volumes, ok := server["volumes"].(map[string]any)
		if !ok {
			continue
		}
		for _, raw := range volumes {
			vol, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := vol["id"].(string)
			if id != volumeID {
				continue
			}
			return cloneMap(vol), nil
		}
	}
	return nil, models.ErrNotFound
}

func (r *Repository) detachIPsFromServerTx(tx *sql.Tx, serverID string) error {
	rows, err := tx.Query(`SELECT id, data FROM instance_ips WHERE server_id = ?`, serverID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type update struct {
		id   string
		data map[string]any
	}
	updates := []update{}
	for rows.Next() {
		var (
			id  string
			raw []byte
		)
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		data, err := unmarshalData(raw)
		if err != nil {
			return err
		}
		data["server_id"] = nil
		updates = append(updates, update{id: id, data: data})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, u := range updates {
		b, err := marshalData(u.data)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE instance_ips SET server_id = NULL, data = ? WHERE id = ?`, b, u.id); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) detachSecurityGroupFromServersTx(tx *sql.Tx, sgID string) error {
	rows, err := tx.Query(`SELECT id, data FROM instance_servers WHERE security_group_id = ?`, sgID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type update struct {
		id   string
		data map[string]any
	}
	updates := []update{}
	for rows.Next() {
		var (
			id  string
			raw []byte
		)
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		data, err := unmarshalData(raw)
		if err != nil {
			return err
		}
		data["security_group_id"] = nil
		data["security_group"] = nil
		updates = append(updates, update{id: id, data: data})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, u := range updates {
		b, err := marshalData(u.data)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE instance_servers SET security_group_id = NULL, data = ? WHERE id = ?`, b, u.id); err != nil {
			return err
		}
	}
	return nil
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
	data["state"] = "available"
	data["private_ips"] = []any{map[string]any{
		"id":      newID(),
		"address": fakePrivateIP(),
	}}
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
		"id":              newID(),
		"ip_address":      fakePublicIP(),
		"lb_id":           id,
		"reverse":         "",
		"organization_id": "00000000-0000-0000-0000-000000000000",
		"project_id":      "00000000-0000-0000-0000-000000000000",
		"zone":            zone,
		"region":          regionFromZone(zone),
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
func (r *Repository) UpdateLB(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("lbs", "id", id)
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
	if err := r.updateJSONByID("lbs", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteLB(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Cascade private-network attachments (the Scaleway provider does not
	// detach them before deleting the LB). Frontends and backends are NOT
	// cascaded — the provider deletes those explicitly, and 409 correctly
	// signals if any remain.
	if _, err := tx.Exec("DELETE FROM lb_private_networks WHERE lb_id = ?", id); err != nil {
		return mapDeleteSQLError(err)
	}

	res, err := tx.Exec("DELETE FROM lbs WHERE id = ?", id)
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
	return tx.Commit()
}

func (r *Repository) CreateLBIP(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	data["zone"] = zone
	data["ip_address"] = fakePublicIP()
	data["status"] = "ready"
	data["lb_id"] = nil
	data["reverse"] = ""
	data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	data["project_id"] = "00000000-0000-0000-0000-000000000000"
	data["region"] = regionFromZone(zone)
	return r.createSimple("lb_ips", "zone", zone, data)
}
func (r *Repository) GetLBIP(id string) (map[string]any, error) {
	return r.getJSONByID("lb_ips", "id", id)
}
func (r *Repository) ListLBIPs(zone string) ([]map[string]any, error) {
	return r.listJSON("lb_ips", "zone", zone)
}
func (r *Repository) DeleteLBIP(id string) error { return r.deleteBy("lb_ips", "id = ?", id) }

func (r *Repository) CreateFrontend(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	lbID, _ := data["lb_id"].(string)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	// The provider accesses res.LB.ID and res.Backend.ID after create.
	if lbID != "" {
		lb, err := r.GetLB(lbID)
		if err == nil {
			data["lb"] = lb
		}
	}
	if backendID, ok := data["backend_id"].(string); ok && backendID != "" {
		data["backend"] = map[string]any{"id": backendID}
	}
	return r.createSimple("lb_frontends", "lb_id", lbID, data)
}
func (r *Repository) GetFrontend(id string) (map[string]any, error) {
	return r.getJSONByID("lb_frontends", "id", id)
}
func (r *Repository) ListFrontends() ([]map[string]any, error) {
	return r.listJSON("lb_frontends", "", "")
}
func (r *Repository) ListFrontendsByLB(lbID string) ([]map[string]any, error) {
	return r.listJSON("lb_frontends", "lb_id", lbID)
}
func (r *Repository) UpdateFrontend(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("lb_frontends", "id", id)
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
	if err := r.updateJSONByID("lb_frontends", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteFrontend(id string) error { return r.deleteBy("lb_frontends", "id = ?", id) }

func (r *Repository) CreateBackend(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	lbID, _ := data["lb_id"].(string)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	// The provider accesses res.LB.ID after create.
	if lbID != "" {
		lb, err := r.GetLB(lbID)
		if err == nil {
			data["lb"] = lb
		}
	}
	// Provide defaults for fields the provider reads via d.Set.
	if _, ok := data["timeout_server"]; !ok {
		data["timeout_server"] = "5m"
	}
	if _, ok := data["timeout_connect"]; !ok {
		data["timeout_connect"] = "5s"
	}
	if _, ok := data["timeout_tunnel"]; !ok {
		data["timeout_tunnel"] = "15m"
	}
	if _, ok := data["timeout_queue"]; !ok {
		data["timeout_queue"] = "0s"
	}
	if _, ok := data["on_marked_down_action"]; !ok {
		data["on_marked_down_action"] = "none"
	}
	if _, ok := data["health_check"]; !ok {
		data["health_check"] = map[string]any{
			"port":                   data["forward_port"],
			"check_delay":            "60s",
			"check_timeout":          "30s",
			"check_max_retries":      3,
			"transient_check_delay":  "0.5s",
			"tcp_config":             map[string]any{},
		}
	}
	return r.createSimple("lb_backends", "lb_id", lbID, data)
}
func (r *Repository) GetBackend(id string) (map[string]any, error) {
	return r.getJSONByID("lb_backends", "id", id)
}
func (r *Repository) ListBackends() ([]map[string]any, error) {
	return r.listJSON("lb_backends", "", "")
}
func (r *Repository) ListBackendsByLB(lbID string) ([]map[string]any, error) {
	return r.listJSON("lb_backends", "lb_id", lbID)
}
func (r *Repository) UpdateBackend(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("lb_backends", "id", id)
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
	if err := r.updateJSONByID("lb_backends", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteBackend(id string) error { return r.deleteBy("lb_backends", "id = ?", id) }

func (r *Repository) AttachLBPrivateNetwork(lbID, privateNetworkID string) (map[string]any, error) {
	now := nowRFC3339()
	data := map[string]any{
		"lb_id":              lbID,
		"private_network_id": privateNetworkID,
		"status":             "ready",
		"ip_address":         []any{fakePrivateIP()},
		"dhcp_config":        map[string]any{},
		"static_config":      nil,
		"created_at":         now,
		"updated_at":         now,
	}
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

	// Fields the Scaleway TF provider reads and will nil-deref on if missing.
	if _, ok := data["cluster_url"]; !ok {
		data["cluster_url"] = "https://mock-k8s-apiserver.scw.cloud:6443"
	}
	if _, ok := data["wildcard_dns"]; !ok {
		data["wildcard_dns"] = "*.mock-k8s.scw.cloud"
	}
	if _, ok := data["open_id_connect_config"]; !ok {
		data["open_id_connect_config"] = map[string]any{
			"issuer_url":      "",
			"client_id":       "",
			"username_claim":  "",
			"username_prefix": "",
			"groups_claim":    []any{},
			"groups_prefix":   "",
			"required_claim":  []any{},
		}
	}
	if _, ok := data["auto_upgrade"]; !ok {
		data["auto_upgrade"] = map[string]any{
			"enable":                false,
			"maintenance_window":    map[string]any{"day": "any", "start_hour": float64(0)},
		}
	}
	if _, ok := data["autoscaler_config"]; !ok {
		data["autoscaler_config"] = map[string]any{
			"scale_down_disabled":              false,
			"scale_down_delay_after_add":       "10m",
			"estimator":                        "binpacking",
			"expander":                         "random",
			"ignore_daemonsets_utilization":     false,
			"balance_similar_node_groups":       false,
			"expendable_pods_priority_cutoff":   float64(-10),
			"scale_down_unneeded_time":          "10m",
			"scale_down_utilization_threshold":  0.5,
			"max_graceful_termination_sec":      float64(600),
		}
	}
	if _, ok := data["feature_gates"]; !ok {
		data["feature_gates"] = []any{}
	}
	if _, ok := data["admission_plugins"]; !ok {
		data["admission_plugins"] = []any{}
	}
	if _, ok := data["apiserver_cert_sans"]; !ok {
		data["apiserver_cert_sans"] = []any{}
	}
	if _, ok := data["tags"]; !ok {
		data["tags"] = []any{}
	}
	if _, ok := data["organization_id"]; !ok {
		data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	}
	if _, ok := data["project_id"]; !ok {
		data["project_id"] = "00000000-0000-0000-0000-000000000000"
	}

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
func (r *Repository) UpdateCluster(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("k8s_clusters", "id", id)
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
	next["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("k8s_clusters", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteCluster(id string) error {
	res, err := r.db.Exec("DELETE FROM k8s_clusters WHERE id = ?", id)
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

func (r *Repository) CreatePool(region, clusterID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["region"] = region
	data["cluster_id"] = clusterID
	data["status"] = "ready"
	data["created_at"] = now
	data["updated_at"] = now

	// Fields the Scaleway TF provider reads during pool resource reads.
	if _, ok := data["version"]; !ok {
		data["version"] = "1.31.2"
	}
	if _, ok := data["tags"]; !ok {
		data["tags"] = []any{}
	}
	if _, ok := data["upgrade_policy"]; !ok {
		data["upgrade_policy"] = map[string]any{
			"max_unavailable": float64(1),
			"max_surge":       float64(0),
		}
	}
	if _, ok := data["nodes"]; !ok {
		data["nodes"] = []any{}
	}
	if _, ok := data["root_volume_type"]; !ok {
		data["root_volume_type"] = "l_ssd"
	}
	if _, ok := data["root_volume_size"]; !ok {
		data["root_volume_size"] = float64(20000000000)
	}
	if _, ok := data["zone"]; !ok {
		// Default zone from region.
		data["zone"] = region + "-1"
	}

	return r.createSimple("k8s_pools", "region", region, data, colVal{name: "cluster_id", val: clusterID})
}
func (r *Repository) GetPool(id string) (map[string]any, error) {
	return r.getJSONByID("k8s_pools", "id", id)
}
func (r *Repository) ListPoolsByCluster(clusterID string) ([]map[string]any, error) {
	return r.listJSON("k8s_pools", "cluster_id", clusterID)
}
func (r *Repository) UpdatePool(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("k8s_pools", "id", id)
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
	next["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("k8s_pools", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
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
	// Fields required by the TF provider's ResourceRdbInstanceRead to avoid nil derefs.
	if _, ok := data["volume"]; !ok {
		data["volume"] = map[string]any{"type": "lssd", "size": float64(10000000000)}
	}
	if _, ok := data["backup_schedule"]; !ok {
		data["backup_schedule"] = map[string]any{
			"disabled":  false,
			"frequency":  24,
			"retention": 7,
		}
	}
	if _, ok := data["backup_same_region"]; !ok {
		data["backup_same_region"] = false
	}
	if _, ok := data["encryption"]; !ok {
		data["encryption"] = map[string]any{"enabled": false}
	}
	if _, ok := data["settings"]; !ok {
		data["settings"] = []any{}
	}
	if _, ok := data["init_settings"]; !ok {
		data["init_settings"] = []any{}
	}
	if _, ok := data["logs_policy"]; !ok {
		data["logs_policy"] = map[string]any{
			"max_age_retention":    30,
			"total_disk_retention": nil,
		}
	}
	if _, ok := data["tags"]; !ok {
		data["tags"] = []any{}
	}
	if _, ok := data["upgradable_version"]; !ok {
		data["upgradable_version"] = []any{}
	}
	if _, ok := data["organization_id"]; !ok {
		data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	}
	if _, ok := data["project_id"]; !ok {
		data["project_id"] = "00000000-0000-0000-0000-000000000000"
	}
	if _, ok := data["read_replicas"]; !ok {
		data["read_replicas"] = []any{}
	}
	if _, ok := data["maintenances"]; !ok {
		data["maintenances"] = []any{}
	}
	return r.createSimple("rdb_instances", "region", region, data)
}
func (r *Repository) GetRDBInstance(id string) (map[string]any, error) {
	return r.getJSONByID("rdb_instances", "id", id)
}
func (r *Repository) ListRDBInstances(region string) ([]map[string]any, error) {
	return r.listJSON("rdb_instances", "region", region)
}
func (r *Repository) UpdateRDBInstance(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("rdb_instances", "id", id)
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
	next["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("rdb_instances", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteRDBInstance(id string) error {
	res, err := r.db.Exec("DELETE FROM rdb_instances WHERE id = ?", id)
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

func (r *Repository) SetRDBPrivileges(instanceID string, privileges []any) ([]map[string]any, error) {
	// Replace all privileges for this instance with the provided set.
	if _, err := r.db.Exec("DELETE FROM rdb_privileges WHERE instance_id = ?", instanceID); err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(privileges))
	for _, raw := range privileges {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		p = cloneMap(p)
		userName, _ := p["user_name"].(string)
		dbName, _ := p["database_name"].(string)
		if err := r.insertJSON("rdb_privileges", []colVal{
			{name: "instance_id", val: instanceID},
			{name: "user_name", val: userName},
			{name: "database_name", val: dbName},
		}, p); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func (r *Repository) ListRDBPrivileges(instanceID string) ([]map[string]any, error) {
	return r.listJSON("rdb_privileges", "instance_id", instanceID)
}

func (r *Repository) PatchDomainRecords(dnsZone string, changes []any) ([]map[string]any, error) {
	for _, raw := range changes {
		change, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if add, ok := change["add"].(map[string]any); ok {
			records, _ := add["records"].([]any)
			for _, rec := range records {
				recMap, ok := rec.(map[string]any)
				if !ok {
					continue
				}
				recMap = cloneMap(recMap)
				recMap["id"] = newID()
				if err := r.insertJSON("domain_records", []colVal{{name: "id", val: recMap["id"]}, {name: "dns_zone", val: dnsZone}}, recMap); err != nil {
					return nil, err
				}
			}
		}
		if del, ok := change["delete"].(map[string]any); ok {
			if id, ok := del["id"].(string); ok && id != "" {
				_ = r.deleteBy("domain_records", "id = ?", id)
			}
		}
	}
	return r.ListDomainRecords(dnsZone)
}

func (r *Repository) ListDomainRecords(dnsZone string) ([]map[string]any, error) {
	return r.listJSON("domain_records", "dns_zone", dnsZone)
}

func (r *Repository) DeleteDomainRecord(id string) error {
	return r.deleteBy("domain_records", "id = ?", id)
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

// --- Redis ---

func (r *Repository) CreateRedisCluster(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	id := newID()
	now := nowRFC3339()
	data["id"] = id
	data["zone"] = zone
	data["status"] = "ready"
	data["cluster_size"] = float64(1)
	data["tls_enabled"] = false
	data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	data["project_id"] = "00000000-0000-0000-0000-000000000000"
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["tags"]; !ok {
		data["tags"] = []any{}
	}
	if _, ok := data["acl_rules"]; !ok {
		data["acl_rules"] = []any{}
	}
	if _, ok := data["endpoints"]; !ok {
		data["endpoints"] = []any{map[string]any{
			"id":   newID(),
			"ips":  []any{fakePrivateIP()},
			"port": float64(6379),
		}}
	}
	if _, ok := data["public_network"]; !ok {
		data["public_network"] = []any{}
	}
	if _, ok := data["settings"]; !ok {
		data["settings"] = map[string]any{}
	}
	if _, ok := data["user_name"]; !ok {
		data["user_name"] = "default"
	}

	cols := []colVal{
		{name: "id", val: id},
		{name: "zone", val: zone},
	}
	if err := r.insertJSON("redis_clusters", cols, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) GetRedisCluster(id string) (map[string]any, error) {
	return r.getJSONByID("redis_clusters", "id", id)
}

func (r *Repository) ListRedisClusters(zone string) ([]map[string]any, error) {
	return r.listJSON("redis_clusters", "zone", zone)
}

func (r *Repository) UpdateRedisCluster(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("redis_clusters", "id", id)
	if err != nil {
		return nil, err
	}
	merged := cloneMap(current)
	for k, v := range patch {
		if k == "id" {
			continue
		}
		merged[k] = v
	}
	merged["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("redis_clusters", "id", id, merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func (r *Repository) DeleteRedisCluster(id string) error {
	return r.deleteBy("redis_clusters", "id = ?", id)
}

// --- Container Registry ---

func (r *Repository) CreateRegistryNamespace(region string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	id := newID()
	now := nowRFC3339()
	data["id"] = id
	data["region"] = region
	data["status"] = "ready"
	data["endpoint"] = fmt.Sprintf("rg.%s.scw.cloud/%s", region, data["name"])
	data["image_count"] = float64(0)
	data["size"] = float64(0)
	data["is_public"] = false
	data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	data["project_id"] = "00000000-0000-0000-0000-000000000000"
	data["created_at"] = now
	data["updated_at"] = now

	cols := []colVal{
		{name: "id", val: id},
		{name: "region", val: region},
	}
	if err := r.insertJSON("registry_namespaces", cols, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) GetRegistryNamespace(id string) (map[string]any, error) {
	return r.getJSONByID("registry_namespaces", "id", id)
}

func (r *Repository) ListRegistryNamespaces(region string) ([]map[string]any, error) {
	return r.listJSON("registry_namespaces", "region", region)
}

func (r *Repository) UpdateRegistryNamespace(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("registry_namespaces", "id", id)
	if err != nil {
		return nil, err
	}
	merged := cloneMap(current)
	for k, v := range patch {
		if k == "id" {
			continue
		}
		merged[k] = v
	}
	merged["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("registry_namespaces", "id", id, merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func (r *Repository) DeleteRegistryNamespace(id string) error {
	return r.deleteBy("registry_namespaces", "id = ?", id)
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
	lbIPs, err := r.listJSON("lb_ips", "", "")
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
	rdbPrivileges, err := r.listJSON("rdb_privileges", "", "")
	if err != nil {
		return nil, err
	}
	redisClusters, err := r.listJSON("redis_clusters", "", "")
	if err != nil {
		return nil, err
	}
	registryNamespaces, err := r.listJSON("registry_namespaces", "", "")
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
	domainRecords, err := r.listJSON("domain_records", "", "")
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
			"ips":              lbIPs,
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
			"instances":  rdbInstances,
			"databases":  rdbDatabases,
			"users":      rdbUsers,
			"privileges": rdbPrivileges,
		},
		"redis": map[string]any{
			"clusters": redisClusters,
		},
		"registry": map[string]any{
			"namespaces": registryNamespaces,
		},
		"iam": map[string]any{
			"applications": iamApplications,
			"api_keys":     iamAPIKeys,
			"policies":     iamPolicies,
			"ssh_keys":     iamSSHKeys,
		},
		"domain": map[string]any{
			"records": domainRecords,
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
		lbIPs, err := r.listJSON("lb_ips", "", "")
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
		return map[string]any{
			"ips":              lbIPs,
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
	case "redis":
		clusters, err := r.listJSON("redis_clusters", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"clusters": clusters,
		}, nil
	case "registry":
		namespaces, err := r.listJSON("registry_namespaces", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"namespaces": namespaces,
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

// regionFromZone extracts the region prefix from a zone string (e.g.
// "fr-par-1" -> "fr-par"). Returns zone unchanged if it has fewer than 2
// dash-separated parts, avoiding index-out-of-range panics on malformed input.
func regionFromZone(zone string) string {
	parts := strings.SplitN(zone, "-", 3)
	if len(parts) < 2 {
		return zone
	}
	return parts[0] + "-" + parts[1]
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
		// If no private_network block, return a public endpoint.
		return []any{map[string]any{"ip": fakePublicIP(), "port": port}}, nil
	}
	// Accept both "id" and "private_network_id" as the PN identifier.
	pnID, _ := pn["id"].(string)
	if pnID == "" {
		pnID, _ = pn["private_network_id"].(string)
	}
	if pnID == "" {
		return nil, fmt.Errorf("invalid init_endpoints: private_network present but missing id")
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
