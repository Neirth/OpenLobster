package ai

import (
	"context"

	"github.com/neirth/openlobster/internal/domain/ports"
)

type MockAIAdapter struct {
	ChatCalls             []ports.ChatRequest
	ChatWithAudioCalls    []ports.ChatRequestWithAudio
	ChatToAudioCalls      []ports.ChatRequest
	ChatResponse          ports.ChatResponse
	ChatWithAudioResponse ports.ChatResponse
	ChatToAudioResponse   ports.ChatResponseWithAudio
	Error                 error

	OnChat          func(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error)
	OnChatWithAudio func(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error)
	OnChatToAudio   func(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error)
}

func NewMockAIAdapter() *MockAIAdapter {
	return &MockAIAdapter{
		ChatCalls:          make([]ports.ChatRequest, 0),
		ChatWithAudioCalls: make([]ports.ChatRequestWithAudio, 0),
		ChatToAudioCalls:   make([]ports.ChatRequest, 0),
	}
}

func (m *MockAIAdapter) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	m.ChatCalls = append(m.ChatCalls, req)
	if m.OnChat != nil {
		return m.OnChat(ctx, req)
	}
	if m.Error != nil {
		return ports.ChatResponse{}, m.Error
	}
	return m.ChatResponse, nil
}

func (m *MockAIAdapter) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	m.ChatWithAudioCalls = append(m.ChatWithAudioCalls, req)
	if m.OnChatWithAudio != nil {
		return m.OnChatWithAudio(ctx, req)
	}
	if m.Error != nil {
		return ports.ChatResponse{}, m.Error
	}
	return m.ChatWithAudioResponse, nil
}

func (m *MockAIAdapter) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	m.ChatToAudioCalls = append(m.ChatToAudioCalls, req)
	if m.OnChatToAudio != nil {
		return m.OnChatToAudio(ctx, req)
	}
	if m.Error != nil {
		return ports.ChatResponseWithAudio{}, m.Error
	}
	return m.ChatToAudioResponse, nil
}

func (m *MockAIAdapter) SupportsAudioInput() bool {
	return true
}

func (m *MockAIAdapter) SupportsAudioOutput() bool {
	return true
}

func (m *MockAIAdapter) GetMaxTokens() int {
	return 4096
}

func (m *MockAIAdapter) Close() error {
	return nil
}

var _ ports.AIProviderPort = (*MockAIAdapter)(nil)

type MockOpenAIAdapter struct {
	MockAIAdapter
	APIKey string
}

func NewMockOpenAIAdapter() *MockOpenAIAdapter {
	return &MockOpenAIAdapter{
		MockAIAdapter: *NewMockAIAdapter(),
	}
}

type MockOllamaAdapter struct {
	MockAIAdapter
	Endpoint string
	Model    string
}

func NewMockOllamaAdapter() *MockOllamaAdapter {
	return &MockOllamaAdapter{
		MockAIAdapter: *NewMockAIAdapter(),
	}
}

type MockOpenRouterAdapter struct {
	MockAIAdapter
	APIKey string
}

func NewMockOpenRouterAdapter() *MockOpenRouterAdapter {
	return &MockOpenRouterAdapter{
		MockAIAdapter: *NewMockAIAdapter(),
	}
}
