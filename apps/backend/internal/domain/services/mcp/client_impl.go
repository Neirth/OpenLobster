package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	mcpc "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/neirth/openlobster/internal/infrastructure/secrets"
)

type MCPClientSDK struct {
	secrets secrets.SecretsProvider
	servers map[string]MCPServerConnection
	mu      sync.RWMutex
}

type MCPServerConnection struct {
	Config  ServerConfig
	Client  *mcpc.Client
	Tools   []ToolDefinition
	Favicon string // base64 data URI, may be empty
}

func NewMCPClientSDK(secretsProvider secrets.SecretsProvider) *MCPClientSDK {
	return &MCPClientSDK{
		secrets: secretsProvider,
		servers: make(map[string]MCPServerConnection),
	}
}

func (c *MCPClientSDK) Connect(ctx context.Context, server ServerConfig) error {
	if server.Type != "http" {
		return fmt.Errorf("unknown server type: %s — only streamable HTTP transport is supported", server.Type)
	}
	return c.connectHTTP(ctx, server)
}

func (c *MCPClientSDK) connectHTTP(ctx context.Context, server ServerConfig) error {
	headers := make(map[string]string)
	if server.Name != "" {
		tokenKey := fmt.Sprintf("mcp/remote/%s/token", server.Name)
		token, err := c.secrets.Get(ctx, tokenKey)
		if err != nil {
			return fmt.Errorf("failed to read OAuth token from secrets backend for server %q (key %s): %w", server.Name, tokenKey, err)
		}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}

	for k, v := range server.Headers {
		headers[k] = v
	}

	mcpClient, err := mcpc.NewStreamableHttpClient(
		server.URL,
		transport.WithHTTPHeaders(headers),
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP MCP client: %w", err)
	}

	err = mcpClient.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start MCP client: %w", err)
	}

	_, err = mcpClient.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "openlobster",
				Version: "0.1.0",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	toolsResult, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	toolDefs := make([]ToolDefinition, 0)
	for _, tool := range toolsResult.Tools {
		inputSchema, _ := json.Marshal(tool.InputSchema)
		toolDefs = append(toolDefs, ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: inputSchema,
		})
	}

	// Store connection.
	c.mu.Lock()
	c.servers[server.Name] = MCPServerConnection{
		Config: server,
		Client: mcpClient,
		Tools:  toolDefs,
	}
	c.mu.Unlock()

	return nil
}

// GetServerURL returns the configured endpoint URL for the named server,
// or an empty string if the server is not found.
func (c *MCPClientSDK) GetServerURL(name string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if conn, ok := c.servers[name]; ok {
		return conn.Config.URL
	}
	return ""
}

func (c *MCPClientSDK) CallTool(ctx context.Context, tool string, params map[string]interface{}) (json.RawMessage, error) {
	parts := splitToolName(tool)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid tool name: %s", tool)
	}

	serverName := parts[0]
	toolName := parts[1]

	c.mu.RLock()
	conn, ok := c.servers[serverName]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("server not connected: %s", serverName)
	}

	result, err := conn.Client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: params,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	content, _ := json.Marshal(result.Content)
	return content, nil
}

func (c *MCPClientSDK) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allTools []ToolDefinition
	for _, conn := range c.servers {
		allTools = append(allTools, conn.Tools...)
	}
	return allTools, nil
}

// Disconnect closes and removes a single named server connection.
func (c *MCPClientSDK) Disconnect(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	conn, ok := c.servers[name]
	if !ok {
		return nil
	}
	conn.Client.Close()
	delete(c.servers, name)
	return nil
}

func (c *MCPClientSDK) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name := range c.servers {
		if conn, ok := c.servers[name]; ok {
			conn.Client.Close()
		}
		delete(c.servers, name)
	}
	return nil
}

func (c *MCPClientSDK) GetServerTools(serverName string) []ToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if conn, ok := c.servers[serverName]; ok {
		return conn.Tools
	}
	return nil
}

func splitToolName(name string) []string {
	parts := strings.SplitN(name, ":", 2)
	if len(parts) == 2 {
		return parts
	}
	return []string{"internal", name}
}

var _ MCPClient = (*MCPClientSDK)(nil)
