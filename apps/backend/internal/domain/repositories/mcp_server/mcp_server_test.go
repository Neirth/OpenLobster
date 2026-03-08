package mcp_server

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMCPServerRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewMCPServerRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_SaveAndListAll(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := NewMCPServerRepository(db.GormDB())
	ctx := context.Background()

	err = repo.Save(ctx, "test-server", "http://localhost:8080")
	assert.NoError(t, err)

	all, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "test-server", all[0].Name)
	assert.Equal(t, "http://localhost:8080", all[0].URL)
}
