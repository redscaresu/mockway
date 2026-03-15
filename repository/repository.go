package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redscaresu/mockway/models"
	_ "modernc.org/sqlite"
)

type Repository struct {
	db             *sql.DB
	path           string
	snapshotPath   string
	cleanupOnClose bool
}

type colVal struct {
	name string
	val  any
}

func New(path string) (*Repository, error) {
	actualPath := path
	cleanupOnClose := false
	if path == ":memory:" {
		file, err := os.CreateTemp("", "mockway-*.sqlite")
		if err != nil {
			return nil, fmt.Errorf("create temp db: %w", err)
		}
		actualPath = file.Name()
		cleanupOnClose = true
		if err := file.Close(); err != nil {
			_ = os.Remove(actualPath)
			return nil, fmt.Errorf("close temp db file: %w", err)
		}
	}

	db, err := openDB(actualPath)
	if err != nil {
		if cleanupOnClose {
			_ = os.Remove(actualPath)
		}
		return nil, err
	}

	r := &Repository{
		db:             db,
		path:           actualPath,
		snapshotPath:   actualPath + ".snapshot",
		cleanupOnClose: cleanupOnClose,
	}
	if err := r.init(); err != nil {
		_ = db.Close()
		if cleanupOnClose {
			_ = os.Remove(actualPath)
		}
		return nil, err
	}
	return r, nil
}

func (r *Repository) Close() error {
	if r.db == nil {
		return nil
	}
	err := r.db.Close()
	if r.cleanupOnClose {
		_ = os.Remove(r.path)
		_ = os.Remove(r.snapshotPath)
		_ = os.Remove(r.path + ".restore")
	}
	return err
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
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
			cluster_id TEXT NOT NULL REFERENCES k8s_clusters(id) ON DELETE CASCADE,
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_instances (
			id TEXT PRIMARY KEY,
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_databases (
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			data JSON NOT NULL,
			PRIMARY KEY (instance_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_users (
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			data JSON NOT NULL,
			PRIMARY KEY (instance_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_privileges (
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id) ON DELETE CASCADE,
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
		`CREATE TABLE IF NOT EXISTS iam_rules (
			id TEXT PRIMARY KEY,
			policy_id TEXT NOT NULL REFERENCES iam_policies(id) ON DELETE CASCADE,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS iam_users (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS iam_groups (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS iam_group_members (
			group_id TEXT NOT NULL REFERENCES iam_groups(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES iam_users(id) ON DELETE CASCADE,
			PRIMARY KEY (group_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS block_volumes (
			id TEXT PRIMARY KEY,
			zone TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS block_snapshots (
			id TEXT PRIMARY KEY,
			zone TEXT NOT NULL,
			volume_id TEXT REFERENCES block_volumes(id),
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS ipam_ips (
			id TEXT PRIMARY KEY,
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_read_replicas (
			id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id) ON DELETE CASCADE,
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_snapshots (
			id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id) ON DELETE CASCADE,
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rdb_backups (
			id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL REFERENCES rdb_instances(id) ON DELETE CASCADE,
			region TEXT NOT NULL,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lb_acls (
			id TEXT PRIMARY KEY,
			frontend_id TEXT NOT NULL REFERENCES lb_frontends(id) ON DELETE CASCADE,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lb_routes (
			id TEXT PRIMARY KEY,
			lb_id TEXT NOT NULL REFERENCES lbs(id) ON DELETE CASCADE,
			data JSON NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lb_certificates (
			id TEXT PRIMARY KEY,
			lb_id TEXT NOT NULL REFERENCES lbs(id) ON DELETE CASCADE,
			data JSON NOT NULL
		)`,
	}

	stmts = append(stmts, `CREATE TABLE IF NOT EXISTS schema_versions (
		version INTEGER PRIMARY KEY
	)`)

	for _, stmt := range stmts {
		if _, err := r.db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}

	return r.migrate()
}

// migrate runs versioned schema migrations for tables that already exist but
// need their FK constraints updated (SQLite CREATE TABLE IF NOT EXISTS is a
// no-op on existing tables, so we have to recreate them).
func (r *Repository) migrate() error {
	type migration struct {
		version int
		stmts   []string
	}

	migrations := []migration{
		{
			// Add ON DELETE CASCADE to k8s_pools, rdb_databases, rdb_users, and
			// iam_group_members.user_id so that deleting a parent row also removes
			// the child rows on existing file-backed databases.
			version: 1,
			stmts: []string{
				// k8s_pools
				`CREATE TABLE IF NOT EXISTS k8s_pools_new (
					id TEXT PRIMARY KEY,
					cluster_id TEXT NOT NULL REFERENCES k8s_clusters(id) ON DELETE CASCADE,
					region TEXT NOT NULL,
					data JSON NOT NULL
				)`,
				`INSERT OR IGNORE INTO k8s_pools_new SELECT id, cluster_id, region, data FROM k8s_pools`,
				`DROP TABLE k8s_pools`,
				`ALTER TABLE k8s_pools_new RENAME TO k8s_pools`,
				// rdb_databases
				`CREATE TABLE IF NOT EXISTS rdb_databases_new (
					instance_id TEXT NOT NULL REFERENCES rdb_instances(id) ON DELETE CASCADE,
					name TEXT NOT NULL,
					data JSON NOT NULL,
					PRIMARY KEY (instance_id, name)
				)`,
				`INSERT OR IGNORE INTO rdb_databases_new SELECT instance_id, name, data FROM rdb_databases`,
				`DROP TABLE rdb_databases`,
				`ALTER TABLE rdb_databases_new RENAME TO rdb_databases`,
				// rdb_users
				`CREATE TABLE IF NOT EXISTS rdb_users_new (
					instance_id TEXT NOT NULL REFERENCES rdb_instances(id) ON DELETE CASCADE,
					name TEXT NOT NULL,
					data JSON NOT NULL,
					PRIMARY KEY (instance_id, name)
				)`,
				`INSERT OR IGNORE INTO rdb_users_new SELECT instance_id, name, data FROM rdb_users`,
				`DROP TABLE rdb_users`,
				`ALTER TABLE rdb_users_new RENAME TO rdb_users`,
				// iam_group_members
				`CREATE TABLE IF NOT EXISTS iam_group_members_new (
					group_id TEXT NOT NULL REFERENCES iam_groups(id) ON DELETE CASCADE,
					user_id TEXT NOT NULL REFERENCES iam_users(id) ON DELETE CASCADE,
					PRIMARY KEY (group_id, user_id)
				)`,
				`INSERT OR IGNORE INTO iam_group_members_new SELECT group_id, user_id FROM iam_group_members`,
				`DROP TABLE iam_group_members`,
				`ALTER TABLE iam_group_members_new RENAME TO iam_group_members`,
			},
		},
	}

	for _, m := range migrations {
		var applied int
		_ = r.db.QueryRow(`SELECT 1 FROM schema_versions WHERE version = ?`, m.version).Scan(&applied)
		if applied == 1 {
			continue
		}
		// Disable FK checks during table recreation to avoid constraint errors
		// while the _new tables are being populated.
		if _, err := r.db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
			return fmt.Errorf("migration %d: disable FK: %w", m.version, err)
		}
		for _, stmt := range m.stmts {
			if _, err := r.db.Exec(stmt); err != nil {
				_, _ = r.db.Exec(`PRAGMA foreign_keys = ON`)
				return fmt.Errorf("migration %d: %w", m.version, err)
			}
		}
		if _, err := r.db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
			return fmt.Errorf("migration %d: re-enable FK: %w", m.version, err)
		}
		if _, err := r.db.Exec(`INSERT INTO schema_versions (version) VALUES (?)`, m.version); err != nil {
			return fmt.Errorf("migration %d: record version: %w", m.version, err)
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
		"lb_acls",
		"lb_routes",
		"lb_certificates",
		"lb_private_networks",
		"lb_frontends",
		"lb_backends",
		"lbs",
		"lb_ips",
		"block_snapshots",
		"block_volumes",
		"ipam_ips",
		"instance_private_nics",
		"instance_ips",
		"instance_servers",
		"instance_security_groups",
		"k8s_pools",
		"k8s_clusters",
		"rdb_backups",
		"rdb_snapshots",
		"rdb_read_replicas",
		"rdb_privileges",
		"rdb_databases",
		"rdb_users",
		"rdb_instances",
		"redis_clusters",
		"registry_namespaces",
		"iam_rules",
		"iam_api_keys",
		"iam_policies",
		"iam_ssh_keys",
		"iam_group_members",
		"iam_groups",
		"iam_users",
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
	return r.clearSnapshot()
}

func (r *Repository) Snapshot() error {
	if err := r.clearSnapshot(); err != nil {
		return err
	}
	if _, err := r.db.Exec(`VACUUM main INTO ` + sqliteStringLiteral(r.snapshotPath)); err != nil {
		return fmt.Errorf("snapshot db: %w", err)
	}
	return nil
}

func (r *Repository) Restore() error {
	if _, err := os.Stat(r.snapshotPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return models.ErrNotFound
		}
		return fmt.Errorf("stat snapshot: %w", err)
	}

	restorePath := r.path + ".restore"
	if err := os.Remove(restorePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale restore db: %w", err)
	}
	if err := copyFile(r.snapshotPath, restorePath); err != nil {
		return fmt.Errorf("copy snapshot: %w", err)
	}
	if err := r.db.Close(); err != nil {
		return fmt.Errorf("close db for restore: %w", err)
	}
	if err := os.Rename(restorePath, r.path); err != nil {
		db, reopenErr := openDB(r.path)
		if reopenErr == nil {
			r.db = db
		}
		return fmt.Errorf("swap restored db: %w", err)
	}

	db, err := openDB(r.path)
	if err != nil {
		return err
	}
	r.db = db
	return r.init()
}

func (r *Repository) clearSnapshot() error {
	if err := os.Remove(r.snapshotPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove snapshot: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func sqliteStringLiteral(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
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
	now := nowRFC3339()
	data["zone"] = zone
	data["created_at"] = now
	data["updated_at"] = now
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
	next["updated_at"] = nowRFC3339()
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

func (r *Repository) SetServerState(id, state string) error {
	server, err := r.getJSONByID("instance_servers", "id", id)
	if err != nil {
		return err
	}
	server["state"] = state
	server["modification_date"] = nowRFC3339()
	return r.updateJSONByID("instance_servers", "id", id, server)
}

func (r *Repository) CreateServer(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["zone"] = zone
	data["state"] = "stopped"
	data["creation_date"] = now
	data["modification_date"] = now
	var resolvedIPs []any
	if rawIPs, ok := data["public_ips"].([]any); ok {
		for _, raw := range rawIPs {
			if ipID, ok := raw.(string); ok && ipID != "" {
				ipRec, err := r.GetIP(ipID)
				if err != nil {
					return nil, models.ErrNotFound
				}
				resolvedIPs = append(resolvedIPs, map[string]any{
					"id":      ipID,
					"address": ipRec["address"],
					"dynamic": false,
				})
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

// DeleteInstanceVolume removes a volume from the embedded volumes map inside a server.
func (r *Repository) DeleteInstanceVolume(zone, volumeID string) error {
	servers, err := r.listJSON("instance_servers", "zone", zone)
	if err != nil {
		return err
	}
	for _, server := range servers {
		volumes, ok := server["volumes"].(map[string]any)
		if !ok {
			continue
		}
		for key, raw := range volumes {
			vol, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := vol["id"].(string); id == volumeID {
				delete(volumes, key)
				server["volumes"] = volumes
				return r.updateJSONByID("instance_servers", "id", server["id"].(string), server)
			}
		}
	}
	return models.ErrNotFound
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
	data["updated_at"] = now
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
	next["updated_at"] = nowRFC3339()
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
	now := nowRFC3339()
	data["zone"] = zone
	data["ip_address"] = fakePublicIP()
	data["status"] = "ready"
	data["created_at"] = now
	data["updated_at"] = now
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
		backend, err := r.GetBackend(backendID)
		if err != nil {
			return nil, models.ErrNotFound
		}
		data["backend"] = backend
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
	if backendID, ok := next["backend_id"].(string); ok && backendID != "" {
		backend, err := r.GetBackend(backendID)
		if err != nil {
			return nil, models.ErrNotFound
		}
		next["backend"] = backend
	}
	next["updated_at"] = nowRFC3339()
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
		data["on_marked_down_action"] = "on_marked_down_action_none"
	}
	if _, ok := data["health_check"]; !ok {
		data["health_check"] = map[string]any{
			"port":                  data["forward_port"],
			"check_delay":           "60s",
			"check_timeout":         "30s",
			"check_max_retries":     3,
			"transient_check_delay": "0.5s",
			"tcp_config":            map[string]any{},
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
	next["updated_at"] = nowRFC3339()
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
			"enable":             false,
			"maintenance_window": map[string]any{"day": "any", "start_hour": float64(0)},
		}
	}
	if _, ok := data["autoscaler_config"]; !ok {
		data["autoscaler_config"] = map[string]any{
			"scale_down_disabled":              false,
			"scale_down_delay_after_add":       "10m",
			"estimator":                        "binpacking",
			"expander":                         "random",
			"ignore_daemonsets_utilization":    false,
			"balance_similar_node_groups":      false,
			"expendable_pods_priority_cutoff":  float64(-10),
			"scale_down_unneeded_time":         "10m",
			"scale_down_utilization_threshold": 0.5,
			"max_graceful_termination_sec":     float64(600),
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
	if _, ok := data["min_size"]; !ok {
		if s, ok := data["size"]; ok {
			data["min_size"] = s
		} else {
			data["min_size"] = float64(1)
		}
	}
	if _, ok := data["max_size"]; !ok {
		if s, ok := data["size"]; ok {
			data["max_size"] = s
		} else {
			data["max_size"] = float64(1)
		}
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
	if _, ok := data["autohealing"]; !ok {
		// Real API defaults autohealing to false; provider reads this field and
		// stores it in state — missing it causes perpetual drift.
		data["autohealing"] = false
	}
	if _, ok := data["autoscaling"]; !ok {
		data["autoscaling"] = false
	}

	return r.createSimple("k8s_pools", "region", region, data, colVal{name: "cluster_id", val: clusterID})
}
func (r *Repository) GetPool(id string) (map[string]any, error) {
	return r.getJSONByID("k8s_pools", "id", id)
}
func (r *Repository) ListPoolsByCluster(clusterID string) ([]map[string]any, error) {
	return r.listJSON("k8s_pools", "cluster_id", clusterID)
}

func (r *Repository) ListAllPools() ([]map[string]any, error) {
	return r.listJSON("k8s_pools", "", "")
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
	data["updated_at"] = now
	port := rdbPortFromEngine(data["engine"])
	if _, ok := data["endpoints"]; !ok {
		// The provider classifies endpoint type by checking load_balancer != nil.
		// The id is required for endpoint deletion references.
		data["endpoints"] = []any{map[string]any{
			"id":              newID(),
			"ip":              fakePublicIP(),
			"port":            port,
			"name":            nil,
			"load_balancer":   map[string]any{},
			"private_network": nil,
		}}
	}
	// Fields required by the TF provider's ResourceRdbInstanceRead to avoid nil derefs.
	if _, ok := data["volume"]; !ok {
		data["volume"] = map[string]any{"type": "lssd", "size": float64(10000000000)}
	}
	// The Terraform provider sends disable_backup as a flat field; translate it
	// to backup_schedule.disabled so that GET returns the shape the provider reads.
	if v, ok := data["disable_backup"]; ok {
		disabled, _ := v.(bool)
		if _, hasBS := data["backup_schedule"]; !hasBS {
			data["backup_schedule"] = map[string]any{
				"disabled":  disabled,
				"frequency": 24,
				"retention": 7,
			}
		} else if bs, ok := data["backup_schedule"].(map[string]any); ok {
			bs["disabled"] = disabled
		}
		delete(data, "disable_backup")
	} else if _, ok := data["backup_schedule"]; !ok {
		data["backup_schedule"] = map[string]any{
			"disabled":  false,
			"frequency": 24,
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
	if _, ok := data["managed"]; !ok {
		data["managed"] = false
	}
	if _, ok := data["owner"]; !ok {
		data["owner"] = ""
	}
	if _, ok := data["size"]; !ok {
		data["size"] = float64(0)
	}
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
	if _, ok := data["is_admin"]; !ok {
		data["is_admin"] = false
	}
	if err := r.insertJSON("rdb_users", []colVal{{name: "instance_id", val: instanceID}, {name: "name", val: name}}, data); err != nil {
		return nil, err
	}
	return data, nil
}
func (r *Repository) UpdateRDBUser(instanceID, name string, patch map[string]any) (map[string]any, error) {
	row := r.db.QueryRow("SELECT data FROM rdb_users WHERE instance_id = ? AND name = ?", instanceID, name)
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, models.ErrNotFound
	}
	current, err := unmarshalData(raw)
	if err != nil {
		return nil, err
	}
	next := cloneMap(current)
	for k, v := range patch {
		if k == "instance_id" || k == "name" {
			continue
		}
		next[k] = v
	}
	b, err := marshalData(next)
	if err != nil {
		return nil, err
	}
	if _, err := r.db.Exec("UPDATE rdb_users SET data = ? WHERE instance_id = ? AND name = ?", b, instanceID, name); err != nil {
		return nil, err
	}
	return next, nil
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
		if set, ok := change["set"].(map[string]any); ok {
			// "set" replaces matching records: delete by id_fields match, then add.
			if idFields, ok := set["id_fields"].(map[string]any); ok {
				existing, _ := r.ListDomainRecords(dnsZone)
				for _, rec := range existing {
					match := true
					for k, v := range idFields {
						if rec[k] != v {
							match = false
							break
						}
					}
					if match {
						if id, ok := rec["id"].(string); ok {
							_ = r.deleteBy("domain_records", "id = ?", id)
						}
					}
				}
			}
			if records, ok := set["records"].([]any); ok {
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

	// If the provider included rules in the create body, store them in iam_rules so
	// GET /iam/v1alpha1/rules?policy_id=xxx returns them on the subsequent read.
	if rulesRaw, ok := data["rules"]; ok {
		if rules, ok := rulesRaw.([]any); ok {
			for _, r2 := range rules {
				ruleData, ok := r2.(map[string]any)
				if !ok {
					continue
				}
				ruleData = cloneMap(ruleData)
				ruleData["id"] = newID()
				ruleData["policy_id"] = policyID
				if err := r.insertJSON("iam_rules", []colVal{
					{name: "id", val: ruleData["id"]},
					{name: "policy_id", val: policyID},
				}, ruleData); err != nil {
					return nil, err
				}
			}
		}
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

func (r *Repository) CreateIAMRule(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	id := newID()
	data["id"] = id
	policyID, _ := data["policy_id"].(string)
	if err := r.insertJSON("iam_rules", []colVal{{name: "id", val: id}, {name: "policy_id", val: policyID}}, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) ListIAMRulesByPolicy(policyID string) ([]map[string]any, error) {
	return r.listJSON("iam_rules", "policy_id", policyID)
}

// SetIAMRules replaces all rules for a policy (PUT /iam/v1alpha1/rules).
func (r *Repository) SetIAMRules(policyID string, rules []any) ([]map[string]any, error) {
	if _, err := r.getJSONByID("iam_policies", "id", policyID); err != nil {
		return nil, err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("DELETE FROM iam_rules WHERE policy_id = ?", policyID); err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		ruleMap = cloneMap(ruleMap)
		id := newID()
		ruleMap["id"] = id
		ruleMap["policy_id"] = policyID
		data, err := json.Marshal(ruleMap)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec("INSERT INTO iam_rules (id, policy_id, data) VALUES (?, ?, ?)", id, policyID, string(data)); err != nil {
			return nil, err
		}
		result = append(result, ruleMap)
	}
	return result, tx.Commit()
}

func (r *Repository) UpdateVPC(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("vpcs", "id", id)
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
	// Keep the indexed region SQL column in sync so ListVPCs filtering stays correct.
	region, _ := next["region"].(string)
	b, err := marshalData(next)
	if err != nil {
		return nil, err
	}
	if _, err := r.db.Exec(`UPDATE vpcs SET data = ?, region = ? WHERE id = ?`, b, region, id); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) UpdatePrivateNetwork(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("private_networks", "id", id)
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
	// Keep vpc_id and region SQL columns in sync so FK cascades and list
	// filtering remain correct after a PATCH that moves the network to a new VPC.
	vpcID, _ := next["vpc_id"].(string)
	region, _ := next["region"].(string)
	b, err := marshalData(next)
	if err != nil {
		return nil, err
	}
	var vpcIDArg any
	if vpcID != "" {
		vpcIDArg = vpcID
	}
	if _, err := r.db.Exec(
		`UPDATE private_networks SET data = ?, vpc_id = ?, region = ? WHERE id = ?`,
		b, vpcIDArg, region, id,
	); err != nil {
		return nil, mapInsertSQLError(err)
	}
	return next, nil
}

func (r *Repository) UpdateServer(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("instance_servers", "id", id)
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
	next["modification_date"] = nowRFC3339()

	// Reconcile security_group and security_group_id so that both JSON fields
	// and the SQL FK column stay consistent after a patch.
	// Priority: explicit security_group_id in the patch > security_group.id from
	// the merged doc > existing security_group_id from the merged doc.
	var sgID string
	if patchedSGID, ok := patch["security_group_id"].(string); ok {
		// Explicit patch for security_group_id (may be "" to detach).
		sgID = patchedSGID
		next["security_group_id"] = sgID
		if sgID == "" {
			delete(next, "security_group")
		} else {
			// Rebuild the embedded security_group object from the stored record
			// so that security_group.id and security_group.name are consistent.
			sgName := sgID // fallback: use ID as name if lookup fails
			if sgRecord, err := r.GetSecurityGroup(sgID); err == nil {
				if n, ok := sgRecord["name"].(string); ok && n != "" {
					sgName = n
				}
			}
			next["security_group"] = map[string]any{"id": sgID, "name": sgName}
		}
	} else if sgStr, ok := next["security_group"].(string); ok && sgStr != "" {
		// The Scaleway SDK sometimes sends security_group as a plain string (UUID).
		sgID = sgStr
		next["security_group_id"] = sgID
		sgName := sgID
		if sgRecord, err := r.GetSecurityGroup(sgID); err == nil {
			if n, ok := sgRecord["name"].(string); ok && n != "" {
				sgName = n
			}
		}
		next["security_group"] = map[string]any{"id": sgID, "name": sgName}
	} else if sg, ok := next["security_group"].(map[string]any); ok {
		if v, ok := sg["id"].(string); ok {
			sgID = v
			next["security_group_id"] = sgID
		}
	} else {
		sgID, _ = next["security_group_id"].(string)
	}

	b, err := marshalData(next)
	if err != nil {
		return nil, err
	}
	var sgIDArg any
	if sgID != "" {
		sgIDArg = sgID
	}
	_, err = r.db.Exec(
		`UPDATE instance_servers SET data = ?, security_group_id = ? WHERE id = ?`,
		b, sgIDArg, id,
	)
	if err != nil {
		return nil, mapInsertSQLError(err)
	}
	return next, nil
}

func (r *Repository) UpdateIP(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("instance_ips", "id", id)
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
	// The Scaleway API sends the server reference as "server" (a nullable string
	// value), not "server_id". Normalise both field names so that attach/detach
	// works regardless of which name the caller uses.
	if s, ok := patch["server"]; ok {
		serverVal, _ := s.(string)
		next["server_id"] = serverVal
		delete(next, "server")
	}
	b, err := marshalData(next)
	if err != nil {
		return nil, err
	}
	// Keep server_id SQL column in sync so cascade/detach logic stays correct.
	serverID, _ := next["server_id"].(string)
	var serverIDArg any
	if serverID != "" {
		serverIDArg = serverID
	}
	_, err = r.db.Exec(
		`UPDATE instance_ips SET data = ?, server_id = ? WHERE id = ?`,
		b, serverIDArg, id,
	)
	if err != nil {
		return nil, mapInsertSQLError(err)
	}
	return next, nil
}

func (r *Repository) UpdateLBIP(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("lb_ips", "id", id)
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
	if err := r.updateJSONByID("lb_ips", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) UpdateIAMApplication(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("iam_applications", "id", id)
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
	if err := r.updateJSONByID("iam_applications", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) UpdateIAMAPIKey(accessKey string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("iam_api_keys", "access_key", accessKey)
	if err != nil {
		return nil, err
	}
	next := cloneMap(current)
	for k, v := range patch {
		if k == "access_key" {
			continue
		}
		next[k] = v
	}
	next["updated_at"] = nowRFC3339()
	delete(next, "secret_key") // never return secret_key on update
	// Keep application_id SQL column in sync when the FK field changes.
	var appIDArg any
	if v, ok := next["application_id"].(string); ok && v != "" {
		appIDArg = v
	}
	b, err := marshalData(next)
	if err != nil {
		return nil, err
	}
	_, err = r.db.Exec(
		`UPDATE iam_api_keys SET data = ?, application_id = ? WHERE access_key = ?`,
		b, appIDArg, accessKey,
	)
	if err != nil {
		return nil, mapInsertSQLError(err)
	}
	return next, nil
}

func (r *Repository) UpdateIAMPolicy(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("iam_policies", "id", id)
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
	// Keep application_id SQL column in sync when the FK field changes.
	var appIDArg any
	if v, ok := next["application_id"].(string); ok && v != "" {
		appIDArg = v
	}
	b, err := marshalData(next)
	if err != nil {
		return nil, err
	}
	_, err = r.db.Exec(
		`UPDATE iam_policies SET data = ?, application_id = ? WHERE id = ?`,
		b, appIDArg, id,
	)
	if err != nil {
		return nil, mapInsertSQLError(err)
	}
	return next, nil
}

func (r *Repository) UpdateIAMSSHKey(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("iam_ssh_keys", "id", id)
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
	if err := r.updateJSONByID("iam_ssh_keys", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
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
	if _, ok := data["cluster_size"]; !ok {
		data["cluster_size"] = float64(1)
	}
	if _, ok := data["tls_enabled"]; !ok {
		data["tls_enabled"] = false
	}
	if _, ok := data["organization_id"]; !ok {
		data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	}
	if _, ok := data["project_id"]; !ok {
		data["project_id"] = "00000000-0000-0000-0000-000000000000"
	}
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
		data["settings"] = []any{}
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

// --- IAM Users ---

func (r *Repository) CreateIAMUser(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["status"]; !ok {
		data["status"] = "activated"
	}
	if _, ok := data["organization_id"]; !ok {
		data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	}
	if _, ok := data["project_id"]; !ok {
		data["project_id"] = "00000000-0000-0000-0000-000000000000"
	}
	id := newID()
	data["id"] = id
	if err := r.insertJSON("iam_users", []colVal{{name: "id", val: id}}, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) GetIAMUser(id string) (map[string]any, error) {
	return r.getJSONByID("iam_users", "id", id)
}

func (r *Repository) ListIAMUsers() ([]map[string]any, error) {
	return r.listJSON("iam_users", "", "")
}

func (r *Repository) UpdateIAMUser(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("iam_users", "id", id)
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
	if err := r.updateJSONByID("iam_users", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) UpdateIAMUserUsername(id string, username string) (map[string]any, error) {
	return r.UpdateIAMUser(id, map[string]any{"username": username})
}

func (r *Repository) DeleteIAMUser(id string) error {
	return r.deleteBy("iam_users", "id = ?", id)
}

// --- IAM Groups ---

func (r *Repository) getIAMGroupWithMembers(id string) (map[string]any, error) {
	data, err := r.getJSONByID("iam_groups", "id", id)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query("SELECT user_id FROM iam_group_members WHERE group_id = ?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	userIDs := []any{}
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, uid)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	data["user_ids"] = userIDs
	return data, nil
}

func (r *Repository) CreateIAMGroup(data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	data["user_ids"] = []any{}
	if _, ok := data["organization_id"]; !ok {
		data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	}
	if _, ok := data["project_id"]; !ok {
		data["project_id"] = "00000000-0000-0000-0000-000000000000"
	}
	id := newID()
	data["id"] = id
	if err := r.insertJSON("iam_groups", []colVal{{name: "id", val: id}}, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Repository) GetIAMGroup(id string) (map[string]any, error) {
	return r.getIAMGroupWithMembers(id)
}

func (r *Repository) ListIAMGroups() ([]map[string]any, error) {
	items, err := r.listJSON("iam_groups", "", "")
	if err != nil {
		return nil, err
	}
	for i, item := range items {
		id, _ := item["id"].(string)
		if id != "" {
			enriched, err := r.getIAMGroupWithMembers(id)
			if err == nil {
				items[i] = enriched
			}
		}
	}
	return items, nil
}

func (r *Repository) UpdateIAMGroup(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("iam_groups", "id", id)
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
	if err := r.updateJSONByID("iam_groups", "id", id, next); err != nil {
		return nil, err
	}
	return r.getIAMGroupWithMembers(id)
}

func (r *Repository) DeleteIAMGroup(id string) error {
	return r.deleteBy("iam_groups", "id = ?", id)
}

func (r *Repository) AddIAMGroupMember(groupID, userID string) (map[string]any, error) {
	_, err := r.db.Exec("INSERT OR IGNORE INTO iam_group_members (group_id, user_id) VALUES (?, ?)", groupID, userID)
	if err != nil {
		return nil, mapInsertSQLError(err) // FOREIGN KEY → ErrNotFound (user or group missing)
	}
	return r.getIAMGroupWithMembers(groupID)
}

func (r *Repository) RemoveIAMGroupMember(groupID, userID string) (map[string]any, error) {
	_, err := r.db.Exec("DELETE FROM iam_group_members WHERE group_id = ? AND user_id = ?", groupID, userID)
	if err != nil {
		return nil, err
	}
	return r.getIAMGroupWithMembers(groupID)
}

func (r *Repository) SetIAMGroupMembers(groupID string, userIDs []string) (map[string]any, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("DELETE FROM iam_group_members WHERE group_id = ?", groupID); err != nil {
		return nil, err
	}
	for _, uid := range userIDs {
		if _, err := tx.Exec("INSERT INTO iam_group_members (group_id, user_id) VALUES (?, ?)", groupID, uid); err != nil {
			return nil, mapInsertSQLError(err) // FOREIGN KEY → ErrNotFound
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getIAMGroupWithMembers(groupID)
}

// --- Block Volumes ---

func (r *Repository) CreateBlockVolume(zone string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["zone"] = zone
	data["status"] = "available"
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["size"]; !ok {
		data["size"] = float64(20000000000)
	}
	if _, ok := data["type"]; !ok {
		data["type"] = "sbs_5k"
	}
	if _, ok := data["storage_class"]; !ok {
		data["storage_class"] = "sbs"
	}
	// Provider reads volume.Specs.PerfIops — store the field to avoid perpetual diff.
	if _, ok := data["specs"]; !ok {
		iops := float64(5000)
		if t, _ := data["type"].(string); t == "sbs_15k" {
			iops = 15000
		}
		data["specs"] = map[string]any{"perf_iops": iops}
	}
	return r.createSimple("block_volumes", "zone", zone, data)
}

func (r *Repository) GetBlockVolume(id string) (map[string]any, error) {
	return r.getJSONByID("block_volumes", "id", id)
}

func (r *Repository) ListBlockVolumes(zone string) ([]map[string]any, error) {
	return r.listJSON("block_volumes", "zone", zone)
}

func (r *Repository) UpdateBlockVolume(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("block_volumes", "id", id)
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
	// Recompute perf_iops when the volume type changes.
	if newType, ok := next["type"].(string); ok {
		iops := float64(5000)
		if newType == "sbs_15k" {
			iops = 15000
		}
		next["specs"] = map[string]any{"perf_iops": iops}
	}
	if err := r.updateJSONByID("block_volumes", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteBlockVolume(id string) error {
	return r.deleteBy("block_volumes", "id = ?", id)
}

func (r *Repository) CreateBlockSnapshot(zone, volumeID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["zone"] = zone
	data["volume_id"] = volumeID
	data["status"] = "available"
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["size"]; !ok {
		data["size"] = float64(20000000000)
	}
	return r.createSimple("block_snapshots", "zone", zone, data, colVal{name: "volume_id", val: volumeID})
}

func (r *Repository) GetBlockSnapshot(id string) (map[string]any, error) {
	return r.getJSONByID("block_snapshots", "id", id)
}

func (r *Repository) ListBlockSnapshots(zone string) ([]map[string]any, error) {
	return r.listJSON("block_snapshots", "zone", zone)
}

func (r *Repository) UpdateBlockSnapshot(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("block_snapshots", "id", id)
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
	if err := r.updateJSONByID("block_snapshots", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteBlockSnapshot(id string) error {
	return r.deleteBy("block_snapshots", "id = ?", id)
}

// --- IPAM IPs ---

func (r *Repository) CreateIPAMIP(region string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["region"] = region
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["address"]; !ok {
		// IPAM address must be CIDR notation — provider uses expandIPNet() to parse it.
		data["address"] = fakePrivateIP() + "/32"
	}
	if _, ok := data["is_ipv6"]; !ok {
		data["is_ipv6"] = false
	}
	if _, ok := data["source"]; !ok {
		data["source"] = map[string]any{}
	}
	if _, ok := data["resource"]; !ok {
		data["resource"] = map[string]any{}
	}
	return r.createSimple("ipam_ips", "region", region, data)
}

func (r *Repository) GetIPAMIP(id string) (map[string]any, error) {
	return r.getJSONByID("ipam_ips", "id", id)
}

func (r *Repository) ListIPAMIPs(region string) ([]map[string]any, error) {
	return r.listJSON("ipam_ips", "region", region)
}

func (r *Repository) UpdateIPAMIP(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("ipam_ips", "id", id)
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
	if err := r.updateJSONByID("ipam_ips", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteIPAMIP(id string) error {
	return r.deleteBy("ipam_ips", "id = ?", id)
}

// --- RDB Read Replicas ---

func (r *Repository) CreateRDBReadReplica(region, instanceID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["region"] = region
	data["instance_id"] = instanceID
	data["status"] = "ready"
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["endpoints"]; !ok {
		data["endpoints"] = []any{}
	}
	return r.createSimple("rdb_read_replicas", "region", region, data, colVal{name: "instance_id", val: instanceID})
}

func (r *Repository) GetRDBReadReplica(id string) (map[string]any, error) {
	return r.getJSONByID("rdb_read_replicas", "id", id)
}

func (r *Repository) ListRDBReadReplicas(instanceID string) ([]map[string]any, error) {
	return r.listJSON("rdb_read_replicas", "instance_id", instanceID)
}

func (r *Repository) DeleteRDBReadReplica(id string) (map[string]any, error) {
	current, err := r.getJSONByID("rdb_read_replicas", "id", id)
	if err != nil {
		return nil, err
	}
	if err := r.deleteBy("rdb_read_replicas", "id = ?", id); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *Repository) CreateRDBReadReplicaEndpoint(id string, data map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("rdb_read_replicas", "id", id)
	if err != nil {
		return nil, err
	}
	next := cloneMap(current)
	eps, _ := next["endpoints"].([]any)
	ep := cloneMap(data)
	ep["id"] = newID()
	eps = append(eps, ep)
	next["endpoints"] = eps
	next["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("rdb_read_replicas", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) PromoteRDBReadReplica(id string) (map[string]any, error) {
	return r.getJSONByID("rdb_read_replicas", "id", id)
}

func (r *Repository) ResetRDBReadReplica(id string) (map[string]any, error) {
	return r.getJSONByID("rdb_read_replicas", "id", id)
}

// --- RDB Snapshots ---

func (r *Repository) CreateRDBSnapshot(region, instanceID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["region"] = region
	data["instance_id"] = instanceID
	data["status"] = "ready"
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["size"]; !ok {
		data["size"] = float64(10000000000)
	}
	return r.createSimple("rdb_snapshots", "region", region, data, colVal{name: "instance_id", val: instanceID})
}

func (r *Repository) GetRDBSnapshot(id string) (map[string]any, error) {
	return r.getJSONByID("rdb_snapshots", "id", id)
}

func (r *Repository) ListRDBSnapshots(region string) ([]map[string]any, error) {
	return r.listJSON("rdb_snapshots", "region", region)
}

func (r *Repository) UpdateRDBSnapshot(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("rdb_snapshots", "id", id)
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
	if err := r.updateJSONByID("rdb_snapshots", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteRDBSnapshot(id string) (map[string]any, error) {
	current, err := r.getJSONByID("rdb_snapshots", "id", id)
	if err != nil {
		return nil, err
	}
	if err := r.deleteBy("rdb_snapshots", "id = ?", id); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *Repository) CreateRDBInstanceFromSnapshot(region, snapshotID string, data map[string]any) (map[string]any, error) {
	snap, err := r.getJSONByID("rdb_snapshots", "id", snapshotID)
	if err != nil {
		return nil, err
	}
	base := cloneMap(snap)
	for k, v := range data {
		if k == "id" {
			continue
		}
		base[k] = v
	}
	delete(base, "instance_id")
	return r.CreateRDBInstance(region, base)
}

// --- RDB Backups ---

func (r *Repository) CreateRDBBackup(region, instanceID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	now := nowRFC3339()
	data["region"] = region
	data["instance_id"] = instanceID
	data["status"] = "ready"
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["size"]; !ok {
		data["size"] = float64(10000000000)
	}
	return r.createSimple("rdb_backups", "region", region, data, colVal{name: "instance_id", val: instanceID})
}

func (r *Repository) GetRDBBackup(id string) (map[string]any, error) {
	return r.getJSONByID("rdb_backups", "id", id)
}

func (r *Repository) ListRDBBackups(region, instanceID string) ([]map[string]any, error) {
	if instanceID != "" {
		return r.listJSON("rdb_backups", "instance_id", instanceID)
	}
	return r.listJSON("rdb_backups", "region", region)
}

func (r *Repository) UpdateRDBBackup(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("rdb_backups", "id", id)
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
	if err := r.updateJSONByID("rdb_backups", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteRDBBackup(id string) (map[string]any, error) {
	current, err := r.getJSONByID("rdb_backups", "id", id)
	if err != nil {
		return nil, err
	}
	if err := r.deleteBy("rdb_backups", "id = ?", id); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *Repository) ExportRDBBackup(id string) (map[string]any, error) {
	current, err := r.getJSONByID("rdb_backups", "id", id)
	if err != nil {
		return nil, err
	}
	current["download_url"] = "https://mock-backup-download.scw.cloud/" + id
	current["download_url_expires_at"] = nowRFC3339()
	return current, nil
}

func (r *Repository) RestoreRDBBackup(instanceID, backupID string, data map[string]any) (map[string]any, error) {
	// Validate both the referenced backup and target instance exist.
	// The Scaleway RDB restore-backup API returns the backup object on success.
	backup, err := r.getJSONByID("rdb_backups", "id", backupID)
	if err != nil {
		return nil, err
	}
	if _, err := r.getJSONByID("rdb_instances", "id", instanceID); err != nil {
		return nil, err
	}
	return backup, nil
}

func (r *Repository) CreateRDBEndpoint(instanceID string, data map[string]any) (map[string]any, error) {
	instance, err := r.getJSONByID("rdb_instances", "id", instanceID)
	if err != nil {
		return nil, err
	}
	ep := cloneMap(data)
	ep["id"] = newID()
	eps, _ := instance["endpoints"].([]any)
	eps = append(eps, ep)
	instance["endpoints"] = eps
	instance["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("rdb_instances", "id", instanceID, instance); err != nil {
		return nil, err
	}
	return ep, nil
}

func (r *Repository) DeleteRDBEndpoint(endpointID string) error {
	instances, err := r.listJSON("rdb_instances", "", "")
	if err != nil {
		return err
	}
	for _, inst := range instances {
		eps, _ := inst["endpoints"].([]any)
		newEps := make([]any, 0, len(eps))
		found := false
		for _, ep := range eps {
			epMap, ok := ep.(map[string]any)
			if !ok {
				newEps = append(newEps, ep)
				continue
			}
			if id, _ := epMap["id"].(string); id == endpointID {
				found = true
				continue
			}
			newEps = append(newEps, ep)
		}
		if found {
			inst["endpoints"] = newEps
			inst["updated_at"] = nowRFC3339()
			if id, ok := inst["id"].(string); ok {
				if err := r.updateJSONByID("rdb_instances", "id", id, inst); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return models.ErrNotFound
}

// --- Redis ACLs, Endpoints, Settings ---

func (r *Repository) SetRedisACLRules(clusterID string, rules []any) (map[string]any, error) {
	current, err := r.getJSONByID("redis_clusters", "id", clusterID)
	if err != nil {
		return nil, err
	}
	next := cloneMap(current)
	next["acl_rules"] = rules
	next["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("redis_clusters", "id", clusterID, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) SetRedisEndpoints(clusterID string, endpoints []any) (map[string]any, error) {
	current, err := r.getJSONByID("redis_clusters", "id", clusterID)
	if err != nil {
		return nil, err
	}
	next := cloneMap(current)
	next["endpoints"] = endpoints
	next["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("redis_clusters", "id", clusterID, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) SetRedisClusterSettings(clusterID string, settings []any) (map[string]any, error) {
	current, err := r.getJSONByID("redis_clusters", "id", clusterID)
	if err != nil {
		return nil, err
	}
	next := cloneMap(current)
	next["settings"] = settings
	next["updated_at"] = nowRFC3339()
	if err := r.updateJSONByID("redis_clusters", "id", clusterID, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteRedisEndpoint(zone, endpointID string) (map[string]any, error) {
	clusters, err := r.listJSON("redis_clusters", "zone", zone)
	if err != nil {
		return nil, err
	}
	for _, cluster := range clusters {
		eps, _ := cluster["endpoints"].([]any)
		newEps := make([]any, 0, len(eps))
		found := false
		for _, ep := range eps {
			epMap, ok := ep.(map[string]any)
			if !ok {
				newEps = append(newEps, ep)
				continue
			}
			if id, _ := epMap["id"].(string); id == endpointID {
				found = true
				continue
			}
			newEps = append(newEps, ep)
		}
		if found {
			cluster["endpoints"] = newEps
			cluster["updated_at"] = nowRFC3339()
			if id, ok := cluster["id"].(string); ok {
				if err := r.updateJSONByID("redis_clusters", "id", id, cluster); err != nil {
					return nil, err
				}
			}
			return cluster, nil
		}
	}
	return nil, models.ErrNotFound
}

// --- LB ACLs ---

func (r *Repository) CreateLBACL(frontendID string, data map[string]any) (map[string]any, error) {
	if _, err := r.GetFrontend(frontendID); err != nil {
		return nil, models.ErrNotFound
	}
	data = cloneMap(data)
	data["frontend_id"] = frontendID
	data["frontend"] = map[string]any{"id": frontendID}
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	return r.createSimple("lb_acls", "frontend_id", frontendID, data)
}

func (r *Repository) GetLBACL(id string) (map[string]any, error) {
	return r.getJSONByID("lb_acls", "id", id)
}

func (r *Repository) ListLBACLsByFrontend(frontendID string) ([]map[string]any, error) {
	return r.listJSON("lb_acls", "frontend_id", frontendID)
}

func (r *Repository) UpdateLBACL(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("lb_acls", "id", id)
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
	if err := r.updateJSONByID("lb_acls", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteLBACL(id string) error {
	return r.deleteBy("lb_acls", "id = ?", id)
}

// --- LB Routes ---

func (r *Repository) CreateLBRoute(lbID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	data["lb_id"] = lbID
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	return r.createSimple("lb_routes", "lb_id", lbID, data)
}

func (r *Repository) GetLBRoute(id string) (map[string]any, error) {
	return r.getJSONByID("lb_routes", "id", id)
}

func (r *Repository) ListLBRoutes(lbID, frontendID string) ([]map[string]any, error) {
	var all []map[string]any
	var err error
	if lbID != "" {
		all, err = r.listJSON("lb_routes", "lb_id", lbID)
	} else {
		all, err = r.listJSON("lb_routes", "", "")
	}
	if err != nil {
		return nil, err
	}
	if frontendID == "" {
		return all, nil
	}
	filtered := make([]map[string]any, 0, len(all))
	for _, route := range all {
		if fid, _ := route["frontend_id"].(string); fid == frontendID {
			filtered = append(filtered, route)
		}
	}
	return filtered, nil
}

func (r *Repository) UpdateLBRoute(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("lb_routes", "id", id)
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
	if err := r.updateJSONByID("lb_routes", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteLBRoute(id string) error {
	return r.deleteBy("lb_routes", "id = ?", id)
}

// --- LB Certificates ---

func (r *Repository) CreateLBCertificate(lbID string, data map[string]any) (map[string]any, error) {
	data = cloneMap(data)
	data["lb_id"] = lbID
	now := nowRFC3339()
	data["created_at"] = now
	data["updated_at"] = now
	if _, ok := data["status"]; !ok {
		data["status"] = "ready"
	}
	return r.createSimple("lb_certificates", "lb_id", lbID, data)
}

func (r *Repository) GetLBCertificate(id string) (map[string]any, error) {
	return r.getJSONByID("lb_certificates", "id", id)
}

func (r *Repository) ListLBCertificatesByLB(lbID string) ([]map[string]any, error) {
	return r.listJSON("lb_certificates", "lb_id", lbID)
}

func (r *Repository) UpdateLBCertificate(id string, patch map[string]any) (map[string]any, error) {
	current, err := r.getJSONByID("lb_certificates", "id", id)
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
	if err := r.updateJSONByID("lb_certificates", "id", id, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *Repository) DeleteLBCertificate(id string) error {
	return r.deleteBy("lb_certificates", "id = ?", id)
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
	if _, ok := data["is_public"]; !ok {
		data["is_public"] = false
	}
	if _, ok := data["organization_id"]; !ok {
		data["organization_id"] = "00000000-0000-0000-0000-000000000000"
	}
	if _, ok := data["project_id"]; !ok {
		data["project_id"] = "00000000-0000-0000-0000-000000000000"
	}
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
	iamUsers, err := r.listJSON("iam_users", "", "")
	if err != nil {
		return nil, err
	}
	iamGroups, err := r.ListIAMGroups()
	if err != nil {
		return nil, err
	}
	blockVolumes, err := r.listJSON("block_volumes", "", "")
	if err != nil {
		return nil, err
	}
	blockSnapshots, err := r.listJSON("block_snapshots", "", "")
	if err != nil {
		return nil, err
	}
	ipamIPs, err := r.listJSON("ipam_ips", "", "")
	if err != nil {
		return nil, err
	}
	rdbReadReplicas, err := r.listJSON("rdb_read_replicas", "", "")
	if err != nil {
		return nil, err
	}
	rdbSnapshots, err := r.listJSON("rdb_snapshots", "", "")
	if err != nil {
		return nil, err
	}
	rdbBackups, err := r.listJSON("rdb_backups", "", "")
	if err != nil {
		return nil, err
	}
	lbACLs, err := r.listJSON("lb_acls", "", "")
	if err != nil {
		return nil, err
	}
	lbRoutes, err := r.listJSON("lb_routes", "", "")
	if err != nil {
		return nil, err
	}
	lbCertificates, err := r.listJSON("lb_certificates", "", "")
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
			"acls":             lbACLs,
			"routes":           lbRoutes,
			"certificates":     lbCertificates,
		},
		"k8s": map[string]any{
			"clusters": clusters,
			"pools":    pools,
		},
		"rdb": map[string]any{
			"instances":     rdbInstances,
			"databases":     rdbDatabases,
			"users":         rdbUsers,
			"privileges":    rdbPrivileges,
			"read_replicas": rdbReadReplicas,
			"snapshots":     rdbSnapshots,
			"backups":       rdbBackups,
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
			"users":        iamUsers,
			"groups":       iamGroups,
		},
		"domain": map[string]any{
			"records": domainRecords,
		},
		"block": map[string]any{
			"volumes":   blockVolumes,
			"snapshots": blockSnapshots,
		},
		"ipam": map[string]any{
			"ips": ipamIPs,
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
		lbACLs, err := r.listJSON("lb_acls", "", "")
		if err != nil {
			return nil, err
		}
		lbRoutes, err := r.listJSON("lb_routes", "", "")
		if err != nil {
			return nil, err
		}
		lbCertificates, err := r.listJSON("lb_certificates", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"ips":              lbIPs,
			"lbs":              lbs,
			"frontends":        frontends,
			"backends":         backends,
			"private_networks": lbPNs,
			"acls":             lbACLs,
			"routes":           lbRoutes,
			"certificates":     lbCertificates,
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
		privileges, err := r.listJSON("rdb_privileges", "", "")
		if err != nil {
			return nil, err
		}
		readReplicas, err := r.listJSON("rdb_read_replicas", "", "")
		if err != nil {
			return nil, err
		}
		snapshots, err := r.listJSON("rdb_snapshots", "", "")
		if err != nil {
			return nil, err
		}
		backups, err := r.listJSON("rdb_backups", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"instances":     instances,
			"databases":     databases,
			"users":         users,
			"privileges":    privileges,
			"read_replicas": readReplicas,
			"snapshots":     snapshots,
			"backups":       backups,
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
		iamUsers, err := r.listJSON("iam_users", "", "")
		if err != nil {
			return nil, err
		}
		iamGroups, err := r.ListIAMGroups()
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"applications": applications,
			"api_keys":     apiKeys,
			"policies":     policies,
			"ssh_keys":     sshKeys,
			"users":        iamUsers,
			"groups":       iamGroups,
		}, nil
	case "block":
		blockVolumes, err := r.listJSON("block_volumes", "", "")
		if err != nil {
			return nil, err
		}
		blockSnapshots, err := r.listJSON("block_snapshots", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"volumes":   blockVolumes,
			"snapshots": blockSnapshots,
		}, nil
	case "ipam":
		ipamIPs, err := r.listJSON("ipam_ips", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"ips": ipamIPs,
		}, nil
	case "domain":
		records, err := r.listJSON("domain_records", "", "")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"records": records,
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
