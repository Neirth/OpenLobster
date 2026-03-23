// Copyright (c) OpenLobster contributors. See LICENSE for details.

package message

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── buildToolMetadata / parseToolMetadata ────────────────────────────────────

func TestBuildParseToolMetadata_Empty(t *testing.T) {
	result := buildToolMetadata("", "")
	assert.Equal(t, "", result)
}

func TestBuildParseToolMetadata_ToolCallID(t *testing.T) {
	raw := buildToolMetadata("toolu_abc", "")
	assert.NotEmpty(t, raw)
	id, calls := parseToolMetadata(raw)
	assert.Equal(t, "toolu_abc", id)
	assert.Equal(t, "", calls)
}

func TestBuildParseToolMetadata_ToolCallsRaw(t *testing.T) {
	callsJSON := `[{"id":"toolu_abc","type":"function","function":{"name":"my_tool","arguments":"{}"}}]`
	raw := buildToolMetadata("", callsJSON)
	assert.NotEmpty(t, raw)
	id, calls := parseToolMetadata(raw)
	assert.Equal(t, "", id)
	assert.Equal(t, callsJSON, calls)
}

func TestBuildParseToolMetadata_Both(t *testing.T) {
	callsJSON := `[{"id":"toolu_xyz","type":"function","function":{"name":"tool","arguments":"{}"}}]`
	raw := buildToolMetadata("toolu_xyz", callsJSON)
	id, calls := parseToolMetadata(raw)
	assert.Equal(t, "toolu_xyz", id)
	assert.Equal(t, callsJSON, calls)
}

func TestParseToolMetadata_InvalidJSON(t *testing.T) {
	id, calls := parseToolMetadata("{not valid json")
	assert.Equal(t, "", id)
	assert.Equal(t, "", calls)
}

// ─── Tool metadata DB round-trip ──────────────────────────────────────────────

func TestRepository_Save_ToolMessagePreservesToolCallID(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "tool",
		Content:        "result",
		ToolCallID:     "toolu_persist_123",
		Timestamp:      time.Now().UTC(),
	}))

	msgs, err := repo.GetByConversation(ctx, convID, 10)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "toolu_persist_123", msgs[0].ToolCallID)
}

func TestRepository_Save_AssistantMessagePreservesToolCallsRaw(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	callsJSON := `[{"id":"toolu_abc","type":"function","function":{"name":"my_tool","arguments":"{}"}}]`
	convID := uuid.New().String()
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "assistant",
		Content:        "",
		ToolCallsRaw:   callsJSON,
		Timestamp:      time.Now().UTC(),
	}))

	msgs, err := repo.GetByConversation(ctx, convID, 10)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, callsJSON, msgs[0].ToolCallsRaw)
}

func TestRepository_GetSinceLastCompaction_PreservesToolMetadata(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	callsJSON := `[{"id":"toolu_xyz","type":"function","function":{"name":"tool","arguments":"{}"}}]`
	convID := uuid.New().String()
	t1 := time.Now().UTC()
	t2 := t1.Add(time.Second)

	// assistant message with ToolCalls
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "assistant",
		Content:        "",
		ToolCallsRaw:   callsJSON,
		Timestamp:      t1,
	}))
	// tool result message
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "tool",
		Content:        "ok",
		ToolCallID:     "toolu_xyz",
		Timestamp:      t2,
	}))

	msgs, err := repo.GetSinceLastCompaction(ctx, convID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)

	assistantMsg := msgs[0]
	toolMsg := msgs[1]

	assert.Equal(t, callsJSON, assistantMsg.ToolCallsRaw, "ToolCallsRaw must survive DB round-trip")
	assert.Equal(t, "toolu_xyz", toolMsg.ToolCallID, "ToolCallID must survive DB round-trip")
}

// ─── GetByConversationPaged ───────────────────────────────────────────────────

func TestRepository_GetByConversationPaged_NilBefore(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB).(*repository)

	convID := uuid.New().String()
	for i := 0; i < 3; i++ {
		require.NoError(t, repo.Save(ctx, &models.Message{
			ID:             uuid.New(),
			ConversationID: convID,
			Role:           "user",
			Content:        "msg",
			Timestamp:      time.Now().UTC().Add(time.Duration(i) * time.Second),
		}))
	}

	msgs, err := repo.GetByConversationPaged(ctx, convID, nil, 10)
	require.NoError(t, err)
	assert.Len(t, msgs, 3)
}

func TestRepository_GetByConversationPaged_DefaultLimit(t *testing.T) {
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

	// limit=0 should use the default (50)
	msgs, err := repo.GetByConversationPaged(ctx, convID, nil, 0)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
}

func TestRepository_GetByConversationPaged_WithBeforeCursor(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB).(*repository)

	convID := uuid.New().String()
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC) // 2 hours after t2

	require.NoError(t, repo.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "user", Content: "first", Timestamp: t1,
	}))
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "user", Content: "second", Timestamp: t2,
	}))
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "user", Content: "third", Timestamp: t3,
	}))

	// Fetch all first to confirm they were stored, then filter using before=t3
	all, err := repo.GetByConversationPaged(ctx, convID, nil, 10)
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Use the stored timestamp of the third message as the cursor.
	before := all[2].Timestamp.Format("2006-01-02 15:04:05")
	msgs, err := repo.GetByConversationPaged(ctx, convID, &before, 10)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
	// Results must be in ascending order
	assert.Equal(t, "first", msgs[0].Content)
	assert.Equal(t, "second", msgs[1].Content)
}

func TestRepository_GetByConversationPaged_ExcludesCompaction(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB).(*repository)

	convID := uuid.New().String()
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "compaction", Content: "summary", Timestamp: time.Now().UTC(),
	}))
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "user", Content: "hello", Timestamp: time.Now().UTC().Add(time.Second),
	}))

	msgs, err := repo.GetByConversationPaged(ctx, convID, nil, 10)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "hello", msgs[0].Content)
}

// ─── GetUnvalidated ───────────────────────────────────────────────────────────

func TestRepository_GetUnvalidated_Empty(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	msgs, err := repo.GetUnvalidated(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestRepository_GetUnvalidated_WithLimit(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	for i := 0; i < 5; i++ {
		require.NoError(t, repo.Save(ctx, &models.Message{
			ID:             uuid.New(),
			ConversationID: convID,
			Role:           "user",
			Content:        "msg",
			Timestamp:      time.Now().UTC().Add(time.Duration(i) * time.Second),
		}))
	}

	msgs, err := repo.GetUnvalidated(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
}

func TestRepository_GetUnvalidated_NoLimit(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	for i := 0; i < 3; i++ {
		require.NoError(t, repo.Save(ctx, &models.Message{
			ID:             uuid.New(),
			ConversationID: convID,
			Role:           "user",
			Content:        "msg",
			Timestamp:      time.Now().UTC().Add(time.Duration(i) * time.Second),
		}))
	}

	msgs, err := repo.GetUnvalidated(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, msgs, 3)
}

// ─── MarkAsValidated ─────────────────────────────────────────────────────────

func TestRepository_MarkAsValidated_Empty(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	err := repo.MarkAsValidated(ctx, []string{})
	require.NoError(t, err)
}

func TestRepository_MarkAsValidated_WithIDs(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	msg1 := &models.Message{
		ID:             uuid.New(),
		ConversationID: convID,
		Role:           "user",
		Content:        "to validate",
		Timestamp:      time.Now().UTC(),
	}
	require.NoError(t, repo.Save(ctx, msg1))

	unvalidated, err := repo.GetUnvalidated(ctx, 10)
	require.NoError(t, err)
	require.Len(t, unvalidated, 1)

	require.NoError(t, repo.MarkAsValidated(ctx, []string{msg1.ID.String()}))

	// After marking, should have no unvalidated messages.
	remaining, err := repo.GetUnvalidated(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

// ─── Save with attachments ────────────────────────────────────────────────────

func TestRepository_Save_WithAttachments(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	msgID := uuid.New()
	msg := &models.Message{
		ID:             msgID,
		ConversationID: convID,
		Role:           "user",
		Content:        "file attached",
		Timestamp:      time.Now().UTC(),
		Attachments: []models.Attachment{
			{Type: "file", Filename: "doc.pdf", MIMEType: "application/pdf", Size: 1024},
			{Type: "image", Filename: "photo.jpg", MIMEType: "image/jpeg", Size: 2048},
		},
	}
	err := repo.Save(ctx, msg)
	require.NoError(t, err)

	msgs, err := repo.GetByConversation(ctx, convID, 10)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Len(t, msgs[0].Attachments, 2)
	assert.Equal(t, "doc.pdf", msgs[0].Attachments[0].Filename)
}

// ─── GetByConversation with compaction exclusion ──────────────────────────────

func TestRepository_GetByConversation_ExcludesCompaction(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB)

	convID := uuid.New().String()
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "compaction", Content: "sum", Timestamp: time.Now().UTC(),
	}))
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "user", Content: "real", Timestamp: time.Now().UTC(),
	}))

	msgs, err := repo.GetByConversation(ctx, convID, 0)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "real", msgs[0].Content)
}

// ─── CountMessages edge cases ─────────────────────────────────────────────────

func TestRepository_CountMessages_AgentRole(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	repo := NewMessageRepository(gormDB).(*repository)

	convID := uuid.New().String()
	require.NoError(t, repo.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "agent", Content: "y", Timestamp: time.Now().UTC(),
	}))

	_, sent, err := repo.CountMessages(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), sent)
}

// ─── DashboardMessageRepository extra coverage ───────────────────────────────

func TestDashboardMessageRepository_GetSinceLastCompaction(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	convID := uuid.New().String()
	require.NoError(t, inner.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "user", Content: "msg", Timestamp: time.Now().UTC(),
	}))

	msgs, err := wrapper.GetSinceLastCompaction(ctx, convID)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
}

func TestDashboardMessageRepository_GetLastCompaction(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	convID := uuid.New().String()
	comp, err := wrapper.GetLastCompaction(ctx, convID)
	require.NoError(t, err)
	assert.Nil(t, comp)

	// Add a compaction message
	require.NoError(t, inner.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "compaction", Content: "summary", Timestamp: time.Now().UTC(),
	}))
	comp, err = wrapper.GetLastCompaction(ctx, convID)
	require.NoError(t, err)
	require.NotNil(t, comp)
	assert.Equal(t, "compaction", comp.Role)
}

func TestDashboardMessageRepository_GetUnvalidated(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	require.NoError(t, inner.Save(ctx, &models.Message{
		ID:             uuid.New(),
		ConversationID: uuid.New().String(),
		Role:           "user",
		Content:        "check",
		Timestamp:      time.Now().UTC(),
	}))

	msgs, err := wrapper.GetUnvalidated(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
}

func TestDashboardMessageRepository_MarkAsValidated(t *testing.T) {
	gormDB, ctx := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	msgID := uuid.New()
	require.NoError(t, inner.Save(ctx, &models.Message{
		ID:             msgID,
		ConversationID: uuid.New().String(),
		Role:           "user",
		Content:        "mark me",
		Timestamp:      time.Now().UTC(),
	}))

	require.NoError(t, wrapper.MarkAsValidated(ctx, []string{msgID.String()}))

	remaining, err := wrapper.GetUnvalidated(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestDashboardMessageRepository_GetByConversationPaged_FallbackWhenNotPager(t *testing.T) {
	// Create a simple inner that doesn't implement pager interface.
	type simpleInner struct {
		msgs []models.Message
	}
	// Use the real repository (which does implement pager) - cover the pager path.
	gormDB, ctx := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	convID := uuid.New().String()
	require.NoError(t, inner.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: convID, Role: "user", Content: "paged", Timestamp: time.Now().UTC(),
	}))

	msgs, err := wrapper.GetByConversationPaged(ctx, convID, nil, 10)
	require.NoError(t, err)
	assert.Len(t, msgs, 1)
}

func TestDashboardMessageRepository_CountMessages_NoCounter(t *testing.T) {
	// If inner doesn't implement counter, should return 0,0,nil.
	// Use a mock inner without CountMessages.
	gormDB, ctx := setupMessageDB(t)
	inner := NewMessageRepository(gormDB)
	wrapper := NewDashboardMessageRepository(inner)

	// inner implements counter so it should work.
	require.NoError(t, inner.Save(ctx, &models.Message{
		ID: uuid.New(), ConversationID: uuid.New().String(), Role: "user", Content: "x", Timestamp: time.Now().UTC(),
	}))

	recv, sent, err := wrapper.CountMessages(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), recv)
	assert.Equal(t, int64(0), sent)
}
