// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import "testing"

func TestSmokeIssueSummary(t *testing.T) {
	report := Report{
		Plugins: []PluginReport{
			{Issues: []Issue{{Rule: smokeFailRule}, {Rule: "other"}}},
			{Issues: []Issue{{Rule: smokeFailRule}}},
		},
	}
	failed := smokeIssueSummary(report)
	if failed != 2 {
		t.Fatalf("failed = %d, want 2", failed)
	}
}

func TestParseAIChatOutput(t *testing.T) {
	raw := []byte(`{"content":"ok","tool_calls":[{"id":"1"}],"error":""}`)
	out, err := parseAIChatOutput(raw)
	if err != nil {
		t.Fatalf("parseAIChatOutput error: %v", err)
	}
	if out.Content != "ok" {
		t.Fatalf("content = %q, want %q", out.Content, "ok")
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("tool_calls len = %d, want 1", len(out.ToolCalls))
	}
	if out.Error != "" {
		t.Fatalf("error = %q, want empty", out.Error)
	}
}

func TestParseAIChatOutputInvalidJSON(t *testing.T) {
	if _, err := parseAIChatOutput([]byte(`{"content":`)); err == nil {
		t.Fatalf("expected JSON parse error")
	}
}

func TestFirstAIChatToolCall(t *testing.T) {
	rawCalls := []any{
		map[string]any{
			"id": "call_1",
			"function": map[string]any{
				"name":      "math:add",
				"arguments": `{"a":2,"b":3}`,
			},
		},
	}

	call, ok := firstAIChatToolCall(rawCalls)
	if !ok {
		t.Fatalf("expected valid tool call")
	}
	if call.ID != "call_1" || call.Name != "math:add" {
		t.Fatalf("unexpected call: %+v", call)
	}
	if call.Arguments != `{"a":2,"b":3}` {
		t.Fatalf("unexpected arguments: %q", call.Arguments)
	}
}

func TestFirstAIChatToolCallObjectArgs(t *testing.T) {
	rawCalls := []any{
		map[string]any{
			"id": "call_2",
			"function": map[string]any{
				"name": "math:add",
				"arguments": map[string]any{
					"a": 7,
					"b": 8,
				},
			},
		},
	}

	call, ok := firstAIChatToolCall(rawCalls)
	if !ok {
		t.Fatalf("expected valid tool call")
	}
	if call.Arguments == "" {
		t.Fatalf("expected serialized arguments")
	}
}

func TestToolResultForCall(t *testing.T) {
	call := aiSmokeToolCall{ID: "call_1", Name: "math:add", Arguments: `{"a":2,"b":3}`}
	result := toolResultForCall(call)
	if result != `{"sum":5}` {
		t.Fatalf("result = %s, want %s", result, `{"sum":5}`)
	}
}

func TestHasAIChatSignal(t *testing.T) {
	if hasAIChatSignal(aiChatOutput{}) {
		t.Fatalf("expected false for empty output")
	}
	if !hasAIChatSignal(aiChatOutput{Content: "OK"}) {
		t.Fatalf("expected true for content")
	}
	if !hasAIChatSignal(aiChatOutput{ToolCalls: []any{map[string]any{"id": "x"}}}) {
		t.Fatalf("expected true for tool calls")
	}
}


func TestFillMissingConfigFromEnv(t *testing.T) {
	t.Setenv("OPENLOBSTER_SMOKE_TEST_MODEL", "model-x")
	t.Setenv("OPENLOBSTER_SMOKE_TEST_API_KEY", "secret")

	cfg := map[string]any{"model": ""}
	fillMissingConfigFromEnv(cfg, map[string][]string{
		"model":   {"OPENLOBSTER_SMOKE_TEST_MODEL"},
		"api_key": {"OPENLOBSTER_SMOKE_TEST_API_KEY"},
	})

	if got := cfg["model"]; got != "model-x" {
		t.Fatalf("model = %v, want model-x", got)
	}
	if got := cfg["api_key"]; got != "secret" {
		t.Fatalf("api_key = %v, want secret", got)
	}
}
