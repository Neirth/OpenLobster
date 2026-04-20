// Copyright (c) OpenLobster contributors. See LICENSE for details.

package tool_permission

import (
	"context"
	"time"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"gorm.io/gorm"
)

// ToolPermissionRecord holds one row of the tool_permissions table.
type ToolPermissionRecord struct {
	UserID   string
	ToolName string
	Mode     string
}

// ToolPermissionRepositoryPort defines the persistence operations for tool permissions.
type ToolPermissionRepositoryPort interface {
	Set(ctx context.Context, userID, toolName, mode string) error
	Delete(ctx context.Context, userID, toolName string) error
	ListByUser(ctx context.Context, userID string) ([]ToolPermissionRecord, error)
	ListAll(ctx context.Context) ([]ToolPermissionRecord, error)
}

type repository struct{ db *gorm.DB }

// NewToolPermissionRepository returns a ToolPermissionRepositoryPort backed by the given *gorm.DB.
func NewToolPermissionRepository(db *gorm.DB) ToolPermissionRepositoryPort {
	return &repository{db: db}
}

func (r *repository) Set(ctx context.Context, userID, toolName, mode string) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO tool_permissions (user_id, tool_name, mode, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, tool_name) DO UPDATE SET mode=excluded.mode, updated_at=excluded.updated_at`,
		userID, toolName, mode, time.Now().UTC(),
	).Error
}

func (r *repository) Delete(ctx context.Context, userID, toolName string) error {
	return r.db.WithContext(ctx).Delete(&domainmodels.ToolPermissionModel{}, "user_id = ? AND tool_name = ?", userID, toolName).Error
}

func (r *repository) ListByUser(ctx context.Context, userID string) ([]ToolPermissionRecord, error) {
	var models []domainmodels.ToolPermissionModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("tool_name ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	return modelsToRecords(models), nil
}

func (r *repository) ListAll(ctx context.Context) ([]ToolPermissionRecord, error) {
	var models []domainmodels.ToolPermissionModel
	if err := r.db.WithContext(ctx).Order("user_id, tool_name ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	return modelsToRecords(models), nil
}

func modelsToRecords(models []domainmodels.ToolPermissionModel) []ToolPermissionRecord {
	records := make([]ToolPermissionRecord, len(models))
	for i, m := range models {
		records[i] = ToolPermissionRecord{UserID: m.UserID, ToolName: m.ToolName, Mode: m.Mode}
	}
	return records
}
