// Copyright (c) OpenLobster contributors. See LICENSE for details.

package compaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// Service (domain) provides the older ShouldCompact(ctx, conversationID) API.
// MessageHandler uses MessageCompactionService (message_compaction) which has a
// different API (ShouldCompact(messages, modelMaxTokens), BuildMessages).
// This domain version is kept for tests and backwards compatibility.
type Service struct {
	messageRepo ports.MessageRepositoryPort
	aiProvider  ports.AIProviderPort
	maxTokens   int
	threshold   float64
}

// NewService creates a domain CompactionService.
func NewService(
	messageRepo ports.MessageRepositoryPort,
	aiProvider ports.AIProviderPort,
	maxTokens int,
	threshold float64,
) *Service {
	return &Service{
		messageRepo: messageRepo,
		aiProvider:  aiProvider,
		maxTokens:   maxTokens,
		threshold:   threshold,
	}
}

// ShouldCompact returns true when token usage exceeds the threshold.
func (s *Service) ShouldCompact(ctx context.Context, conversationID string) (bool, error) {
	messages, err := s.messageRepo.GetByConversation(ctx, conversationID, 0)
	if err != nil {
		return false, err
	}

	totalTokens := estimateTokens(messages)
	return float64(totalTokens)/float64(s.maxTokens) >= s.threshold, nil
}

// Compact summarises the conversation history via the AI provider.
func (s *Service) Compact(ctx context.Context, conversationID string) (string, error) {
	messages, err := s.messageRepo.GetSinceLastCompaction(ctx, conversationID)
	if err != nil {
		return "", err
	}

	content := messagesToContent(messages)

	req := ports.ChatRequest{
		Model: "compact",
		Messages: []ports.ChatMessage{
			{Role: "system", Content: "Summarize the following conversation without omitting relevant details. This summary will replace the conversation history as context."},
			{Role: "user", Content: content},
		},
	}

	resp, err := s.aiProvider.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// MessagesToContent is exported for tests.
func MessagesToContent(messages []models.Message) string {
	return messagesToContent(messages)
}

func messagesToContent(messages []models.Message) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}
	return sb.String()
}

// EstimateTokens is exported for tests.
func EstimateTokens(messages []models.Message) int {
	return estimateTokens(messages)
}

func estimateTokens(messages []models.Message) int {
	var total int
	for _, m := range messages {
		total += strings.Count(m.Content, " ") * 2
	}
	return total
}
