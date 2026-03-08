package group

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGroupDB(t *testing.T) (*persistence.Database, context.Context) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db, context.Background()
}

func TestNewGroupRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewGroupRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_GetOrCreate_CreateNew(t *testing.T) {
	db, ctx := setupGroupDB(t)
	repo := NewGroupRepository(db.GormDB())

	id, err := repo.GetOrCreate(ctx, "telegram", "grp-123", "Mi Grupo")
	require.NoError(t, err)
	require.NotEmpty(t, id)
}

func TestRepository_GetOrCreate_GetExisting(t *testing.T) {
	db, ctx := setupGroupDB(t)
	repo := NewGroupRepository(db.GormDB())

	id1, err := repo.GetOrCreate(ctx, "telegram", "grp-456", "Grupo A")
	require.NoError(t, err)

	id2, err := repo.GetOrCreate(ctx, "telegram", "grp-456", "Grupo A")
	require.NoError(t, err)
	assert.Equal(t, id1, id2)
}

func TestRepository_GetOrCreate_UpdateName(t *testing.T) {
	db, ctx := setupGroupDB(t)
	repo := NewGroupRepository(db.GormDB())

	id, err := repo.GetOrCreate(ctx, "discord", "grp-789", "Nombre Original")
	require.NoError(t, err)

	id2, err := repo.GetOrCreate(ctx, "discord", "grp-789", "Nombre Actualizado")
	require.NoError(t, err)
	assert.Equal(t, id, id2)

	g, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Nombre Actualizado", g.Name)
}

func TestRepository_AddMember(t *testing.T) {
	db, ctx := setupGroupDB(t)
	repo := NewGroupRepository(db.GormDB())

	id, err := repo.GetOrCreate(ctx, "telegram", "grp-add", "Grupo")
	require.NoError(t, err)

	err = repo.AddMember(ctx, id, "user-1")
	require.NoError(t, err)

	err = repo.AddMember(ctx, id, "user-1") // idempotent
	require.NoError(t, err)

	members, err := repo.GetMembers(ctx, id)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "user-1", members[0])
}

func TestRepository_GetByID(t *testing.T) {
	db, ctx := setupGroupDB(t)
	repo := NewGroupRepository(db.GormDB())

	id, err := repo.GetOrCreate(ctx, "telegram", "grp-get", "Test Group")
	require.NoError(t, err)

	g, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, g)
	assert.Equal(t, id, g.ID)
	assert.Equal(t, "telegram", g.ChannelID)
	assert.Equal(t, "grp-get", g.PlatformGroupID)
	assert.Equal(t, "Test Group", g.Name)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, ctx := setupGroupDB(t)
	repo := NewGroupRepository(db.GormDB())

	g, err := repo.GetByID(ctx, "nonexistent")
	require.Error(t, err)
	assert.Nil(t, g)
}
