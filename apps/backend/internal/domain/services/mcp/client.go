package mcp

import (
	"context"
	"encoding/json"
)

type MCPClient interface {
	Connect(ctx context.Context, server ServerConfig) error
	CallTool(ctx context.Context, tool string, params map[string]interface{}) (json.RawMessage, error)
	ListTools(ctx context.Context) ([]ToolDefinition, error)
	GetServerURL(name string) string
	Close() error
}

// ServerConfig holds the configuration for a remote MCP server.
// Only Streamable HTTP (Type == "http") is supported.
type ServerConfig struct {
	Name    string
	Type    string            // must be "http" (Streamable HTTP is the only supported transport)
	URL     string            // Endpoint of the MCP server
	Headers map[string]string // Additional HTTP headers
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}
