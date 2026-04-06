// Copyright (c) OpenLobster contributors. See LICENSE for details.

package subagent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
)

// CapabilitiesChecker decides whether a given capability is enabled.
type CapabilitiesChecker func(cap string) bool

// Service manages spawned sub-agents.
type Service struct {
	mu             sync.RWMutex
	agents         map[string]*Instance
	aiProvider     ports.AIProviderPort
	maxConcurrent  int
	defaultTimeout time.Duration

	toolRegistry      *mcp.ToolRegistry
	permManager       *permissions.Manager
	capabilitiesCheck CapabilitiesChecker
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

// SetToolRegistry wires the MCP tool registry so subagents can perform tool_use
// loops (Claude Code style).
func (s *Service) SetToolRegistry(tr *mcp.ToolRegistry) {
	s.toolRegistry = tr
	// We don't access the registry's internal permission manager directly;
	// the caller should wire it via SetPermissionManager if needed.
}

// SetPermissionManager wires the permission manager used to filter tools exposed
// to subagents.
func (s *Service) SetPermissionManager(pm *permissions.Manager) {
	s.permManager = pm
}

// SetCapabilitiesChecker wires the global capabilities checker used to hide tools
// when a capability is disabled.
func (s *Service) SetCapabilitiesChecker(fn CapabilitiesChecker) {
	s.capabilitiesCheck = fn
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

	// If tool registry is not wired yet, fall back to a single Chat call.
	if s.toolRegistry == nil {
		messages := []ports.ChatMessage{
			{Role: "system", Content: agent.Config.SystemPrompt},
			{Role: "user", Content: agent.Task},
		}
		req := ports.ChatRequest{Model: agent.Config.Model, Messages: messages}
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
		return
	}

	channelID, _ := ctx.Value(mcp.ContextKeyChannelID).(string)
	tools := s.buildToolsForSubAgent(channelID)
	if tools == nil {
		tools = []ports.Tool{}
	}

	messages := []ports.ChatMessage{
		{Role: "system", Content: agent.Config.SystemPrompt},
		{Role: "user", Content: agent.Task},
	}

	result, err := s.runAgenticLoop(ctx, agent.Config.Model, messages, tools)
	if err != nil {
		s.mu.Lock()
		agent.Status = StatusFailed
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	agent.LastActivity = time.Now()
	agent.Result = result
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

const maxToolDescriptionLen = 120
const maxToolRounds = 5

func summarizeForAgent(desc string) string {
	s := strings.TrimSpace(desc)
	if s == "" {
		return s
	}
	// Prefer first sentence (up to . ! or ?)
	for _, end := range []rune{'.', '!', '?'} {
		if i := strings.IndexRune(s, end); i >= 0 && i < 200 {
			return strings.TrimSpace(s[:i+1])
		}
	}
	if len(s) <= maxToolDescriptionLen {
		return s
	}
	trunc := s[:maxToolDescriptionLen]
	if last := strings.LastIndex(trunc, " "); last > maxToolDescriptionLen/2 {
		trunc = trunc[:last]
	}
	return strings.TrimSpace(trunc) + "..."
}

func (s *Service) buildToolsForSubAgent(userID string) []ports.Tool {
	if s.toolRegistry == nil {
		return nil
	}
	defs := s.toolRegistry.AllTools()
	tools := make([]ports.Tool, 0, len(defs))
	for _, def := range defs {
		// Subagents should not be able to spawn more execution contexts, nor
		// observe the results of spawns.
		// This prevents runaway delegation chains and keeps subagents bounded.
		if def.Name == "terminal_spawn" ||
			def.Name == "terminal_list_processes" ||
			def.Name == "terminal_get_output" ||
			def.Name == "subagent_spawn" {
			continue
		}
		if s.capabilitiesCheck != nil {
			if cap := mcp.CapabilityForTool(def.Name); cap != "" && !s.capabilitiesCheck(cap) {
				continue
			}
		}
		if s.permManager != nil {
			mode := s.permManager.GetPermission(userID, def.Name)
			if mode == permissions.PermissionDeny {
				continue
			}
		}
		var params map[string]interface{}
		if len(def.InputSchema) > 0 {
			_ = json.Unmarshal(def.InputSchema, &params)
		}
		tools = append(tools, ports.Tool{
			Type: "function",
			Function: &ports.FunctionTool{
				Name:        def.Name,
				Description: summarizeForAgent(def.Description),
				Parameters:  params,
			},
		})
	}
	return tools
}

func (s *Service) dispatchToolCall(ctx context.Context, tc ports.ToolCall) ports.ChatMessage {
	toolName := strings.ReplaceAll(tc.Function.Name, ":", "__")
	base := ports.ChatMessage{
		Role:       "tool",
		ToolCallID: tc.ID,
		ToolName:   toolName,
	}
	raw, err := s.toolRegistry.Dispatch(ctx, tc.Function.Name, jsonArgumentsToMap(tc.Function.Arguments))
	if err != nil {
		base.Content = fmt.Sprintf(`{"error":"%s"}`, err.Error())
		return base
	}

	// Detect multimodal blocks encoded by ReadFileTool (and potentially other tools).
	var multimodal struct {
		Blocks []struct {
			Type     string `json:"type"`
			MIMEType string `json:"mime_type"`
			Data     string `json:"data"` // base64
			Text     string `json:"text"`
		} `json:"_openlobster_blocks"`
	}
	if err := json.Unmarshal(raw, &multimodal); err == nil && len(multimodal.Blocks) > 0 {
		blocks := make([]ports.ContentBlock, 0, len(multimodal.Blocks))
		for _, b := range multimodal.Blocks {
			decoded, decErr := base64.StdEncoding.DecodeString(b.Data)
			if decErr != nil {
				log.Printf("subagent: base64 decode error for block type %q: %v", b.Type, decErr)
				continue
			}
			switch b.Type {
			case "image":
				blocks = append(blocks, ports.ContentBlock{
					Type:     ports.ContentBlockImage,
					MIMEType: b.MIMEType,
					Data:     decoded,
					Text:     b.Text,
				})
			case "audio":
				blocks = append(blocks, ports.ContentBlock{
					Type:     ports.ContentBlockAudio,
					MIMEType: b.MIMEType,
					Data:     decoded,
					Text:     b.Text,
				})
			}
		}
		if len(blocks) > 0 {
			base.Blocks = blocks
			return base
		}
	}

	base.Content = string(raw)
	return base
}

func jsonArgumentsToMap(args string) map[string]interface{} {
	var params map[string]interface{}
	if args == "" {
		return params
	}
	_ = json.Unmarshal([]byte(args), &params)
	return params
}

func (s *Service) runAgenticLoop(ctx context.Context, model string, messages []ports.ChatMessage, tools []ports.Tool) (string, error) {
	req := ports.ChatRequest{Model: model, Messages: messages, Tools: tools}
	toolsExecuted := false

	for round := 0; round < maxToolRounds; round++ {
		resp, err := s.aiProvider.Chat(ctx, req)
		if err != nil {
			return "", err
		}
		hasToolCalls := len(resp.ToolCalls) > 0
		if !hasToolCalls {
			if strings.TrimSpace(resp.Content) != "" {
				return resp.Content, nil
			}
			break
		}

		toolsExecuted = true
		req.Messages = append(req.Messages, ports.ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})
		for _, tc := range resp.ToolCalls {
			msg := s.dispatchToolCall(ctx, tc)
			req.Messages = append(req.Messages, msg)
		}
	}

	if !toolsExecuted {
		return "NO_REPLY", nil
	}

	synthReq := ports.ChatRequest{Model: model, Messages: req.Messages, Tools: nil}
	synthResp, err := s.aiProvider.Chat(ctx, synthReq)
	if err != nil {
		return "", err
	}
	return synthResp.Content, nil
}
