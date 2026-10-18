package repository

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A --db file from before bind_seq kept a server's IP order only in the
// server's stored public_ips. Upgrading must read that order back, not
// the order the IPs were created in. Internal test because the old
// schema can only be rebuilt below the public API.
func TestMigrationBackfillsBindOrderFromStoredPublicIPs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mockway.db")
	repo, err := New(path)
	require.NoError(t, err)

	newIP := func() string {
		ip, err := repo.CreateIP("fr-par-1", map[string]any{})
		require.NoError(t, err)
		return ip["id"].(string)
	}
	a, b, c := newIP(), newIP(), newIP()
	server, err := repo.CreateServer("fr-par-1", map[string]any{"name": "web", "public_ips": []any{b, a}})
	require.NoError(t, err)
	serverID := server["id"].(string)

	// Rebuild what older mockway left: no bind_seq column, the order in
	// the server's blob, and c attached later through PATCH /ips, which
	// that blob never saw.
	for _, stmt := range []string{
		`UPDATE instance_ips SET server_id = '` + serverID + `' WHERE id = '` + c + `'`,
		`ALTER TABLE instance_ips DROP COLUMN bind_seq`,
		`DELETE FROM schema_versions WHERE version = 2`,
		`UPDATE instance_servers SET data = json_set(data, '$.public_ips', json_array(
			json_object('id', '` + b + `', 'dynamic', json('false')),
			json_object('id', '` + a + `', 'dynamic', json('false'))))`,
	} {
		_, err := repo.db.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	require.NoError(t, repo.Close())

	repo, err = New(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Close() })
	got, err := repo.GetServer(serverID)
	require.NoError(t, err)
	var ids []any
	for _, ip := range got["public_ips"].([]any) {
		ids = append(ids, ip.(map[string]any)["id"])
	}
	assert.Equal(t, []any{b, a, c}, ids)
}
