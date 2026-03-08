// Copyright (c) OpenLobster contributors. See LICENSE for details.

package channel

import (
	"context"
	"time"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"gorm.io/gorm"
)

// Repository implements ports.ChannelRepositoryPort.
type Repository struct{ db *gorm.DB }

// NewChannelRepository returns a ChannelRepository backed by the given *gorm.DB.
func NewChannelRepository(db *gorm.DB) ports.ChannelRepositoryPort {
	return &Repository{db: db}
}

// EnsurePlatform creates a channels row for the given platform slug if it does not exist.
func (r *Repository) EnsurePlatform(ctx context.Context, platformSlug, name string) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO channels (id, type, name, created_at) VALUES (?, ?, ?, ?) ON CONFLICT(id) DO NOTHING`,
		platformSlug, platformSlug, name, time.Now().UTC(),
	).Error
}

// GetByID returns the channel by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*ports.Channel, error) {
	var m domainmodels.ChannelModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &ports.Channel{ID: m.ID, Type: m.Type, Name: m.Name, CreatedAt: m.CreatedAt.Unix()}, nil
}
