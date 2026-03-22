// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: see LICENSE

// Package ollama provides an AI provider adapter backed by the official Ollama Go SDK.
package ollama

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	ollamaapi "github.com/ollama/ollama/api"
	"golang.org/x/crypto/ssh"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/logging"
)

// chatClient is a narrow interface over the SDK client so that tests can inject
// a mock without spinning up an HTTP server. *ollamaapi.Client satisfies it.
type chatClient interface {
	Chat(ctx context.Context, req *ollamaapi.ChatRequest, fn ollamaapi.ChatResponseFunc) error
	Show(ctx context.Context, req *ollamaapi.ShowRequest) (*ollamaapi.ShowResponse, error)
}

var ensureKeyOnce sync.Once

// ensureOllamaPrivateKey creates ~/.ollama/id_ed25519 if missing so the SDK
// does not log "Failed to load private key" on every startup.
func ensureOllamaPrivateKey() {
	ensureKeyOnce.Do(func() {
		home := os.Getenv("HOME")
		if home == "" {
			if d, err := os.UserHomeDir(); err == nil && d != "" {
				home = d
			} else {
				home = "/app"
			}
		}
		keyPath := filepath.Join(home, ".ollama", "id_ed25519")
		if _, err := os.Stat(keyPath); err == nil {
			return
		}
		dir := filepath.Dir(keyPath)
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Printf("ollama: cannot create %s: %v", dir, err)
			return
		}
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			log.Printf("ollama: cannot generate key: %v", err)
			return
		}
		block, err := ssh.MarshalPrivateKey(priv, "")
		if err != nil {
			log.Printf("ollama: cannot marshal key: %v", err)
			return
		}
		if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0600); err != nil {
			log.Printf("ollama: cannot write %s: %v", keyPath, err)
			return
		}
		log.Printf("ollama: created private key at %s", keyPath)
	})
}

// Adapter implements ports.AIProviderPort using the official Ollama Go SDK.
type Adapter struct {
	client        chatClient
	initErr       error
	model         string
	maxTokens     int
	contextWindow int
}

// NewAdapter constructs an Adapter pointing at the given Ollama endpoint.
func NewAdapter(baseURL, model string, maxTokens int) *Adapter {
	return NewAdapterWithAuth(baseURL, "", model, maxTokens)
}

// NewAdapterWithAuth constructs an Adapter with an optional Bearer token.
func NewAdapterWithAuth(baseURL, apiKey, model string, maxTokens int) *Adapter {
	ensureOllamaPrivateKey()

	u, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("ollama: invalid endpoint %q, falling back to environment: %v", baseURL, err)
		c, envErr := ollamaapi.ClientFromEnvironment()
		if envErr != nil {
			log.Printf("ollama: ClientFromEnvironment failed: %v", envErr)
			return &Adapter{initErr: envErr, model: model, maxTokens: maxTokens}
		}
		a := &Adapter{client: c, model: model, maxTokens: maxTokens}
		a.probeContextWindow()
		return a
	}

	httpClient := http.DefaultClient
	if apiKey != "" {
		httpClient = &http.Client{
			Transport: &bearerTransport{token: apiKey, base: http.DefaultTransport},
		}
	}
	c := ollamaapi.NewClient(u, httpClient)
	a := &Adapter{client: c, model: model, maxTokens: maxTokens}
	a.probeContextWindow()
	return a
}

// NewAdapterWithOptions constructs an Adapter with all options.
// Kept for backwards compatibility; debug verbosity is controlled by the global logging level.
func NewAdapterWithOptions(baseURL, apiKey, model string, maxTokens int, _ string) *Adapter {
	return NewAdapterWithAuth(baseURL, apiKey, model, maxTokens)
}

// probeContextWindow calls Show to determine the model's actual context window.
// model_info keys follow the pattern "<arch>.context_length" (llama, mistral, phi, etc.).
func (a *Adapter) probeContextWindow() {
	show, err := a.client.Show(context.Background(), &ollamaapi.ShowRequest{Model: a.model})
	if err != nil {
		log.Printf("ollama: could not probe context window for %q: %v (using default)", a.model, err)
		return
	}
	for k, v := range show.ModelInfo {
		if strings.HasSuffix(k, ".context_length") {
			if n, ok := v.(float64); ok && n > 0 {
				a.contextWindow = int(n)
				log.Printf("ollama: model %q context window = %d tokens", a.model, a.contextWindow)
				return
			}
		}
	}
}

// bearerTransport adds Authorization: Bearer to every outgoing request.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

// Chat sends a chat request to Ollama and returns the response.
func (a *Adapter) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	if a.initErr != nil {
		return ports.ChatResponse{}, a.initErr
	}

	messages := a.convertMessages(sanitizeMessagesForOllama(req.Messages))
	tools := convertTools(req.Tools)

	logging.Debugf("ollama: request model=%s msgs=%d tools=%d", a.model, len(messages), len(tools))

	numPredict := a.maxTokens
	if req.MaxTokens > 0 {
		numPredict = req.MaxTokens
	}
	streamFalse := false
	options := map[string]any{"num_predict": numPredict}
	if cw := a.GetContextWindow(); cw > 0 {
		// Safety cap: explicitly tell Ollama how much context to allocate for this session.
		// Exceeding default 2048/4096 without num_ctx causes 500 Errors on large prompts.
		// We cap at 32k to avoid OOM on consumer hardware if a model reports 128k+.
		numCtx := cw
		if numCtx > 32768 {
			numCtx = 32768
		}
		options["num_ctx"] = numCtx
		logging.Debugf("ollama: context_limit=%d (max_probed=%d)", numCtx, cw)
	}

	ollamaReq := &ollamaapi.ChatRequest{
		Model:    a.model,
		Messages: messages,
		Tools:    tools,
		Stream:   &streamFalse,
		Options:  options,
	}

	var sdkResp ollamaapi.ChatResponse
	if err := a.client.Chat(ctx, ollamaReq, func(r ollamaapi.ChatResponse) error {
		sdkResp = r
		return nil
	}); err != nil {
		log.Printf("ollama: Chat error: %v", err)
		return ports.ChatResponse{}, err
	}

	logging.Debugf("ollama: done_reason=%q content_len=%d tool_calls=%d",
		sdkResp.DoneReason, len(sdkResp.Message.Content), len(sdkResp.Message.ToolCalls))

	result := ports.ChatResponse{
		Content:    sdkResp.Message.Content,
		StopReason: "stop",
		Usage: ports.TokenUsage{
			PromptTokens:     sdkResp.PromptEvalCount,
			CompletionTokens: sdkResp.EvalCount,
		},
	}

	// Standard path: SDK parsed tool_calls into typed structs.
	if len(sdkResp.Message.ToolCalls) > 0 {
		for _, tc := range sdkResp.Message.ToolCalls {
			name := strings.ReplaceAll(tc.Function.Name, "__", ":")
			argsBytes, err := json.Marshal(tc.Function.Arguments)
			if err != nil {
				argsBytes = []byte("{}")
			}
			result.ToolCalls = append(result.ToolCalls, ports.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: ports.FunctionCall{
					Name:      name,
					Arguments: string(argsBytes),
				},
			})
		}
		result.StopReason = "tool_use"
		logging.Debugf("ollama: %d tool_calls (SDK), stop_reason=tool_use", len(result.ToolCalls))
	}

	// Strip <thought>...</thought> reasoning blocks emitted by DeepSeek-R1 etc.
	if strings.Contains(result.Content, "<thought>") {
		matches := thoughtBlockRe.FindAllStringSubmatch(result.Content, -1)
		if len(matches) > 0 {
			result.Content = strings.TrimSpace(thoughtBlockRe.ReplaceAllString(result.Content, ""))
			logging.Debugf("ollama: stripped %d reasoning blocks", len(matches))
		}
	}

	// Fallback: some fine-tuned models embed <tool> blocks in content.
	if len(result.ToolCalls) == 0 && strings.Contains(result.Content, "<tool>") {
		parsed := parseToolBlocks(result.Content)
		if len(parsed) > 0 {
			result.ToolCalls = parsed
			result.StopReason = "tool_use"
			result.Content = strings.TrimSpace(toolBlockRe.ReplaceAllString(result.Content, ""))
			logging.Debugf("ollama: %d tool_calls (<tool> blocks), stop_reason=tool_use", len(result.ToolCalls))
		}
	}

	log.Printf("ollama: returning content_len=%d stop_reason=%s", len(result.Content), result.StopReason)
	return result, nil
}

// ChatWithAudio is not supported by Ollama.
func (a *Adapter) ChatWithAudio(_ context.Context, _ ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, fmt.Errorf("audio input not supported by Ollama")
}

// ChatToAudio is not supported by Ollama.
func (a *Adapter) ChatToAudio(_ context.Context, _ ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, fmt.Errorf("audio output not supported by Ollama")
}

// SupportsAudioInput reports false.
func (a *Adapter) SupportsAudioInput() bool { return false }

// SupportsAudioOutput reports false.
func (a *Adapter) SupportsAudioOutput() bool { return false }

// GetMaxTokens returns the configured output token limit.
func (a *Adapter) GetMaxTokens() int { return a.maxTokens }

// GetContextWindow returns the model's context window as probed from /api/show.
// Falls back to 8192 if the probe failed or the model does not advertise a length.
func (a *Adapter) GetContextWindow() int {
	if a.contextWindow > 0 {
		return a.contextWindow
	}
	return 8192
}

// OverrideContextWindow sets the context window explicitly, overriding the probed value.
func (a *Adapter) OverrideContextWindow(n int) { a.contextWindow = n }

var _ ports.AIProviderPort = (*Adapter)(nil)

// sanitizeMessagesForOllama removes orphan and duplicate tool result messages.
func sanitizeMessagesForOllama(messages []ports.ChatMessage) []ports.ChatMessage {
	validIDs := make(map[string]bool)
	for _, m := range messages {
		if m.Role == "assistant" {
			for _, tc := range m.ToolCalls {
				if tc.ID != "" {
					validIDs[tc.ID] = true
				}
			}
		}
	}
	seen := make(map[string]bool)
	out := make([]ports.ChatMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == "tool" {
			if m.ToolCallID == "" {
				continue
			}
			if !validIDs[m.ToolCallID] {
				log.Printf("ollama: dropping orphan tool message (tool_call_id=%q)", m.ToolCallID)
				continue
			}
			if seen[m.ToolCallID] {
				log.Printf("ollama: dropping duplicate tool result for id=%q", m.ToolCallID)
				continue
			}
			seen[m.ToolCallID] = true
		}
		out = append(out, m)
	}
	return out
}

// convertMessages translates domain ChatMessages into SDK Message types.
func (a *Adapter) convertMessages(messages []ports.ChatMessage) []ollamaapi.Message {
	result := make([]ollamaapi.Message, len(messages))
	for i, msg := range messages {
		m := ollamaapi.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Role == "user" && len(msg.Blocks) > 0 {
			m.Images = collectImageBlocks(msg.Blocks)
			logging.Debugf("ollama: user msg has %d blocks → %d images", len(msg.Blocks), len(m.Images))
			if len(m.Images) > 0 && strings.TrimSpace(m.Content) == "" {
				m.Content = "Describe and analyse this image. Respond using the same language as the user."
			}
			hasAudio := false
			for _, b := range msg.Blocks {
				if b.Type == ports.ContentBlockAudio {
					hasAudio = true
					break
				}
			}
			if len(m.Images) == 0 && hasAudio && strings.TrimSpace(m.Content) == "" {
				m.Content = "Interpret the attached voice message and respond appropriately. Respond using the same language as the user."
			}
		}
		if msg.Role == "tool" {
			m.ToolCallID = msg.ToolCallID
			m.ToolName = msg.ToolName
		}
		for idx, tc := range msg.ToolCalls {
			name := strings.ReplaceAll(tc.Function.Name, ":", "__")
			var args ollamaapi.ToolCallFunctionArguments
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				log.Printf("ollama: failed to unmarshal tool call args for %q: %v", tc.Function.Name, err)
			}
			m.ToolCalls = append(m.ToolCalls, ollamaapi.ToolCall{
				ID: tc.ID,
				Function: ollamaapi.ToolCallFunction{
					Index:     idx,
					Name:      name,
					Arguments: args,
				},
			})
		}
		result[i] = m
	}
	return result
}

// collectImageBlocks extracts image bytes from multimodal content blocks.
func collectImageBlocks(blocks []ports.ContentBlock) []ollamaapi.ImageData {
	var images []ollamaapi.ImageData
	for _, b := range blocks {
		if b.Type != ports.ContentBlockImage {
			continue
		}
		if len(b.Data) > 0 {
			images = append(images, ollamaapi.ImageData(b.Data))
			continue
		}
		log.Printf("ollama: image block has no Data (mime=%s) — skipping", b.MIMEType)
	}
	return images
}

// convertTools translates domain Tool definitions into SDK Tool types.
// Colons in tool names are replaced with __ (Ollama enforces [a-zA-Z0-9_-]).
func convertTools(tools []ports.Tool) ollamaapi.Tools {
	result := make(ollamaapi.Tools, 0, len(tools))
	for _, t := range tools {
		if t.Function == nil {
			continue
		}
		name := strings.ReplaceAll(t.Function.Name, ":", "__")
		result = append(result, ollamaapi.Tool{
			Type: "function",
			Function: ollamaapi.ToolFunction{
				Name:        name,
				Description: t.Function.Description,
				Parameters: ollamaapi.ToolFunctionParameters{
					Type:       paramType(t.Function.Parameters),
					Required:   paramRequired(t.Function.Parameters),
					Properties: paramProperties(t.Function.Parameters),
				},
			},
		})
	}
	return result
}

func paramType(p map[string]any) string {
	if t, ok := p["type"].(string); ok {
		return t
	}
	return "object"
}

func paramRequired(p map[string]any) []string {
	raw, ok := p["required"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func paramProperties(p map[string]any) *ollamaapi.ToolPropertiesMap {
	raw, ok := p["properties"].(map[string]any)
	if !ok {
		return nil
	}
	out := ollamaapi.NewToolPropertiesMap()
	for k, v := range raw {
		propMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		prop := ollamaapi.ToolProperty{}
		if t, ok := propMap["type"].(string); ok {
			prop.Type = ollamaapi.PropertyType{t}
		}
		prop.Description, _ = propMap["description"].(string)
		if enums, ok := propMap["enum"].([]any); ok {
			prop.Enum = append(prop.Enum, enums...)
		}
		out.Set(k, prop)
	}
	return out
}

var toolBlockRe = regexp.MustCompile(`(?s)<tool>\s*(.*?)\s*</tool>`)
var thoughtBlockRe = regexp.MustCompile(`(?s)<thought>\s*(.*?)\s*</thought>`)

// parseToolBlocks extracts tool calls from <tool> JSON blocks embedded in content.
// Fallback for models with custom templates that don't use the standard tool_calls field.
func parseToolBlocks(content string) []ports.ToolCall {
	matches := toolBlockRe.FindAllStringSubmatch(content, -1)
	var calls []ports.ToolCall
	for i, m := range matches {
		if len(m) < 2 {
			continue
		}
		var payload struct {
			Name       string         `json:"name"`
			Parameters map[string]any `json:"parameters"`
		}
		if err := json.Unmarshal([]byte(m[1]), &payload); err != nil {
			log.Printf("ollama: failed to parse <tool> block: %v", err)
			continue
		}
		argsBytes, _ := json.Marshal(payload.Parameters)
		calls = append(calls, ports.ToolCall{
			ID:   fmt.Sprintf("tool_%d", i),
			Type: "function",
			Function: ports.FunctionCall{
				Name:      strings.ReplaceAll(payload.Name, "__", ":"),
				Arguments: string(argsBytes),
			},
		})
	}
	return calls
}
