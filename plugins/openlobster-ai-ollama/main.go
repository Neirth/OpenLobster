package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
	ollamaAPI "github.com/ollama/ollama/api"
	_ "github.com/stealthrocket/net/http"
	"golang.org/x/crypto/ssh"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func getName() int32 { pdk.OutputString("openlobster-ai-ollama"); return 0 }

func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

func getDescription() int32 {
	pdk.OutputString("Ollama local AI provider plugin for OpenLobster")
	return 0
}

func getType() int32 { pdk.OutputString("ai"); return 0 }

func supportsAudioInput() int32 { pdk.OutputString("false"); return 0 }

func supportsAudioOutput() int32 { pdk.OutputString("false"); return 0 }

func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"base_url":{"type":"string","title":"Base URL","default":"http://localhost:11434","description":"Ollama endpoint (local or remote), for example http://localhost:11434"},"default_model":{"type":"string","title":"Default Model","default":"llama3.2","description":"Model used when the request does not specify one"},"api_key":{"type":"string","title":"API Key","description":"Optional Bearer token for protected or cloud Ollama endpoints"}},"required":[]}`)
	return 0
}

func getMetadata() int32 {
	metadata := map[string]interface{}{
		"id":          "openlobster-ai-ollama",
		"name":        "openlobster-ai-ollama",
		"version":     "0.1.0",
		"description": "Ollama local AI provider plugin for OpenLobster",
		"type":        "ai",
		"schema":      json.RawMessage(`{"type":"object","properties":{"base_url":{"type":"string","title":"Base URL","default":"http://localhost:11434","description":"Ollama endpoint (local or remote), for example http://localhost:11434"},"default_model":{"type":"string","title":"Default Model","default":"llama3.2","description":"Model used when the request does not specify one"},"api_key":{"type":"string","title":"API Key","description":"Optional Bearer token for protected or cloud Ollama endpoints"}},"required":[]}`),
		"properties":  json.RawMessage(`{"supports_audio_input":false,"supports_audio_output":false}`),
	}
	if err := pdk.OutputJSON(metadata); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolName   string     `json:"tool_name,omitempty"`
}

type toolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
	} `json:"function"`
}

type inputPayload struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	Tools     []toolDef     `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Config    struct {
		BaseURL      string `json:"base_url"`
		DefaultModel string `json:"default_model"`
		APIKey       string `json:"api_key"`
		PluginHome   string `json:"__plugin_home"`
	} `json:"config"`
}

type outputPayload struct {
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason"`
	Usage      struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error string `json:"error,omitempty"`
}

type bearerAuthTransport struct {
	base       http.RoundTripper
	authHeader string
}

var ensureKeyMu sync.Mutex

const (
	defaultHTTPTimeout = 45 * time.Second
	defaultChatTimeout = 60 * time.Second
)

func normalizePluginHome(preferredHome string) string {
	home := strings.TrimSpace(preferredHome)
	if home == "" {
		home = strings.TrimSpace(os.Getenv("HOME"))
	}
	if home == "" {
		if resolvedHome, err := os.UserHomeDir(); err == nil {
			home = strings.TrimSpace(resolvedHome)
		}
	}
	if home == "" {
		home = "/tmp/openlobster-plugin-home"
	}
	if !filepath.IsAbs(home) {
		clean := strings.TrimLeft(home, "./")
		if clean == "" {
			clean = "tmp/openlobster-plugin-home"
		}
		home = "/" + clean
	}
	return filepath.Clean(home)
}

func homeCandidates(preferredHome string) []string {
	seeds := []string{
		preferredHome,
		os.Getenv("HOME"),
		"/tmp/openlobster-plugin-home",
		"/tmp",
		"plugin-home",
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(seeds)*2)
	push := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	for _, seed := range seeds {
		normalized := normalizePluginHome(seed)
		push(normalized)
		if strings.HasPrefix(normalized, "/") {
			relative := strings.TrimPrefix(normalized, "/")
			push(relative)
		}
	}

	return out
}

func keyReadable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func ensureKeyAtHome(home string) bool {
	home = strings.TrimSpace(home)
	if home == "" {
		return false
	}
	if err := os.Setenv("HOME", home); err != nil {
		return false
	}

	keyDir := filepath.Join(home, ".ollama")
	keyPath := filepath.Join(keyDir, "id_ed25519")
	if keyReadable(keyPath) {
		return true
	}

	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return false
	}
	if keyReadable(keyPath) {
		return true
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return false
	}

	privateKey, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return false
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(privateKey), 0o600); err != nil {
		return false
	}

	return keyReadable(keyPath)
}

func ensureOllamaPrivateKey(preferredHome string) {
	ensureKeyMu.Lock()
	defer ensureKeyMu.Unlock()

	for _, home := range homeCandidates(preferredHome) {
		if ensureKeyAtHome(home) {
			return
		}
	}
}

func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil {
		return nil, fmt.Errorf("nil auth transport")
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	if strings.TrimSpace(t.authHeader) != "" {
		clone.Header.Set("Authorization", t.authHeader)
	}
	return base.RoundTrip(clone)
}

func sanitizeMessagesForOllama(messages []chatMessage) []chatMessage {
	validIDs := make(map[string]struct{})
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		for _, tc := range m.ToolCalls {
			if strings.TrimSpace(tc.ID) == "" {
				continue
			}
			validIDs[tc.ID] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	out := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == "tool" {
			if strings.TrimSpace(m.ToolCallID) == "" {
				continue
			}
			if _, ok := validIDs[m.ToolCallID]; !ok {
				continue
			}
			if _, duplicated := seen[m.ToolCallID]; duplicated {
				continue
			}
			seen[m.ToolCallID] = struct{}{}
		}
		out = append(out, m)
	}

	return out
}

func sanitizeURLForError(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// ---------------------------------------------------------------------------
// chat
// ---------------------------------------------------------------------------

func chat() int32 {
	var input inputPayload
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	baseURL := input.Config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := input.Model
	if model == "" {
		model = input.Config.DefaultModel
	}
	if model == "" {
		model = "llama3.2"
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		pdk.SetError(fmt.Errorf("invalid base_url: %w", err))
		return 1
	}

	ensureOllamaPrivateKey(input.Config.PluginHome)

	apiKey := strings.TrimSpace(input.Config.APIKey)

	httpClient := &http.Client{Timeout: defaultHTTPTimeout}
	if apiKey != "" {
		authHeader := apiKey
		if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			authHeader = "Bearer " + apiKey
		}
		httpClient = &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: &bearerAuthTransport{base: http.DefaultTransport, authHeader: authHeader},
		}
	}

	client := ollamaAPI.NewClient(u, httpClient)

	// Build ollama messages
	var messages []ollamaAPI.Message
	for _, m := range sanitizeMessagesForOllama(input.Messages) {
		msg := ollamaAPI.Message{
			Role:    m.Role,
			Content: m.Content,
		}
		if m.Role == "tool" {
			msg.ToolCallID = m.ToolCallID
			msg.ToolName = strings.ReplaceAll(m.ToolName, ":", "__")
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for idx, tc := range m.ToolCalls {
				name := strings.ReplaceAll(tc.Function.Name, ":", "__")
				var args ollamaAPI.ToolCallFunctionArguments
				if strings.TrimSpace(tc.Function.Arguments) != "" {
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				msg.ToolCalls = append(msg.ToolCalls, ollamaAPI.ToolCall{
					ID: tc.ID,
					Function: ollamaAPI.ToolCallFunction{
						Index:     idx,
						Name:      name,
						Arguments: args,
					},
				})
			}
		}
		messages = append(messages, msg)
	}

	// Build ollama tools — use json round-trip to convert to the exact SDK type
	var ollamaTools ollamaAPI.Tools
	for _, t := range input.Tools {
		name := strings.TrimSpace(t.Function.Name)
		if name == "" {
			continue
		}
		parameters := t.Function.Parameters
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		toolJSON, _ := json.Marshal(map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        strings.ReplaceAll(name, ":", "__"),
				"description": t.Function.Description,
				"parameters":  parameters,
			},
		})
		var ot ollamaAPI.Tool
		if err := json.Unmarshal(toolJSON, &ot); err == nil {
			ollamaTools = append(ollamaTools, ot)
		}
	}

	stream := false
	req := &ollamaAPI.ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   &stream,
		Tools:    ollamaTools,
	}
	if input.MaxTokens > 0 {
		req.Options = map[string]any{"num_predict": input.MaxTokens}
	}

	var resp ollamaAPI.ChatResponse
	chatCtx, cancel := context.WithTimeout(context.Background(), defaultChatTimeout)
	defer cancel()

	err = client.Chat(chatCtx, req, func(r ollamaAPI.ChatResponse) error {
		resp = r
		return nil
	})
	if err != nil {
		pdk.SetError(fmt.Errorf("ollama chat request failed: model=%s base_url=%s: %w", model, sanitizeURLForError(baseURL), err))
		out := outputPayload{Error: err.Error()}
		_ = pdk.OutputJSON(out)
		return 1
	}

	out := outputPayload{
		Content:    resp.Message.Content,
		StopReason: resp.DoneReason,
	}
	out.Usage.PromptTokens = resp.PromptEvalCount
	out.Usage.CompletionTokens = resp.EvalCount

	for i, tc := range resp.Message.ToolCalls {
		argsJSON, _ := json.Marshal(tc.Function.Arguments)
		out.ToolCalls = append(out.ToolCalls, toolCall{
			ID:   fmt.Sprintf("call_%d", i),
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: strings.ReplaceAll(tc.Function.Name, "__", ":"), Arguments: string(argsJSON)},
		})
	}
	if len(out.ToolCalls) > 0 {
		out.StopReason = "tool_use"
	}

	if err := pdk.OutputJSON(out); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func main() {
	pdk.MustRun(pdk.Plugin{
		ID: "openlobster-ai-ollama",
		Exports: map[string]pdk.Function{
			"get_metadata": getMetadata,
			"configure":    configureHot,
			"chat":         chat,
		},
	})
}
