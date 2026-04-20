// Copyright (c) OpenLobster contributors. See LICENSE for details.

package channel

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := &Repository{db: db.GormDB()}
	ctx := context.Background()

	ch, err := repo.GetByID(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, ch)
}

func TestRepository_EnsurePlatform_MultipleChannels(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := &Repository{db: db.GormDB()}
	ctx := context.Background()

	require.NoError(t, repo.EnsurePlatform(ctx, "telegram", "Telegram"))
	require.NoError(t, repo.EnsurePlatform(ctx, "discord", "Discord"))
	require.NoError(t, repo.EnsurePlatform(ctx, "whatsapp", "WhatsApp"))

	ch, err := repo.GetByID(ctx, "discord")
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Equal(t, "Discord", ch.Name)
}

func TestRepository_GetByID_CreatedAt(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	repo := &Repository{db: db.GormDB()}
	ctx := context.Background()

	require.NoError(t, repo.EnsurePlatform(ctx, "slack", "Slack"))

	ch, err := repo.GetByID(ctx, "slack")
	require.NoError(t, err)
	require.NotNil(t, ch)
	assert.Greater(t, ch.CreatedAt, int64(0))
}
