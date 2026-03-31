package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
func getName() int64 { return writeStringResult("openlobster-ai-ollama") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("Ollama local AI plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("ai") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"base_url":{"type":"string","title":"Base URL","default":"http://localhost:11434"},"default_model":{"type":"string","title":"Default Model","default":"llama3.2"}}}`)
}

// ---------------------------------------------------------------------------
// Ollama uses OpenAI-compatible /api/chat endpoint
// ---------------------------------------------------------------------------

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
	} `json:"function"`
}

type InputPayload struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	Tools     []Tool        `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Config    struct {
		BaseURL      string `json:"base_url"`
		DefaultModel string `json:"default_model"`
	} `json:"config"`
}

type OllamaRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
	Options  struct {
		NumPredict int `json:"num_predict,omitempty"`
	} `json:"options,omitempty"`
}

type OllamaToolCall struct {
	Function struct {
		Name      string `json:"name"`
		Arguments any    `json:"arguments"`
	} `json:"function"`
}

type OllamaResponse struct {
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	DoneReason         string `json:"done_reason"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	EvalCount          int    `json:"eval_count"`
	Error              string `json:"error,omitempty"`
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

	reqBody := OllamaRequest{
		Model:    model,
		Messages: input.Messages,
		Tools:    input.Tools,
		Stream:   false,
	}
	if input.MaxTokens > 0 {
		reqBody.Options.NumPredict = input.MaxTokens
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		resultBuf = []byte(`{"error":"failed to marshal request"}`)
		return 1
	}

	req, err := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		resultBuf = []byte(`{"error":"failed to create request"}`)
		return 1
	}
	req.Header.Set("Content-Type", "application/json")

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

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(respBytes, &ollamaResp); err != nil {
		resultBuf = []byte(`{"error":"failed to parse Ollama response"}`)
		return 1
	}

	if ollamaResp.Error != "" {
		writeResult(ErrorPayload{Error: ollamaResp.Error})
		return 1
	}

	out := OutputPayload{
		Content:    ollamaResp.Message.Content,
		StopReason: ollamaResp.DoneReason,
		Usage: OutputUsage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
		},
	}

	for i, tc := range ollamaResp.Message.ToolCalls {
		argJSON, _ := json.Marshal(tc.Function.Arguments)
		out.ToolCalls = append(out.ToolCalls, OutputToolCall{
			ID:    fmt.Sprintf("call_%d", i),
			Name:  tc.Function.Name,
			Input: string(argJSON),
		})
	}

	return writeResult(out)
}

func main() {}
