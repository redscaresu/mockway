package repository_test

import (
	"testing"

	"github.com/redscaresu/mockway/models"
	"github.com/redscaresu/mockway/repository"
	"github.com/stretchr/testify/require"
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
