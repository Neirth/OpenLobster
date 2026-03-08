// Copyright (c) OpenLobster contributors. See LICENSE for details.

package user_channel

import (
	"context"
	"errors"
	"time"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"gorm.io/gorm"
)

type repository struct{ db *gorm.DB }

// NewUserChannelRepository returns a UserChannelRepository backed by the given *gorm.DB.
func NewUserChannelRepository(db *gorm.DB) ports.UserChannelRepositoryPort {
	return &repository{db: db}
}

func (r *repository) ExistsByPlatformUserID(ctx context.Context, platformUserID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domainmodels.UserChannelModel{}).Where("platform_user_id = ?", platformUserID).Count(&count).Error
	return count > 0, err
}

func (r *repository) GetUserIDByPlatformUserID(ctx context.Context, platformUserID string) (string, error) {
	var m domainmodels.UserChannelModel
	err := r.db.WithContext(ctx).Select("user_id").
		Where("platform_user_id = ?", platformUserID).
		Order("paired_at DESC").Limit(1).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return m.UserID, err
}

func (r *repository) GetDisplayNameByPlatformUserID(ctx context.Context, platformUserID string) (string, error) {
	var displayName string
	err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(NULLIF(u.name,''), NULLIF(uc.username,''), uc.platform_user_id) AS display_name
		FROM user_channels uc
		LEFT JOIN users u ON u.id = uc.user_id
		WHERE uc.platform_user_id = ?
		ORDER BY uc.last_seen DESC LIMIT 1`, platformUserID).Scan(&displayName).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return displayName, err
}

func (r *repository) GetDisplayNameByUserID(ctx context.Context, userID string) (string, error) {
	var name string
	err := r.db.WithContext(ctx).Raw("SELECT COALESCE(NULLIF(name,''), primary_id) FROM users WHERE id = ? LIMIT 1", userID).Scan(&name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return name, err
}

func (r *repository) Create(ctx context.Context, userID, channelType, platformUserID, username string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO user_channels (id, user_id, channel_id, platform_user_id, username, paired_at, last_seen)
		 VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(channel_id, platform_user_id) DO UPDATE SET
		     username  = COALESCE(excluded.username, username),
		     last_seen = excluded.last_seen`,
		userID, channelType, platformUserID, username, now, now,
	).Error
}

func (r *repository) GetLastChannelForUser(ctx context.Context, userID string) (channelType, platformChannelID string, err error) {
	var m domainmodels.UserChannelModel
	err = r.db.WithContext(ctx).Select("channel_id", "platform_user_id").
		Where("user_id = ?", userID).
		Order("last_seen DESC").Limit(1).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return m.ChannelID, m.PlatformUserID, nil
}

func (r *repository) UpdateLastSeen(ctx context.Context, channelType, platformUserID string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domainmodels.UserChannelModel{}).
		Where("channel_id = ? AND platform_user_id = ?", channelType, platformUserID).
		Update("last_seen", now).Error
}
