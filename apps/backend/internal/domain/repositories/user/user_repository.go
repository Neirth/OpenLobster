// Copyright (c) OpenLobster contributors. See LICENSE for details.

package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"gorm.io/gorm"
)

type repository struct{ db *gorm.DB }

// NewUserRepository returns a UserRepository backed by the given *gorm.DB.
func NewUserRepository(db *gorm.DB) ports.UserRepositoryPort {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, user *domainmodels.User) error {
	m := domainmodels.UserModel{
		ID:        user.ID.String(),
		PrimaryID: user.PrimaryID,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	return r.db.WithContext(ctx).Create(&m).Error
}

func (r *repository) GetByID(ctx context.Context, id string) (*domainmodels.User, error) {
	var m domainmodels.UserModel
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &domainmodels.User{ID: uuid.MustParse(m.ID), PrimaryID: m.PrimaryID, Name: m.Name, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}

func (r *repository) GetByPrimaryID(ctx context.Context, primaryID string) (*domainmodels.User, error) {
	var m domainmodels.UserModel
	if err := r.db.WithContext(ctx).First(&m, "primary_id = ?", primaryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &domainmodels.User{ID: uuid.MustParse(m.ID), PrimaryID: m.PrimaryID, Name: m.Name, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}, nil
}

func (r *repository) Update(ctx context.Context, user *domainmodels.User) error {
	return r.db.WithContext(ctx).Model(&domainmodels.UserModel{}).
		Where("id = ?", user.ID.String()).
		Updates(map[string]interface{}{"primary_id": user.PrimaryID, "name": user.Name, "updated_at": user.UpdatedAt}).Error
}

func (r *repository) ListAll(ctx context.Context) ([]domainmodels.User, error) {
	var userModels []domainmodels.UserModel
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&userModels).Error; err != nil {
		return nil, err
	}
	users := make([]domainmodels.User, len(userModels))
	for i, m := range userModels {
		users[i] = domainmodels.User{
			ID:        uuid.MustParse(m.ID),
			PrimaryID: m.PrimaryID,
			Name:      m.Name,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
		}
	}
	return users, nil
}
