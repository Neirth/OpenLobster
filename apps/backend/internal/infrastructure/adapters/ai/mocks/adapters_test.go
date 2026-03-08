package ai

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

func TestMockAIAdapter_Chat(t *testing.T) {
	adapter := NewMockAIAdapter()
	adapter.ChatResponse = ports.ChatResponse{
		Content: "Hello!",
	}

	req := ports.ChatRequest{
		Model: "gpt-4",
		Messages: []ports.ChatMessage{
			{Role: "user", Content: "Hi"},
		},
	}

	resp, err := adapter.Chat(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "Hello!", resp.Content)
	assert.Len(t, adapter.ChatCalls, 1)
}

func TestMockAIAdapter_ChatWithError(t *testing.T) {
	adapter := NewMockAIAdapter()
	adapter.Error = assert.AnError

	req := ports.ChatRequest{Model: "gpt-4"}

	_, err := adapter.Chat(context.Background(), req)
	assert.Error(t, err)
}

func TestMockAIAdapter_ChatWithAudio(t *testing.T) {
	adapter := NewMockAIAdapter()
	adapter.ChatWithAudioResponse = ports.ChatResponse{
		Content: "Response with audio",
	}

	req := ports.ChatRequestWithAudio{
		Model: "gpt-4o-audio",
	}

	resp, err := adapter.ChatWithAudio(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "Response with audio", resp.Content)
}

func TestMockAIAdapter_ChatToAudio(t *testing.T) {
	adapter := NewMockAIAdapter()
	adapter.ChatToAudioResponse = ports.ChatResponseWithAudio{
		AudioData: []byte{1, 2, 3},
	}

	req := ports.ChatRequest{Model: "gpt-4o-audio"}

	resp, err := adapter.ChatToAudio(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3}, resp.AudioData)
}

func TestMockAIAdapter_Callbacks(t *testing.T) {
	adapter := NewMockAIAdapter()

	chatCalled := false
	adapter.OnChat = func(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
		chatCalled = true
		return ports.ChatResponse{Content: "callback response"}, nil
	}

	resp, err := adapter.Chat(context.Background(), ports.ChatRequest{Model: "gpt-4"})
	assert.NoError(t, err)
	assert.True(t, chatCalled)
	assert.Equal(t, "callback response", resp.Content)
}

func TestMockAIAdapter_SupportsAudio(t *testing.T) {
	adapter := NewMockAIAdapter()
	assert.True(t, adapter.SupportsAudioInput())
	assert.True(t, adapter.SupportsAudioOutput())
}

func TestMockAIAdapter_GetMaxTokens(t *testing.T) {
	adapter := NewMockAIAdapter()
	assert.Equal(t, 4096, adapter.GetMaxTokens())
}

func TestMockAIAdapter_Close(t *testing.T) {
	adapter := NewMockAIAdapter()
	assert.NoError(t, adapter.Close())
}

func TestMockOpenAIAdapter(t *testing.T) {
	adapter := NewMockOpenAIAdapter()
	adapter.APIKey = "test-key"
	adapter.ChatResponse = ports.ChatResponse{Content: "OpenAI response"}

	resp, err := adapter.Chat(context.Background(), ports.ChatRequest{Model: "gpt-4"})
	assert.NoError(t, err)
	assert.Equal(t, "OpenAI response", resp.Content)
}

func TestMockOllamaAdapter(t *testing.T) {
	adapter := NewMockOllamaAdapter()
	adapter.Endpoint = "http://localhost:11434"
	adapter.Model = "llama3"
	adapter.ChatResponse = ports.ChatResponse{Content: "Ollama response"}

	resp, err := adapter.Chat(context.Background(), ports.ChatRequest{Model: "llama3"})
	assert.NoError(t, err)
	assert.Equal(t, "Ollama response", resp.Content)
}

func TestMockOpenRouterAdapter(t *testing.T) {
	adapter := NewMockOpenRouterAdapter()
	adapter.APIKey = "test-key"
	adapter.ChatResponse = ports.ChatResponse{Content: "OpenRouter response"}

	resp, err := adapter.Chat(context.Background(), ports.ChatRequest{Model: "openai/gpt-4o"})
	assert.NoError(t, err)
	assert.Equal(t, "OpenRouter response", resp.Content)
}
