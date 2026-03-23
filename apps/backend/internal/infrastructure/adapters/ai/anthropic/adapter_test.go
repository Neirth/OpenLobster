// Copyright (c) OpenLobster contributors. See LICENSE for details.

package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// encodeToolName / decodeToolName
// ---------------------------------------------------------------------------

func TestEncodeDecodeToolName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		encoded string
	}{
		{"simple", "my_tool", "my_tool"},
		{"colon", "ns:tool", "ns__tool"},
		{"multi", "a:b:c", "a__b__c"},
		{"no change", "no_colon_here", "no_colon_here"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := encodeToolName(tc.input)
			assert.Equal(t, tc.encoded, got)
			assert.Equal(t, tc.input, decodeToolName(got))
		})
	}
}

// ---------------------------------------------------------------------------
// convertTools
// ---------------------------------------------------------------------------

func TestConvertTools_NilFunction(t *testing.T) {
	tools := []ports.Tool{
		{Function: nil},
	}
	result := convertTools(tools)
	assert.Empty(t, result)
}

func TestConvertTools_WithProperties(t *testing.T) {
	tools := []ports.Tool{
		{
			Function: &ports.FunctionTool{
				Name:        "read_file",
				Description: "Read a file",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}
	result := convertTools(tools)
	require.Len(t, result, 1)
	require.NotNil(t, result[0].OfTool)
	assert.Equal(t, "read_file", result[0].OfTool.Name)
}

func TestConvertTools_WithColonInName(t *testing.T) {
	tools := []ports.Tool{
		{
			Function: &ports.FunctionTool{
				Name:        "fs:read_file",
				Description: "Read",
				Parameters:  map[string]interface{}{},
			},
		},
	}
	result := convertTools(tools)
	require.Len(t, result, 1)
	assert.Equal(t, "fs__read_file", result[0].OfTool.Name)
}

func TestConvertTools_NoPropertiesKey(t *testing.T) {
	tools := []ports.Tool{
		{
			Function: &ports.FunctionTool{
				Name:        "simple",
				Description: "No props",
				Parameters:  map[string]interface{}{"type": "object"},
			},
		},
	}
	result := convertTools(tools)
	require.Len(t, result, 1)
}

// ---------------------------------------------------------------------------
// convertMessages – assistant role with tool calls
// ---------------------------------------------------------------------------

func TestConvertMessages_AssistantWithToolCalls(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ports.ToolCall{
				{
					ID:   "tc1",
					Type: "function",
					Function: ports.FunctionCall{
						Name:      "ns:my_tool",
						Arguments: `{"key":"value"}`,
					},
				},
			},
		},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_AssistantEmptyContent(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "assistant", Content: ""},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_AssistantWithContent(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "assistant", Content: "Hello!"},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_ToolRole_TextContent(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "tool", ToolCallID: "tc1", Content: "result text"},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_ToolRole_WithBlocks(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:       "tool",
			ToolCallID: "tc2",
			Content:    "",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockText, Text: "text result"},
			},
		},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_ToolRole_WithImageURLBlock(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:       "tool",
			ToolCallID: "tc3",
			Content:    "",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockImage, URL: "https://example.com/img.png", MIMEType: "image/png"},
			},
		},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_ToolRole_WithImageDataBlock(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:       "tool",
			ToolCallID: "tc4",
			Content:    "",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockImage, Data: []byte{0xFF, 0xD8}, MIMEType: "image/jpeg"},
			},
		},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_ToolRole_WithImageAndText(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:       "tool",
			ToolCallID: "tc5",
			Content:    "",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockImage, URL: "https://example.com/img.png", Text: "caption"},
			},
		},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_ToolRole_AudioBlock_FallsBackToText(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:       "tool",
			ToolCallID: "tc6",
			Content:    "fallback",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockAudio, Text: "audio text fallback"},
			},
		},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_ToolRole_EmptyResultFallsToContent(t *testing.T) {
	// All blocks produce no output → use Content
	msgs := []ports.ChatMessage{
		{
			Role:       "tool",
			ToolCallID: "tc7",
			Content:    "the result",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockImage}, // no URL, no Data
			},
		},
	}
	_, params := convertMessages(msgs)
	require.Len(t, params, 1)
}

func TestConvertMessages_MultipleSystemMessages(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "system", Content: "Instruction 1."},
		{Role: "system", Content: "Instruction 2."},
		{Role: "user", Content: "Hi"},
	}
	sys, params := convertMessages(msgs)
	assert.Len(t, sys, 2)
	assert.Len(t, params, 1)
}

func TestConvertMessages_SystemEmptyContentSkipped(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "system", Content: ""},
		{Role: "user", Content: "Hi"},
	}
	sys, _ := convertMessages(msgs)
	assert.Empty(t, sys)
}

// ---------------------------------------------------------------------------
// parseResponse
// ---------------------------------------------------------------------------

func TestParseResponse_TextBlock(t *testing.T) {
	resp := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: "Hello world"},
		},
		StopReason: "end_turn",
	}
	cr := parseResponse(resp)
	assert.Equal(t, "Hello world", cr.Content)
	assert.Equal(t, "stop", cr.StopReason)
}

func TestParseResponse_MaxTokens(t *testing.T) {
	resp := &anthropic.Message{
		StopReason: "max_tokens",
	}
	cr := parseResponse(resp)
	assert.Equal(t, "max_tokens", cr.StopReason)
}

func TestParseResponse_ToolUse(t *testing.T) {
	inputMap := map[string]interface{}{"path": "/tmp/file.txt"}
	inputJSON, _ := json.Marshal(inputMap)
	resp := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "call_1",
				Name:  "fs__read_file",
				Input: json.RawMessage(inputJSON),
			},
		},
		StopReason: "tool_use",
	}
	cr := parseResponse(resp)
	require.Len(t, cr.ToolCalls, 1)
	assert.Equal(t, "call_1", cr.ToolCalls[0].ID)
	assert.Equal(t, "fs:read_file", cr.ToolCalls[0].Function.Name)
	assert.Equal(t, "tool_use", cr.StopReason)
}

func TestParseResponse_ThinkingBlock(t *testing.T) {
	resp := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "thinking", Thinking: "I should help the user."},
			{Type: "text", Text: "Sure!"},
		},
		StopReason: "end_turn",
	}
	cr := parseResponse(resp)
	assert.Equal(t, "Sure!", cr.Content)
	assert.Equal(t, "stop", cr.StopReason)
}

func TestParseResponse_EmptyTextBlockSkipped(t *testing.T) {
	resp := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: ""},
			{Type: "text", Text: "actual"},
		},
		StopReason: "end_turn",
	}
	cr := parseResponse(resp)
	assert.Equal(t, "actual", cr.Content)
}

func TestParseResponse_MultipleTextBlocks(t *testing.T) {
	resp := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "World"},
		},
		StopReason: "end_turn",
	}
	cr := parseResponse(resp)
	assert.Equal(t, "Hello World", cr.Content)
}

func TestParseResponse_StopSequence(t *testing.T) {
	resp := &anthropic.Message{
		StopReason: "stop_sequence",
	}
	cr := parseResponse(resp)
	assert.Equal(t, "stop_sequence", cr.StopReason)
}

// ---------------------------------------------------------------------------
// Adapter — static capability checks (no network)
// ---------------------------------------------------------------------------

func TestAdapter_SupportsAudio(t *testing.T) {
	// Use a minimal adapter (probeContextWindow will fail gracefully)
	a := &Adapter{model: "claude-3-5-sonnet-20241022", maxTokens: 4096}
	assert.False(t, a.SupportsAudioInput())
	assert.False(t, a.SupportsAudioOutput())
}

func TestAdapter_GetMaxTokens(t *testing.T) {
	a := &Adapter{maxTokens: 2048}
	assert.Equal(t, 2048, a.GetMaxTokens())
}

func TestAdapter_GetContextWindow_Default(t *testing.T) {
	a := &Adapter{}
	assert.Equal(t, 200000, a.GetContextWindow())
}

func TestAdapter_GetContextWindow_Override(t *testing.T) {
	a := &Adapter{}
	a.OverrideContextWindow(128000)
	assert.Equal(t, 128000, a.GetContextWindow())
}

func TestAdapter_ChatToAudio_ForwardsToChat(t *testing.T) {
	// ChatToAudio delegates to Chat; use a stub that records calls.
	// We cannot easily mock the SDK client, so test that the adapter
	// propagates errors returned by Chat.
	a := &Adapter{model: "test", maxTokens: 10, client: anthropic.NewClient()}
	_, err := a.ChatToAudio(context.Background(), ports.ChatRequest{
		Messages: []ports.ChatMessage{{Role: "user", Content: "hi"}},
	})
	// Will get an auth/network error — that proves the delegation path is exercised.
	assert.Error(t, err)
}

func TestAdapter_ChatWithAudio_ForwardsToChat(t *testing.T) {
	a := &Adapter{model: "test", maxTokens: 10, client: anthropic.NewClient()}
	_, err := a.ChatWithAudio(context.Background(), ports.ChatRequestWithAudio{
		Messages: []ports.ChatMessage{{Role: "user", Content: "hi"}},
	})
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// NewAdapter helpers — defaultMaxTokens guard
// ---------------------------------------------------------------------------

func TestNewAdapterWithBaseURL_DefaultMaxTokens(t *testing.T) {
	// maxTokens <= 0 must be replaced with defaultMaxTokens.
	// NewAdapterWithBaseURL will call probeContextWindow which will fail
	// gracefully because the URL is fake.
	a := NewAdapterWithBaseURL("http://127.0.0.1:0", "key", "claude-3-opus", 0, "")
	assert.Equal(t, defaultMaxTokens, a.maxTokens)
}
