// Copyright (c) OpenLobster contributors. See LICENSE for details.

package mcp_server

import (
	"context"
	"time"

	domainmodels "github.com/neirth/openlobster/internal/domain/models"
	"gorm.io/gorm"
)

// MCPServerRecord represents a persisted MCP server registration.
type MCPServerRecord struct {
	Name string
	URL  string
}

// MCPServerRepositoryPort persists and retrieves registered MCP server configs.
type MCPServerRepositoryPort interface {
	Save(ctx context.Context, name, url string) error
	Delete(ctx context.Context, name string) error
	ListAll(ctx context.Context) ([]MCPServerRecord, error)
}

type repository struct{ db *gorm.DB }

// NewMCPServerRepository constructs a DB-backed MCP server repository.
func NewMCPServerRepository(db *gorm.DB) MCPServerRepositoryPort {
	return &repository{db: db}
}

func (r *repository) Save(ctx context.Context, name, url string) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO mcp_servers (name, url, added_at) VALUES (?, ?, ?) ON CONFLICT(name) DO UPDATE SET url=excluded.url`,
		name, url, time.Now().UTC(),
	).Error
}

func (r *repository) Delete(ctx context.Context, name string) error {
	return r.db.WithContext(ctx).Delete(&domainmodels.MCPServerModel{}, "name = ?", name).Error
}

func (r *repository) ListAll(ctx context.Context) ([]MCPServerRecord, error) {
	var models []domainmodels.MCPServerModel
	if err := r.db.WithContext(ctx).Order("added_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	result := make([]MCPServerRecord, len(models))
	for i, m := range models {
		result[i] = MCPServerRecord{Name: m.Name, URL: m.URL}
	}
	return result, nil
}
