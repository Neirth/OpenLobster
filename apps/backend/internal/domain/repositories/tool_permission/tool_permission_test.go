package tool_permission

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupToolPermDB(t *testing.T) (*persistence.Database, context.Context) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db, context.Background()
}

func TestNewToolPermissionRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewToolPermissionRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_SetAndListByUser(t *testing.T) {
	db, ctx := setupToolPermDB(t)
	repo := NewToolPermissionRepository(db.GormDB())

	require.NoError(t, repo.Set(ctx, "user1", "read_file", "allow"))

	rows, err := repo.ListByUser(ctx, "user1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "user1", rows[0].UserID)
	assert.Equal(t, "read_file", rows[0].ToolName)
	assert.Equal(t, "allow", rows[0].Mode)
}

func TestRepository_Set_Update(t *testing.T) {
	db, ctx := setupToolPermDB(t)
	repo := NewToolPermissionRepository(db.GormDB())

	require.NoError(t, repo.Set(ctx, "user1", "write_file", "deny"))
	require.NoError(t, repo.Set(ctx, "user1", "write_file", "allow"))

	rows, _ := repo.ListByUser(ctx, "user1")
	require.Len(t, rows, 1)
	assert.Equal(t, "allow", rows[0].Mode)
}

func TestRepository_Delete(t *testing.T) {
	db, ctx := setupToolPermDB(t)
	repo := NewToolPermissionRepository(db.GormDB())

	require.NoError(t, repo.Set(ctx, "user1", "delete_me", "ask"))
	require.NoError(t, repo.Delete(ctx, "user1", "delete_me"))

	rows, _ := repo.ListByUser(ctx, "user1")
	assert.Len(t, rows, 0)
}

func TestRepository_ListAll(t *testing.T) {
	db, ctx := setupToolPermDB(t)
	repo := NewToolPermissionRepository(db.GormDB())

	require.NoError(t, repo.Set(ctx, "user1", "tool_a", "allow"))
	require.NoError(t, repo.Set(ctx, "user2", "tool_b", "ask"))

	rows, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
}
