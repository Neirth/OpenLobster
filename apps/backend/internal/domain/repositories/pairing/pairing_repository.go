// Copyright (c) OpenLobster contributors. See LICENSE for details.

package pairing

import (
	"context"
	"time"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"gorm.io/gorm"
)

const PairingStatusPending = "pending"

type repository struct{ db *gorm.DB }

// NewPairingRepository returns a PairingRepository backed by the given *gorm.DB.
func NewPairingRepository(db *gorm.DB) ports.PairingRepositoryPort {
	return &repository{db: db}
}

func (r *repository) UpdateStatusIfPending(ctx context.Context, code, newStatus string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&domainmodels.PairingModel{}).
		Where("code = ? AND status = ?", code, PairingStatusPending).
		Update("status", newStatus)
	return result.RowsAffected > 0, result.Error
}

func (r *repository) Create(ctx context.Context, p *ports.Pairing) error {
	m := domainmodels.PairingModel{
		Code:             p.Code,
		ChannelID:        p.ChannelID,
		PlatformUserID:   p.PlatformUserID,
		PlatformUserName: p.PlatformUserName,
		ChannelType:      p.ChannelType,
		ExpiresAt:        time.Unix(p.ExpiresAt, 0),
		Status:           p.Status,
		CreatedAt:        time.Unix(p.CreatedAt, 0),
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *repository) GetByCode(ctx context.Context, code string) (*ports.Pairing, error) {
	var m domainmodels.PairingModel
	if err := r.db.WithContext(ctx).First(&m, "code = ?", code).Error; err != nil {
		return nil, err
	}
	return &ports.Pairing{
		Code:             m.Code,
		ChannelID:        m.ChannelID,
		PlatformUserID:   m.PlatformUserID,
		PlatformUserName: m.PlatformUserName,
		ChannelType:      m.ChannelType,
		ExpiresAt:        m.ExpiresAt.Unix(),
		Status:           m.Status,
		CreatedAt:        m.CreatedAt.Unix(),
	}, nil
}

func (r *repository) UpdateStatus(ctx context.Context, code, status string) error {
	return r.db.WithContext(ctx).Model(&domainmodels.PairingModel{}).Where("code = ?", code).Update("status", status).Error
}

func (r *repository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).Delete(&domainmodels.PairingModel{}, "expires_at < ?", time.Now().UTC()).Error
}

func (r *repository) ListActive(ctx context.Context) ([]ports.Pairing, error) {
	var models []domainmodels.PairingModel
	if err := r.db.WithContext(ctx).
		Where("expires_at > ?", time.Now().UTC()).
		Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	pairings := make([]ports.Pairing, len(models))
	for i, m := range models {
		pairings[i] = ports.Pairing{
			Code:             m.Code,
			ChannelID:        m.ChannelID,
			PlatformUserID:   m.PlatformUserID,
			PlatformUserName: m.PlatformUserName,
			ChannelType:      m.ChannelType,
			ExpiresAt:        m.ExpiresAt.Unix(),
			Status:           m.Status,
			CreatedAt:        m.CreatedAt.Unix(),
		}
	}
	return pairings, nil
}
