// Copyright (c) OpenLobster contributors. See LICENSE for details.

package message

import (
	"context"
	"errors"

	"github.com/google/uuid"
	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"gorm.io/gorm"
)

type repository struct{ db *gorm.DB }

// NewMessageRepository returns a MessageRepository backed by the given *gorm.DB.
func NewMessageRepository(db *gorm.DB) ports.MessageRepositoryPort {
	return &repository{db: db}
}

func (r *repository) Save(ctx context.Context, message *domainmodels.Message) error {
	var audioData []byte
	if message.Audio != nil {
		audioData = message.Audio.Data
	}
	// Domain Message no longer contains UserID/GroupID — persistence stores them separately.
	// Map attachments metadata to persistence models (do not store raw Data)
	attModels := make([]domainmodels.MessageAttachmentModel, 0, len(message.Attachments))
	for _, a := range message.Attachments {
		attModels = append(attModels, domainmodels.MessageAttachmentModel{
			MessageID: message.ID.String(),
			Type:      a.Type,
			Filename:  a.Filename,
			MIMEType:  a.MIMEType,
			Size:      a.Size,
		})
	}

	m := domainmodels.MessageModel{
		ID:             message.ID.String(),
		ConversationID: message.ConversationID,
		Role:           message.Role,
		Content:        message.Content,
		AudioData:      audioData,
		CreatedAt:      message.Timestamp,
		Attachments:    attModels,
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *repository) GetByConversation(ctx context.Context, conversationID string, limit int) ([]domainmodels.Message, error) {
	q := r.db.WithContext(ctx).
		Where("conversation_id = ? AND role != 'compaction'", conversationID).
		Preload("Attachments").
		Order("created_at ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var models []domainmodels.MessageModel
	if err := q.Find(&models).Error; err != nil {
		return nil, err
	}
	return msgModelsToEntities(models), nil
}

// GetByConversationPaged returns up to limit messages before the given cursor (exclusive),
// ordered by created_at DESC (newest-first for efficient keyset pagination), excluding compaction messages.
// A nil before fetches from the newest end. Results are returned in ascending order.
func (r *repository) GetByConversationPaged(ctx context.Context, conversationID string, before *string, limit int) ([]domainmodels.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	q := r.db.WithContext(ctx).
		Where("conversation_id = ? AND role != 'compaction'", conversationID).
		Preload("Attachments")
	if before != nil && *before != "" {
		q = q.Where("created_at < ?", *before)
	}
	q = q.Order("created_at DESC").Limit(limit)
	var ms []domainmodels.MessageModel
	if err := q.Find(&ms).Error; err != nil {
		return nil, err
	}
	// Reverse to return ascending order (oldest first)
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
	return msgModelsToEntities(ms), nil
}

func (r *repository) GetSinceLastCompaction(ctx context.Context, conversationID string) ([]domainmodels.Message, error) {
	compaction, err := r.GetLastCompaction(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if compaction == nil {
		return r.GetByConversation(ctx, conversationID, 0)
	}
	var models []domainmodels.MessageModel
	err = r.db.WithContext(ctx).
		Where("conversation_id = ? AND created_at > ?", conversationID, compaction.Timestamp).
		Preload("Attachments").
		Order("created_at").Find(&models).Error
	return msgModelsToEntities(models), err
}

func (r *repository) CountMessages(ctx context.Context) (int64, int64, error) {
	type counts struct {
		Recv int64
		Sent int64
	}
	var c counts
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN role = 'user' THEN 1 ELSE 0 END), 0)                 AS recv,
			COALESCE(SUM(CASE WHEN role IN ('agent', 'assistant') THEN 1 ELSE 0 END), 0) AS sent
		FROM messages WHERE role != 'compaction'`).Scan(&c).Error
	return c.Recv, c.Sent, err
}

func (r *repository) GetLastCompaction(ctx context.Context, conversationID string) (*domainmodels.Message, error) {
	var m domainmodels.MessageModel
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND role = 'compaction'", conversationID).
		Preload("Attachments").
		Order("created_at DESC").Limit(1).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return msgModelToEntity(m), nil
}

// DashboardMessageRepository wraps MessageRepositoryPort for context-free use.
type DashboardMessageRepository struct {
	inner ports.MessageRepositoryPort
}

// NewDashboardMessageRepository wraps a MessageRepositoryPort for use in the GraphQL dashboard.
func NewDashboardMessageRepository(repo ports.MessageRepositoryPort) *DashboardMessageRepository {
	return &DashboardMessageRepository{inner: repo}
}

func (r *DashboardMessageRepository) Save(message *domainmodels.Message) error {
	return r.inner.Save(context.Background(), message)
}

func (r *DashboardMessageRepository) GetByConversation(conversationID string, limit int) ([]domainmodels.Message, error) {
	return r.inner.GetByConversation(context.Background(), conversationID, limit)
}

func (r *DashboardMessageRepository) GetByConversationPaged(ctx context.Context, conversationID string, before *string, limit int) ([]domainmodels.Message, error) {
	type pager interface {
		GetByConversationPaged(ctx context.Context, conversationID string, before *string, limit int) ([]domainmodels.Message, error)
	}
	if p, ok := r.inner.(pager); ok {
		return p.GetByConversationPaged(ctx, conversationID, before, limit)
	}
	return r.inner.GetByConversation(ctx, conversationID, limit)
}

func (r *DashboardMessageRepository) GetSinceLastCompaction(ctx context.Context, conversationID string) ([]domainmodels.Message, error) {
	return r.inner.GetSinceLastCompaction(ctx, conversationID)
}

func (r *DashboardMessageRepository) CountMessages(ctx context.Context) (int64, int64, error) {
	type counter interface {
		CountMessages(ctx context.Context) (int64, int64, error)
	}
	if c, ok := r.inner.(counter); ok {
		return c.CountMessages(ctx)
	}
	return 0, 0, nil
}

func msgModelToEntity(m domainmodels.MessageModel) *domainmodels.Message {
	msgID, _ := uuid.Parse(m.ID)
	msg := &domainmodels.Message{
		ID:             msgID,
		ConversationID: m.ConversationID,
		Role:           m.Role,
		Content:        m.Content,
		Timestamp:      m.CreatedAt,
	}
	if len(m.Attachments) > 0 {
		atts := make([]domainmodels.Attachment, 0, len(m.Attachments))
		for _, a := range m.Attachments {
			atts = append(atts, domainmodels.Attachment{
				Type:     a.Type,
				Filename: a.Filename,
				Size:     a.Size,
				MIMEType: a.MIMEType,
			})
		}
		msg.Attachments = atts
	}
	return msg
}

func msgModelsToEntities(models []domainmodels.MessageModel) []domainmodels.Message {
	messages := make([]domainmodels.Message, len(models))
	for i, m := range models {
		messages[i] = *msgModelToEntity(m)
	}
	return messages
}
