package ollama

import (
	"context"
	"testing"

	ollamaapi "github.com/ollama/ollama/api"

	"github.com/neirth/openlobster/internal/domain/ports"
)

type mockChatClient struct {
	response ollamaapi.ChatResponse
}

func (m *mockChatClient) Chat(ctx context.Context, req *ollamaapi.ChatRequest, fn ollamaapi.ChatResponseFunc) error {
	return fn(m.response)
}

func (m *mockChatClient) Show(ctx context.Context, req *ollamaapi.ShowRequest) (*ollamaapi.ShowResponse, error) {
	return &ollamaapi.ShowResponse{}, nil
}

func TestAdapter_Chat_Reasoning(t *testing.T) {
	mockClient := &mockChatClient{
		response: ollamaapi.ChatResponse{
			Message: ollamaapi.Message{
				Role:    "assistant",
				Content: "<thought>\nI should greet the user.\n</thought>\nHello there!",
			},
			Done: true,
		},
	}

	adapter := &Adapter{
		client: mockClient,
		model:  "deepseek-r1",
	}

	resp, err := adapter.Chat(context.Background(), ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	expectedContent := "Hello there!"
	if resp.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, resp.Content)
	}
}

func TestAdapter_Chat_MultipleReasoning(t *testing.T) {
	mockClient := &mockChatClient{
		response: ollamaapi.ChatResponse{
			Message: ollamaapi.Message{
				Role:    "assistant",
				Content: "<thought>Step 1</thought> Intermediate <thought>Step 2</thought> Final answer",
			},
			Done: true,
		},
	}

	adapter := &Adapter{
		client: mockClient,
		model:  "deepseek-r1",
	}

	resp, err := adapter.Chat(context.Background(), ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	expectedContent := "Intermediate  Final answer"
	if resp.Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, resp.Content)
	}
}
