package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMCPTool(t *testing.T) {
	client := &MockMCPClient{}

	tool := MCPTool{
		Client: client,
		Tool: ToolDefinition{
			Name:        "test_tool",
			Description: "A test tool",
		},
	}

	assert.Equal(t, "test_tool", tool.Tool.Name)
}

func TestServerConfig(t *testing.T) {
	config := ServerConfig{
		Name: "test-server",
		Type: "http",
		URL:  "https://mcp.example.com/mcp",
	}

	assert.Equal(t, "test-server", config.Name)
	assert.Equal(t, "http", config.Type)
	assert.Equal(t, "https://mcp.example.com/mcp", config.URL)
}

func TestToolDefinition(t *testing.T) {
	schema := json.RawMessage(`{"type": "object", "properties": {"name": {"type": "string"}}}`)
	td := ToolDefinition{
		Name:        "test",
		Description: "Test tool",
		InputSchema: schema,
	}

	assert.Equal(t, "test", td.Name)
	assert.NotNil(t, td.InputSchema)
}

func TestMockMCPClient_Connect(t *testing.T) {
	client := &MockMCPClient{}

	client.On("Connect", mock.Anything, mock.Anything).Return(nil)

	err := client.Connect(context.Background(), ServerConfig{Name: "test"})

	assert.NoError(t, err)
	client.AssertExpectations(t)
}

func TestMockMCPClient_CallTool(t *testing.T) {
	client := &MockMCPClient{}

	client.On("CallTool", mock.Anything, "tool1", mock.Anything).Return(json.RawMessage(`{"result": "ok"}`), nil)

	result, err := client.CallTool(context.Background(), "tool1", map[string]interface{}{})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "ok")
	client.AssertExpectations(t)
}

func TestMockMCPClient_ListTools(t *testing.T) {
	client := &MockMCPClient{}

	client.On("ListTools", mock.Anything).Return([]ToolDefinition{
		{Name: "tool1"},
		{Name: "tool2"},
	}, nil)

	tools, err := client.ListTools(context.Background())

	assert.NoError(t, err)
	assert.Len(t, tools, 2)
	client.AssertExpectations(t)
}

func TestMockMCPClient_Close(t *testing.T) {
	client := &MockMCPClient{}

	client.On("Close").Return(nil)

	err := client.Close()

	assert.NoError(t, err)
	client.AssertExpectations(t)
}
