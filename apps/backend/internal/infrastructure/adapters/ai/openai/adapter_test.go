// Copyright (c) OpenLobster contributors. See LICENSE for details.

package openai

import (
	"testing"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// sanitizeMessages
// ---------------------------------------------------------------------------

func TestSanitizeMessages_PassesNonToolMessages(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi"},
		{Role: "assistant", Content: "Hello!"},
	}
	out := sanitizeMessages(msgs)
	assert.Len(t, out, 3)
}

func TestSanitizeMessages_DropsToolMessageWithEmptyID(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role: "assistant",
			ToolCalls: []ports.ToolCall{
				{ID: "call_abc", Type: "function", Function: ports.FunctionCall{Name: "my_tool", Arguments: "{}"}},
			},
		},
		{Role: "tool", ToolCallID: "", Content: "result"},
	}
	out := sanitizeMessages(msgs)
	require.Len(t, out, 1)
	assert.Equal(t, "assistant", out[0].Role)
}

func TestSanitizeMessages_DropsOrphanToolMessage(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "assistant", Content: "sure"},
		{Role: "tool", ToolCallID: "call_ghost", Content: "orphan"},
	}
	out := sanitizeMessages(msgs)
	require.Len(t, out, 1)
	assert.Equal(t, "assistant", out[0].Role)
}

func TestSanitizeMessages_KeepsValidToolMessage(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role: "assistant",
			ToolCalls: []ports.ToolCall{
				{ID: "call_123", Type: "function", Function: ports.FunctionCall{Name: "my_tool", Arguments: "{}"}},
			},
		},
		{Role: "tool", ToolCallID: "call_123", Content: "ok"},
	}
	out := sanitizeMessages(msgs)
	assert.Len(t, out, 2)
	assert.Equal(t, "call_123", out[1].ToolCallID)
}

func TestSanitizeMessages_MultipleToolCallsAllValid(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role: "assistant",
			ToolCalls: []ports.ToolCall{
				{ID: "call_1", Type: "function", Function: ports.FunctionCall{Name: "tool_a", Arguments: "{}"}},
				{ID: "call_2", Type: "function", Function: ports.FunctionCall{Name: "tool_b", Arguments: "{}"}},
			},
		},
		{Role: "tool", ToolCallID: "call_1", Content: "result_a"},
		{Role: "tool", ToolCallID: "call_2", Content: "result_b"},
	}
	out := sanitizeMessages(msgs)
	assert.Len(t, out, 3)
}

func TestSanitizeMessages_MixedValidAndOrphan(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role: "assistant",
			ToolCalls: []ports.ToolCall{
				{ID: "call_good", Type: "function", Function: ports.FunctionCall{Name: "tool_a", Arguments: "{}"}},
			},
		},
		{Role: "tool", ToolCallID: "call_good", Content: "ok"},
		{Role: "tool", ToolCallID: "call_bad", Content: "orphan"},
		{Role: "tool", ToolCallID: "", Content: "empty id"},
	}
	out := sanitizeMessages(msgs)
	require.Len(t, out, 2)
	assert.Equal(t, "call_good", out[1].ToolCallID)
}
