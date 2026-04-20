package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/neirth/openlobster/internal/domain/services/permissions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockInternalTool struct {
	mock.Mock
}

func (m *MockInternalTool) Definition() ToolDefinition {
	args := m.Called()
	return args.Get(0).(ToolDefinition)
}

func (m *MockInternalTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(json.RawMessage), args.Error(1)
}

type MockMCPClient struct {
	mock.Mock
}

func (m *MockMCPClient) Connect(ctx context.Context, server ServerConfig) error {
	args := m.Called(ctx, server)
	return args.Error(0)
}

func (m *MockMCPClient) CallTool(ctx context.Context, tool string, params map[string]interface{}) (json.RawMessage, error) {
	args := m.Called(ctx, tool, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(json.RawMessage), args.Error(1)
}

func (m *MockMCPClient) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	args := m.Called(ctx)
	return args.Get(0).([]ToolDefinition), args.Error(1)
}

func (m *MockMCPClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMCPClient) GetServerURL(name string) string {
	return ""
}

func TestNewToolRegistry(t *testing.T) {
	registry := NewToolRegistry(true, nil)

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.internal)
	assert.NotNil(t, registry.mcp)
	assert.NotNil(t, registry.sanitizer)
	assert.True(t, registry.isMaster)
}

func TestToolRegistry_RegisterInternal(t *testing.T) {
	registry := NewToolRegistry(false, nil)
	tool := &MockInternalTool{}

	tool.On("Definition").Return(ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
	})

	registry.RegisterInternal("test_tool", tool)

	assert.True(t, registry.HasTool("test_tool"))
	assert.True(t, registry.IsInternal("test_tool"))
}

func TestToolRegistry_Dispatch_InternalTool(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "test_tool", permissions.PermissionAlways)

	registry := NewToolRegistry(false, permManager)
	tool := &MockInternalTool{}

	tool.On("Execute", mock.Anything, mock.Anything).Return(json.RawMessage(`{"result": "ok"}`), nil)

	registry.RegisterInternal("test_tool", tool)

	result, err := registry.Dispatch(context.Background(), "test_tool", map[string]interface{}{})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "ok")
	tool.AssertExpectations(t)
}

func TestToolRegistry_Dispatch_RestrictedTool(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "terminal_spawn", permissions.PermissionDeny)

	registry := NewToolRegistry(false, permManager)
	tool := &MockInternalTool{}

	tool.On("Definition").Return(ToolDefinition{Name: "terminal_spawn"})
	tool.On("Execute", mock.Anything, mock.Anything).Return(json.RawMessage(`{}`), nil)

	registry.RegisterInternal("terminal_spawn", tool)

	result, err := registry.Dispatch(context.Background(), "terminal_spawn", map[string]interface{}{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not permitted")
}

func TestToolRegistry_Dispatch_MasterCanUseRestrictedTool(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "terminal_spawn", permissions.PermissionAlways)

	registry := NewToolRegistry(true, permManager)
	tool := &MockInternalTool{}

	tool.On("Execute", mock.Anything, mock.Anything).Return(json.RawMessage(`{"result": "ok"}`), nil)

	registry.RegisterInternal("terminal_spawn", tool)

	result, err := registry.Dispatch(context.Background(), "terminal_spawn", map[string]interface{}{})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "ok")
}

func TestToolRegistry_Dispatch_ToolNotFound(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "nonexistent_tool", permissions.PermissionAlways)

	registry := NewToolRegistry(true, permManager)

	result, err := registry.Dispatch(context.Background(), "nonexistent_tool", map[string]interface{}{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestToolRegistry_HasTool(t *testing.T) {
	registry := NewToolRegistry(true, nil)

	assert.False(t, registry.HasTool("some_tool"))

	tool := &MockInternalTool{}
	tool.On("Definition").Return(ToolDefinition{Name: "some_tool"})
	registry.RegisterInternal("some_tool", tool)

	assert.True(t, registry.HasTool("some_tool"))
}

func TestToolRegistry_IsInternal(t *testing.T) {
	registry := NewToolRegistry(true, nil)

	tool := &MockInternalTool{}
	tool.On("Definition").Return(ToolDefinition{Name: "internal_tool"})
	registry.RegisterInternal("internal_tool", tool)

	assert.True(t, registry.IsInternal("internal_tool"))
	assert.False(t, registry.IsInternal("nonexistent"))
}

func TestToolRegistry_SetMaster(t *testing.T) {
	registry := NewToolRegistry(false, nil)

	assert.False(t, registry.isMaster)

	registry.SetMaster(true)

	assert.True(t, registry.isMaster)
}

func TestToolRegistry_AllTools(t *testing.T) {
	registry := NewToolRegistry(true, nil)

	tool1 := &MockInternalTool{}
	tool1.On("Definition").Return(ToolDefinition{Name: "tool1", Description: "Tool 1"})
	registry.RegisterInternal("tool1", tool1)

	tools := registry.AllTools()

	assert.Len(t, tools, 1)
	assert.Equal(t, "tool1", tools[0].Name)
}

func TestToolRegistry_Dispatch_Success(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "test_tool", permissions.PermissionAlways)

	registry := NewToolRegistry(true, permManager)
	tool := &MockInternalTool{}

	tool.On("Execute", mock.Anything, mock.Anything).Return(json.RawMessage(`{"result": "ok"}`), nil)

	registry.RegisterInternal("test_tool", tool)

	result, err := registry.Dispatch(context.Background(), "test_tool", map[string]interface{}{})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "ok")
}

func TestToolRegistry_Dispatch_Error(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "nonexistent", permissions.PermissionAlways)

	registry := NewToolRegistry(true, permManager)

	_, err := registry.Dispatch(context.Background(), "nonexistent", map[string]interface{}{})

	assert.Error(t, err)
}

func TestToolRegistry_Dispatch_MCPTool(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "test-server:mcp_tool", permissions.PermissionAlways)

	registry := NewToolRegistry(true, permManager)
	mcpClient := &MockMCPClient{}

	mcpClient.On("CallTool", mock.Anything, "test-server:mcp_tool", mock.Anything).Return(json.RawMessage(`{"result": "mcp"}`), nil)

	err := registry.RegisterMCP("test-server", mcpClient, []ToolDefinition{
		{Name: "mcp_tool", Description: "MCP tool"},
	})
	assert.NoError(t, err)

	result, err := registry.Dispatch(context.Background(), "test-server:mcp_tool", map[string]interface{}{})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "mcp")
	assert.Contains(t, string(result), "EXTERNAL")
	mcpClient.AssertExpectations(t)
}

func TestToolRegistry_Dispatch_MCPToolError(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "test-server:mcp_tool", permissions.PermissionAlways)

	registry := NewToolRegistry(true, permManager)
	mcpClient := &MockMCPClient{}

	mcpClient.On("CallTool", mock.Anything, "test-server:mcp_tool", mock.Anything).Return(nil, assert.AnError)

	err := registry.RegisterMCP("test-server", mcpClient, []ToolDefinition{
		{Name: "mcp_tool", Description: "MCP tool"},
	})
	assert.NoError(t, err)

	result, err := registry.Dispatch(context.Background(), "test-server:mcp_tool", map[string]interface{}{})

	assert.Error(t, err)
	assert.Nil(t, result)
	mcpClient.AssertExpectations(t)
}

func TestToolRegistry_AllTools_Empty(t *testing.T) {
	registry := NewToolRegistry(true, nil)

	tools := registry.AllTools()

	assert.Len(t, tools, 0)
}

func TestToolRegistry_AllTools_WithMCP(t *testing.T) {
	registry := NewToolRegistry(true, nil)
	mcpClient := &MockMCPClient{}

	registry.RegisterMCP("test-server", mcpClient, []ToolDefinition{
		{Name: "mcp_tool", Description: "MCP tool"},
	})

	tools := registry.AllTools()

	assert.Len(t, tools, 1)
}

func TestToolRegistry_Dispatch_InternalToolError(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("default", "test_tool", permissions.PermissionAlways)

	registry := NewToolRegistry(true, permManager)
	tool := &MockInternalTool{}

	tool.On("Execute", mock.Anything, mock.Anything).Return(nil, assert.AnError)

	registry.RegisterInternal("test_tool", tool)

	result, err := registry.Dispatch(context.Background(), "test_tool", map[string]interface{}{})

	assert.Error(t, err)
	assert.Nil(t, result)
	tool.AssertExpectations(t)
}

func TestToolRegistry_UnregisterMCP(t *testing.T) {
	registry := NewToolRegistry(true, nil)
	mcpClient := &MockMCPClient{}

	err := registry.RegisterMCP("server1", mcpClient, []ToolDefinition{
		{Name: "tool_a", Description: "A"},
		{Name: "tool_b", Description: "B"},
	})
	assert.NoError(t, err)
	assert.True(t, registry.HasTool("server1:tool_a"))
	assert.True(t, registry.HasTool("server1:tool_b"))

	registry.UnregisterMCP("server1")
	assert.False(t, registry.HasTool("server1:tool_a"))
	assert.False(t, registry.HasTool("server1:tool_b"))
}

func TestToolRegistry_Dispatch_WithUserIDInContext(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("user-1", "my_tool", permissions.PermissionAlways)

	registry := NewToolRegistry(true, permManager)
	tool := &MockInternalTool{}
	tool.On("Execute", mock.Anything, mock.Anything).Return(json.RawMessage(`{}`), nil)
	registry.RegisterInternal("my_tool", tool)

	ctx := context.WithValue(context.Background(), ContextKeyUserID, "user-1")
	result, err := registry.Dispatch(ctx, "my_tool", nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestToolRegistry_Dispatch_UserDenied(t *testing.T) {
	permManager := permissions.NewManager()
	permManager.SetPermission("user-1", "my_tool", permissions.PermissionDeny)

	registry := NewToolRegistry(true, permManager)
	tool := &MockInternalTool{}
	tool.On("Definition").Return(ToolDefinition{Name: "my_tool"})
	registry.RegisterInternal("my_tool", tool)

	ctx := context.WithValue(context.Background(), ContextKeyUserID, "user-1")
	_, err := registry.Dispatch(ctx, "my_tool", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not permitted")
}
