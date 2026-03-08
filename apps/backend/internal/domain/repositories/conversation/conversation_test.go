package conversation

import (
	"context"
	"testing"
	"time"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupConversationDB(t *testing.T) *gorm.DB {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db.GormDB()
}

func TestNewConversationRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewConversationRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestConversationRepository_ListConversations(t *testing.T) {
	gormDB := setupConversationDB(t)
	repo := NewConversationRepository(gormDB)

	rows, err := repo.ListConversations()
	require.NoError(t, err)
	require.NotNil(t, rows)
	assert.Len(t, rows, 0)
}

func TestConversationRepository_ListConversations_WithData(t *testing.T) {
	gormDB := setupConversationDB(t)
	repo := NewConversationRepository(gormDB)

	// Create a conversation via the model
	conv := domainmodels.ConversationModel{
		ID:        "conv-1",
		ChannelID: "telegram",
		UserID:    "user-1",
		ModelID:   "gpt-4",
		IsActive:  true,
		StartedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	require.NoError(t, gormDB.Create(&conv).Error)

	rows, err := repo.ListConversations()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "conv-1", rows[0].ID)
	assert.Equal(t, "telegram", rows[0].ChannelID)
}

func TestConversationRepository_DeleteUser(t *testing.T) {
	gormDB := setupConversationDB(t)
	repo := NewConversationRepository(gormDB)
	ctx := context.Background()
	now := time.Now().UTC()

	conv := domainmodels.ConversationModel{
		ID:        "conv-del",
		ChannelID: "telegram",
		UserID:    "user-to-delete",
		ModelID:   "gpt-4",
		IsActive:  true,
		StartedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, gormDB.Create(&conv).Error)

	err := repo.DeleteUser(ctx, "conv-del")
	require.NoError(t, err)

	var count int64
	gormDB.Model(&domainmodels.ConversationModel{}).Where("id = ?", "conv-del").Count(&count)
	assert.Equal(t, int64(0), count)
}
