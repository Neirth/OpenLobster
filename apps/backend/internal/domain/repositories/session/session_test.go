package session

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSessionDB(t *testing.T) (*persistence.Database, context.Context) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db, context.Background()
}

func TestNewSessionRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewSessionRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_Create(t *testing.T) {
	db, ctx := setupSessionDB(t)
	repo := NewSessionRepository(db.GormDB())

	now := time.Now().UTC()
	sess := &models.Session{
		ID:        uuid.New(),
		UserID:    "user-1",
		ChannelID: "telegram",
		ModelID:   "gpt-4",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.Create(ctx, sess)
	require.NoError(t, err)
}

func TestRepository_Create_WithGroupID(t *testing.T) {
	db, ctx := setupSessionDB(t)
	repo := NewSessionRepository(db.GormDB())

	gid := uuid.New()
	sess := &models.Session{
		ID:        uuid.New(),
		UserID:    "user-1",
		GroupID:   &gid,
		ChannelID: "telegram",
		ModelID:   "gpt-4",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	err := repo.Create(ctx, sess)
	require.NoError(t, err)
}

func TestRepository_GetByID(t *testing.T) {
	db, ctx := setupSessionDB(t)
	repo := NewSessionRepository(db.GormDB())

	sess := &models.Session{
		ID:        uuid.New(),
		UserID:    "user-1",
		ChannelID: "telegram",
		ModelID:   "gpt-4",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, sess))

	got, err := repo.GetByID(ctx, sess.ID.String())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, sess.ID, got.ID)
	assert.Equal(t, "user-1", got.UserID)
	assert.Equal(t, "telegram", got.ChannelID)
}

func TestRepository_Update(t *testing.T) {
	db, ctx := setupSessionDB(t)
	repo := NewSessionRepository(db.GormDB())

	sess := &models.Session{
		ID:        uuid.New(),
		UserID:    "user-1",
		ChannelID: "telegram",
		ModelID:   "gpt-4",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, sess))

	sess.IsActive = false
	sess.UpdatedAt = time.Now().UTC()
	err := repo.Update(ctx, sess)
	require.NoError(t, err)

	got, _ := repo.GetByID(ctx, sess.ID.String())
	assert.False(t, got.IsActive)
}

func TestRepository_GetActiveByUser(t *testing.T) {
	db, ctx := setupSessionDB(t)
	repo := NewSessionRepository(db.GormDB())

	sess := &models.Session{
		ID:        uuid.New(),
		UserID:    "user-1",
		ChannelID: "telegram",
		ModelID:   "gpt-4",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, sess))

	list, err := repo.GetActiveByUser(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, sess.ID, list[0].ID)
}

func TestRepository_GetActiveByChannel(t *testing.T) {
	db, ctx := setupSessionDB(t)
	repo := NewSessionRepository(db.GormDB())

	sess := &models.Session{
		ID:        uuid.New(),
		UserID:    "user-1",
		ChannelID: "telegram",
		ModelID:   "gpt-4",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, sess))

	list, err := repo.GetActiveByChannel(ctx, "telegram")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "telegram", list[0].ChannelID)
}

func TestRepository_GetActiveByGroup(t *testing.T) {
	db, ctx := setupSessionDB(t)
	repo := NewSessionRepository(db.GormDB())

	gid := uuid.New()
	sess := &models.Session{
		ID:        uuid.New(),
		UserID:    "user-1",
		GroupID:   &gid,
		ChannelID: "telegram",
		ModelID:   "gpt-4",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, sess))

	list, err := repo.GetActiveByGroup(ctx, gid.String())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.NotNil(t, list[0].GroupID)
	assert.Equal(t, gid, *list[0].GroupID)
}

func TestRepository_GetActiveByUserNoCtx(t *testing.T) {
	db, ctx := setupSessionDB(t)
	repo := NewSessionRepository(db.GormDB())

	sess := &models.Session{
		ID:        uuid.New(),
		UserID:    "user-1",
		ChannelID: "telegram",
		ModelID:   "gpt-4",
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, repo.Create(ctx, sess))

	list, err := repo.GetActiveByUserNoCtx("user-1")
	require.NoError(t, err)
	require.Len(t, list, 1)
}
