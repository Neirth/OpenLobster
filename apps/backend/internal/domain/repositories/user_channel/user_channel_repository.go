// Copyright (c) OpenLobster contributors. See LICENSE for details.

package user_channel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
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
	id := uuid.New().String()
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO user_channels (id, user_id, channel_id, platform_user_id, username, paired_at, last_seen)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(channel_id, platform_user_id) DO UPDATE SET
		     username  = COALESCE(excluded.username, user_channels.username),
		     last_seen = excluded.last_seen`,
		id, userID, channelType, platformUserID, username, now, now,
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

func (r *repository) GetUserIDByName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	var id string
	err := r.db.WithContext(ctx).Raw(
		"SELECT id FROM users WHERE LOWER(TRIM(name)) = LOWER(?) LIMIT 1", name,
	).Scan(&id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return id, err
}

func normalizeStoredUsername(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	return strings.ToLower(s)
}

// usernameContainsSQL returns a dialect-specific predicate "needle appears in normalized username column".
func usernameContainsSQL(dialect, normExpr string) string {
	switch dialect {
	case "postgres":
		return `POSITION(? IN ` + normExpr + `) > 0`
	case "mysql":
		return `LOCATE(?, ` + normExpr + `) > 0`
	default:
		return `INSTR(` + normExpr + `, ?) > 0`
	}
}

func (r *repository) ResolveChannelByStoredUsername(ctx context.Context, username, platform string) (string, string, error) {
	u := normalizeStoredUsername(username)
	if u == "" {
		return "", "", nil
	}
	platform = strings.TrimSpace(strings.ToLower(platform))

	normExpr := `LOWER(TRIM(REPLACE(username, '@', '')))`
	containsPred := usernameContainsSQL(r.db.Name(), normExpr)

	type rowT struct {
		ChannelID      string    `gorm:"column:channel_id"`
		PlatformUserID string    `gorm:"column:platform_user_id"`
		NormUsername   string    `gorm:"column:norm_username"`
		LastSeen       time.Time `gorm:"column:last_seen"`
	}
	var rows []rowT

	var err error
	if platform != "" {
		err = r.db.WithContext(ctx).Raw(`
			SELECT channel_id, platform_user_id,
				`+normExpr+` AS norm_username,
				last_seen
			FROM user_channels
			WHERE channel_id = ?
			AND (`+normExpr+` = ? OR `+containsPred+`)
			ORDER BY last_seen DESC
			LIMIT 80`, platform, u, u).Scan(&rows).Error
	} else {
		err = r.db.WithContext(ctx).Raw(`
			SELECT channel_id, platform_user_id,
				`+normExpr+` AS norm_username,
				last_seen
			FROM user_channels
			WHERE `+normExpr+` = ? OR `+containsPred+`
			ORDER BY last_seen DESC
			LIMIT 80`, u, u).Scan(&rows).Error
	}
	if err != nil {
		return "", "", err
	}
	cands := make([]usernameCandidate, 0, len(rows))
	for _, row := range rows {
		cands = append(cands, usernameCandidate(row))
	}
	if ct, pid, ok := pickBestUsernameMatch(cands, u); ok {
		return ct, pid, nil
	}

	rows = nil
	if platform != "" {
		err = r.db.WithContext(ctx).Raw(`
			SELECT channel_id, platform_user_id,
				LOWER(TRIM(REPLACE(username, '@', ''))) AS norm_username,
				last_seen
			FROM user_channels
			WHERE channel_id = ?
			ORDER BY last_seen DESC
			LIMIT 200`, platform).Scan(&rows).Error
	} else {
		err = r.db.WithContext(ctx).Raw(`
			SELECT channel_id, platform_user_id,
				LOWER(TRIM(REPLACE(username, '@', ''))) AS norm_username,
				last_seen
			FROM user_channels
			ORDER BY last_seen DESC
			LIMIT 200`).Scan(&rows).Error
	}
	if err != nil {
		return "", "", err
	}
	cands = make([]usernameCandidate, 0, len(rows))
	for _, row := range rows {
		cands = append(cands, usernameCandidate(row))
	}
	if ct, pid, ok := pickBestUsernameMatch(cands, u); ok {
		return ct, pid, nil
	}
	return "", "", nil
}

func (r *repository) ListKnownUsers(ctx context.Context) ([]string, error) {
	var names []string
	err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(NULLIF(u.name,''), uc.platform_user_id) AS display_name
		FROM user_channels uc
		LEFT JOIN users u ON u.id = uc.user_id
		GROUP BY uc.user_id
		ORDER BY MAX(uc.last_seen) DESC`).Scan(&names).Error
	if err != nil {
		return nil, err
	}
	return names, nil
}

func (r *repository) UpdateLastSeen(ctx context.Context, channelType, platformUserID string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Model(&domainmodels.UserChannelModel{}).
		Where("channel_id = ? AND platform_user_id = ?", channelType, platformUserID).
		Update("last_seen", now).Error
}

