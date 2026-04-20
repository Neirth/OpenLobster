// Copyright (c) OpenLobster contributors. See LICENSE for details.

package user

import (
	"testing"
	"time"

	"github.com/google/uuid"
	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_ListAll_Empty(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	list, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestRepository_ListAll_SkipsNonUUID(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	// The migration already seeds a reserved `loopback` user. No need to insert it
	// again; the intent of this test is to ensure non-UUID IDs are skipped.

	// Also create a normal user.
	u := domainmodels.NewUser("real-user")
	require.NoError(t, repo.Create(ctx, u))

	list, err := repo.ListAll(ctx)
	require.NoError(t, err)
	// Only the real UUID user should be returned; loopback skipped.
	require.Len(t, list, 1)
	assert.Equal(t, "real-user", list[0].PrimaryID)
}

func TestRepository_ListAll_OrderByCreatedAt(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	u1 := &domainmodels.User{
		ID:        uuid.New(),
		PrimaryID: "first",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
		UpdatedAt: time.Now().UTC().Add(-time.Hour),
	}
	u2 := &domainmodels.User{
		ID:        uuid.New(),
		PrimaryID: "second",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, u1))
	require.NoError(t, repo.Create(ctx, u2))

	list, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	// ListAll orders DESC by created_at, so newest first.
	assert.Equal(t, "second", list[0].PrimaryID)
	assert.Equal(t, "first", list[1].PrimaryID)
}

func TestRepository_Update_Name(t *testing.T) {
	db, ctx := setupUserDB(t)
	repo := NewUserRepository(db.GormDB())

	u := domainmodels.NewUser("test-update")
	require.NoError(t, repo.Create(ctx, u))

	u.Name = "Alice"
	u.UpdatedAt = time.Now().UTC()
	require.NoError(t, repo.Update(ctx, u))

	found, err := repo.GetByID(ctx, u.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Alice", found.Name)
}
