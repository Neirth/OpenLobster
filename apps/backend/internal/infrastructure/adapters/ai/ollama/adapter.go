// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: see LICENSE

// Package ollama provides an AI provider adapter backed by a local Ollama instance.
// It communicates directly with the Ollama REST API over HTTP, with no external SDK dependency.
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

	"golang.org/x/crypto/ssh"

	"github.com/neirth/openlobster/internal/domain/ports"
)

var ensureKeyOnce sync.Once

// ensureOllamaPrivateKey creates ~/.ollama/id_ed25519 if missing, so the Ollama
// SDK does not log "Failed to load private key". The key is used by the SDK for
// ollama.com auth; we use Bearer token (ollamaApiKey) for our own connections.
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
	client    *Client
	model     string
	maxTokens int
	debug     bool
}

// debugf logs a message only when the adapter was created with debug enabled.
func (a *Adapter) debugf(format string, args ...interface{}) {
	if a.debug {
		log.Printf(format, args...)
	}
}

// NewAdapter constructs an Adapter pointing at the given Ollama endpoint.
func NewAdapter(baseURL, model string, maxTokens int) *Adapter {
	return NewAdapterWithAuth(baseURL, "", model, maxTokens)
}

// NewAdapterWithAuth constructs an Adapter with an optional Bearer token.
// Pass an empty apiKey to skip authentication (standard local Ollama instances).
func NewAdapterWithAuth(baseURL, apiKey, model string, maxTokens int) *Adapter {
	return newAdapter(baseURL, apiKey, model, maxTokens, false)
}

// NewAdapterWithOptions constructs an Adapter with full options including log level.
// logLevel should be the value of cfg.Logging.Level; "debug" enables verbose request logs.
func NewAdapterWithOptions(baseURL, apiKey, model string, maxTokens int, logLevel string) *Adapter {
	debug := strings.EqualFold(logLevel, "debug")
	return newAdapter(baseURL, apiKey, model, maxTokens, debug)
}

func newAdapter(baseURL, apiKey, model string, maxTokens int, debug bool) *Adapter {
	ensureOllamaPrivateKey()
	u, err := url.Parse(baseURL)
	if err != nil {
		log.Printf("ollama: invalid endpoint %q, using default: %v", baseURL, err)
		c, _ := ClientFromEnvironment()
		return &Adapter{client: c, model: model, maxTokens: maxTokens, debug: debug}
	}
	httpClient := http.DefaultClient
	if apiKey != "" {
		httpClient = &http.Client{
			Transport: &bearerTransport{token: apiKey, base: http.DefaultTransport},
		}
	}
	return &Adapter{
		client:    NewClient(u, httpClient),
		model:     model,
		maxTokens: maxTokens,
		debug:     debug,
	}
}

// bearerTransport adds an Authorization: Bearer <token> header to every request.
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
	messages := a.convertMessages(sanitizeMessagesForOllama(req.Messages))
	tools := a.convertTools(req.Tools)

	a.debugf("ollama: sending request with model=%s, %d messages, %d tools",
		a.model, len(messages), len(tools))
	for i, t := range req.Tools {
		if t.Function != nil {
			a.debugf("ollama:   tool[%d] name=%q", i, t.Function.Name)
		}
	}
	for i, msg := range messages {
		preview := msg.Content
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		a.debugf("ollama:   [%d] role=%s content=%q", i, msg.Role, preview)
	}

	stream := false
	ollamaReq := &ChatRequest{
		Model:    a.model,
		Messages: messages,
		Tools:    tools,
		Stream:   &stream,
		Options: map[string]interface{}{
			"num_predict": a.maxTokens,
		},
	}

	var sdkResp ChatResponse
	err := a.client.Chat(ctx, ollamaReq, func(r ChatResponse) error {
		sdkResp = r
		return nil
	})
	if err != nil {
		log.Printf("ollama: Chat error: %v", err)
		return ports.ChatResponse{}, err
	}

	a.debugf("ollama: done_reason=%q content_len=%d tool_calls=%d",
		sdkResp.DoneReason, len(sdkResp.Message.Content), len(sdkResp.Message.ToolCalls))

	result := ports.ChatResponse{
		Content:    sdkResp.Message.Content,
		StopReason: "stop",
	}

	// Standard path: SDK already parsed tool_calls into typed structs.
	if len(sdkResp.Message.ToolCalls) > 0 {
		for _, tc := range sdkResp.Message.ToolCalls {
			// Restore the __ to : substitution used for API name compatibility.
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
		a.debugf("ollama: extracted %d tool_calls (SDK), stop_reason=tool_use", len(result.ToolCalls))
	}

	// Fallback path: some fine-tuned models embed <tool> blocks in content
	// instead of the standard tool_calls field.
	if len(result.ToolCalls) == 0 && strings.Contains(result.Content, "<tool>") {
		parsed := parseToolBlocks(result.Content)
		if len(parsed) > 0 {
			result.ToolCalls = parsed
			result.StopReason = "tool_use"
			result.Content = strings.TrimSpace(toolBlockRe.ReplaceAllString(result.Content, ""))
			a.debugf("ollama: extracted %d tool_calls (<tool> blocks), stop_reason=tool_use", len(result.ToolCalls))
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

// SupportsAudioInput reports false; Ollama does not accept audio input.
func (a *Adapter) SupportsAudioInput() bool { return false }

// SupportsAudioOutput reports false; Ollama does not produce audio output.
func (a *Adapter) SupportsAudioOutput() bool { return false }

// GetMaxTokens returns the configured token limit.
func (a *Adapter) GetMaxTokens() int { return a.maxTokens }

// sanitizeMessagesForOllama removes tool messages whose ToolCallID does not match
// any assistant message's ToolCalls. Ollama Cloud returns "Unexpected tool call id"
// when tool results reference IDs it doesn't know (e.g. from stale history).
// Also drops duplicate tool results for the same ID (keeps first only).
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
	seenToolIDs := make(map[string]bool)
	out := make([]ports.ChatMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == "tool" {
			if m.ToolCallID == "" {
				// Tool messages without a tool_call_id come from internal logging
				// (e.g. persisted MCP/tool outputs) and are never part of the
				// function-calling protocol expected by Ollama. Drop them
				// silently from the history we send to the provider to avoid
				// noisy logs and potential confusion, while keeping the user
				//‑visible history unaffected.
				continue // drop tool messages that cannot be correlated
			}
			if !validIDs[m.ToolCallID] {
				log.Printf("ollama: dropping orphan tool message (tool_call_id=%q not in any assistant)", m.ToolCallID)
				continue
			}
			if seenToolIDs[m.ToolCallID] {
				log.Printf("ollama: dropping duplicate tool result for id=%q", m.ToolCallID)
				continue
			}
			seenToolIDs[m.ToolCallID] = true
		}
		out = append(out, m)
	}
	return out
}

// collectImageBlocks extracts image bytes from multimodal content blocks.
// Only blocks with pre-downloaded Data are used; audio and text blocks are ignored.
func collectImageBlocks(blocks []ports.ContentBlock) []ImageData {
	var images []ImageData
	for _, b := range blocks {
		if b.Type != ports.ContentBlockImage {
			continue
		}
		// Prefer pre-downloaded data when available.
		if len(b.Data) > 0 {
			log.Printf("ollama: image block attached (%d bytes, mime=%s)", len(b.Data), b.MIMEType)
			images = append(images, ImageData(b.Data))
			continue
		}

		// URL fallback removed: ContentBlock.URL is not populated from attachments
		// in current flows. Keep a diagnostic log to aid debugging when needed.
		log.Printf("ollama: image block has no Data (mime=%s) — skipping", b.MIMEType)
	}
	return images
}

// convertMessages translates domain ChatMessages into SDK Message types.
// Assistant messages that triggered tool use include their ToolCalls so that
// Ollama can correlate subsequent tool-result messages correctly.
// User messages with multimodal Blocks have their image data attached via the
// Images field; other block types (audio, text) are carried in Content.
func (a *Adapter) convertMessages(messages []ports.ChatMessage) []Message {
	result := make([]Message, len(messages))
	for i, msg := range messages {
		m := Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
		if msg.Role == "user" && len(msg.Blocks) > 0 {
			m.Images = collectImageBlocks(msg.Blocks)
			a.debugf("ollama: user message has %d blocks → %d images extracted", len(msg.Blocks), len(m.Images))
			// Ollama requires a non-empty Content when images are present.
			// If the user sent only an image without text, provide a default prompt.
			if len(m.Images) > 0 && strings.TrimSpace(m.Content) == "" {
				m.Content = "Describe and analyse this image. Respond using the same language as the user."
				a.debugf("ollama: injected default prompt for image-only message")
			}

			// If the user sent a voice message without a caption, instruct the
			// model to interpret the audio and respond accordingly. Prefer the
			// image prompt when images were present; only inject the audio prompt
			// when no images were attached and content is empty.
			hasAudio := false
			for _, b := range msg.Blocks {
				if b.Type == ports.ContentBlockAudio {
					hasAudio = true
					break
				}
			}
			if len(m.Images) == 0 && hasAudio && strings.TrimSpace(m.Content) == "" {
				m.Content = "Interpret the attached voice message and respond appropriately. Respond using the same language as the user."
				a.debugf("ollama: injected default prompt for audio-only message")
			}
		}
		if msg.Role == "tool" {
			if msg.ToolCallID != "" {
				m.ToolCallID = msg.ToolCallID
			}
			if msg.ToolName != "" {
				m.ToolName = msg.ToolName
			}
		}
		if len(msg.ToolCalls) > 0 {
			for idx, tc := range msg.ToolCalls {
				name := strings.ReplaceAll(tc.Function.Name, ":", "__")
				var argsObj ToolCallFunctionArguments
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsObj); err != nil {
					argsObj = ToolCallFunctionArguments{}
				}
				m.ToolCalls = append(m.ToolCalls, ToolCall{
					ID: tc.ID,
					Function: ToolCallFunction{
						Index:     idx,
						Name:      name,
						Arguments: argsObj,
					},
				})
			}
		}
		result[i] = m
	}
	return result
}

// convertTools translates domain Tool definitions into SDK Tool types.
// Colons in qualified MCP tool names (server:tool) are replaced with __ because
// Ollama enforces the OpenAI function-name character set ([a-zA-Z0-9_-]).
func (a *Adapter) convertTools(tools []ports.Tool) Tools {
	result := make(Tools, 0, len(tools))
	for _, t := range tools {
		if t.Function == nil {
			continue
		}
		name := strings.ReplaceAll(t.Function.Name, ":", "__")
		result = append(result, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        name,
				Description: t.Function.Description,
				Parameters: ToolFunctionParameters{
					Type:       paramType(t.Function.Parameters),
					Required:   paramRequired(t.Function.Parameters),
					Properties: paramProperties(t.Function.Parameters),
				},
			},
		})
	}
	return result
}

// paramType extracts the "type" field from a JSON-schema parameters map.
func paramType(p map[string]interface{}) string {
	if t, ok := p["type"].(string); ok {
		return t
	}
	return "object"
}

// paramRequired extracts the "required" array from a JSON-schema parameters map.
func paramRequired(p map[string]interface{}) []string {
	raw, ok := p["required"].([]interface{})
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

// paramProperties converts the "properties" field of a JSON-schema parameters map
// into the ToolPropertiesMap expected by the Ollama SDK.
func paramProperties(p map[string]interface{}) *ToolPropertiesMap {
	raw, ok := p["properties"].(map[string]interface{})
	if !ok {
		return nil
	}
	out := NewToolPropertiesMap()
	for k, v := range raw {
		propMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		prop := ToolProperty{}
		if t, ok := propMap["type"].(string); ok {
			prop.Type = PropertyType{t}
		}
		prop.Description, _ = propMap["description"].(string)
		if enums, ok := propMap["enum"].([]interface{}); ok {
			for _, e := range enums {
				prop.Enum = append(prop.Enum, e)
			}
		}
		out.Set(k, prop)
	}
	return out
}

var _ ports.AIProviderPort = (*Adapter)(nil)

// toolBlockRe matches custom <tool>...</tool> blocks emitted by some fine-tuned
// models instead of using the standard tool_calls API field.
var toolBlockRe = regexp.MustCompile(`(?s)<tool>\s*(.*?)\s*</tool>`)

// parseToolBlocks extracts tool calls from <tool> JSON blocks embedded in the
// model text response. This is a fallback for models with custom templates.
// Generates deterministic IDs so tool results can be correlated (Ollama Cloud
// requires matching tool_call_id in tool results).
func parseToolBlocks(content string) []ports.ToolCall {
	matches := toolBlockRe.FindAllStringSubmatch(content, -1)
	var calls []ports.ToolCall
	for i, m := range matches {
		if len(m) < 2 {
			continue
		}
		var payload struct {
			Name       string                 `json:"name"`
			Parameters map[string]interface{} `json:"parameters"`
		}
		if err := json.Unmarshal([]byte(m[1]), &payload); err != nil {
			log.Printf("ollama: failed to parse <tool> block: %v", err)
			continue
		}
		name := strings.ReplaceAll(payload.Name, "__", ":")
		argsBytes, _ := json.Marshal(payload.Parameters)
		id := fmt.Sprintf("tool_%d", i)
		calls = append(calls, ports.ToolCall{
			ID:   id,
			Type: "function",
			Function: ports.FunctionCall{
				Name:      name,
				Arguments: string(argsBytes),
			},
		})
	}
	return calls
}
