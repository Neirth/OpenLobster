package registry

import (
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
)

// AgentRegistry mantiene el estado runtime del agente (hot-reload por main).
type AgentRegistry struct {
	mu        sync.RWMutex
	agent     *dto.AgentSnapshot
	channels  []dto.ChannelStatus
	mcpTools  []dto.ToolSnapshot
	mcps      []dto.MCPSnapshot
	startTime int64
	errorsCnt int64
}

// NewAgentRegistry crea un registro vacío.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		startTime: time.Now().Unix(),
	}
}

// GetAgent devuelve el snapshot actual del agente.
func (r *AgentRegistry) GetAgent() *dto.AgentSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agent
}

// UpdateAgent reemplaza el snapshot del agente.
func (r *AgentRegistry) UpdateAgent(a *dto.AgentSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agent = a
}

// GetChannels devuelve los canales actuales.
func (r *AgentRegistry) GetChannels() []dto.ChannelStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.channels
}

// UpdateAgentChannels actualiza el agente con nuevos canales.
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

// GetMCPTools devuelve las herramientas MCP actuales.
func (r *AgentRegistry) GetMCPTools() []dto.ToolSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mcpTools
}

// UpdateMCPTools actualiza la lista de herramientas MCP.
func (r *AgentRegistry) UpdateMCPTools(tools []dto.ToolSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mcpTools = tools
}

// GetMCPs devuelve la lista de servidores MCP.
func (r *AgentRegistry) GetMCPs() []dto.MCPSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.mcps
}

// UpdateMCPs actualiza la lista de servidores MCP.
func (r *AgentRegistry) UpdateMCPs(mcps []dto.MCPSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mcps = mcps
}

// StartTime devuelve el Unix timestamp de arranque.
func (r *AgentRegistry) StartTime() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.startTime
}

// IncErrors incrementa el contador de errores.
func (r *AgentRegistry) IncErrors() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorsCnt++
}

// ErrorsCount devuelve el contador de errores.
func (r *AgentRegistry) ErrorsCount() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.errorsCnt
}
