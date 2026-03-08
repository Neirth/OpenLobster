package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/neirth/openlobster/internal/domain/services/permissions"
)

type InternalTool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error)
}

type ToolRegistry struct {
	internal    map[string]InternalTool
	mcp         map[string]MCPTool
	sanitizer   *ToolResultSanitizer
	isMaster    bool
	permManager *permissions.Manager
}

type MCPTool struct {
	Client MCPClient
	Tool   ToolDefinition
}

func NewToolRegistry(isMaster bool, permManager *permissions.Manager) *ToolRegistry {
	if permManager == nil {
		permManager = permissions.Default()
	}
	return &ToolRegistry{
		internal:    make(map[string]InternalTool),
		mcp:         make(map[string]MCPTool),
		sanitizer:   NewToolResultSanitizer(),
		isMaster:    isMaster,
		permManager: permManager,
	}
}

func (r *ToolRegistry) RegisterInternal(name string, tool InternalTool) {
	r.internal[name] = tool
}

func (r *ToolRegistry) RegisterMCP(serverName string, client MCPClient, tools []ToolDefinition) error {
	for _, t := range tools {
		qualifiedName := fmt.Sprintf("%s:%s", serverName, t.Name)
		// Store the definition with the qualified name so that AllTools() exposes
		// the same name the dispatcher uses for lookup (serverName:toolName).
		t.Name = qualifiedName
		r.mcp[qualifiedName] = MCPTool{Client: client, Tool: t}
	}
	return nil
}

func (r *ToolRegistry) UnregisterMCP(serverName string) {
	prefix := serverName + ":"
	for k := range r.mcp {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(r.mcp, k)
		}
	}
}

func (r *ToolRegistry) AllTools() []ToolDefinition {
	tools := make([]ToolDefinition, 0, len(r.internal)+len(r.mcp))
	for _, t := range r.internal {
		tools = append(tools, t.Definition())
	}
	for _, t := range r.mcp {
		tools = append(tools, t.Tool)
	}
	return tools
}

func (r *ToolRegistry) Dispatch(ctx context.Context, toolName string, params map[string]interface{}) (json.RawMessage, error) {
	userID := "default"
	if u, ok := ctx.Value("user_id").(string); ok {
		userID = u
	}

	if !r.permManager.CheckPermission(userID, toolName) {
		return nil, fmt.Errorf("tool %q is not permitted", toolName)
	}

	if t, ok := r.internal[toolName]; ok {
		if isRestrictedTool(toolName) && !r.isMaster {
			return nil, fmt.Errorf("tool %q is only available to the master agent", toolName)
		}
		return t.Execute(ctx, params)
	}
	if t, ok := r.mcp[toolName]; ok {
		raw, err := t.Client.CallTool(ctx, toolName, params)
		if err != nil {
			return nil, err
		}
		// Sanitize the raw result (truncate, strip control chars) but do NOT wrap
		// in <tool_result> XML — standard OpenAI/Ollama tool_calls protocol expects
		// plain content in the tool role message.
		return r.sanitizer.Sanitize(raw), nil
	}
	return nil, fmt.Errorf("tool %q not found in registry", toolName)
}

func (r *ToolRegistry) HasTool(name string) bool {
	_, hasInternal := r.internal[name]
	_, hasMCP := r.mcp[name]
	return hasInternal || hasMCP
}

func (r *ToolRegistry) IsInternal(name string) bool {
	_, ok := r.internal[name]
	return ok
}

func (r *ToolRegistry) SetMaster(isMaster bool) {
	r.isMaster = isMaster
}

func isRestrictedTool(name string) bool {
	restricted := map[string]bool{
		"terminal_spawn": true,
		"subagent_spawn": true,
	}
	return restricted[name]
}
