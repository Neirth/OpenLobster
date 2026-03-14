package registry

import (
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
)

// AgentRegistry keeps the agent runtime state (hot-reloaded from main).
type AgentRegistry struct {
	mu        sync.RWMutex
	agent     *dto.AgentSnapshot
	channels  []dto.ChannelStatus
	mcpTools  []dto.ToolSnapshot
	mcps      []dto.MCPSnapshot
	startTime int64
	errorsCnt int64
}

// NewAgentRegistry creates an empty registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		startTime: time.Now().Unix(),
	}
}

// GetAgent returns the current agent snapshot.
func (r *AgentRegistry) GetAgent() *dto.AgentSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agent
}

// UpdateAgent replaces the agent snapshot.
func (r *AgentRegistry) UpdateAgent(a *dto.AgentSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agent = a
}

// GetChannels returns the current channels.
func (r *AgentRegistry) GetChannels() []dto.ChannelStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channels
}

// UpdateAgentChannels updates the agent with new channels.
func (r *AgentRegistry) UpdateAgentChannels(channels []dto.ChannelStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels = channels
	if r.agent != nil {
		updated := *r.agent
		updated.Channels = channels
		r.agent = &updated
	}
}

// GetMCPTools returns the current MCP tools.
func (r *AgentRegistry) GetMCPTools() []dto.ToolSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mcpTools
}

// UpdateMCPTools updates the list of MCP tools.
func (r *AgentRegistry) UpdateMCPTools(tools []dto.ToolSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mcpTools = tools
}

// GetMCPs returns the list of MCP servers.
func (r *AgentRegistry) GetMCPs() []dto.MCPSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mcps
}

// UpdateMCPs updates the list of MCP servers.
func (r *AgentRegistry) UpdateMCPs(mcps []dto.MCPSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mcps = mcps
}

// StartTime returns the Unix startup timestamp.
func (r *AgentRegistry) StartTime() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.startTime
}

// IncErrors increments the error counter.
func (r *AgentRegistry) IncErrors() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorsCnt++
}

// ErrorsCount returns the error counter.
func (r *AgentRegistry) ErrorsCount() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.errorsCnt
}
