// Copyright (c) OpenLobster contributors. See LICENSE for details.

package memory_consolidation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	toon "github.com/toon-format/toon-go"

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
You are a memory extraction sub-agent for user "%s".
Extract only persistent facts from the messages below. Output one fact per line, starting with "- ".
Each line must be a single short sentence (max 15 words). No explanations, no summaries, no filler.
Only include: preferences, habits, personal details, significant events. Skip greetings, questions, and transient content.

Messages:
%s
`

	reductionPrompt = `
You are a memory filtering engine. You will receive several fact summaries and the current state of the Knowledge Graph for user "%s".
Your task is to produce a **final condensed text** containing ONLY facts that are NEW or that UPDATE existing information about this user.
Discard anything already present and unchanged in the Knowledge Graph.

Current Knowledge Graph:
%s

New Fact Summaries:
%s

Provide the final findings in a clear, bulleted text format.
`

	syncPrompt = `
You are a memory synchronization specialist. You will receive a list of new/updated findings about user "%s".
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

// chunkMessages splits msgs into variable-size chunks that fit within the model's
// token budget. Token count is estimated from character length (1 token ≈ 4 chars).
// Half the context window is reserved for the extraction response and prompt overhead.
func chunkMessages(msgs []models.Message, maxTokens int) [][]models.Message {
	const (
		charsPerToken  = 4
		budgetRatio    = 0.5 // reserve half the context for prompt overhead and response
		promptOverhead = 400 // estimated tokens for the extraction prompt template
		msgFormatChars = 60  // estimated chars for [id] label: prefix per message
	)

	tokenBudget := int(float64(maxTokens)*budgetRatio) - promptOverhead
	if tokenBudget <= 0 {
		tokenBudget = 512
	}

	var chunks [][]models.Message
	var current []models.Message
	currentTokens := 0

	for _, msg := range msgs {
		msgTokens := (len(msg.Content) + msgFormatChars) / charsPerToken
		if len(current) > 0 && currentTokens+msgTokens > tokenBudget {
			chunks = append(chunks, current)
			current = nil
			currentTokens = 0
		}
		current = append(current, msg)
		currentTokens += msgTokens
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

func (s *service) processUserBatch(ctx context.Context, userID string, msgs []models.Message) error {
	// Resolve user display name for attribution in extraction prompts.
	userName := userID
	if user, err := s.userRepo.GetByID(ctx, userID); err == nil && user.Name != "" {
		userName = user.Name
	}

	// Filter to only conversational turns — tool outputs and system messages
	// carry no user-attributable memory and would inflate context for no gain.
	var conversational []models.Message
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "assistant" {
			conversational = append(conversational, m)
		}
	}

	// 1. Extraction (Map) - adaptive chunks sized to the model's context window
	var summaries []string
	for _, chunk := range chunkMessages(conversational, s.aiProvider.GetMaxTokens()) {
		summary, err := s.extractSummary(ctx, chunk, userName)
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
	finalFindings, err := s.filterAgainstMemory(ctx, userID, userName, summaries)
	if err != nil {
		return fmt.Errorf("reduction: %w", err)
	}

	if strings.TrimSpace(finalFindings) == "" {
		return s.markAsValidated(ctx, msgs)
	}

	// 3. Synchronization (Final Sync) - Save findings to Neo4j
	if err := s.syncFindings(ctx, userID, userName, finalFindings); err != nil {
		return fmt.Errorf("synchronization: %w", err)
	}

	// Cleanup
	return s.markAsValidated(ctx, msgs)
}

func (s *service) extractSummary(ctx context.Context, msgs []models.Message, userName string) (string, error) {
	var sb strings.Builder
	for _, m := range msgs {
		label := m.Role
		if m.Role == "user" {
			if m.SenderName != "" {
				label = m.SenderName
			} else {
				label = userName
			}
		}
		fmt.Fprintf(&sb, "[%s] %s: %s\n", m.ID, label, m.Content)
	}

	prompt := fmt.Sprintf(extractionPrompt, userName, sb.String())

	// Cap output to ~20 tokens per message in the chunk (one short fact each).
	maxOutputTokens := len(msgs) * 20

	resp, err := s.aiProvider.Chat(ctx, ports.ChatRequest{
		Messages:  []ports.ChatMessage{{Role: "system", Content: prompt}},
		MaxTokens: maxOutputTokens,
	})
	if err != nil {
		return "", err
	}

	s.logPhase("extraction", userName, resp.Usage)
	return resp.Content, nil
}

func (s *service) filterAgainstMemory(ctx context.Context, userID, userName string, summaries []string) (string, error) {
	graph, err := s.memoryRepo.GetUserGraph(ctx, userID)
	if err != nil {
		return "", err
	}

	type toonNode struct {
		ID    string `toon:"id"`
		Label string `toon:"label"`
		Type  string `toon:"type"`
		Value string `toon:"value"`
	}
	type toonEdge struct {
		Source string `toon:"source"`
		Target string `toon:"target"`
		Label  string `toon:"label"`
	}
	type toonGraph struct {
		Nodes []toonNode `toon:"nodes"`
		Edges []toonEdge `toon:"edges"`
	}

	nodes := make([]toonNode, len(graph.Nodes))
	for i, n := range graph.Nodes {
		nodes[i] = toonNode{ID: n.ID, Label: n.Label, Type: n.Type, Value: n.Value}
	}
	edges := make([]toonEdge, len(graph.Edges))
	for i, e := range graph.Edges {
		edges[i] = toonEdge{Source: e.Source, Target: e.Target, Label: e.Label}
	}
	graphTOON, err := toon.MarshalString(toonGraph{Nodes: nodes, Edges: edges})
	if err != nil {
		return "", fmt.Errorf("toon: %w", err)
	}

	combinedSummaries := strings.Join(summaries, "\n---\n")
	prompt := fmt.Sprintf(reductionPrompt, userName, graphTOON, combinedSummaries)

	resp, err := s.aiProvider.Chat(ctx, ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "system", Content: prompt}},
	})
	if err != nil {
		return "", err
	}

	s.logPhase("reduction", userName, resp.Usage)
	return resp.Content, nil
}

func (s *service) syncFindings(ctx context.Context, userID, userName, findings string) error {
	// 1. Define memory tools for the LLM
	tools := []ports.Tool{
		{
			Type: "function",
			Function: &ports.FunctionTool{
				Name:        "add_memory",
				Description: "Add a new fact or knowledge about the user to long-term memory.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content": map[string]any{
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
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key": map[string]any{
							"type":        "string",
							"description": "The property name (e.g., 'name', 'language').",
						},
						"value": map[string]any{
							"type":        "string",
							"description": "The property value.",
						},
					},
					"required": []string{"key", "value"},
				},
			},
		},
	}

	prompt := fmt.Sprintf(syncPrompt, userName, findings)

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

	s.logPhase("synchronization", userID, resp.Usage)

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

func (s *service) logPhase(phase, user string, usage ports.TokenUsage) {
	maxTokens := s.aiProvider.GetMaxTokens()
	pct := 0.0
	if maxTokens > 0 {
		pct = float64(usage.Total()) / float64(maxTokens) * 100
	}
	log.Printf("memory_consolidation: [%s] user=%s prompt=%d completion=%d total=%d (%.1f%% of %d)",
		phase, user, usage.PromptTokens, usage.CompletionTokens, usage.Total(), pct, maxTokens)
}

func (s *service) markAsValidated(ctx context.Context, msgs []models.Message) error {
	var ids []string
	for _, m := range msgs {
		ids = append(ids, m.ID.String())
	}
	return s.msgRepo.MarkAsValidated(ctx, ids)
}
