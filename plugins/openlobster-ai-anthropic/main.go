package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"unsafe"

	_ "github.com/stealthrocket/net/wasip1"
)

var (
	inputBuf  []byte
	resultBuf []byte
)

// ---------------------------------------------------------------------------
// ABI buffer helpers
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_alloc_input
func allocInput(size uint32) uint32 {
	inputBuf = make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&inputBuf[0])))
}

//go:wasmexport openlobster_result_ptr
func resultPtr() uint32 {
	if len(resultBuf) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}

//go:wasmexport openlobster_result_len
func resultLen() uint32 {
	return uint32(len(resultBuf))
}

func writeResult(v interface{}) int32 {
	b, err := json.Marshal(v)
	if err != nil {
		resultBuf = []byte(`{"error":"marshal failed"}`)
		return 1
	}
	resultBuf = b
	return 0
}

func writeStringResult(s string) int64 {
	resultBuf = []byte(s)
	ptr := uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
	return int64(ptr)<<32 | int64(len(resultBuf))
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_get_name
func getName() int64 { return writeStringResult("openlobster-ai-anthropic") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("Anthropic Claude API plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("ai") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key"},"default_model":{"type":"string","title":"Default Model","default":"claude-opus-4-6"}},"required":["api_key"]}`)
}

// ---------------------------------------------------------------------------
// Anthropic request / response types
// ---------------------------------------------------------------------------

type InputMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

type InputPayload struct {
	Model     string         `json:"model"`
	Messages  []InputMessage `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
	MaxTokens int            `json:"max_tokens,omitempty"`
	Config    struct {
		APIKey       string `json:"api_key"`
		DefaultModel string `json:"default_model,omitempty"`
	} `json:"config"`
}

type AnthropicRequest struct {
	Model     string         `json:"model"`
	Messages  []InputMessage `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
	MaxTokens int            `json:"max_tokens"`
}

type AnthropicContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicResponse struct {
	Content    []AnthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      AnthropicUsage          `json:"usage"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type OutputToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

type OutputUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type OutputPayload struct {
	Content    string           `json:"content"`
	ToolCalls  []OutputToolCall `json:"tool_calls,omitempty"`
	StopReason string           `json:"stop_reason"`
	Usage      OutputUsage      `json:"usage"`
}

type ErrorPayload struct {
	Error string `json:"error"`
}

// ---------------------------------------------------------------------------
// openlobster_chat
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_chat
func chat() int32 {
	var input InputPayload
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input JSON"}`)
		return 1
	}

	model := input.Model
	if model == "" {
		model = input.Config.DefaultModel
	}
	if model == "" {
		model = "claude-opus-4-6"
	}

	maxTokens := input.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	reqBody := AnthropicRequest{
		Model:     model,
		Messages:  input.Messages,
		Tools:     input.Tools,
		MaxTokens: maxTokens,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		resultBuf = []byte(`{"error":"failed to marshal request"}`)
		return 1
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		resultBuf = []byte(`{"error":"failed to create request"}`)
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", input.Config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeResult(ErrorPayload{Error: "http request failed: " + err.Error()})
		return 1
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		resultBuf = []byte(`{"error":"failed to read response"}`)
		return 1
	}

	var antResp AnthropicResponse
	if err := json.Unmarshal(respBytes, &antResp); err != nil {
		resultBuf = []byte(`{"error":"failed to parse Anthropic response"}`)
		return 1
	}

	if antResp.Error != nil {
		writeResult(ErrorPayload{Error: antResp.Error.Message})
		return 1
	}

	out := OutputPayload{
		StopReason: antResp.StopReason,
		Usage: OutputUsage{
			PromptTokens:     antResp.Usage.InputTokens,
			CompletionTokens: antResp.Usage.OutputTokens,
		},
	}

	for _, block := range antResp.Content {
		switch block.Type {
		case "text":
			out.Content += block.Text
		case "tool_use":
			inputJSON, _ := json.Marshal(block.Input)
			out.ToolCalls = append(out.ToolCalls, OutputToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: string(inputJSON),
			})
		}
	}

	return writeResult(out)
}

func main() {}
