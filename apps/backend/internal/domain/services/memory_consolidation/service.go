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
	"github.com/neirth/openlobster/internal/domain/services/mcp"
)

type service struct {
	msgRepo      ports.MessageRepositoryPort
	memoryRepo   ports.MemoryPort
	aiProvider   ports.AIProviderPort
	userRepo     ports.UserRepositoryPort
	convoRepo    ports.SessionRepositoryPort
	toolRegistry *mcp.ToolRegistry
}

const (
	extractionSystemPrompt = `You are a memory extraction sub-agent for user "%s".
Extract only persistent facts from the provided messages. Output one fact per line, starting with "- ".
Each line must be a single short sentence (max 15 words). No explanations, no summaries, no filler.
Only include: preferences, habits, personal details, significant events. Skip greetings, questions, and transient content.`

	reductionSystemPrompt = `You are a memory filtering engine for user "%s". 
You will receive fact summaries and the current state of the Knowledge Graph.
Your task is to produce a **final condensed text** containing ONLY facts that are NEW or that UPDATE existing information.
Discard anything already present and unchanged in the Knowledge Graph.
Use tools if you need to verify specific facts against the long-term memory.`

	syncSystemPrompt = `You are a memory synchronization specialist for user "%s".
Your task is to update the long-term memory (Neo4j) using the provided tools based on the new findings.
- Use 'add_memory' for new facts.
- Use 'set_user_property' for core user attributes (name, age, language, etc.).
- Be precise and avoid duplicating information.`
)

// NewService creates a new memory consolidation service.
func NewService(
	msgRepo ports.MessageRepositoryPort,
	memoryRepo ports.MemoryPort,
	aiProvider ports.AIProviderPort,
	userRepo ports.UserRepositoryPort,
	convoRepo ports.SessionRepositoryPort,
	toolRegistry *mcp.ToolRegistry,
) ports.MemoryConsolidationPort {
	return &service{
		msgRepo:      msgRepo,
		memoryRepo:   memoryRepo,
		aiProvider:   aiProvider,
		userRepo:     userRepo,
		convoRepo:    convoRepo,
		toolRegistry: toolRegistry,
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
	for _, chunk := range chunkMessages(conversational, s.aiProvider.GetContextWindow()) {
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

	// Cap output to ~20 tokens per message in the chunk (one short fact each).
	maxOutputTokens := len(msgs) * 20

	s.logPhase("extraction", userName, sb.String())
	resp, err := s.aiProvider.Chat(ctx, ports.ChatRequest{
		Messages: []ports.ChatMessage{
			{Role: "system", Content: fmt.Sprintf(extractionSystemPrompt, userName)},
			{Role: "user", Content: "Messages to process:\n" + sb.String()},
		},
		MaxTokens: maxOutputTokens,
	})
	if err != nil {
		log.Printf("memory_consolidation: extraction failed for %s: %v", userName, err)
		return "", err
	}

	return resp.Content, nil
}

func (s *service) filterAgainstMemory(ctx context.Context, userID, userName string, summaries []string) (string, error) {
	combinedSummaries := strings.Join(summaries, "\n---\n")
	// In the reduction phase, we allow the model to use memory tools to verify if facts already exist.
	tools := s.buildTools()

	s.logPhase("reduction", userName, combinedSummaries)
	messages := []ports.ChatMessage{
		{Role: "system", Content: fmt.Sprintf(reductionSystemPrompt, userName)},
		{Role: "user", Content: fmt.Sprintf("Current Knowledge Graph Context: %s\n\nNew Findings to Evaluate:\n%s", "[KNOWLEDGE GRAPH GATED BEHIND TOOLS]", combinedSummaries)},
	}
	resp, err := s.runAgenticLoop(ctx, messages, tools)
	if err != nil {
		log.Printf("memory_consolidation: reduction failed for %s: %v", userName, err)
		return "", err
	}

	return resp.Content, nil
}

func (s *service) syncFindings(ctx context.Context, userID, userName, findings string) error {
	tools := s.buildTools()

	s.logPhase("synchronization", userID, findings)
	messages := []ports.ChatMessage{
		{Role: "system", Content: fmt.Sprintf(syncSystemPrompt, userName)},
		{Role: "user", Content: "Final findings to synchronize:\n" + findings},
	}
	_, err := s.runAgenticLoop(ctx, messages, tools)
	if err != nil {
		log.Printf("memory_consolidation: synchronization failed for %s: %v", userID, err)
		return err
	}

	return nil
}

func (s *service) buildTools() []ports.Tool {
	if s.toolRegistry == nil {
		return nil
	}
	// Fetch all tools with "memory" capability
	defs := s.toolRegistry.GetToolsByCapability("memory")
	tools := make([]ports.Tool, 0, len(defs))
	for _, def := range defs {
		var params map[string]interface{}
		if len(def.InputSchema) > 0 {
			_ = json.Unmarshal(def.InputSchema, &params)
		}
		tools = append(tools, ports.Tool{
			Type: "function",
			Function: &ports.FunctionTool{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  params,
			},
		})
	}
	return tools
}

func (s *service) runAgenticLoop(ctx context.Context, messages []ports.ChatMessage, tools []ports.Tool) (*ports.ChatResponse, error) {
	const maxRounds = 5
	req := ports.ChatRequest{Messages: messages, Tools: tools}

	var lastResp ports.ChatResponse
	for round := 0; round < maxRounds; round++ {
		resp, err := s.aiProvider.Chat(ctx, req)
		if err != nil {
			return nil, err
		}
		lastResp = resp

		if resp.StopReason != "tool_use" || len(resp.ToolCalls) == 0 {
			// If we have content, we are done. If not, and we executed tools,
			// we might need one more pass to synthesize.
			if strings.TrimSpace(resp.Content) != "" || round == 0 {
				return &resp, nil
			}
			break
		}

		req.Messages = append(req.Messages, ports.ChatMessage{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			result, err := s.executeTool(ctx, tc)
			content := ""
			if err != nil {
				content = fmt.Sprintf(`{"error":"%v"}`, err)
			} else {
				content = string(result)
			}
			req.Messages = append(req.Messages, ports.ChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
				Content:    content,
			})
		}
	}

	// Synthesis pass if the last response was a tool call with no content.
	if len(lastResp.ToolCalls) > 0 && strings.TrimSpace(lastResp.Content) == "" {
		synthResp, err := s.aiProvider.Chat(ctx, ports.ChatRequest{Messages: req.Messages})
		if err != nil {
			return nil, err
		}
		return &synthResp, nil
	}

	return &lastResp, nil
}

func (s *service) executeTool(ctx context.Context, tc ports.ToolCall) (json.RawMessage, error) {
	if s.toolRegistry == nil {
		return nil, fmt.Errorf("no tool registry")
	}
	// Inject userID in context for permission checks if needed,
	// though memory consolidation usually runs with master/loopback privileges.
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err != nil {
		return nil, err
	}
	return s.toolRegistry.Dispatch(ctx, tc.Function.Name, params)
}

func (s *service) logPhase(phase, user string, prompt string) {
	const charsPerToken = 4
	promptTokens := len(prompt) / charsPerToken
	contextWindow := s.aiProvider.GetContextWindow()

	pct := 0.0
	if contextWindow > 0 {
		pct = float64(promptTokens) / float64(contextWindow) * 100
	}

	log.Printf("memory_consolidation: [%s] user=%s estimated_prompt=%d context_window=%d (%.1f%% of context)",
		phase, user, promptTokens, contextWindow, pct)
}

func (s *service) markAsValidated(ctx context.Context, msgs []models.Message) error {
	var ids []string
	for _, m := range msgs {
		ids = append(ids, m.ID.String())
	}
	return s.msgRepo.MarkAsValidated(ctx, ids)
}
