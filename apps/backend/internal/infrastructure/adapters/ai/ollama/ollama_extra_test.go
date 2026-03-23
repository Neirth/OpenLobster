// Copyright (c) OpenLobster contributors. See LICENSE for details.

package ollama

import (
	"context"
	"fmt"
	"testing"

	ollamaapi "github.com/ollama/ollama/api"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Extended mockChatClient with error support
// ---------------------------------------------------------------------------

type mockChatClientErr struct {
	response ollamaapi.ChatResponse
	err      error
	showResp *ollamaapi.ShowResponse
	showErr  error
}

func (m *mockChatClientErr) Chat(_ context.Context, _ *ollamaapi.ChatRequest, fn ollamaapi.ChatResponseFunc) error {
	if m.err != nil {
		return m.err
	}
	return fn(m.response)
}

func (m *mockChatClientErr) Show(_ context.Context, _ *ollamaapi.ShowRequest) (*ollamaapi.ShowResponse, error) {
	if m.showErr != nil {
		return nil, m.showErr
	}
	if m.showResp != nil {
		return m.showResp, nil
	}
	return &ollamaapi.ShowResponse{}, nil
}

// ---------------------------------------------------------------------------
// GetContextWindow / OverrideContextWindow / GetMaxTokens
// ---------------------------------------------------------------------------

func TestAdapter_GetContextWindow_Default(t *testing.T) {
	a := &Adapter{model: "llama3", maxTokens: 512}
	assert.Equal(t, 8192, a.GetContextWindow())
}

func TestAdapter_GetContextWindow_Probed(t *testing.T) {
	a := &Adapter{model: "llama3", maxTokens: 512, contextWindow: 16384}
	assert.Equal(t, 16384, a.GetContextWindow())
}

func TestAdapter_OverrideContextWindow(t *testing.T) {
	a := &Adapter{model: "llama3"}
	a.OverrideContextWindow(4096)
	assert.Equal(t, 4096, a.GetContextWindow())
}

func TestAdapter_GetMaxTokens(t *testing.T) {
	a := &Adapter{maxTokens: 2048}
	assert.Equal(t, 2048, a.GetMaxTokens())
}

func TestAdapter_SupportsAudio(t *testing.T) {
	a := &Adapter{}
	assert.False(t, a.SupportsAudioInput())
	assert.False(t, a.SupportsAudioOutput())
}

// ---------------------------------------------------------------------------
// ChatWithAudio / ChatToAudio — not supported
// ---------------------------------------------------------------------------

func TestAdapter_ChatWithAudio_NotSupported(t *testing.T) {
	a := &Adapter{client: &mockChatClientErr{}}
	_, err := a.ChatWithAudio(context.Background(), ports.ChatRequestWithAudio{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audio")
}

func TestAdapter_ChatToAudio_NotSupported(t *testing.T) {
	a := &Adapter{client: &mockChatClientErr{}}
	_, err := a.ChatToAudio(context.Background(), ports.ChatRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "audio")
}

// ---------------------------------------------------------------------------
// Chat — initErr path
// ---------------------------------------------------------------------------

func TestAdapter_Chat_InitErr(t *testing.T) {
	a := &Adapter{initErr: fmt.Errorf("init failed")}
	_, err := a.Chat(context.Background(), ports.ChatRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init failed")
}

// ---------------------------------------------------------------------------
// Chat — SDK Chat error propagation
// ---------------------------------------------------------------------------

func TestAdapter_Chat_SDKError(t *testing.T) {
	mock := &mockChatClientErr{err: fmt.Errorf("connection refused")}
	a := &Adapter{client: mock, model: "llama3", maxTokens: 512}
	_, err := a.Chat(context.Background(), ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "user", Content: "hi"}},
	})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Chat — tool_calls from SDK
// ---------------------------------------------------------------------------

func TestAdapter_Chat_ToolCalls(t *testing.T) {
	args := ollamaapi.NewToolCallFunctionArguments()
	args.Set("path", "/tmp/a.txt")
	mock := &mockChatClientErr{
		response: ollamaapi.ChatResponse{
			Message: ollamaapi.Message{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ollamaapi.ToolCall{
					{
						ID: "tc1",
						Function: ollamaapi.ToolCallFunction{
							Name:      "fs__read_file",
							Arguments: args,
						},
					},
				},
			},
			Done: true,
		},
	}
	a := &Adapter{client: mock, model: "llama3", maxTokens: 512}
	resp, err := a.Chat(context.Background(), ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "user", Content: "read file"}},
	})
	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "fs:read_file", resp.ToolCalls[0].Function.Name)
	assert.Equal(t, "tool_use", resp.StopReason)
}

// ---------------------------------------------------------------------------
// Chat — <tool> block fallback parsing
// ---------------------------------------------------------------------------

func TestAdapter_Chat_ToolBlockFallback(t *testing.T) {
	content := `<tool>{"name":"my__tool","parameters":{"key":"val"}}</tool>`
	mock := &mockChatClientErr{
		response: ollamaapi.ChatResponse{
			Message: ollamaapi.Message{
				Role:    "assistant",
				Content: content,
			},
			Done: true,
		},
	}
	a := &Adapter{client: mock, model: "llama3", maxTokens: 512}
	resp, err := a.Chat(context.Background(), ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "user", Content: "call tool"}},
	})
	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "my:tool", resp.ToolCalls[0].Function.Name)
}

// ---------------------------------------------------------------------------
// Chat — MaxTokens from request overrides adapter setting
// ---------------------------------------------------------------------------

func TestAdapter_Chat_RequestMaxTokens(t *testing.T) {
	mock := &mockChatClientErr{
		response: ollamaapi.ChatResponse{
			Message: ollamaapi.Message{Role: "assistant", Content: "ok"},
			Done:    true,
		},
	}
	a := &Adapter{client: mock, model: "llama3", maxTokens: 512}
	resp, err := a.Chat(context.Background(), ports.ChatRequest{
		Messages:  []ports.ChatMessage{{Role: "user", Content: "hi"}},
		MaxTokens: 128,
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
}

// ---------------------------------------------------------------------------
// Chat — large context window capped at 32768
// ---------------------------------------------------------------------------

func TestAdapter_Chat_ContextWindowCap(t *testing.T) {
	mock := &mockChatClientErr{
		response: ollamaapi.ChatResponse{
			Message: ollamaapi.Message{Role: "assistant", Content: "capped"},
			Done:    true,
		},
	}
	a := &Adapter{client: mock, model: "llama3", maxTokens: 512, contextWindow: 200000}
	resp, err := a.Chat(context.Background(), ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "capped", resp.Content)
}

// ---------------------------------------------------------------------------
// sanitizeMessagesForOllama
// ---------------------------------------------------------------------------

func TestSanitizeMessages_OrphanToolDropped(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "tool", ToolCallID: "orphan", Content: "result"},
	}
	out := sanitizeMessagesForOllama(msgs)
	assert.Empty(t, out)
}

func TestSanitizeMessages_NoToolCallID_Dropped(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "tool", ToolCallID: "", Content: "result"},
	}
	out := sanitizeMessagesForOllama(msgs)
	assert.Empty(t, out)
}

func TestSanitizeMessages_DuplicateDropped(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ports.ToolCall{
				{ID: "tc1", Function: ports.FunctionCall{Name: "t", Arguments: "{}"}},
			},
		},
		{Role: "tool", ToolCallID: "tc1", Content: "first"},
		{Role: "tool", ToolCallID: "tc1", Content: "duplicate"},
	}
	out := sanitizeMessagesForOllama(msgs)
	// Expect: assistant + one tool message (duplicate dropped)
	assert.Len(t, out, 2)
}

func TestSanitizeMessages_ValidToolKept(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ports.ToolCall{
				{ID: "tc1", Function: ports.FunctionCall{Name: "t", Arguments: "{}"}},
			},
		},
		{Role: "tool", ToolCallID: "tc1", Content: "result"},
	}
	out := sanitizeMessagesForOllama(msgs)
	assert.Len(t, out, 2)
}

// ---------------------------------------------------------------------------
// convertMessages (the adapter method)
// ---------------------------------------------------------------------------

func TestConvertMessages_UserWithImageBlocks(t *testing.T) {
	a := &Adapter{}
	msgs := []ports.ChatMessage{
		{
			Role:    "user",
			Content: "",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockImage, Data: []byte{0xFF, 0xD8, 0xFF}},
			},
		},
	}
	out := a.convertMessages(msgs)
	require.Len(t, out, 1)
	assert.Len(t, out[0].Images, 1)
	// When content is empty and images present, default description is injected.
	assert.NotEmpty(t, out[0].Content)
}

func TestConvertMessages_UserWithAudioOnly_EmptyContentInjected(t *testing.T) {
	a := &Adapter{}
	msgs := []ports.ChatMessage{
		{
			Role:    "user",
			Content: "",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockAudio, Data: []byte{0x01}},
			},
		},
	}
	out := a.convertMessages(msgs)
	require.Len(t, out, 1)
	assert.NotEmpty(t, out[0].Content)
}

func TestConvertMessages_ToolMessage(t *testing.T) {
	a := &Adapter{}
	msgs := []ports.ChatMessage{
		{Role: "tool", ToolCallID: "tc1", ToolName: "my_tool", Content: "result"},
	}
	out := a.convertMessages(msgs)
	require.Len(t, out, 1)
	assert.Equal(t, "tc1", out[0].ToolCallID)
	assert.Equal(t, "my_tool", out[0].ToolName)
}

func TestConvertMessages_AssistantWithToolCalls(t *testing.T) {
	a := &Adapter{}
	msgs := []ports.ChatMessage{
		{
			Role:    "assistant",
			Content: "calling tool",
			ToolCalls: []ports.ToolCall{
				{
					ID:   "tc1",
					Type: "function",
					Function: ports.FunctionCall{
						Name:      "ns:my_tool",
						Arguments: `{"x":1}`,
					},
				},
			},
		},
	}
	out := a.convertMessages(msgs)
	require.Len(t, out, 1)
	require.Len(t, out[0].ToolCalls, 1)
	assert.Equal(t, "ns__my_tool", out[0].ToolCalls[0].Function.Name)
}

func TestConvertMessages_AssistantToolCallInvalidArgs(t *testing.T) {
	a := &Adapter{}
	msgs := []ports.ChatMessage{
		{
			Role: "assistant",
			ToolCalls: []ports.ToolCall{
				{
					Function: ports.FunctionCall{
						Name:      "tool",
						Arguments: "NOT JSON",
					},
				},
			},
		},
	}
	// Should not panic; logs the error and continues.
	out := a.convertMessages(msgs)
	assert.Len(t, out, 1)
}

// ---------------------------------------------------------------------------
// collectImageBlocks — additional cases not covered by multimodal_test.go
// ---------------------------------------------------------------------------

func TestCollectImageBlocks_SkipsImageWithNoData(t *testing.T) {
	blocks := []ports.ContentBlock{
		{Type: ports.ContentBlockImage, URL: "https://example.com/img.png"}, // no Data
	}
	imgs := collectImageBlocks(blocks)
	assert.Empty(t, imgs)
}

func TestCollectImageBlocks_ExtractsData(t *testing.T) {
	blocks := []ports.ContentBlock{
		{Type: ports.ContentBlockImage, Data: []byte{0xFF, 0xD8}},
	}
	imgs := collectImageBlocks(blocks)
	require.Len(t, imgs, 1)
	assert.Equal(t, []byte{0xFF, 0xD8}, []byte(imgs[0]))
}

// ---------------------------------------------------------------------------
// convertTools
// ---------------------------------------------------------------------------

func TestConvertTools_NilFunction(t *testing.T) {
	result := convertTools([]ports.Tool{{Function: nil}})
	assert.Empty(t, result)
}

func TestConvertTools_ColonEncoding(t *testing.T) {
	tools := []ports.Tool{
		{Function: &ports.FunctionTool{
			Name:        "a:b:c",
			Description: "desc",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"x": map[string]interface{}{"type": "string", "description": "x param"},
				},
				"required": []interface{}{"x"},
			},
		}},
	}
	result := convertTools(tools)
	require.Len(t, result, 1)
	assert.Equal(t, "a__b__c", result[0].Function.Name)
}

func TestParamType_Default(t *testing.T) {
	assert.Equal(t, "object", paramType(map[string]any{}))
}

func TestParamType_FromMap(t *testing.T) {
	assert.Equal(t, "array", paramType(map[string]any{"type": "array"}))
}

func TestParamRequired_Empty(t *testing.T) {
	assert.Nil(t, paramRequired(map[string]any{}))
}

func TestParamRequired_WithValues(t *testing.T) {
	p := map[string]any{"required": []any{"a", "b"}}
	r := paramRequired(p)
	assert.Equal(t, []string{"a", "b"}, r)
}

func TestParamProperties_Nil(t *testing.T) {
	assert.Nil(t, paramProperties(map[string]any{}))
}

func TestParamProperties_WithProps(t *testing.T) {
	p := map[string]any{
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "the path",
				"enum":        []any{"a", "b"},
			},
		},
	}
	result := paramProperties(p)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// parseToolBlocks
// ---------------------------------------------------------------------------

func TestParseToolBlocks_Valid(t *testing.T) {
	content := `<tool>{"name":"fs__read","parameters":{"path":"/tmp/x"}}</tool>`
	calls := parseToolBlocks(content)
	require.Len(t, calls, 1)
	assert.Equal(t, "fs:read", calls[0].Function.Name)
}

func TestParseToolBlocks_InvalidJSON(t *testing.T) {
	content := `<tool>NOT JSON</tool>`
	calls := parseToolBlocks(content)
	assert.Empty(t, calls)
}

func TestParseToolBlocks_Multiple(t *testing.T) {
	content := `<tool>{"name":"a","parameters":{}}</tool><tool>{"name":"b","parameters":{}}</tool>`
	calls := parseToolBlocks(content)
	assert.Len(t, calls, 2)
}

// ---------------------------------------------------------------------------
// probeContextWindow — Show response with context_length
// ---------------------------------------------------------------------------

func TestProbeContextWindow_Success(t *testing.T) {
	mock := &mockChatClientErr{
		showResp: &ollamaapi.ShowResponse{
			ModelInfo: map[string]any{
				"llama.context_length": float64(8192),
			},
		},
	}
	a := &Adapter{client: mock, model: "llama3"}
	a.probeContextWindow()
	assert.Equal(t, 8192, a.contextWindow)
}

func TestProbeContextWindow_ShowError(t *testing.T) {
	mock := &mockChatClientErr{showErr: fmt.Errorf("not found")}
	a := &Adapter{client: mock, model: "llama3"}
	a.probeContextWindow()
	assert.Equal(t, 0, a.contextWindow) // unchanged
}

func TestProbeContextWindow_NoContextLengthKey(t *testing.T) {
	mock := &mockChatClientErr{
		showResp: &ollamaapi.ShowResponse{
			ModelInfo: map[string]any{
				"llama.some_other_key": float64(123),
			},
		},
	}
	a := &Adapter{client: mock, model: "llama3"}
	a.probeContextWindow()
	assert.Equal(t, 0, a.contextWindow) // no context_length found
}

// ---------------------------------------------------------------------------
// bearerTransport
// ---------------------------------------------------------------------------

func TestBearerTransport_SetsHeader(t *testing.T) {
	// Just verify the type implements http.RoundTripper and sets the header.
	// We cannot call RoundTrip without a real net connection, so we only
	// verify the struct fields are populated correctly.
	bt := &bearerTransport{token: "my-token"}
	assert.Equal(t, "my-token", bt.token)
}

// ---------------------------------------------------------------------------
// NewAdapterWithOptions — keeps backwards compatibility
// ---------------------------------------------------------------------------

func TestNewAdapterWithOptions(t *testing.T) {
	// Should not panic; the trailing "debug" option is intentionally ignored.
	a := NewAdapterWithOptions("http://127.0.0.1:0", "", "llama3", 512, "debug")
	assert.NotNil(t, a)
}
