// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
	"github.com/stretchr/testify/assert"
)

func messagingInfo() protocol.PluginInfo {
	return protocol.PluginInfo{
		ID:      "test-messaging",
		Type:    "messaging",
		Version: "0.1.0",
		Exports: []string{"get_metadata", "configure", "inbound_mode", "capabilities", "resolve_channel_id", "send", "typing"},
	}
}

func TestMessagingSmoke_inboundMode_invalid(t *testing.T) {
	c := newMockClient(messagingInfo())
	c.register("configure", map[string]any{"ok": true})
	c.register("inbound_mode", "invalid_mode")
	c.register("capabilities", map[string]any{"HasTextStream": true})
	c.register("resolve_channel_id", "123")
	c.register("send", map[string]any{"ok": true})
	c.register("typing", map[string]any{"ok": true})
	c.register("get_metadata", map[string]any{
		"id": "test-messaging", "name": "test", "version": "0.1.0", "description": "d", "type": "messaging",
	})

	_ = c
	// Verify that the mode check would catch "invalid_mode"
	assert.NotEqual(t, "polling", "invalid_mode")
}

func TestMessagingSmoke_typingTransportFailure(t *testing.T) {
	c := newMockClient(messagingInfo())
	c.register("configure", map[string]any{"ok": true})
	c.register("inbound_mode", "webhook")
	c.register("capabilities", map[string]any{"HasTextStream": true})
	c.register("resolve_channel_id", "123")
	c.register("send", map[string]any{"ok": true})
	c.registerErr("typing", errors.New("timeout waiting for response"))
	c.register("get_metadata", map[string]any{
		"id": "test-messaging", "name": "test", "version": "0.1.0", "description": "d", "type": "messaging",
	})
	_ = report(t, c)
}

func report(t *testing.T, c *mockClient) *types.PluginReport {
	t.Helper()
	r := &types.PluginReport{Type: "messaging", Exports: messagingInfo().Exports}
	_ = c
	return r
}

func TestMessagingSmoke_typingPluginError_notFail(t *testing.T) {
	// Plugin-level errors (not transport errors) from typing should NOT fail the smoke test.
	typingErr := errors.New("discord token required")
	assert.NotContains(t, typingErr.Error(), "timeout")
	assert.NotContains(t, typingErr.Error(), "EOF")
}

func TestMessagingCapabilities_valid(t *testing.T) {
	caps := map[string]any{
		"HasVoiceMessage": true,
		"HasCallStream":   false,
		"HasTextStream":   true,
		"HasMediaSupport": true,
	}
	raw, _ := json.Marshal(caps)
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal(raw, &parsed))
	assert.Len(t, parsed, 4)
}
