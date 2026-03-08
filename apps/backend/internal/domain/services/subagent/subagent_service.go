// Copyright (c) OpenLobster contributors. See LICENSE for details.

package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
)

// Service manages spawned sub-agents.
type Service struct {
	mu             sync.RWMutex
	agents         map[string]*Instance
	aiProvider     ports.AIProviderPort
	maxConcurrent  int
	defaultTimeout time.Duration
}

// Instance represents a running sub-agent.
type Instance struct {
	ID           string
	Name         string
	Config       mcp.SubAgentConfig
	Status       string
	Result       string
	CreatedAt    time.Time
	LastActivity time.Time
	Cancel       context.CancelFunc
	Task         string
}

const (
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
	StatusKilled  = "killed"
)

// NewService creates a SubAgentService.
func NewService(aiProvider ports.AIProviderPort, maxConcurrent int, defaultTimeout time.Duration) *Service {
	return &Service{
		aiProvider:     aiProvider,
		agents:         make(map[string]*Instance),
		maxConcurrent:  maxConcurrent,
		defaultTimeout: defaultTimeout,
	}
}

// Spawn creates a new sub-agent.
func (s *Service) Spawn(ctx context.Context, config mcp.SubAgentConfig, task string) (mcp.SubAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.agents) >= s.maxConcurrent {
		return nil, fmt.Errorf("max concurrent sub-agents reached (%d)", s.maxConcurrent)
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = s.defaultTimeout
	}

	agentCtx, cancel := context.WithTimeout(ctx, timeout)

	agent := &Instance{
		ID:           uuid.New().String(),
		Name:         config.Name,
		Config:       config,
		Status:       StatusRunning,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Cancel:       cancel,
		Task:         task,
	}

	s.agents[agent.ID] = agent

	go s.runAgent(agentCtx, agent)

	return &adapter{agent: agent, mu: &s.mu}, nil
}

func (s *Service) runAgent(ctx context.Context, agent *Instance) {
	defer func() {
		s.mu.Lock()
		if agent.Status == StatusRunning {
			agent.Status = StatusDone
		}
		s.mu.Unlock()
		agent.Cancel()
	}()

	if s.aiProvider == nil {
		s.mu.Lock()
		agent.Status = StatusFailed
		s.mu.Unlock()
		return
	}

	messages := []ports.ChatMessage{
		{Role: "system", Content: agent.Config.SystemPrompt},
		{Role: "user", Content: agent.Task},
	}

	req := ports.ChatRequest{
		Model:    agent.Config.Model,
		Messages: messages,
	}

	resp, err := s.aiProvider.Chat(ctx, req)
	if err != nil {
		s.mu.Lock()
		agent.Status = StatusFailed
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	agent.LastActivity = time.Now()
	agent.Result = resp.Content
	s.mu.Unlock()
}

// List returns all sub-agents.
func (s *Service) List(ctx context.Context) ([]mcp.SubAgentInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]mcp.SubAgentInfo, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, mcp.SubAgentInfo{
			ID:     agent.ID,
			Name:   agent.Name,
			Status: agent.Status,
		})
	}

	return agents, nil
}

// Kill terminates a sub-agent.
func (s *Service) Kill(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[id]
	if !ok {
		return fmt.Errorf("sub-agent not found: %s", id)
	}

	agent.Status = StatusKilled
	agent.Cancel()
	delete(s.agents, id)

	return nil
}

// Cleanup terminates all sub-agents.
func (s *Service) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, agent := range s.agents {
		agent.Cancel()
	}
	s.agents = make(map[string]*Instance)
}

type adapter struct {
	agent *Instance
	mu    *sync.RWMutex
}

func (a *adapter) ID() string {
	return a.agent.ID
}

func (a *adapter) Name() string {
	return a.agent.Name
}

func (a *adapter) Status() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.agent.Status
}

func (a *adapter) Result() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.agent.Result
}

var _ mcp.SubAgentService = (*Service)(nil)
