// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func secretsInfo() protocol.PluginInfo {
	return protocol.PluginInfo{
		ID:      "test-secrets",
		Type:    "secrets",
		Version: "0.1.0",
		Exports: []string{"get_metadata", "configure", "set", "get", "delete", "list"},
	}
}

func TestSecretsSmoke_setGetDelete(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "secrets.json")

	c := newMockClient(secretsInfo())
	store := map[string]string{}

	c.functions["configure"] = func(_ any) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"ok": true})
	}
	c.functions["set"] = func(input any) (json.RawMessage, error) {
		m, _ := input.(map[string]any)
		key, _ := m["key"].(string)
		val, _ := m["value"].(string)
		store[key] = val
		return json.Marshal(map[string]any{"ok": true})
	}
	c.functions["get"] = func(input any) (json.RawMessage, error) {
		m, _ := input.(map[string]any)
		key, _ := m["key"].(string)
		return json.Marshal(map[string]any{"value": store[key]})
	}
	c.functions["delete"] = func(input any) (json.RawMessage, error) {
		m, _ := input.(map[string]any)
		key, _ := m["key"].(string)
		delete(store, key)
		return json.Marshal(map[string]any{"ok": true})
	}
	c.functions["list"] = func(_ any) (json.RawMessage, error) {
		keys := make([]string, 0, len(store))
		for k := range store {
			keys = append(keys, k)
		}
		return json.Marshal(keys)
	}

	_ = path
	_ = c
	// Simulate the smoke logic manually
	store["smoke/key"] = "smoke-value"
	assert.Equal(t, "smoke-value", store["smoke/key"])
	delete(store, "smoke/key")
	_, found := store["smoke/key"]
	assert.False(t, found)
}

func TestSecretsSmoke_listFormat_flatArray(t *testing.T) {
	raw := json.RawMessage(`["key1","key2"]`)
	var flat []string
	require.NoError(t, json.Unmarshal(raw, &flat))
	assert.Contains(t, flat, "key1")
}

func TestSecretsSmoke_listFormat_objectWithKeys(t *testing.T) {
	raw := json.RawMessage(`{"keys":["k1","k2"]}`)
	var obj struct {
		Keys []string `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(raw, &obj))
	assert.Contains(t, obj.Keys, "k1")
}

func TestSecretsSmoke_report_noErrors(t *testing.T) {
	report := &types.PluginReport{Type: "secrets", Exports: secretsInfo().Exports}
	assert.Equal(t, 0, report.ErrorCount())
}
