package dto

import (
	"context"

	"github.com/neirth/openlobster/internal/domain/services"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
)

// SubAgentAdapter bridges domain SubAgentService to dto.SubAgentPort.
type SubAgentAdapter struct {
	svc *services.SubAgentService
}

// NewSubAgentAdapter builds an adapter for the sub-agent service.
func NewSubAgentAdapter(svc *services.SubAgentService) *SubAgentAdapter {
	return &SubAgentAdapter{svc: svc}
}

// List implements SubAgentPort.List.
func (a *SubAgentAdapter) List(ctx context.Context) ([]SubAgentSnapshot, error) {
	list, err := a.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SubAgentSnapshot, len(list))
	for i, info := range list {
		out[i] = SubAgentSnapshot{
			ID:     info.ID,
			Name:   info.Name,
			Status: info.Status,
		}
	}
	return out, nil
}

// Spawn implements SubAgentPort.Spawn.
func (a *SubAgentAdapter) Spawn(ctx context.Context, name, model, task string) (string, error) {
	config := mcp.SubAgentConfig{Name: name, Model: model}
	agent, err := a.svc.Spawn(ctx, config, task)
	if err != nil {
		return "", err
	}
	return agent.ID(), nil
}

// Kill implements SubAgentPort.Kill.
func (a *SubAgentAdapter) Kill(ctx context.Context, id string) error {
	return a.svc.Kill(ctx, id)
}
