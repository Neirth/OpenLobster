// Copyright (c) OpenLobster contributors. See LICENSE for details.

package message_compaction

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// Service summarizes conversation history when it approaches the model's token limit.
type Service struct {
	repo           ports.MessageRepositoryPort
	aiProvider     ports.AIProviderPort
	ThresholdRatio float64
}

// NewService creates a MessageCompactionService with 85% threshold.
func NewService(repo ports.MessageRepositoryPort, aiProvider ports.AIProviderPort) *Service {
	return &Service{
		repo:           repo,
		aiProvider:     aiProvider,
		ThresholdRatio: 0.85,
	}
}

// SetAIProvider updates the AI provider (used after config soft-reboot).
func (c *Service) SetAIProvider(p ports.AIProviderPort) {
	c.aiProvider = p
}

// MinUserMessagesForCompaction: no compactar hasta tener conversación real.
const MinUserMessagesForCompaction = 2

// ShouldCompact returns true when the CONVERSATION (user/assistant/tool) token usage
// exceeds the threshold. System prompts are excluded — they are not compactable and
// must not inflate the decision.
func (c *Service) ShouldCompact(messages []ports.ChatMessage, modelMaxTokens int) bool {
	userCount := 0
	for _, m := range messages {
		if m.Role == "user" {
			userCount++
		}
	}
	if userCount < MinUserMessagesForCompaction {
		return false
	}
	// Solo contar tokens de la conversación (user, assistant, tool), no system.
	conversationTokens := estimateConversationTokens(messages)
	threshold := int(float64(modelMaxTokens) * c.ThresholdRatio)
	return conversationTokens >= threshold
}

// Compact summarises the conversation and stores the result as a compaction message.
func (c *Service) Compact(ctx context.Context, conversationID string) (*models.Message, error) {
	messages, err := c.repo.GetSinceLastCompaction(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to compact")
	}

	history := make([]ports.ChatMessage, 0, len(messages)+1)
	history = append(history, ports.ChatMessage{
		Role:    "system",
		Content: "Summarise the following conversation concisely, preserving all key facts, decisions and context.",
	})
	for _, m := range messages {
		history = append(history, ports.ChatMessage{Role: m.Role, Content: m.Content})
	}

	resp, err := c.aiProvider.Chat(ctx, ports.ChatRequest{Messages: history})
	if err != nil {
		return nil, fmt.Errorf("compaction summarisation failed: %w", err)
	}

	trimmed := resp.Content
	if len(trimmed) == 0 || isWhitespace(trimmed) {
		return nil, fmt.Errorf("compaction summarisation returned empty content")
	}

	compactionMsg := &models.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		Role:           "compaction",
		Content:        resp.Content,
		Timestamp:      time.Now(),
		Metadata:       make(map[string]interface{}),
	}

	if err := c.repo.Save(ctx, compactionMsg); err != nil {
		return nil, fmt.Errorf("failed to save compaction message: %w", err)
	}

	return compactionMsg, nil
}

// BuildMessages returns the slice of ChatMessages for the next LLM call.
func (c *Service) BuildMessages(ctx context.Context, conversationID string, systemPrompt string) ([]ports.ChatMessage, error) {
	messages := make([]ports.ChatMessage, 0)

	if systemPrompt != "" {
		messages = append(messages, ports.ChatMessage{Role: "system", Content: systemPrompt})
	}

	lastCompaction, err := c.repo.GetLastCompaction(ctx, conversationID)
	if err != nil {
		if !errors.Is(err, ports.ErrNotFound) {
			return nil, fmt.Errorf("GetLastCompaction: %w", err)
		}
	} else if lastCompaction != nil {
		messages = append(messages, ports.ChatMessage{
			Role:    "system",
			Content: "[Previous conversation summary]\n" + lastCompaction.Content,
		})
	}

	history, err := c.repo.GetSinceLastCompaction(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("GetSinceLastCompaction: %w", err)
	}

	for _, m := range history {
		if m.Role == "compaction" {
			continue
		}
		messages = append(messages, ports.ChatMessage{Role: m.Role, Content: m.Content})
	}

	return messages, nil
}

func isWhitespace(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

// estimateConversationTokens counts tokens only for user/assistant/tool messages.
// System prompts are excluded — they are not compactable and must not inflate the decision.
func estimateConversationTokens(messages []ports.ChatMessage) int {
	total := 0
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		total += utf8.RuneCountInString(m.Content)/4 + 4
	}
	return total
}
