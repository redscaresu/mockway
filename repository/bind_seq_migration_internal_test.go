package repository

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oldBindSchemaDB writes a --db file as mockway left it before bind_seq:
// no bind_seq column, migration 2 unrecorded, and a server whose stored
// public_ips lists three IPs since deleted, then b, then a. spare, created
// before them all, was attached later through PATCH /ips, which that copy
// never saw. Internal because the old schema is only reachable below the
// public API.
func oldBindSchemaDB(t *testing.T) (path, serverID, spare, a, b string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "mockway.db")
	repo, err := New(path)
	require.NoError(t, err)
	newIP := func() string {
		ip, err := repo.CreateIP("fr-par-1", map[string]any{})
		require.NoError(t, err)
		return ip["id"].(string)
	}
	spare, a, b = newIP(), newIP(), newIP()
	server, err := repo.CreateServer("fr-par-1", map[string]any{"name": "web", "public_ips": []any{b, a}})
	require.NoError(t, err)
	serverID = server["id"].(string)
	gone := func(id string) string {
		return `json_object('id', '` + id + `', 'dynamic', json('false'))`
	}
	for _, stmt := range []string{
		`UPDATE instance_ips SET server_id = '` + serverID + `' WHERE id = '` + spare + `'`,
		`ALTER TABLE instance_ips DROP COLUMN bind_seq`,
		`DELETE FROM schema_versions WHERE version = 2`,
		`UPDATE instance_servers SET data = json_set(data,
			'$.public_ips', json_array(` + gone("gone-1") + `, ` + gone("gone-2") + `, ` + gone("gone-3") + `, ` + gone(b) + `, ` + gone(a) + `),
			'$.public_ip', ` + gone("gone-1") + `)`,
	} {
		_, err := repo.db.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	require.NoError(t, repo.Close())
	return path, serverID, spare, a, b
}

func publicIPIDs(t *testing.T, repo *Repository, serverID string) []any {
	t.Helper()
	server, err := repo.GetServer(serverID)
	require.NoError(t, err)
	var ids []any
	for _, ip := range server["public_ips"].([]any) {
		ids = append(ids, ip.(map[string]any)["id"])
	}
	return ids
}

// Upgrading reads a server's IP order from what older mockway stored,
// not the order the IPs were created in. An IP missing from that copy
// goes after every listed one, even when listed IPs were since deleted:
// counting rows put spare level with b. The copy is then dropped, so no
// writer that keeps a blob as it found it can carry it forward.
func TestMigrationBackfillsBindOrderFromStoredPublicIPs(t *testing.T) {
	path, serverID, spare, a, b := oldBindSchemaDB(t)

	repo, err := New(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Close() })
	assert.Equal(t, []any{b, a, spare}, publicIPIDs(t, repo, serverID))

	var stale int
	require.NoError(t, repo.db.QueryRow(`SELECT COUNT(*) FROM instance_servers
		WHERE json_type(data, '$.public_ips') IS NOT NULL OR json_type(data, '$.public_ip') IS NOT NULL`).Scan(&stale))
	assert.Zero(t, stale, "the server blob still holds a copy of its IPs")
}

// A migration that fails part-way leaves nothing behind, so the next
// boot runs it whole instead of failing on "duplicate column name".
func TestMigrationFailingPartWayIsRetriedWhole(t *testing.T) {
	path, serverID, spare, a, b := oldBindSchemaDB(t)
	repo, err := openDB(path)
	require.NoError(t, err)
	// The backfill reads the server blob; malformed JSON fails it after
	// the ALTER TABLE has run.
	_, err = repo.Exec(`UPDATE instance_servers SET data = '{"broken' WHERE id = ?`, serverID)
	require.NoError(t, err)

	_, err = New(path)
	require.Error(t, err)

	_, err = repo.Exec(`UPDATE instance_servers SET data = json_object('id', ?1, 'name', 'web',
		'public_ips', json_array(json_object('id', ?2), json_object('id', ?3)))
		WHERE id = ?1`, serverID, b, a)
	require.NoError(t, err)
	require.NoError(t, repo.Close())

	upgraded, err := New(path)
	require.NoError(t, err, "a half-applied migration blocks every later boot")
	t.Cleanup(func() { _ = upgraded.Close() })
	assert.Equal(t, []any{b, a, spare}, publicIPIDs(t, upgraded, serverID))
}
