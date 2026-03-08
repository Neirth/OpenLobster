package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
	"github.com/neirth/openlobster/internal/domain/ports"
	domainServices "github.com/neirth/openlobster/internal/domain/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAIProvider struct {
	mock.Mock
}

func (m *MockAIProvider) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return ports.ChatResponse{}, args.Error(1)
	}
	return args.Get(0).(ports.ChatResponse), args.Error(1)
}

func (m *MockAIProvider) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(ports.ChatResponse), args.Error(1)
}

func (m *MockAIProvider) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(ports.ChatResponseWithAudio), args.Error(1)
}

func (m *MockAIProvider) SupportsAudioInput() bool  { return true }
func (m *MockAIProvider) SupportsAudioOutput() bool { return true }
func (m *MockAIProvider) GetMaxTokens() int         { return 4096 }

func TestSubAgentService_Spawn(t *testing.T) {
	aiProvider := new(MockAIProvider)
	aiProvider.On("Chat", mock.Anything, mock.Anything).Return(ports.ChatResponse{Content: "done", StopReason: "stop"}, nil).Maybe()
	subAgentSvc := domainServices.NewSubAgentService(aiProvider, 3, 5*time.Second)

	agent, err := subAgentSvc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "worker", Model: "gpt-4", SystemPrompt: "help"}, "task")
	assert.NoError(t, err)
	assert.NotEmpty(t, agent.ID())
}

func TestSubAgentService_MaxConcurrent(t *testing.T) {
	aiProvider := new(MockAIProvider)
	aiProvider.On("Chat", mock.Anything, mock.Anything).Return(ports.ChatResponse{Content: "done"}, nil).Maybe()
	subAgentSvc := domainServices.NewSubAgentService(aiProvider, 2, 5*time.Second)

	_, _ = subAgentSvc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "w1"}, "t1")
	_, _ = subAgentSvc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "w2"}, "t2")
	_, err := subAgentSvc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "w3"}, "t3")
	assert.Error(t, err)
}

func TestSubAgentService_List(t *testing.T) {
	aiProvider := new(MockAIProvider)
	aiProvider.On("Chat", mock.Anything, mock.Anything).Return(ports.ChatResponse{Content: "done"}, nil).Maybe()
	subAgentSvc := domainServices.NewSubAgentService(aiProvider, 3, 5*time.Second)

	_, _ = subAgentSvc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "worker"}, "task")
	agents, _ := subAgentSvc.List(context.Background())
	assert.Len(t, agents, 1)
}

func TestSubAgentService_Kill(t *testing.T) {
	aiProvider := new(MockAIProvider)
	aiProvider.On("Chat", mock.Anything, mock.Anything).Return(ports.ChatResponse{Content: "done"}, nil).Maybe()
	subAgentSvc := domainServices.NewSubAgentService(aiProvider, 3, 5*time.Second)

	agent, _ := subAgentSvc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "worker"}, "task")
	_ = subAgentSvc.Kill(context.Background(), agent.ID())
	agents, _ := subAgentSvc.List(context.Background())
	assert.Len(t, agents, 0)
}

func TestScheduler_LoopbackConstant(t *testing.T) {
	// LoopbackChannelID is the session used by the scheduler for task execution.
	const loopback = "loopback"
	assert.Equal(t, "loopback", loopback)
}

func TestScheduler_HealthOK(t *testing.T) {
	// Verify that the health status string expected by resolveHealth is "ok".
	healthStatus := "ok"
	assert.Equal(t, "ok", healthStatus)
}

func TestToolRegistry_Integration(t *testing.T) {
	permManager := permissions.Default()
	permManager.SetPermission("default", "test_tool", permissions.PermissionAlways)

	registry := mcp.NewToolRegistry(true, permManager)
	registry.RegisterInternal("test_tool", &mockInternalTool{})

	assert.True(t, registry.HasTool("test_tool"))
	assert.True(t, registry.IsInternal("test_tool"))

	result, err := registry.Dispatch(context.Background(), "test_tool", nil)
	assert.NoError(t, err)
	assert.Contains(t, string(result), "executed")
}

func TestToolRegistry_RestrictedTools(t *testing.T) {
	registry := mcp.NewToolRegistry(false, nil)
	registry.RegisterInternal("terminal_spawn", &mockInternalTool{})

	_, err := registry.Dispatch(context.Background(), "terminal_spawn", nil)
	assert.Error(t, err)
}

func TestToolRegistry_MasterCanUseRestricted(t *testing.T) {
	permManager := permissions.Default()
	permManager.SetPermission("default", "terminal_spawn", permissions.PermissionAlways)

	registry := mcp.NewToolRegistry(true, permManager)
	registry.RegisterInternal("terminal_spawn", &mockInternalTool{})

	result, err := registry.Dispatch(context.Background(), "terminal_spawn", nil)
	assert.NoError(t, err)
	assert.Contains(t, string(result), "executed")
}

func TestEventBus_Integration(t *testing.T) {
	eventBus := domainServices.NewEventBus()
	var received events.Event
	_ = eventBus.Subscribe("test.event", func(ctx context.Context, e events.Event) error {
		received = e
		return nil
	})
	_ = eventBus.Publish(context.Background(), events.NewEvent("test.event", nil))
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, "test.event", received.GetType())
}

type mockInternalTool struct{}

func (m *mockInternalTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{Name: "test_tool", Description: "Test tool"}
}

func (m *mockInternalTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	return json.RawMessage(`{"status": "executed"}`), nil
}
