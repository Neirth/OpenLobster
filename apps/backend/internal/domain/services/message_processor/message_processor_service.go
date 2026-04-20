package message_processor

import (
	"context"
	"strings"

	ctxutil "github.com/neirth/openlobster/internal/domain/context"
	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// MessageProcessorService processes incoming messages.
type MessageProcessorService struct {
	aiProvider  ports.AIProviderPort
	messageRepo ports.MessageRepositoryPort
	sessionRepo ports.SessionRepositoryPort
	eventBus    EventBus
}

// NewMessageProcessorService creates a MessageProcessorService.
func NewMessageProcessorService(
	aiProvider ports.AIProviderPort,
	messageRepo ports.MessageRepositoryPort,
	sessionRepo ports.SessionRepositoryPort,
	eventBus EventBus,
) *MessageProcessorService {
	return &MessageProcessorService{
		aiProvider:  aiProvider,
		messageRepo: messageRepo,
		sessionRepo: sessionRepo,
		eventBus:    eventBus,
	}
}

// Process processes an incoming message.
func (s *MessageProcessorService) Process(ctx context.Context, msg *models.Message) error {
	if msg.Content == "" && msg.Audio == nil {
		return nil
	}

	msg.Role = "user"
	if err := s.messageRepo.Save(ctx, msg); err != nil {
		return err
	}

	s.eventBus.Publish(context.Background(), events.NewEvent(events.EventMessageReceived, events.MessageReceivedPayload{
		MessageID: msg.ID.String(),
		ChannelID: msg.ChannelID,
		Content:   msg.Content,
		Timestamp: msg.Timestamp,
	}))

	return nil
}

// Validate validates a message.
func (s *MessageProcessorService) Validate(msg *models.Message) error {
	if msg.ChannelID == "" {
		return ErrEmptyChannel
	}
	return nil
}

var ErrEmptyChannel = &ValidationError{Field: "channel_id", Message: "channel cannot be empty"}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// EventBus is the interface for publishing and subscribing to events.
type EventBus interface {
	Publish(ctx context.Context, event events.Event) error
	Subscribe(eventType string, handler EventHandler) error
}

// EventHandler is a function that handles events.
type EventHandler func(ctx context.Context, event events.Event) error

// DefaultEventBus implements EventBus.
type DefaultEventBus struct {
	handlers map[string][]EventHandler
}

// NewEventBus creates a new DefaultEventBus.
func NewEventBus() *DefaultEventBus {
	return &DefaultEventBus{
		handlers: make(map[string][]EventHandler),
	}
}

// Publish publishes an event to all subscribers.
func (b *DefaultEventBus) Publish(ctx context.Context, event events.Event) error {
	handlers, ok := b.handlers[event.GetType()]
	if !ok {
		return nil
	}
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe subscribes to an event type.
func (b *DefaultEventBus) Subscribe(eventType string, handler EventHandler) error {
	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}

// PromptBuilderService builds prompts for the LLM.
type PromptBuilderService struct {
	systemPrompt    string
	aiProvider      ports.AIProviderPort
	contextInjector ctxutil.ContextInjector
}

// NewPromptBuilderService creates a PromptBuilderService.
func NewPromptBuilderService(systemPrompt string, aiProvider ports.AIProviderPort) *PromptBuilderService {
	return &PromptBuilderService{
		systemPrompt: systemPrompt,
		aiProvider:   aiProvider,
	}
}

// NewPromptBuilderServiceWithContext creates a PromptBuilderService with context injection.
func NewPromptBuilderServiceWithContext(systemPrompt string, aiProvider ports.AIProviderPort, injector ctxutil.ContextInjector) *PromptBuilderService {
	return &PromptBuilderService{
		systemPrompt:    systemPrompt,
		aiProvider:      aiProvider,
		contextInjector: injector,
	}
}

// Build builds the messages for an LLM call.
func (s *PromptBuilderService) Build(ctx context.Context, session *models.Session, userContext string, tools []ports.Tool) ([]ports.ChatMessage, error) {
	messages := make([]ports.ChatMessage, 0)

	if s.systemPrompt != "" {
		messages = append(messages, ports.ChatMessage{
			Role:    "system",
			Content: s.systemPrompt,
		})
	}

	if userContext != "" {
		messages = append(messages, ports.ChatMessage{
			Role:    "system",
			Content: "User context:\n" + userContext,
		})
	}

	if session != nil {
		for _, msg := range session.Messages {
			messages = append(messages, ports.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return messages, nil
}

// BuildWithContext builds messages with injected context.
func (s *PromptBuilderService) BuildWithContext(ctx context.Context, session *models.Session, userID string, tools []ports.Tool) ([]ports.ChatMessage, error) {
	messages := make([]ports.ChatMessage, 0)

	if s.contextInjector != nil {
		agentCtx, err := s.contextInjector.BuildContext(ctx, userID, "")
		if err == nil && agentCtx != nil {
			if agentCtx.AgentsMD != "" {
				messages = append(messages, ports.ChatMessage{
					Role:    "system",
					Content: agentCtx.AgentsMD,
				})
			}
			if agentCtx.SoulMD != "" {
				messages = append(messages, ports.ChatMessage{
					Role:    "system",
					Content: agentCtx.SoulMD,
				})
			}
			if agentCtx.IdentityMD != "" {
				messages = append(messages, ports.ChatMessage{
					Role:    "system",
					Content: agentCtx.IdentityMD,
				})
			}
			if agentCtx.UserMemory != "" {
				messages = append(messages, ports.ChatMessage{
					Role:    "system",
					Content: agentCtx.UserMemory,
				})
			}
		}
	}

	if s.systemPrompt != "" {
		messages = append(messages, ports.ChatMessage{
			Role:    "system",
			Content: s.systemPrompt,
		})
	}

	if session != nil {
		for _, msg := range session.Messages {
			messages = append(messages, ports.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	return messages, nil
}

// EstimateTokens estimates token count for text.
func (s *PromptBuilderService) EstimateTokens(text string) int {
	return len(strings.Fields(text)) * 4 / 3
}

// ShouldCompact returns true if compaction is needed based on token usage.
func (s *PromptBuilderService) ShouldCompact(totalTokens, maxTokens int) bool {
	return float64(totalTokens)/float64(maxTokens) >= 0.9
}
