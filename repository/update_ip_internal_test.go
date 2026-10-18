package repository

import (
	"testing"

	"github.com/redscaresu/mockway/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UpdateIP answers a constraint failure on its write as the domain error
// it means, not a bare SQL error the handler turns into a 500. Internal
// because no schema constraint can fail that write today; a trigger
// stands in for one.
func TestUpdateIPMapsConstraintErrors(t *testing.T) {
	repo, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Close() })

	ip, err := repo.CreateIP("fr-par-1", map[string]any{})
	require.NoError(t, err)
	_, err = repo.db.Exec(`CREATE TRIGGER fail_ip_data BEFORE UPDATE OF data ON instance_ips
		BEGIN SELECT RAISE(ABORT, 'UNIQUE constraint failed: instance_ips.data'); END`)
	require.NoError(t, err)

	_, err = repo.UpdateIP(ip["id"].(string), map[string]any{"tags": []any{"x"}})
	assert.ErrorIs(t, err, models.ErrConflict)
}
