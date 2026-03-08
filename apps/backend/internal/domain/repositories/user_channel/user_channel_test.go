package user_channel

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/domain/repositories/channel"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserChannelDB(t *testing.T) (*persistence.Database, context.Context) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	chRepo := channel.NewChannelRepository(db.GormDB())
	require.NoError(t, chRepo.EnsurePlatform(context.Background(), "telegram", "Telegram"))
	return db, context.Background()
}

func TestNewUserChannelRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewUserChannelRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_ExistsByPlatformUserID(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	exists, err := repo.ExistsByPlatformUserID(ctx, "unknown")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, repo.Create(ctx, "user-1", "telegram", "platform-123", "alice"))

	exists, err = repo.ExistsByPlatformUserID(ctx, "platform-123")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRepository_GetUserIDByPlatformUserID(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	userID, err := repo.GetUserIDByPlatformUserID(ctx, "unknown")
	require.NoError(t, err)
	assert.Empty(t, userID)

	require.NoError(t, repo.Create(ctx, "user-1", "telegram", "plat-456", "bob"))

	userID, err = repo.GetUserIDByPlatformUserID(ctx, "plat-456")
	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
}

func TestRepository_GetDisplayNameByPlatformUserID_FallsBackToUsername(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	name, err := repo.GetDisplayNameByPlatformUserID(ctx, "unknown")
	require.NoError(t, err)
	assert.Empty(t, name)

	// No users.name set — should fall back to username, then platform_user_id.
	require.NoError(t, repo.Create(ctx, "user-1", "telegram", "plat-789", "carol"))

	name, err = repo.GetDisplayNameByPlatformUserID(ctx, "plat-789")
	require.NoError(t, err)
	assert.Equal(t, "carol", name)
}

func TestRepository_GetDisplayNameByUserID_FallsBackToPrimaryID(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	require.NoError(t, repo.Create(ctx, "user-1", "telegram", "plat-disp", "charlie"))

	// users table has no row for user-1, so result should be empty.
	name, err := repo.GetDisplayNameByUserID(ctx, "user-1")
	require.NoError(t, err)
	assert.Empty(t, name)
}

func TestRepository_Create(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	err := repo.Create(ctx, "user-1", "telegram", "platform-new", "dave")
	require.NoError(t, err)
}
