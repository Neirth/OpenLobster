// Copyright (c) OpenLobster contributors. See LICENSE for details.

package group

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"gorm.io/gorm"
)

type repository struct{ db *gorm.DB }

// NewGroupRepository returns a GroupRepository backed by the given *gorm.DB.
func NewGroupRepository(db *gorm.DB) ports.GroupRepositoryPort {
	return &repository{db: db}
}

func (r *repository) GetOrCreate(ctx context.Context, channelType, platformGroupID, name string) (string, error) {
	var m domainmodels.GroupModel
	err := r.db.WithContext(ctx).
		Where("channel_id = ? AND platform_group_id = ?", channelType, platformGroupID).
		First(&m).Error
	if err == nil {
		if name != "" && name != m.Name {
			r.db.WithContext(ctx).Model(&m).Update("name", name)
		}
		return m.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	m = domainmodels.GroupModel{
		ID:              uuid.New().String(),
		ChannelID:       channelType,
		PlatformGroupID: platformGroupID,
		Name:            name,
		CreatedAt:       time.Now().UTC(),
	}
	return m.ID, r.db.WithContext(ctx).Create(&m).Error
}

func (r *repository) AddMember(ctx context.Context, groupID, userID string) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO group_users (group_id, user_id, joined_at) VALUES (?, ?, ?) ON CONFLICT(group_id, user_id) DO NOTHING`,
		groupID, userID, time.Now().UTC(),
	).Error
}

func (r *repository) GetByID(ctx context.Context, id string) (*ports.Group, error) {
	var m domainmodels.GroupModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &ports.Group{
		ID:              m.ID,
		ChannelID:       m.ChannelID,
		PlatformGroupID: m.PlatformGroupID,
		Name:            m.Name,
		CreatedAt:       m.CreatedAt.Unix(),
	}, nil
}

func (r *repository) GetMembers(ctx context.Context, groupID string) ([]string, error) {
	var members []domainmodels.GroupUserModel
	if err := r.db.WithContext(ctx).Select("user_id").Where("group_id = ?", groupID).Find(&members).Error; err != nil {
		return nil, err
	}
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	return ids, nil
}
