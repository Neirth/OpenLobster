package pairing

import (
	"context"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPairingDB(t *testing.T) (*persistence.Database, context.Context) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db, context.Background()
}

func TestNewPairingRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewPairingRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestRepository_Create(t *testing.T) {
	db, ctx := setupPairingDB(t)
	repo := NewPairingRepository(db.GormDB())

	now := time.Now().UTC().Add(5 * time.Minute)
	p := &ports.Pairing{
		Code:             "ABC123",
		ChannelID:        "telegram",
		PlatformUserID:   "user-123",
		PlatformUserName: "Alice",
		ChannelType:      "telegram",
		ExpiresAt:        now.Unix(),
		Status:           PairingStatusPending,
		CreatedAt:        time.Now().UTC().Unix(),
	}
	err := repo.Create(ctx, p)
	require.NoError(t, err)
}

func TestRepository_GetByCode(t *testing.T) {
	db, ctx := setupPairingDB(t)
	repo := NewPairingRepository(db.GormDB())

	now := time.Now().UTC().Add(10 * time.Minute)
	p := &ports.Pairing{
		Code:             "XYZ789",
		ChannelID:        "telegram",
		PlatformUserID:   "user-456",
		PlatformUserName: "Bob",
		ChannelType:      "telegram",
		ExpiresAt:        now.Unix(),
		Status:           PairingStatusPending,
		CreatedAt:        time.Now().UTC().Unix(),
	}
	require.NoError(t, repo.Create(ctx, p))

	got, err := repo.GetByCode(ctx, "XYZ789")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "XYZ789", got.Code)
	assert.Equal(t, "Bob", got.PlatformUserName)
}

func TestRepository_UpdateStatus(t *testing.T) {
	db, ctx := setupPairingDB(t)
	repo := NewPairingRepository(db.GormDB())

	p := &ports.Pairing{
		Code:             "STAT1",
		ChannelID:        "telegram",
		PlatformUserID:   "u1",
		PlatformUserName: "User",
		ChannelType:      "telegram",
		ExpiresAt:        time.Now().UTC().Add(time.Hour).Unix(),
		Status:           PairingStatusPending,
		CreatedAt:        time.Now().UTC().Unix(),
	}
	require.NoError(t, repo.Create(ctx, p))

	err := repo.UpdateStatus(ctx, "STAT1", "paired")
	require.NoError(t, err)

	got, _ := repo.GetByCode(ctx, "STAT1")
	assert.Equal(t, "paired", got.Status)
}

func TestRepository_UpdateStatusIfPending(t *testing.T) {
	db, ctx := setupPairingDB(t)
	repo := NewPairingRepository(db.GormDB())

	p := &ports.Pairing{
		Code:             "PEND1",
		ChannelID:        "telegram",
		PlatformUserID:   "u1",
		PlatformUserName: "User",
		ChannelType:      "telegram",
		ExpiresAt:        time.Now().UTC().Add(time.Hour).Unix(),
		Status:           PairingStatusPending,
		CreatedAt:        time.Now().UTC().Unix(),
	}
	require.NoError(t, repo.Create(ctx, p))

	ok, err := repo.UpdateStatusIfPending(ctx, "PEND1", "paired")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = repo.UpdateStatusIfPending(ctx, "PEND1", "paired") // already updated
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRepository_ListActive(t *testing.T) {
	db, ctx := setupPairingDB(t)
	repo := NewPairingRepository(db.GormDB())

	p := &ports.Pairing{
		Code:             "ACT1",
		ChannelID:        "telegram",
		PlatformUserID:   "u1",
		PlatformUserName: "User",
		ChannelType:      "telegram",
		ExpiresAt:        time.Now().UTC().Add(time.Hour).Unix(),
		Status:           PairingStatusPending,
		CreatedAt:        time.Now().UTC().Unix(),
	}
	require.NoError(t, repo.Create(ctx, p))

	list, err := repo.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "ACT1", list[0].Code)
}

func TestRepository_DeleteExpired(t *testing.T) {
	db, ctx := setupPairingDB(t)
	repo := NewPairingRepository(db.GormDB())

	p := &ports.Pairing{
		Code:             "EXP1",
		ChannelID:        "telegram",
		PlatformUserID:   "u1",
		PlatformUserName: "User",
		ChannelType:      "telegram",
		ExpiresAt:        time.Now().UTC().Add(-time.Hour).Unix(), // expired
		Status:           PairingStatusPending,
		CreatedAt:        time.Now().UTC().Add(-2 * time.Hour).Unix(),
	}
	require.NoError(t, repo.Create(ctx, p))

	err := repo.DeleteExpired(ctx)
	require.NoError(t, err)

	_, err = repo.GetByCode(ctx, "EXP1")
	require.Error(t, err)
}
