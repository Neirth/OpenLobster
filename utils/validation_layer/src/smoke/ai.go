// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/neirth/openlobster/utils/validation_layer/src/config"
	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
)

func runAISmoke(client protocol.PluginClient, report *types.PluginReport, opts types.ValidateOptions, file string) {
	cfg := cloneMap(opts.SmokeConfig)
	config.FillMissingConfigFromEnv(cfg, map[string][]string{
		"api_key":  {"OPENLOBSTER_SMOKE_AI_API_KEY"},
		"base_url": {"OPENLOBSTER_SMOKE_AI_BASE_URL"},
		"endpoint": {"OPENLOBSTER_SMOKE_AI_ENDPOINT"},
		"model":    {"OPENLOBSTER_SMOKE_AI_MODEL"},
	})

	model := config.ConfigString(cfg, "model")
	if model == "" {
		addSmokeFailure(report, "ai", "missing model: provide via --config or OPENLOBSTER_SMOKE_AI_MODEL", file)
		return
	}

	if err := configurePlugin(client, cfg); err != nil {
		addSmokeFailure(report, "ai", err.Error(), file)
		return
	}

	systemPrompt := "You are running an OpenLobster automated smoke test. Follow instructions exactly. Keep responses minimal. When asked to call a tool, emit a tool call with precise arguments."
	noToolsPrompt := "Smoke test step 1/3: return exactly OK."
	noToolsRaw, err := client.CallJSON("chat", map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": noToolsPrompt},
		},
		"max_tokens": 256,
		"config":     cfg,
	})
	if err != nil {
		addSmokeFailure(report, "ai.chat.no-tools", err.Error(), file)
		return
	}

	noToolsOut, err := parseAIChatOutput(noToolsRaw)
	if err != nil {
		addSmokeFailure(report, "ai.chat.no-tools", "invalid JSON", file)
		return
	}
	if strings.TrimSpace(noToolsOut.Error) != "" {
		addSmokeFailure(report, "ai.chat.no-tools", noToolsOut.Error, file)
		return
	}
	if !hasAIChatSignal(noToolsOut) {
		addSmokeFailure(report, "ai.chat.no-tools", "empty response content", file)
		return
	}

	tool := map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "math:add",
			"description": "Add two numbers",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"a": map[string]any{"type": "number"},
					"b": map[string]any{"type": "number"},
				},
				"required": []string{"a", "b"},
			},
		},
	}

	withToolsRaw, err := client.CallJSON("chat", map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": "Smoke test step 2/3: call math:add with a=2 and b=3."},
		},
		"tools":      []map[string]any{tool},
		"max_tokens": 256,
		"config":     cfg,
	})
	if err != nil {
		addSmokeFailure(report, "ai.chat.with-tools", err.Error(), file)
		return
	}

	withToolsOut, err := parseAIChatOutput(withToolsRaw)
	if err != nil {
		addSmokeFailure(report, "ai.chat.with-tools", "invalid JSON", file)
		return
	}
	if strings.TrimSpace(withToolsOut.Error) != "" {
		addSmokeFailure(report, "ai.chat.with-tools", withToolsOut.Error, file)
		return
	}
	if !hasAIChatSignal(withToolsOut) {
		addSmokeFailure(report, "ai.chat.with-tools", "empty content and no tool_calls", file)
		return
	}

	toolCall, hasToolCall := firstAIChatToolCall(withToolsOut.ToolCalls)

	var historyMessages []map[string]any
	historyMessages = append(historyMessages,
		map[string]any{"role": "system", "content": systemPrompt},
		map[string]any{"role": "user", "content": "Smoke test step 2/3: call math:add with a=2 and b=3."},
		map[string]any{"role": "assistant", "content": strings.TrimSpace(withToolsOut.Content), "tool_calls": withToolsOut.ToolCalls},
	)
	if hasToolCall {
		historyMessages = append(historyMessages, map[string]any{
			"role":         "tool",
			"tool_call_id": toolCall.ID,
			"content":      toolResultForCall(toolCall),
		})
	}
	historyMessages = append(historyMessages,
		map[string]any{"role": "user", "content": "Smoke test step 3/3: use the tool result and answer with one short final sentence."},
	)

	historyRaw, err := client.CallJSON("chat", map[string]any{
		"model":      model,
		"messages":   historyMessages,
		"tools":      []map[string]any{tool},
		"max_tokens": 256,
		"config":     cfg,
	})
	if err != nil {
		addSmokeFailure(report, "ai.chat.tool-history", err.Error(), file)
		return
	}

	historyOut, err := parseAIChatOutput(historyRaw)
	if err != nil {
		addSmokeFailure(report, "ai.chat.tool-history", "invalid JSON", file)
		return
	}
	if strings.TrimSpace(historyOut.Error) != "" {
		addSmokeFailure(report, "ai.chat.tool-history", historyOut.Error, file)
		return
	}
	if !hasAIChatSignal(historyOut) {
		addSmokeFailure(report, "ai.chat.tool-history", "empty follow-up content and no tool_calls", file)
	}
}

type aiChatOutput struct {
	Content   string `json:"content"`
	ToolCalls []any  `json:"tool_calls"`
	Error     string `json:"error"`
}

func parseAIChatOutput(raw []byte) (aiChatOutput, error) {
	var out aiChatOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return aiChatOutput{}, err
	}
	return out, nil
}

func hasAIChatSignal(out aiChatOutput) bool {
	return strings.TrimSpace(out.Content) != "" || len(out.ToolCalls) > 0
}

type aiSmokeToolCall struct {
	ID        string
	Name      string
	Arguments string
}

func firstAIChatToolCall(rawCalls []any) (aiSmokeToolCall, bool) {
	for _, raw := range rawCalls {
		asMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := strings.TrimSpace(fmt.Sprintf("%v", asMap["id"]))
		function, ok := asMap["function"].(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", function["name"]))
		arguments := stringifyToolArguments(function["arguments"])
		if id == "" || name == "" || arguments == "" {
			continue
		}
		return aiSmokeToolCall{ID: id, Name: name, Arguments: arguments}, true
	}
	return aiSmokeToolCall{}, false
}

func stringifyToolArguments(raw any) string {
	if raw == nil {
		return ""
	}
	if args, ok := raw.(string); ok {
		return strings.TrimSpace(args)
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(encoded))
}

func toolResultForCall(call aiSmokeToolCall) string {
	if strings.TrimSpace(call.Name) != "math:add" {
		return `{"ok":true}`
	}

	var args struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		return `{"sum":5}`
	}

	sum := args.A + args.B
	if sum == float64(int64(sum)) {
		return fmt.Sprintf(`{"sum":%d}`, int64(sum))
	}
	return fmt.Sprintf(`{"sum":%g}`, sum)
}
