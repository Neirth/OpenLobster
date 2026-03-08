package channel

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewChannelRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewChannelRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_EnsurePlatform(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := &Repository{db: db.GormDB()}
	ctx := context.Background()

	err = repo.EnsurePlatform(ctx, "telegram", "Telegram")
	assert.NoError(t, err)

	err = repo.EnsurePlatform(ctx, "telegram", "Telegram")
	assert.NoError(t, err)
}

func TestRepository_GetByID(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := &Repository{db: db.GormDB()}
	ctx := context.Background()
	require.NoError(t, repo.EnsurePlatform(ctx, "discord", "Discord"))

	ch, err := repo.GetByID(ctx, "discord")
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, "discord", ch.ID)
	assert.Equal(t, "discord", ch.Type)
	assert.Equal(t, "Discord", ch.Name)
}
