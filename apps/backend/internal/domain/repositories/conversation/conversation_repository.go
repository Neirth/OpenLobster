// Copyright (c) OpenLobster contributors. See LICENSE for details.

package conversation

import (
	"context"
	"fmt"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"gorm.io/gorm"
)

// ConversationRow is the data returned by ListConversations.
type ConversationRow struct {
	ID              string
	ChannelID       string
	ChannelType     string
	ChannelName     string
	GroupName       string
	IsGroup         bool
	ParticipantID   string
	ParticipantName string
	LastMessageAt   string
	UnreadCount     int
}

// ConversationRepository provides dashboard-level queries for conversations.
type ConversationRepository struct{ db *gorm.DB }

// NewConversationRepository returns a repository that satisfies the
// dashboard.ConversationPort interface.
func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

// ListConversations returns all conversations with derived display fields.
func (r *ConversationRepository) ListConversations() ([]ConversationRow, error) {
	var result []ConversationRow
	err := r.db.Raw(`
		SELECT id, channel_id, channel_type, channel_name,
		       group_name, is_group, participant_id, participant_name,
		       last_message_at, unread_count
		FROM v_conversation_summary`,
	).Scan(&result).Error
	if result == nil {
		result = []ConversationRow{}
	}
	return result, err
}

// DeleteUser removes all data related to the participant of a given conversation.
func (r *ConversationRepository) DeleteUser(ctx context.Context, conversationID string) error {
	var userID string
	if err := r.db.WithContext(ctx).Raw(
		"SELECT COALESCE(user_id, '') FROM conversations WHERE id = ?", conversationID,
	).Scan(&userID).Error; err != nil || userID == "" {
		return fmt.Errorf("deleteUser: conversation not found: %v", err)
	}

	r.db.WithContext(ctx).Exec(
		"DELETE FROM tool_permissions WHERE user_id IN (SELECT DISTINCT channel_id FROM conversations WHERE user_id = ? AND channel_id IS NOT NULL)",
		userID,
	)

	if err := r.db.WithContext(ctx).Exec(
		"DELETE FROM messages WHERE conversation_id IN (SELECT id FROM conversations WHERE user_id = ?)", userID,
	).Error; err != nil {
		return fmt.Errorf("deleteUser: delete messages: %w", err)
	}

	if err := r.db.WithContext(ctx).Delete(&domainmodels.ConversationModel{}, "user_id = ?", userID).Error; err != nil {
		return fmt.Errorf("deleteUser: delete conversations: %w", err)
	}

	r.db.WithContext(ctx).Delete(&domainmodels.UserChannelModel{}, "user_id = ?", userID)
	r.db.WithContext(ctx).Delete(&domainmodels.ToolPermissionModel{}, "user_id = ?", userID)
	r.db.WithContext(ctx).Delete(&domainmodels.UserModel{}, "id = ?", userID)

	return nil
}
