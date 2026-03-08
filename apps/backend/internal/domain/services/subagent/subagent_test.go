package subagent

import (
	"context"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAIProvider struct {
	response string
}

func (m *mockAIProvider) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	return ports.ChatResponse{Content: m.response, StopReason: "stop"}, nil
}

func (m *mockAIProvider) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, nil
}

func (m *mockAIProvider) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, nil
}

func (m *mockAIProvider) SupportsAudioInput() bool  { return false }
func (m *mockAIProvider) SupportsAudioOutput() bool { return false }
func (m *mockAIProvider) GetMaxTokens() int         { return 4096 }

func TestNewService(t *testing.T) {
	svc := NewService(nil, 5, time.Minute)
	assert.NotNil(t, svc)
}

func TestService_List_Empty(t *testing.T) {
	svc := NewService(nil, 5, time.Minute)
	list, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestService_Spawn_List_Kill(t *testing.T) {
	ai := &mockAIProvider{response: "done"}
	svc := NewService(ai, 5, 2*time.Second)

	agent, err := svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "test", Model: "gpt-4"}, "do something")
	require.NoError(t, err)
	require.NotNil(t, agent)

	time.Sleep(50 * time.Millisecond) // allow goroutine to start

	list, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "test", list[0].Name)

	err = svc.Kill(context.Background(), agent.ID())
	require.NoError(t, err)

	list, _ = svc.List(context.Background())
	assert.Empty(t, list)
}

func TestService_Kill_NotFound(t *testing.T) {
	svc := NewService(nil, 5, time.Minute)
	err := svc.Kill(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestService_Spawn_MaxConcurrentReached(t *testing.T) {
	ai := &mockAIProvider{response: "slow"}
	svc := NewService(ai, 1, 5*time.Second)

	agent1, err := svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "a1"}, "task1")
	require.NoError(t, err)
	require.NotNil(t, agent1)

	_, err = svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "a2"}, "task2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max concurrent")
}

func TestService_Cleanup(t *testing.T) {
	ai := &mockAIProvider{response: "x"}
	svc := NewService(ai, 5, 2*time.Second)
	agent, err := svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "x"}, "task")
	require.NoError(t, err)
	require.NotNil(t, agent)

	time.Sleep(30 * time.Millisecond)
	svc.Cleanup()

	list, _ := svc.List(context.Background())
	assert.Empty(t, list)
}

func TestService_Spawn_NilAI_StatusFailed(t *testing.T) {
	svc := NewService(nil, 2, 100*time.Millisecond)
	agent, err := svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "x"}, "task")
	require.NoError(t, err)
	require.NotNil(t, agent)

	time.Sleep(150 * time.Millisecond) // wait for runAgent to finish
	assert.Equal(t, "failed", agent.Status())
}

func TestService_Agent_Result(t *testing.T) {
	ai := &mockAIProvider{response: "my result"}
	svc := NewService(ai, 5, 2*time.Second)
	agent, err := svc.Spawn(context.Background(), mcp.SubAgentConfig{Name: "x"}, "task")
	require.NoError(t, err)

	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, "my result", agent.Result())
}
