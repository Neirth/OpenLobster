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
	db.GormDB().Exec("PRAGMA foreign_keys = ON")
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

	// Create a user first (needed for foreign key)
	user := domainmodels.UserModel{ID: "user-1", PrimaryID: "p1", Name: "User 1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	require.NoError(t, gormDB.Create(&user).Error)

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

	// Create a user first (needed for foreign key)
	user := domainmodels.UserModel{ID: "user-to-delete", PrimaryID: "p2", Name: "User To Delete", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, gormDB.Create(&user).Error)

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

func TestConversationRepository_DeleteGroup(t *testing.T) {
	gormDB := setupConversationDB(t)
	repo := NewConversationRepository(gormDB)
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Setup data
	user := domainmodels.UserModel{ID: "user-1", PrimaryID: "p1", Name: "User 1", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, gormDB.Create(&user).Error)

	group := domainmodels.GroupModel{ID: "group-1", ChannelID: "ch1", PlatformGroupID: "pg1", Name: "Group 1", CreatedAt: now}
	require.NoError(t, gormDB.Create(&group).Error)

	groupUser := domainmodels.GroupUserModel{GroupID: "group-1", UserID: "user-1", JoinedAt: now}
	require.NoError(t, gormDB.Create(&groupUser).Error)

	conv := domainmodels.ConversationModel{
		ID:        "conv-group",
		ChannelID: "ch1",
		GroupID:   &group.ID,
		UserID:    "user-1",
		ModelID:   "gpt-4",
		IsActive:  true,
		StartedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, gormDB.Create(&conv).Error)

	msg := domainmodels.MessageModel{
		ID:             "msg-1",
		ConversationID: "conv-group",
		Role:           "user",
		Content:        "Hello",
		CreatedAt:      now,
	}
	require.NoError(t, gormDB.Create(&msg).Error)

	// 2. Delete Group
	err := repo.DeleteGroup(ctx, "conv-group")
	require.NoError(t, err)

	// 3. Verify
	var count int64
	// Group should be gone
	gormDB.Model(&domainmodels.GroupModel{}).Where("id = ?", "group-1").Count(&count)
	assert.Equal(t, int64(0), count, "Group should be deleted")

	// Conversation should be gone (cascade)
	gormDB.Model(&domainmodels.ConversationModel{}).Where("id = ?", "conv-group").Count(&count)
	assert.Equal(t, int64(0), count, "Conversation should be deleted via cascade")

	// Message should be gone (cascade from conversation)
	gormDB.Model(&domainmodels.MessageModel{}).Where("id = ?", "msg-1").Count(&count)
	assert.Equal(t, int64(0), count, "Message should be deleted via cascade")

	// Group-User association should be gone (cascade)
	gormDB.Model(&domainmodels.GroupUserModel{}).Where("group_id = ?", "group-1").Count(&count)
	assert.Equal(t, int64(0), count, "GroupUser association should be deleted via cascade")

	// USER SHOULD STILL BE THERE
	gormDB.Model(&domainmodels.UserModel{}).Where("id = ?", "user-1").Count(&count)
	assert.Equal(t, int64(1), count, "User should NOT be deleted")
}

func TestConversationRepository_DeleteUser_FailsOnGroup(t *testing.T) {
	gormDB := setupConversationDB(t)
	repo := NewConversationRepository(gormDB)
	ctx := context.Background()
	now := time.Now().UTC()

	user := domainmodels.UserModel{ID: "user-1", PrimaryID: "p1", Name: "User 1", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, gormDB.Create(&user).Error)

	group := domainmodels.GroupModel{ID: "group-1", ChannelID: "ch1", PlatformGroupID: "pg1", Name: "Group 1", CreatedAt: now}
	require.NoError(t, gormDB.Create(&group).Error)

	conv := domainmodels.ConversationModel{
		ID:        "conv-group",
		ChannelID: "ch1",
		GroupID:   &group.ID,
		UserID:    "user-1",
		ModelID:   "gpt-4",
		IsActive:  true,
		StartedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, gormDB.Create(&conv).Error)

	err := repo.DeleteUser(ctx, "conv-group")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete user through a group conversation")
}

func TestConversationRepository_DeleteGroup_FailsOnPrivateChat(t *testing.T) {
	gormDB := setupConversationDB(t)
	repo := NewConversationRepository(gormDB)
	ctx := context.Background()
	now := time.Now().UTC()

	user := domainmodels.UserModel{ID: "user-1", PrimaryID: "p1", Name: "User 1", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, gormDB.Create(&user).Error)

	conv := domainmodels.ConversationModel{
		ID:        "conv-private",
		ChannelID: "ch1",
		GroupID:   nil,
		UserID:    "user-1",
		ModelID:   "gpt-4",
		IsActive:  true,
		StartedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, gormDB.Create(&conv).Error)

	err := repo.DeleteGroup(ctx, "conv-private")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "group not found (this might be a private chat)")
}
