package user_channel

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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

func insertUserChannelRow(t *testing.T, db *persistence.Database, userID, platformUID, username string, lastSeen time.Time) {
	t.Helper()
	id := uuid.New().String()
	err := db.GormDB().Exec(`
		INSERT INTO user_channels (id, user_id, channel_id, platform_user_id, username, paired_at, last_seen)
		VALUES (?, ?, 'telegram', ?, ?, ?, ?)`,
		id, userID, platformUID, username, lastSeen, lastSeen).Error
	require.NoError(t, err)
}

func TestRepository_ResolveChannelByStoredUsername_exact(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())
	ts := time.Now().UTC()
	insertUserChannelRow(t, db, "u1", "p100", "ExactUser", ts)

	ct, pid, err := repo.ResolveChannelByStoredUsername(ctx, "@ExactUser", "")
	require.NoError(t, err)
	assert.Equal(t, "telegram", ct)
	assert.Equal(t, "p100", pid)
}

func TestRepository_ResolveChannelByStoredUsername_typoPhase2(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())
	ts := time.Now().UTC()
	// "neirth_handle" — query "neirth_handl" won't match LIKE well; phase 2 fuzzy picks it (1 del)
	insertUserChannelRow(t, db, "u1", "p200", "neirth_handle", ts)

	ct, pid, err := repo.ResolveChannelByStoredUsername(ctx, "neirth_handl", "")
	require.NoError(t, err)
	assert.Equal(t, "telegram", ct)
	assert.Equal(t, "p200", pid)
}

func TestRepository_ResolveChannelByStoredUsername_tieBreaksByLastSeen(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())
	tOld := time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC)
	tNew := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	insertUserChannelRow(t, db, "u1", "p301", "tieusera", tOld)
	insertUserChannelRow(t, db, "u2", "p302", "tieuserz", tNew)
	// "tieuserx" distance 1 to both tieusera and tieuserz
	ct, pid, err := repo.ResolveChannelByStoredUsername(ctx, "tieuserx", "")
	require.NoError(t, err)
	assert.Equal(t, "telegram", ct)
	assert.Equal(t, "p302", pid)
}

func TestRepository_ResolveChannelByStoredUsername_platformFilter(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())
	require.NoError(t, channel.NewChannelRepository(db.GormDB()).EnsurePlatform(ctx, "discord", "Discord"))
	ts := time.Now().UTC()
	insertUserChannelRow(t, db, "u1", "tg1", "samename", ts)
	id := uuid.New().String()
	require.NoError(t, db.GormDB().Exec(`
		INSERT INTO user_channels (id, user_id, channel_id, platform_user_id, username, paired_at, last_seen)
		VALUES (?, ?, 'discord', ?, ?, ?, ?)`,
		id, "u2", "dc1", "samename", ts, ts).Error)

	ct, pid, err := repo.ResolveChannelByStoredUsername(ctx, "samename", "discord")
	require.NoError(t, err)
	assert.Equal(t, "discord", ct)
	assert.Equal(t, "dc1", pid)
}

func TestRepository_ResolveChannelByStoredUsername_noMatch(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())
	insertUserChannelRow(t, db, "u1", "p9", "short", time.Now().UTC())

	ct, pid, err := repo.ResolveChannelByStoredUsername(ctx, "completely_unrelated_string_xyz", "")
	require.NoError(t, err)
	assert.Empty(t, ct)
	assert.Empty(t, pid)
}
