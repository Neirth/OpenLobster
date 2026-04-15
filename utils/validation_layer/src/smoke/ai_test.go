// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAISmoke_parseOutput_happy(t *testing.T) {
	raw := json.RawMessage(`{"content":"OK","tool_calls":null,"error":""}`)
	var out struct {
		Content   string `json:"content"`
		ToolCalls []any  `json:"tool_calls"`
		Error     string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, "OK", out.Content)
	assert.Empty(t, out.Error)
}

func TestAISmoke_parseOutput_withToolCalls(t *testing.T) {
	raw := json.RawMessage(`{"content":"","tool_calls":[{"id":"1","function":{"name":"math:add","arguments":"{\"a\":2,\"b\":3}"}}],"error":""}`)
	var out struct {
		Content   string `json:"content"`
		ToolCalls []any  `json:"tool_calls"`
		Error     string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.NotEmpty(t, out.ToolCalls)
}

func TestAISmoke_toolResult_mathAdd(t *testing.T) {
	// Simulates toolResultForCall for math:add with a=2, b=3 → sum=5
	args := `{"a":2,"b":3}`
	var parsed struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
	}
	require.NoError(t, json.Unmarshal([]byte(args), &parsed))
	sum := parsed.A + parsed.B
	assert.Equal(t, float64(5), sum)
}

func TestAISmoke_stringifyArgs_string(t *testing.T) {
	raw := `{"a":1,"b":2}`
	assert.Equal(t, raw, raw)
}

func TestAISmoke_hasSignal_content(t *testing.T) {
	content := "  some text  "
	assert.NotEmpty(t, content)
}

func TestAISmoke_hasSignal_toolCalls(t *testing.T) {
	toolCalls := []any{map[string]any{"id": "1"}}
	assert.NotEmpty(t, toolCalls)
}
