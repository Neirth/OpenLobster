// Copyright (c) OpenLobster contributors. See LICENSE for details.

package memory_consolidation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

type service struct {
	msgRepo    ports.MessageRepositoryPort
	memoryRepo ports.MemoryPort
	aiProvider ports.AIProviderPort
	userRepo   ports.UserRepositoryPort
	convoRepo  ports.SessionRepositoryPort
}

const (
	extractionPrompt = `
You are a memory extraction sub-agent. Review the following conversation fragments and provide a **condensed text summary** of all relevant facts and knowledge found.
Focus on user preferences, history, habits, and significant events.
Be concise. Use a bulleted list format.
Exclude generic fluff or transient information.

Messages:
%s
`

	reductionPrompt = `
You are a memory filtering engine. You will receive several fact summaries and the current state of the User's Knowledge Graph.
Your task is to produce a **final condensed text** containing ONLY facts that are NEW or that UPDATE existing information.
Discard anything already present and unchanged in the Knowledge Graph.

Current Knowledge Graph:
%s

New Fact Summaries:
%s

Provide the final findings in a clear, bulleted text format.
`

	syncPrompt = `
You are a memory synchronization specialist. You will receive a list of new/updated findings about a user.
Your task is to update the long-term memory (Neo4j) using the provided tools.
- Use 'add_memory' for new facts.
- Use 'set_user_property' for core user attributes (name, age, language, etc.).
- Be precise and avoid duplicating information.

New Findings:
%s
`
)

// NewService creates a new memory consolidation service.
func NewService(
	msgRepo ports.MessageRepositoryPort,
	memoryRepo ports.MemoryPort,
	aiProvider ports.AIProviderPort,
	userRepo ports.UserRepositoryPort,
	convoRepo ports.SessionRepositoryPort,
) ports.MemoryConsolidationPort {
	return &service{
		msgRepo:    msgRepo,
		memoryRepo: memoryRepo,
		aiProvider: aiProvider,
		userRepo:   userRepo,
		convoRepo:  convoRepo,
	}
}

func (s *service) Consolidate(ctx context.Context) error {
	messages, err := s.msgRepo.GetUnvalidated(ctx, 500) // Batch of 500
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	// Group by user
	userMsgs := make(map[string][]models.Message)
	convoMap := make(map[string]string) // conversation_id -> user_id

	for _, msg := range messages {
		userID, ok := convoMap[msg.ConversationID]
		if !ok {
			id, err := s.convoRepo.GetByID(ctx, msg.ConversationID)
			if err != nil {
				log.Printf("memory_consolidation: failed to resolve user for message %s: %v", msg.ID, err)
				continue
			}
			userID = id.UserID
			convoMap[msg.ConversationID] = userID
		}
		userMsgs[userID] = append(userMsgs[userID], msg)
	}

	for userID, msgs := range userMsgs {
		if err := s.processUserBatch(ctx, userID, msgs); err != nil {
			log.Printf("memory_consolidation: failed to process user %s: %v", userID, err)
			continue
		}
	}

	return nil
}

func (s *service) processUserBatch(ctx context.Context, userID string, msgs []models.Message) error {
	// 1. Extraction (Map) - Fragments to Condensed Summaries
	var summaries []string
	const chunkSize = 15 // Increased chunk size for text-based mapping
	for i := 0; i < len(msgs); i += chunkSize {
		end := i + chunkSize
		if end > len(msgs) {
			end = len(msgs)
		}
		chunk := msgs[i:end]
		summary, err := s.extractSummary(ctx, chunk)
		if err != nil {
			return fmt.Errorf("extraction: %w", err)
		}
		if strings.TrimSpace(summary) != "" {
			summaries = append(summaries, summary)
		}
	}

	if len(summaries) == 0 {
		return s.markAsValidated(ctx, msgs)
	}

	// 2. Reduction (Filter) - Filter summaries against memory
	finalFindings, err := s.filterAgainstMemory(ctx, userID, summaries)
	if err != nil {
		return fmt.Errorf("reduction: %w", err)
	}

	if strings.TrimSpace(finalFindings) == "" {
		return s.markAsValidated(ctx, msgs)
	}

	// 3. Synchronization (Final Sync) - Save findings to Neo4j
	if err := s.syncFindings(ctx, userID, finalFindings); err != nil {
		return fmt.Errorf("synchronization: %w", err)
	}

	// Cleanup
	return s.markAsValidated(ctx, msgs)
}

func (s *service) extractSummary(ctx context.Context, msgs []models.Message) (string, error) {
	var sb strings.Builder
	for _, m := range msgs {
			fmt.Fprintf(&sb, "[%s] %s: %s\n", m.ID, m.Role, m.Content)
	}

	prompt := fmt.Sprintf(extractionPrompt, sb.String())

	resp, err := s.aiProvider.Chat(ctx, ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "system", Content: prompt}},
	})
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

func (s *service) filterAgainstMemory(ctx context.Context, userID string, summaries []string) (string, error) {
	graph, err := s.memoryRepo.GetUserGraph(ctx, userID)
	if err != nil {
		return "", err
	}
	
	// Convert graph to a readable plain text representation for the LLM
	var graphSummary strings.Builder
	for _, n := range graph.Nodes {
			fmt.Fprintf(&graphSummary, "- %s (%s): %v\n", n.ID, n.Label, n.Properties)
	}

	combinedSummaries := strings.Join(summaries, "\n---\n")
	prompt := fmt.Sprintf(reductionPrompt, graphSummary.String(), combinedSummaries)

	resp, err := s.aiProvider.Chat(ctx, ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "system", Content: prompt}},
	})
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

func (s *service) syncFindings(ctx context.Context, userID string, findings string) error {
	// 1. Define memory tools for the LLM
	tools := []ports.Tool{
		{
			Type: "function",
			Function: &ports.FunctionTool{
				Name:        "add_memory",
				Description: "Add a new fact or knowledge about the user to long-term memory.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"content": map[string]interface{}{
							"type":        "string",
							"description": "The fact or knowledge to save.",
						},
					},
					"required": []string{"content"},
				},
			},
		},
		{
			Type: "function",
			Function: &ports.FunctionTool{
				Name:        "set_user_property",
				Description: "Set a core attribute for the user (name, age, occupation, etc.).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"key": map[string]interface{}{
							"type":        "string",
							"description": "The property name (e.g., 'name', 'language').",
						},
						"value": map[string]interface{}{
							"type":        "string",
							"description": "The property value.",
						},
					},
					"required": []string{"key", "value"},
				},
			},
		},
	}

	prompt := fmt.Sprintf(syncPrompt, findings)

	// 2. Call LLM with the new findings and defined tools
	resp, err := s.aiProvider.Chat(ctx, ports.ChatRequest{
		Messages: []ports.ChatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: "Process these findings and use your tools to update the memory now."},
		},
		Tools: tools,
	})
	if err != nil {
		return err
	}

	// 3. Execute tool calls returned by the LLM
	for _, tc := range resp.ToolCalls {
		switch tc.Function.Name {
		case "add_memory":
			var args struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				log.Printf("sync: failed to parse add_memory args: %v", err)
				continue
			}
			if err := s.memoryRepo.AddKnowledge(ctx, userID, args.Content, "", "", "", nil); err != nil {
				log.Printf("sync: add_memory failed for user %s: %v", userID, err)
			}
		case "set_user_property":
			var args struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				log.Printf("sync: failed to parse set_user_property args: %v", err)
				continue
			}
			if err := s.memoryRepo.SetUserProperty(ctx, userID, args.Key, args.Value); err != nil {
				log.Printf("sync: set_user_property failed for user %s: %v", userID, err)
			}
		}
	}

	return nil
}

func (s *service) markAsValidated(ctx context.Context, msgs []models.Message) error {
	var ids []string
	for _, m := range msgs {
		ids = append(ids, m.ID.String())
	}
	return s.msgRepo.MarkAsValidated(ctx, ids)
}
