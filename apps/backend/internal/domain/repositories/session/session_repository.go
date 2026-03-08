// Copyright (c) OpenLobster contributors. See LICENSE for details.

package session

import (
	"context"

	"github.com/google/uuid"
	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"gorm.io/gorm"
)

// SessionRepository extends ports.SessionRepositoryPort with dashboard convenience methods.
type SessionRepository interface {
	ports.SessionRepositoryPort
	GetActiveByUserNoCtx(userID string) ([]domainmodels.Session, error)
}

type repository struct{ db *gorm.DB }

// NewSessionRepository returns a SessionRepository backed by the given *gorm.DB.
func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, session *domainmodels.Session) error {
	var groupID *string
	if session.GroupID != nil {
		s := session.GroupID.String()
		groupID = &s
	}
	m := domainmodels.ConversationModel{
		ID:        session.ID.String(),
		ChannelID: session.ChannelID,
		GroupID:   groupID,
		UserID:    session.UserID,
		ModelID:   session.ModelID,
		IsActive:  session.IsActive,
		StartedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *repository) GetByID(ctx context.Context, id string) (*domainmodels.Session, error) {
	var m domainmodels.ConversationModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return convModelToSession(m), nil
}

func (r *repository) Update(ctx context.Context, session *domainmodels.Session) error {
	var groupID *string
	if session.GroupID != nil {
		s := session.GroupID.String()
		groupID = &s
	}
	return r.db.WithContext(ctx).Model(&domainmodels.ConversationModel{}).Where("id = ?", session.ID.String()).
		Updates(map[string]interface{}{
			"channel_id": session.ChannelID,
			"group_id":   groupID,
			"user_id":    session.UserID,
			"model_id":   session.ModelID,
			"is_active":  session.IsActive,
			"updated_at": session.UpdatedAt,
		}).Error
}

func (r *repository) GetActiveByUser(ctx context.Context, userID string) ([]domainmodels.Session, error) {
	var models []domainmodels.ConversationModel
	err := r.db.WithContext(ctx).Raw(
		`SELECT id, channel_id, group_id, user_id, model_id, is_active, started_at, updated_at
		 FROM v_active_conversations WHERE user_id = ? AND group_id IS NULL`, userID,
	).Scan(&models).Error
	return convModelsToSessions(models), err
}

func (r *repository) GetActiveByChannel(ctx context.Context, channelID string) ([]domainmodels.Session, error) {
	var models []domainmodels.ConversationModel
	err := r.db.WithContext(ctx).Raw(
		`SELECT id, channel_id, group_id, user_id, model_id, is_active, started_at, updated_at
		 FROM v_active_conversations WHERE channel_id = ? AND group_id IS NULL ORDER BY started_at DESC`, channelID,
	).Scan(&models).Error
	return convModelsToSessions(models), err
}

func (r *repository) GetActiveByGroup(ctx context.Context, groupID string) ([]domainmodels.Session, error) {
	var models []domainmodels.ConversationModel
	err := r.db.WithContext(ctx).Raw(
		`SELECT id, channel_id, group_id, user_id, model_id, is_active, started_at, updated_at
		 FROM v_active_conversations WHERE group_id = ? ORDER BY started_at DESC`, groupID,
	).Scan(&models).Error
	return convModelsToSessions(models), err
}

func (r *repository) GetActiveByUserNoCtx(userID string) ([]domainmodels.Session, error) {
	return r.GetActiveByUser(context.Background(), userID)
}

func convModelToSession(m domainmodels.ConversationModel) *domainmodels.Session {
	s := &domainmodels.Session{
		ID:        uuid.MustParse(m.ID),
		ChannelID: m.ChannelID,
		UserID:    m.UserID,
		ModelID:   m.ModelID,
		IsActive:  m.IsActive,
		CreatedAt: m.StartedAt,
		UpdatedAt: m.UpdatedAt,
	}
	if m.GroupID != nil && *m.GroupID != "" {
		if parsed, err := uuid.Parse(*m.GroupID); err == nil {
			s.GroupID = &parsed
		}
	}
	return s
}

func convModelsToSessions(models []domainmodels.ConversationModel) []domainmodels.Session {
	sessions := make([]domainmodels.Session, len(models))
	for i, m := range models {
		sessions[i] = *convModelToSession(m)
	}
	return sessions
}
