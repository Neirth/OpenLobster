// Copyright (c) OpenLobster contributors. See LICENSE for details.

package memory_consolidation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/infrastructure/logging"
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
	// noReply is the sentinel value the extraction agent emits when it finds
	// no facts that are new or differ from what is already in memory.
	noReply = "NO_REPLY"

	extractionSystemPrompt = `You are a memory extraction and deduplication agent. The user you are processing is "%s".
Steps you MUST follow for the conversation below:
1. Identify candidate persistent facts about "%s": preferences, habits, personal details, significant events.
   Skip greetings, questions, and transient content.
2. For each candidate fact, call 'search_memory' to check whether it already exists in long-term memory.
3. Output ONLY the facts that are genuinely new or meaningfully different from what is already stored.
   Format: one fact per line, starting with "- ", max 15 words each.
4. If every candidate fact already exists in memory, or if there are no candidate facts at all,
   respond with exactly: NO_REPLY`

	reductionSystemPrompt = `You are a memory filtering engine. The user you are processing is "%s".
You will receive candidate facts about "%s" extracted from recent conversations.
Use 'search_memory' to check each fact against the existing Knowledge Graph for "%s".
Output ONLY facts that are genuinely new or meaningfully different from what is already stored.
Format: one fact per line, starting with "- ", max 15 words each.
If every candidate fact already exists in memory, respond with exactly: NO_REPLY`

	syncSystemPrompt = `You are a memory synchronization specialist. The user you are updating memory for is "%s".
Update the long-term memory (Neo4j) using the provided tools based on the new findings about "%s".
- Use 'add_memory' for new facts about "%s". Choose entity_type carefully:
  - entity_type="person"       → a specific person (colleague, friend, family member, etc.)
  - entity_type="place"        → a location (city, country, address, neighbourhood, etc.)
  - entity_type="organization" → a company, school, team, institution, or club
  - entity_type="event"        → a time-bound occurrence (concert, trip, appointment, interview, etc.)
  - entity_type="thing"        → an object, hobby, topic, or abstract concept
  - entity_type="story"        → a personal narrative or diary-style entry
  - entity_type="fact"         → generic facts that do not fit any category above
- Use 'set_user_property' for core attributes of "%s" (name, age, language, etc.).
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
				anonMsg := anonymizeToken(fmt.Sprintf("%v", msg.ID))
				logging.Printf("memory_consolidation: failed to resolve user for message %s: %v", anonMsg, err)
				continue
			}
			userID = id.UserID
			convoMap[msg.ConversationID] = userID
		}
		userMsgs[userID] = append(userMsgs[userID], msg)
	}

	for userID, msgs := range userMsgs {
		if err := s.processUserBatch(ctx, userID, msgs); err != nil {
			logging.Printf("memory_consolidation: failed to process user %s: %v", userID, err)
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

	// Inject user identity into the context so memory tools (add_memory,
	// set_user_property, search_memory) always write to the correct user node
	// without requiring the model to pass for_user explicitly.
	ctx = context.WithValue(ctx, mcp.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, mcp.ContextKeyUserDisplayName, userName)

	// Filter to only conversational turns that have not yet been validated.
	// The DB query already excludes validated messages, but we guard here too
	// to avoid unnecessary API calls if the flag was set concurrently.
	// Tool outputs and system messages carry no user-attributable memory.
	var conversational []models.Message
	for _, m := range msgs {
		if m.IsValidated {
			continue
		}
		if m.Role == "user" || m.Role == "assistant" {
			conversational = append(conversational, m)
		}
	}

	// All messages already validated — nothing to do, skip API entirely.
	if len(conversational) == 0 {
		return nil
	}

	// 1. Extraction (Map) — one LLM call per chunk, no tools, extracts raw facts.
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

	// 2. Reduction — one LLM call with memory tools, compares summaries against
	// existing memory. Responds with NO_REPLY if nothing new to persist.
	findings, err := s.filterAgainstMemory(ctx, userName, summaries)
	if err != nil {
		return fmt.Errorf("reduction: %w", err)
	}

	if strings.TrimSpace(findings) == noReply || strings.TrimSpace(findings) == "" {
		return s.markAsValidated(ctx, msgs)
	}

	// 3. Synchronization — persist new facts to the graph.
	if err := s.syncFindings(ctx, userName, findings); err != nil {
		return fmt.Errorf("synchronization: %w", err)
	}

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
			{Role: "system", Content: fmt.Sprintf(extractionSystemPrompt, userName, userName)},
			{Role: "user", Content: fmt.Sprintf("Extract persistent facts about %s from the following conversation:\n\n%s", userName, sb.String())},
		},
		MaxTokens: maxOutputTokens,
	})
	if err != nil {
		logging.Printf("memory_consolidation: extraction failed for %s: %v", userName, err)
		return "", err
	}

	return resp.Content, nil
}

func (s *service) filterAgainstMemory(ctx context.Context, userName string, summaries []string) (string, error) {
	combinedSummaries := strings.Join(summaries, "\n---\n")
	tools := s.buildTools()

	s.logPhase("reduction", userName, combinedSummaries)
	resp, err := s.runAgenticLoop(ctx, []ports.ChatMessage{
		{Role: "system", Content: fmt.Sprintf(reductionSystemPrompt, userName, userName, userName)},
		{Role: "user", Content: fmt.Sprintf("User: %s\nNew candidate facts:\n%s", userName, combinedSummaries)},
	}, tools)
	if err != nil {
		logging.Printf("memory_consolidation: reduction failed for %s: %v", userName, err)
		return "", err
	}

	return resp.Content, nil
}

func (s *service) syncFindings(ctx context.Context, userName, findings string) error {
	tools := s.buildTools()

	s.logPhase("synchronization", userName, findings)
	messages := []ports.ChatMessage{
		{Role: "system", Content: fmt.Sprintf(syncSystemPrompt, userName, userName, userName, userName)},
		{Role: "user", Content: fmt.Sprintf("User: %s\nFinal findings to synchronize:\n%s", userName, findings)},
	}
	_, err := s.runAgenticLoop(ctx, messages, tools)
	if err != nil {
		logging.Printf("memory_consolidation: synchronization failed for %s: %v", userName, err)
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

func (s *service) logPhase(phase, ident, prompt string) {
	logging.Printf("memory_consolidation: [%s] user=%s estimated_prompt=%d tokens", phase, ident, len(prompt)/4)

	// By default only log a truncated prompt snippet to reduce PII exposure.
	// Full prompt logging can be enabled via env var OPENLOBSTER_MEMORY_VERBOSE=1
	verbose := os.Getenv("OPENLOBSTER_MEMORY_VERBOSE") == "1"
	if verbose {
		logging.Debugf("memory_consolidation: [%s] full prompt:\n%s", phase, prompt)
		return
	}

	// Truncate the prompt snippet for safe debugging
	maxSnippet := 400
	snippet := prompt
	if len(snippet) > maxSnippet {
		snippet = snippet[:maxSnippet] + "..."
	}
	logging.Debugf("memory_consolidation: [%s] prompt_snippet:\n%s", phase, snippet)
}

func anonymizeToken(s string) string {
	if s == "" {
		return ""
	}
	h := sha256.Sum256([]byte(s))
	hexs := hex.EncodeToString(h[:])
	if len(hexs) > 12 {
		return hexs[:12]
	}
	return hexs
}

func (s *service) markAsValidated(ctx context.Context, msgs []models.Message) error {
	var ids []string
	for _, m := range msgs {
		ids = append(ids, m.ID.String())
	}
	return s.msgRepo.MarkAsValidated(ctx, ids)
}
