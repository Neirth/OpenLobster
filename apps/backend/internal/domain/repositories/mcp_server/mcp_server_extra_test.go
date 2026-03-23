// Copyright (c) OpenLobster contributors. See LICENSE for details.

package mcp_server

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_Save_UpdatesExisting(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := NewMCPServerRepository(db.GormDB())
	ctx := context.Background()

	// Insert initial record.
	require.NoError(t, repo.Save(ctx, "my-server", "http://old-url"))

	// Update via upsert.
	require.NoError(t, repo.Save(ctx, "my-server", "http://new-url"))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "http://new-url", all[0].URL)
}

func TestRepository_Delete(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := NewMCPServerRepository(db.GormDB())
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, "del-server", "http://example.com"))
	require.NoError(t, repo.Delete(ctx, "del-server"))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestRepository_Delete_NonExistent(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := NewMCPServerRepository(db.GormDB())
	ctx := context.Background()

	// Deleting a non-existent record should not error.
	err = repo.Delete(ctx, "does-not-exist")
	assert.NoError(t, err)
}

func TestRepository_ListAll_Empty(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := NewMCPServerRepository(db.GormDB())
	ctx := context.Background()

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestRepository_ListAll_MultipleServers(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := NewMCPServerRepository(db.GormDB())
	ctx := context.Background()

	require.NoError(t, repo.Save(ctx, "alpha", "http://alpha"))
	require.NoError(t, repo.Save(ctx, "beta", "http://beta"))

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}
