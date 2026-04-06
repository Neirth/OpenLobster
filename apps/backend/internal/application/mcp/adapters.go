// Package mcp provides application-layer adapters that wire the MCP
// infrastructure (MCPClientSDK, OAuthManager, ToolRegistry) to the GraphQL
// dto ports consumed by the resolvers, and utility helpers for synchronising
// tool registries with the AgentRegistry.
package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/repositories"
	svmcp "github.com/neirth/openlobster/internal/domain/services/mcp"
)

// SyncToolsToRegistry copies all tools from reg into agentReg so that the
// GraphQL tools/mcpTools queries and Status/Metrics expose the full list
// (internal + MCP) to the frontend.
func SyncToolsToRegistry(reg *svmcp.ToolRegistry, agentReg *registry.AgentRegistry) {
	if reg == nil || agentReg == nil {
		return
	}
	defs := reg.AllTools()
	snapshots := make([]dto.ToolSnapshot, len(defs))
	for i, d := range defs {
		source := "internal"
		serverName := ""
		if strings.Contains(d.Name, ":") {
			source = "mcp"
			if idx := strings.Index(d.Name, ":"); idx >= 0 {
				serverName = d.Name[:idx]
			}
		}
		snapshots[i] = dto.ToolSnapshot{
			Name:        d.Name,
			Description: d.Description,
			Source:      source,
			ServerName:  serverName,
		}
	}
	agentReg.UpdateMCPTools(snapshots)
}

// ToolNamesAdapter exposes the names of all registered tools for the
// SetAllToolPermissions mutation.  Implements dto.ToolNamesSource.
type ToolNamesAdapter struct{ Reg *svmcp.ToolRegistry }

func (a *ToolNamesAdapter) AllToolNames() []string {
	if a.Reg == nil {
		return nil
	}
	defs := a.Reg.AllTools()
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}

// ---------------------------------------------------------------------------
// ConnectAdapter — implements dto.McpConnectPort
// ---------------------------------------------------------------------------

// ConnectAdapter performs MCP server connection/disconnection, persists the
// server URL, registers tools in the ToolRegistry, and fires domain events.
type ConnectAdapter struct {
	Client   *svmcp.MCPClientSDK
	Registry *svmcp.ToolRegistry
	AgentReg *registry.AgentRegistry
	Repo     repositories.MCPServerRepositoryPort
	OAuth    *svmcp.OAuthManager
	EventBus dto.EventBusPort
}

func (a *ConnectAdapter) Connect(ctx context.Context, name, transport, url string) (bool, error) {
	if transport != "http" && transport != "" {
		return false, fmt.Errorf("only http transport is supported, got %q", transport)
	}
	if url == "" {
		return false, fmt.Errorf("url is required")
	}
	err := a.Client.Connect(ctx, svmcp.ServerConfig{Name: name, Type: "http", URL: url})
	if err != nil {
		errStr := err.Error()
		requiresAuth := strings.Contains(errStr, "401") || strings.Contains(strings.ToLower(errStr), "unauthorized")
		if requiresAuth {
			a.OAuth.RegisterPendingServer(name, url)
		}
		return requiresAuth, err
	}
	if err := a.Repo.Save(ctx, name, url); err != nil {
		log.Printf("mcp: failed to persist %q: %v", name, err)
	}
	if tools := a.Client.GetServerTools(name); len(tools) > 0 {
		_ = a.Registry.RegisterMCP(name, a.Client, tools)
		log.Printf("mcp: registered %d tools from %q", len(tools), name)
	}
	SyncToolsToRegistry(a.Registry, a.AgentReg)
	if a.EventBus != nil {
		_ = a.EventBus.Publish(ctx, events.EventMCPServerConnected, map[string]string{"name": name})
	}
	return false, nil
}

func (a *ConnectAdapter) Disconnect(ctx context.Context, name string) error {
	if err := a.Client.Disconnect(name); err != nil {
		return err
	}
	a.Registry.UnregisterMCP(name)
	SyncToolsToRegistry(a.Registry, a.AgentReg)
	return a.Repo.Delete(ctx, name)
}

func (a *ConnectAdapter) GetConnectionStatus(name string) string {
	if tools := a.Client.GetServerTools(name); len(tools) > 0 {
		return "online"
	}
	return "unknown"
}

func (a *ConnectAdapter) GetServerToolCount(name string) int {
	return len(a.Client.GetServerTools(name))
}

// ---------------------------------------------------------------------------
// OAuthAdapter — implements dto.McpOAuthPort
// ---------------------------------------------------------------------------

// OAuthAdapter delegates OAuth initiation and status queries to OAuthManager.
type OAuthAdapter struct {
	OAuth *svmcp.OAuthManager
}

func (a *OAuthAdapter) InitiateOAuth(ctx context.Context, serverName, mcpURL string) (string, error) {
	return a.OAuth.InitiateOAuth(ctx, serverName, mcpURL)
}

func (a *OAuthAdapter) SetClientID(ctx context.Context, serverName, clientID string) error {
	return a.OAuth.SetClientID(ctx, serverName, clientID)
}

func (a *OAuthAdapter) SetClientSecret(ctx context.Context, serverName, clientSecret string) error {
	return a.OAuth.SetClientSecret(ctx, serverName, clientSecret)
}

func (a *OAuthAdapter) Status(serverName string) (status, errMsg string) {
	s, errStr := a.OAuth.Status(serverName)
	return string(s), errStr
}
