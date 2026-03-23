// Copyright (c) OpenLobster contributors. See LICENSE for details.

package user_channel

import (
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/repositories/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_GetLastChannelForUser_NotFound(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	ct, pid, err := repo.GetLastChannelForUser(ctx, "nonexistent-user")
	require.NoError(t, err)
	assert.Empty(t, ct)
	assert.Empty(t, pid)
}

func TestRepository_GetLastChannelForUser_Found(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	require.NoError(t, repo.Create(ctx, "user-99", "telegram", "plat-last", "testuser"))

	ct, pid, err := repo.GetLastChannelForUser(ctx, "user-99")
	require.NoError(t, err)
	assert.Equal(t, "telegram", ct)
	assert.Equal(t, "plat-last", pid)
}

func TestRepository_GetLastChannelForUser_ReturnsLatest(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())
	require.NoError(t, channel.NewChannelRepository(db.GormDB()).EnsurePlatform(ctx, "discord", "Discord"))

	tOld := time.Now().UTC().Add(-2 * time.Hour)
	tNew := time.Now().UTC()

	insertUserChannelRow(t, db, "user-77", "old-plat", "olduser", tOld)
	insertUserChannelRow(t, db, "user-77", "new-plat", "newuser", tNew)

	ct, pid, err := repo.GetLastChannelForUser(ctx, "user-77")
	require.NoError(t, err)
	assert.Equal(t, "new-plat", pid)
	_ = ct
}

func TestRepository_GetUserIDByName_Empty(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	id, err := repo.GetUserIDByName(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestRepository_GetUserIDByName_Whitespace(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	id, err := repo.GetUserIDByName(ctx, "   ")
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestRepository_GetUserIDByName_NotFound(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	id, err := repo.GetUserIDByName(ctx, "nobody")
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestRepository_GetUserIDByName_Found(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	// Insert directly into users table since GetUserIDByName queries the users table.
	require.NoError(t, db.GormDB().Exec(
		`INSERT INTO users (id, primary_id, name, created_at, updated_at) VALUES (?, ?, ?, datetime('now'), datetime('now'))`,
		"user-abc", "prim-abc", "Alice",
	).Error)

	id, err := repo.GetUserIDByName(ctx, "alice") // case-insensitive
	require.NoError(t, err)
	assert.Equal(t, "user-abc", id)
}

func TestRepository_UpdateLastSeen(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	require.NoError(t, repo.Create(ctx, "user-upd", "telegram", "plat-upd", "updateme"))

	err := repo.UpdateLastSeen(ctx, "telegram", "plat-upd")
	require.NoError(t, err)
}

func TestRepository_ResolveChannelByStoredUsername_EmptyUsername(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	ct, pid, err := repo.ResolveChannelByStoredUsername(ctx, "", "")
	require.NoError(t, err)
	assert.Empty(t, ct)
	assert.Empty(t, pid)
}

func TestRepository_ResolveChannelByStoredUsername_WithPlatformFilter(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())
	ts := time.Now().UTC()
	insertUserChannelRow(t, db, "u1", "p400", "filtereduser", ts)

	ct, pid, err := repo.ResolveChannelByStoredUsername(ctx, "filtereduser", "telegram")
	require.NoError(t, err)
	assert.Equal(t, "telegram", ct)
	assert.Equal(t, "p400", pid)
}

func TestRepository_Create_Upsert(t *testing.T) {
	db, ctx := setupUserChannelDB(t)
	repo := NewUserChannelRepository(db.GormDB())

	// Create twice - second should update username via ON CONFLICT.
	require.NoError(t, repo.Create(ctx, "user-u", "telegram", "plat-u", "oldname"))
	require.NoError(t, repo.Create(ctx, "user-u", "telegram", "plat-u", "newname"))

	name, err := repo.GetDisplayNameByPlatformUserID(ctx, "plat-u")
	require.NoError(t, err)
	assert.Equal(t, "newname", name)
}

func TestRepository_levenshtein_edgeCases(t *testing.T) {
	assert.Equal(t, 3, levenshtein("abc", ""))
	assert.Equal(t, 3, levenshtein("", "abc"))
	// Swap so that a > b to hit the swap branch.
	assert.Equal(t, 2, levenshtein("longer", "long"))
}

func TestRepository_maxAllowedEdits(t *testing.T) {
	// needleLen <= 0
	assert.Equal(t, 0, maxAllowedEdits(0))
	assert.Equal(t, 0, maxAllowedEdits(-1))
	// needleLen/4 < 1 → returns 1
	assert.Equal(t, 1, maxAllowedEdits(1))
	assert.Equal(t, 1, maxAllowedEdits(3))
	// needleLen/4 = 2 → returns 2
	assert.Equal(t, 2, maxAllowedEdits(8))
	// needleLen/4 > 3 → capped at 3
	assert.Equal(t, 3, maxAllowedEdits(16))
	assert.Equal(t, 3, maxAllowedEdits(100))
}

func TestRepository_pickBestUsernameMatch_empty(t *testing.T) {
	_, _, ok := pickBestUsernameMatch(nil, "needle")
	assert.False(t, ok)

	_, _, ok = pickBestUsernameMatch([]usernameCandidate{}, "needle")
	assert.False(t, ok)

	_, _, ok = pickBestUsernameMatch([]usernameCandidate{{NormUsername: "x"}}, "")
	assert.False(t, ok)
}

func TestRepository_min3(t *testing.T) {
	assert.Equal(t, 1, min3(1, 2, 3))
	assert.Equal(t, 1, min3(2, 1, 3))
	assert.Equal(t, 1, min3(2, 3, 1))
	assert.Equal(t, 1, min3(1, 1, 1))
}
