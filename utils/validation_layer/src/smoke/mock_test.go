// Copyright (c) OpenLobster contributors. See LICENSE for details.

// mock_test.go provides a shared stub PluginClient for smoke package tests.
package smoke_test

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
)

// mockClient is a test double implementing protocol.PluginClient.
type mockClient struct {
	info      protocol.PluginInfo
	functions map[string]func(any) (json.RawMessage, error)
}

func newMockClient(info protocol.PluginInfo) *mockClient {
	return &mockClient{
		info:      info,
		functions: make(map[string]func(any) (json.RawMessage, error)),
	}
}

func (m *mockClient) Info() protocol.PluginInfo { return m.info }

func (m *mockClient) HasFunction(fn string) bool {
	_, ok := m.functions[fn]
	return ok
}

func (m *mockClient) CallJSON(fn string, input any) (json.RawMessage, error) {
	handler, ok := m.functions[fn]
	if !ok {
		return nil, errors.New("function not found: " + fn)
	}
	return handler(input)
}

func (m *mockClient) CallString(fn string, input any) (string, error) {
	raw, err := m.CallJSON(fn, input)
	if err != nil {
		return "", err
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, nil
	}
	return string(raw), nil
}

func (m *mockClient) Close() error { return nil }

func (m *mockClient) SetVictoryCode(code string) {}

func (m *mockClient) WaitForVictory(timeout time.Duration) bool { return true }

// register adds a simple handler that always returns a JSON-encoded value.
func (m *mockClient) register(fn string, result any) {
	m.functions[fn] = func(_ any) (json.RawMessage, error) {
		b, err := json.Marshal(result)
		return b, err
	}
}

// registerErr adds a handler that always returns an error.
func (m *mockClient) registerErr(fn string, err error) {
	m.functions[fn] = func(_ any) (json.RawMessage, error) {
		return nil, err
	}
}
