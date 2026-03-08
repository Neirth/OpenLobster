package message_processor

import (
	"context"
	"fmt"
	"testing"

	ctxutil "github.com/neirth/openlobster/internal/domain/context"
	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/stretchr/testify/assert"
)

func TestNewMessageProcessorService(t *testing.T) {
	svc := NewMessageProcessorService(nil, nil, nil, NewEventBus())
	assert.NotNil(t, svc)
}

func TestMessageProcessorService_Validate_EmptyChannel(t *testing.T) {
	svc := NewMessageProcessorService(nil, nil, nil, nil)
	err := svc.Validate(&models.Message{ChannelID: ""})
	assert.Error(t, err)
	assert.Equal(t, "channel_id: channel cannot be empty", err.Error())
}

func TestMessageProcessorService_Validate_Ok(t *testing.T) {
	svc := NewMessageProcessorService(nil, nil, nil, nil)
	err := svc.Validate(&models.Message{ChannelID: "telegram"})
	assert.NoError(t, err)
}

func TestMessageProcessorService_Process_EmptyContentSkips(t *testing.T) {
	svc := NewMessageProcessorService(nil, nil, nil, NewEventBus())
	err := svc.Process(context.Background(), &models.Message{ChannelID: "ch", Content: "", Audio: nil})
	assert.NoError(t, err)
}

func TestMessageProcessorService_Process_WithAudio(t *testing.T) {
	mockRepo := &mockMessageRepo{}
	svc := NewMessageProcessorService(nil, mockRepo, nil, NewEventBus())

	msg := &models.Message{
		ChannelID:      "ch",
		Content:        "",
		ConversationID: "conv-1",
		Audio:          &models.AudioContent{Data: []byte{1, 2}, Format: "ogg"},
	}
	msg.ID = models.NewMessage("ch", "x").ID
	err := svc.Process(context.Background(), msg)
	assert.NoError(t, err)
	assert.True(t, mockRepo.saved)
}

func TestMessageProcessorService_Process_SaveError(t *testing.T) {
	mockRepo := &mockMessageRepoErr{err: fmt.Errorf("db full")}
	svc := NewMessageProcessorService(nil, mockRepo, nil, NewEventBus())

	msg := models.NewMessage("ch", "hello")
	msg.ConversationID = "conv-1"
	err := svc.Process(context.Background(), msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db full")
}

type mockMessageRepoErr struct {
	err error
}

func (m *mockMessageRepoErr) Save(ctx context.Context, msg *models.Message) error {
	return m.err
}
func (m *mockMessageRepoErr) GetByConversation(ctx context.Context, id string, limit int) ([]models.Message, error) {
	return nil, nil
}
func (m *mockMessageRepoErr) GetSinceLastCompaction(ctx context.Context, id string) ([]models.Message, error) {
	return nil, nil
}
func (m *mockMessageRepoErr) GetLastCompaction(ctx context.Context, id string) (*models.Message, error) {
	return nil, nil
}

func TestMessageProcessorService_Process_WithContent(t *testing.T) {
	mockRepo := &mockMessageRepo{}
	bus := NewEventBus()
	svc := NewMessageProcessorService(nil, mockRepo, nil, bus)

	msg := models.NewMessage("ch", "hello")
	msg.ConversationID = "conv-1"
	err := svc.Process(context.Background(), msg)
	assert.NoError(t, err)
	assert.True(t, mockRepo.saved)
	assert.Equal(t, "user", msg.Role)
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "x", Message: "invalid"}
	assert.Equal(t, "x: invalid", err.Error())
}

type mockMessageRepo struct {
	saved bool
}

func (m *mockMessageRepo) Save(ctx context.Context, msg *models.Message) error {
	m.saved = true
	return nil
}
func (m *mockMessageRepo) GetByConversation(ctx context.Context, id string, limit int) ([]models.Message, error) {
	return nil, nil
}
func (m *mockMessageRepo) GetSinceLastCompaction(ctx context.Context, id string) ([]models.Message, error) {
	return nil, nil
}
func (m *mockMessageRepo) GetLastCompaction(ctx context.Context, id string) (*models.Message, error) {
	return nil, nil
}

func TestNewEventBus(t *testing.T) {
	bus := NewEventBus()
	assert.NotNil(t, bus)
}

func TestEventBus_Publish_NoSubscribers(t *testing.T) {
	bus := NewEventBus()

	err := bus.Publish(context.Background(), events.NewEvent("test", nil))
	assert.NoError(t, err)
}

func TestEventBus_Subscribe(t *testing.T) {
	bus := NewEventBus()

	handler := func(ctx context.Context, e events.Event) error {
		return nil
	}

	err := bus.Subscribe("test_event", handler)
	assert.NoError(t, err)
}

func TestEventBus_Publish_WithHandler(t *testing.T) {
	bus := NewEventBus()

	handlerCalled := false
	handler := func(ctx context.Context, e events.Event) error {
		handlerCalled = true
		return nil
	}

	bus.Subscribe("test_event", handler)
	err := bus.Publish(context.Background(), events.NewEvent("test_event", nil))

	assert.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestEventBus_Publish_WithHandlerError(t *testing.T) {
	bus := NewEventBus()

	handler := func(ctx context.Context, e events.Event) error {
		return fmt.Errorf("handler error")
	}

	bus.Subscribe("test_event", handler)
	err := bus.Publish(context.Background(), events.NewEvent("test_event", nil))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler error")
}

func TestNewPromptBuilderService(t *testing.T) {
	builder := NewPromptBuilderService("You are helpful", nil)
	assert.NotNil(t, builder)
	assert.Equal(t, "You are helpful", builder.systemPrompt)
}

func TestPromptBuilderService_Build(t *testing.T) {
	builder := NewPromptBuilderService(" System prompt", nil)

	messages, err := builder.Build(context.Background(), nil, "User context", nil)

	assert.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, " System prompt", messages[0].Content)
}

func TestPromptBuilderService_Build_Empty(t *testing.T) {
	builder := NewPromptBuilderService("", nil)

	messages, err := builder.Build(context.Background(), nil, "", nil)

	assert.NoError(t, err)
	assert.Len(t, messages, 0)
}

func TestPromptBuilderService_Build_WithSession(t *testing.T) {
	builder := NewPromptBuilderService("sys", nil)
	session := &models.Session{
		Messages: []models.Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello!"},
		},
	}

	messages, err := builder.Build(context.Background(), session, "", nil)

	assert.NoError(t, err)
	assert.Len(t, messages, 3) // sys + user + assistant
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "Hi", messages[1].Content)
	assert.Equal(t, "assistant", messages[2].Role)
}

func TestPromptBuilderService_Build_WithUserContext(t *testing.T) {
	builder := NewPromptBuilderService("", nil)

	messages, err := builder.Build(context.Background(), nil, "User likes pizza", nil)

	assert.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.Equal(t, "system", messages[0].Role)
	assert.Contains(t, messages[0].Content, "User likes pizza")
}

func TestNewPromptBuilderServiceWithContext(t *testing.T) {
	injector := ctxutil.NewContextInjector("", "", "", "", "", nil, nil)
	builder := NewPromptBuilderServiceWithContext("sys", nil, injector)
	assert.NotNil(t, builder)
}

func TestPromptBuilderService_BuildWithContext(t *testing.T) {
	injector := ctxutil.NewContextInjector("", "", "", "", "", nil, nil)
	builder := NewPromptBuilderServiceWithContext("sys", nil, injector)

	messages, err := builder.BuildWithContext(context.Background(), nil, "", nil)

	assert.NoError(t, err)
	assert.NotNil(t, messages)
	// With empty paths, injector returns empty agentCtx fields
	assert.GreaterOrEqual(t, len(messages), 1)
	assert.Equal(t, "sys", messages[len(messages)-1].Content)
}

func TestPromptBuilderService_EstimateTokens(t *testing.T) {
	builder := NewPromptBuilderService("", nil)

	tokens := builder.EstimateTokens("Hello world this is a test")

	assert.Greater(t, tokens, 0)
}

func TestPromptBuilderService_ShouldCompact(t *testing.T) {
	builder := NewPromptBuilderService("", nil)

	assert.True(t, builder.ShouldCompact(9000, 10000))
	assert.False(t, builder.ShouldCompact(8000, 10000))
	assert.False(t, builder.ShouldCompact(5000, 10000))
}

