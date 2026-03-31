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
func getName() int64 { return writeStringResult("openlobster-ai-openai") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("OpenAI API plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("ai") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key"},"base_url":{"type":"string","title":"Base URL (optional)","default":"https://api.openai.com/v1"},"default_model":{"type":"string","title":"Default Model","default":"gpt-4o"}},"required":["api_key"]}`)
}

// ---------------------------------------------------------------------------
// OpenAI request / response types
// ---------------------------------------------------------------------------

type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type InputPayload struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	Tools     []Tool        `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Config    struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url,omitempty"`
	} `json:"config"`
}

type OpenAIRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	Tools     []Tool        `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type OpenAIChoice struct {
	Message struct {
		Content   *string          `json:"content"`
		ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type OpenAIResponse struct {
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type OutputToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input"`
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

	baseURL := input.Config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := input.Model
	if model == "" {
		model = "gpt-4o"
	}

	reqBody := OpenAIRequest{
		Model:     model,
		Messages:  input.Messages,
		Tools:     input.Tools,
		MaxTokens: input.MaxTokens,
	}
	if reqBody.MaxTokens == 0 {
		reqBody.MaxTokens = 4096
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		resultBuf = []byte(`{"error":"failed to marshal request"}`)
		return 1
	}

	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		resultBuf = []byte(`{"error":"failed to create request"}`)
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+input.Config.APIKey)

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

	var oaiResp OpenAIResponse
	if err := json.Unmarshal(respBytes, &oaiResp); err != nil {
		resultBuf = []byte(`{"error":"failed to parse OpenAI response"}`)
		return 1
	}

	if oaiResp.Error != nil {
		writeResult(ErrorPayload{Error: oaiResp.Error.Message})
		return 1
	}

	if len(oaiResp.Choices) == 0 {
		resultBuf = []byte(`{"error":"no choices in response"}`)
		return 1
	}

	choice := oaiResp.Choices[0]
	out := OutputPayload{
		StopReason: choice.FinishReason,
		Usage: OutputUsage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
		},
	}

	if choice.Message.Content != nil {
		out.Content = *choice.Message.Content
	}

	for _, tc := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, OutputToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}

	return writeResult(out)
}

func main() {}
