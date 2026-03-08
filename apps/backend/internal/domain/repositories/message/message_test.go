package message

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMessageDB(t *testing.T) (*gorm.DB, context.Context) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db.GormDB(), context.Background()
}

func TestNewMessageRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	repo := NewMessageRepository(db.GormDB())
	require.NotNil(t, repo)
}

func TestNewDashboardMessageRepository(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	inner := NewMessageRepository(db.GormDB())
	wrapper := NewDashboardMessageRepository(inner)
	require.NotNil(t, wrapper)
}

func TestRepository_Save(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "user",
		Content:        "hello",
		Timestamp:      time.Now().UTC(),
	}
	err := repo.Save(ctx, msg)
	require.NoError(t, err)
}

func TestRepository_Save_WithAudio(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New().String(),
		Role:           "user",
		Content:        "",
		Timestamp:      time.Now().UTC(),
		Audio:          &models.AudioContent{Data: []byte{1, 2, 3}, Format: "ogg"},
	}
	err := repo.Save(ctx, msg)
	require.NoError(t, err)
}

func TestRepository_Save_WithGroupID(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	// GroupID is handled via conversations; ensure Save accepts messages without group info.
	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New().String(),
		Role:           "assistant",
		Content:        "hi",
		Timestamp:      time.Now().UTC(),
	}
	err := repo.Save(ctx, msg)
	require.NoError(t, err)
}

func TestRepository_GetByConversation(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "user",
		Content:        "first",
		Timestamp:      time.Now().UTC(),
	}
	require.NoError(t, repo.Save(ctx, msg))

	msgs, err := repo.GetByConversation(ctx, convID, 10)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "first", msgs[0].Content)
	assert.Equal(t, "user", msgs[0].Role)
}

func TestRepository_GetByConversation_WithLimit(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	for i := 0; i < 5; i++ {
		msg := &models.Message{
			ID:             uuid.New(),
			ConversationID: convID,
			Role:           "user",
			Content:        "msg",
			Timestamp:      time.Now().UTC().Add(time.Duration(i) * time.Second),
		}
		require.NoError(t, repo.Save(ctx, msg))
	}

	msgs, err := repo.GetByConversation(ctx, convID, 2)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
}

func TestRepository_GetLastCompaction_None(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	comp, err := repo.GetLastCompaction(ctx, uuid.New().String())
	require.NoError(t, err)
	assert.Nil(t, comp)
}

func TestRepository_GetLastCompaction_Exists(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "compaction",
		Content:        "summary",
		Timestamp:      time.Now().UTC(),
	}
	require.NoError(t, repo.Save(ctx, msg))

	comp, err := repo.GetLastCompaction(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, comp)
	assert.Equal(t, "compaction", comp.Role)
}

func TestRepository_GetSinceLastCompaction_NoCompaction(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "user",
		Content:        "hello",
		Timestamp:      time.Now().UTC(),
	}
	require.NoError(t, repo.Save(ctx, msg))

	msgs, err := repo.GetSinceLastCompaction(ctx, convID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
}

func TestRepository_GetSinceLastCompaction_WithCompaction(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	t1 := time.Now().UTC().Add(-2 * time.Hour)
	t2 := time.Now().UTC().Add(-1 * time.Hour)
	t3 := time.Now().UTC()

		require.NoError(t, repo.Save(ctx, &models.Message{
			ID:             uuid.New(),
			ConversationID: convID,
			Role:           "user",
			Content:        "before",
			Timestamp:      t1,
		}))
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "compaction",
		Content:        "summary",
		Timestamp:      t2,
	}))
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "user",
		Content:        "after",
		Timestamp:      t3,
	}))

	msgs, err := repo.GetSinceLastCompaction(ctx, convID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "after", msgs[0].Content)
}

func TestRepository_CountMessages(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB).(*repository)

	convID := uuid.New().String()
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "user",
		Content:        "x",
		Timestamp:      time.Now().UTC(),
	}))
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "assistant",
		Content:        "y",
		Timestamp:      time.Now().UTC(),
	}))

	recv, sent, err := repo.CountMessages(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), recv)
	assert.Equal(t, int64(1), sent)
}

func TestDashboardMessageRepository_Save(t *testing.T) {
	gormDB, _ := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	msg := &models.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New().String(),
		Role:           "user",
		Content:        "test",
		Timestamp:      time.Now().UTC(),
	}
	err := wrapper.Save(msg)
	require.NoError(t, err)
}

func TestDashboardMessageRepository_GetByConversation(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	convID := uuid.New().String()
	require.NoError(t, inner.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "user",
		Content:        "x",
		Timestamp:      time.Now().UTC(),
	}))

	msgs, err := wrapper.GetByConversation(convID, 10)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
}

func TestDashboardMessageRepository_CountMessages(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	require.NoError(t, inner.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New().String(),
		Role:           "user",
		Content:        "x",
		Timestamp:      time.Now().UTC(),
	}))

	recv, sent, err := wrapper.CountMessages(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), recv)
	assert.Equal(t, int64(0), sent)
}
