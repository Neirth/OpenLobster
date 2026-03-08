package user

import (
	"context"
	"database/sql"
	"testing"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserDB(t *testing.T) (*persistence.Database, context.Context) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db, context.Background()
}

func TestNewUserRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewUserRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_CreateAndGetByPrimaryID(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	u := domainmodels.NewUser("primary-123")
	require.NoError(t, repo.Create(ctx, u))

	found, err := repo.GetByPrimaryID(ctx, "primary-123")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, u.ID, found.ID)
	assert.Equal(t, "primary-123", found.PrimaryID)
}

func TestRepository_GetByID(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	u := domainmodels.NewUser("primary-456")
	require.NoError(t, repo.Create(ctx, u))

	found, err := repo.GetByID(ctx, u.ID.String())
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, u.ID, found.ID)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	found, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	require.Error(t, err)
	assert.True(t, err == sql.ErrNoRows)
	assert.Nil(t, found)
}

func TestRepository_GetByPrimaryID_NotFound(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	found, err := repo.GetByPrimaryID(ctx, "nonexistent")
	require.Error(t, err)
	assert.True(t, err == sql.ErrNoRows)
	assert.Nil(t, found)
}

func TestRepository_Update(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	u := domainmodels.NewUser("original")
	require.NoError(t, repo.Create(ctx, u))

	u.PrimaryID = "updated"
	u.UpdatedAt = u.UpdatedAt.Add(1)
	require.NoError(t, repo.Update(ctx, u))

	found, _ := repo.GetByPrimaryID(ctx, "updated")
	require.NotNil(t, found)
	assert.Equal(t, "updated", found.PrimaryID)
}

func TestRepository_ListAll(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	require.NoError(t, repo.Create(ctx, domainmodels.NewUser("p1")))
	require.NoError(t, repo.Create(ctx, domainmodels.NewUser("p2")))

	list, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
}
