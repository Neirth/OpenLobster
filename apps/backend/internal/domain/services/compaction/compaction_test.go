package compaction

import (
	"context"
	"fmt"
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

type testMessageRepo struct{}

func (m *testMessageRepo) Save(ctx context.Context, message *models.Message) error {
	return nil
}

func (m *testMessageRepo) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Message, error) {
	messages := []models.Message{
		{Role: "user", Content: "Hello world test message"},
		{Role: "assistant", Content: "Hi there this is a longer response"},
	}
	if limit > 0 && len(messages) > limit {
		return messages[:limit], nil
	}
	return messages, nil
}

func (m *testMessageRepo) GetSinceLastCompaction(ctx context.Context, conversationID string) ([]models.Message, error) {
	return []models.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}, nil
}

func (m *testMessageRepo) GetLastCompaction(ctx context.Context, conversationID string) (*models.Message, error) {
	return nil, nil
}
func (m *testMessageRepo) GetUnvalidated(ctx context.Context, limit int) ([]models.Message, error) {
	return nil, nil
}
func (m *testMessageRepo) MarkAsValidated(ctx context.Context, ids []string) error {
	return nil
}

type testAIProviderForCompaction struct{}

func (m *testAIProviderForCompaction) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	return ports.ChatResponse{
		Content:    "Summarized conversation",
		StopReason: "stop",
	}, nil
}

func (m *testAIProviderForCompaction) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, nil
}

func (m *testAIProviderForCompaction) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, nil
}

func (m *testAIProviderForCompaction) SupportsAudioInput() bool {
	return false
}

func (m *testAIProviderForCompaction) SupportsAudioOutput() bool {
	return false
}

func (m *testAIProviderForCompaction) GetMaxTokens() int {
	return 10000
}

func TestNewService(t *testing.T) {
	service := NewService(&testMessageRepo{}, &testAIProviderForCompaction{}, 10000, 0.8)
	assert.NotNil(t, service)
}

func TestService_ShouldCompact_True(t *testing.T) {
	service := NewService(&testMessageRepo{}, &testAIProviderForCompaction{}, 10, 0.5)

	shouldCompact, err := service.ShouldCompact(context.Background(), "conv1")

	assert.NoError(t, err)
	assert.True(t, shouldCompact)
}

func TestService_ShouldCompact_False(t *testing.T) {
	service := NewService(&testMessageRepo{}, &testAIProviderForCompaction{}, 100000, 0.5)

	shouldCompact, err := service.ShouldCompact(context.Background(), "conv1")

	assert.NoError(t, err)
	assert.False(t, shouldCompact)
}

func TestService_Compact(t *testing.T) {
	service := NewService(&testMessageRepo{}, &testAIProviderForCompaction{}, 10000, 0.8)

	summary, err := service.Compact(context.Background(), "conv1")

	assert.NoError(t, err)
	assert.Equal(t, "Summarized conversation", summary)
}

func TestMessagesToContent(t *testing.T) {
	messages := []models.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	content := MessagesToContent(messages)

	assert.Contains(t, content, "user: Hello")
	assert.Contains(t, content, "assistant: Hi there")
}

func TestEstimateTokens(t *testing.T) {
	messages := []models.Message{
		{Role: "user", Content: "Hello world"},
		{Role: "assistant", Content: "Hi there"},
	}

	tokens := EstimateTokens(messages)

	assert.Greater(t, tokens, 0)
}

type failingMessageRepo struct{}

func (m *failingMessageRepo) Save(ctx context.Context, message *models.Message) error {
	return nil
}

func (m *failingMessageRepo) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Message, error) {
	return nil, fmt.Errorf("repo error")
}

func (m *failingMessageRepo) GetSinceLastCompaction(ctx context.Context, conversationID string) ([]models.Message, error) {
	return nil, fmt.Errorf("repo error")
}

func (m *failingMessageRepo) GetLastCompaction(ctx context.Context, conversationID string) (*models.Message, error) {
	return nil, nil
}
func (m *failingMessageRepo) GetUnvalidated(ctx context.Context, limit int) ([]models.Message, error) {
	return nil, fmt.Errorf("repo error")
}
func (m *failingMessageRepo) MarkAsValidated(ctx context.Context, ids []string) error {
	return fmt.Errorf("repo error")
}

func TestService_Compact_Error(t *testing.T) {
	service := NewService(&failingMessageRepo{}, &testAIProviderForCompaction{}, 10000, 0.8)

	_, err := service.Compact(context.Background(), "conv1")

	assert.Error(t, err)
}

func TestService_ShouldCompact_Error(t *testing.T) {
	service := NewService(&failingMessageRepo{}, &testAIProviderForCompaction{}, 10000, 0.8)

	_, err := service.ShouldCompact(context.Background(), "conv1")

	assert.Error(t, err)
}
