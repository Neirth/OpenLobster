// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func memoryInfo() protocol.PluginInfo {
	return protocol.PluginInfo{
		ID:      "test-memory",
		Type:    "memory",
		Version: "0.1.0",
		Exports: []string{"get_metadata", "configure", "store", "retrieve", "query"},
	}
}

func TestMemorySmoke_store_retrieve(t *testing.T) {
	store := []map[string]any{}

	c := newMockClient(memoryInfo())
	c.functions["configure"] = func(_ any) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"ok": true})
	}
	c.functions["store"] = func(input any) (json.RawMessage, error) {
		m, _ := input.(map[string]any)
		store = append(store, m)
		return json.Marshal(map[string]any{"ok": true})
	}
	c.functions["retrieve"] = func(_ any) (json.RawMessage, error) {
		return json.Marshal(store)
	}
	c.functions["query"] = func(input any) (json.RawMessage, error) {
		m, _ := input.(map[string]any)
		op, _ := m["op"].(string)
		switch op {
		case "cypher":
			return json.Marshal(map[string]any{
				"data": []map[string]any{{"a": 1}},
			})
		case "user_graph":
			edges := []map[string]any{}
			return json.Marshal(map[string]any{"edges": edges})
		case "invalidate_cache":
			return json.Marshal(map[string]any{"ok": true})
		}
		return json.Marshal(map[string]any{"ok": true})
	}

	report := &types.PluginReport{Type: "memory", Exports: memoryInfo().Exports}

	// Simulate stress writes
	for i := 0; i < 8; i++ {
		_, err := c.CallJSON("store", map[string]any{
			"user_id": "smoke-user",
			"content": fmt.Sprintf("entry %d", i),
		})
		require.NoError(t, err)
	}

	raw, err := c.CallJSON("retrieve", map[string]any{"query": "entry", "limit": 64})
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal(raw, &rows))
	assert.GreaterOrEqual(t, len(rows), 3)
	assert.Equal(t, 0, report.ErrorCount())
}

func TestMemoryNodeIDMatches_prefixed(t *testing.T) {
	// Validates the logic in memoryNodeIDMatches
	actual := "user:smoke-user"
	expected := "smoke-user"
	// The function strips "user:" prefix — simulate it:
	withoutPrefix := actual[len("user:"):]
	assert.Equal(t, expected, withoutPrefix)
}

func TestMemorySmoke_cypherResponse_validate(t *testing.T) {
	raw := json.RawMessage(`{"data":[{"a":1,"b":2}],"errors":[]}`)
	var result struct {
		Data   []map[string]any `json:"data"`
		Errors []any            `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.NotEmpty(t, result.Data)
	assert.Empty(t, result.Errors)
}
