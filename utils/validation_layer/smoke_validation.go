// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
)

const (
	smokeFailRule = "smoke-fail"
)

type ValidateOptions struct {
	SmokeConfig map[string]any
}

func ValidatePluginsWithOptions(pluginsDir string, filter string, options ValidateOptions) (Report, error) {
	dirs, err := resolvePluginTargets(pluginsDir, filter)
	if err != nil {
		return Report{}, err
	}

	report := Report{PluginsDir: pluginsDir, Plugins: make([]PluginReport, 0, len(dirs))}
	for _, dir := range dirs {
		pluginReport, err := ValidatePluginDir(dir)
		if err != nil {
			pluginReport.addIssue(SeverityError, "plugin-validation", err.Error(), "")
		}
		runSmokeTestsForPlugin(dir, &pluginReport, options)
		report.Plugins = append(report.Plugins, pluginReport)
	}

	sort.Slice(report.Plugins, func(i, j int) bool {
		return report.Plugins[i].Name < report.Plugins[j].Name
	})
	return report, nil
}

func smokeIssueSummary(report Report) (failed int) {
	for _, plugin := range report.Plugins {
		for _, issue := range plugin.Issues {
			if issue.Rule == smokeFailRule {
				failed++
			}
		}
	}
	return failed
}

func runSmokeTestsForPlugin(pluginDir string, report *PluginReport, options ValidateOptions) {
	tmpDir, err := os.MkdirTemp("", "openlobster-smoke-")
	if err != nil {
		addSmokeFailure(report, options, "runtime", fmt.Sprintf("temp dir: %v", err), pluginDir)
		return
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "plugin-bin")
	if err := buildPluginBinary(pluginDir, binaryPath); err != nil {
		addSmokeFailure(report, options, "runtime", err.Error(), pluginDir)
		return
	}

	client, err := startRuntimePlugin(binaryPath)
	if err != nil {
		addSmokeFailure(report, options, "runtime", err.Error(), pluginDir)
		return
	}
	defer client.Close()

	if err := smokeManifest(client, report); err != nil {
		addSmokeFailure(report, options, "manifest", err.Error(), pluginDir)
	}
	if client.hasFunction("configure") {
		_ = configurePlugin(client, cloneConfigMap(options.SmokeConfig))
	}

	switch report.Type {
	case "memory":
		runMemorySmoke(client, report, options, pluginDir, tmpDir)
	case "secrets":
		runSecretsSmoke(client, report, options, pluginDir, tmpDir)
	case "audio":
		runAudioSmoke(client, report, options, pluginDir)
	case "ai":
		runAISmoke(client, report, options, pluginDir)
	case "messaging":
		runMessagingSmoke(client, report, options, pluginDir)
	}
}

func runMemorySmoke(client *runtimePluginClient, report *PluginReport, options ValidateOptions, file string, tmpDir string) {
	cfg := cloneConfigMap(options.SmokeConfig)
	ensureConfigValue(cfg, "path", filepath.Join(tmpDir, "memory.gml"))
	fillMissingConfigFromEnv(cfg, map[string][]string{
		"uri":      {"OPENLOBSTER_SMOKE_MEMORY_URI", "OPENLOBSTER_SMOKE_NEO4J_URI"},
		"username": {"OPENLOBSTER_SMOKE_MEMORY_USERNAME", "OPENLOBSTER_SMOKE_NEO4J_USERNAME"},
		"password": {"OPENLOBSTER_SMOKE_MEMORY_PASSWORD", "OPENLOBSTER_SMOKE_NEO4J_PASSWORD"},
		"database": {"OPENLOBSTER_SMOKE_MEMORY_DATABASE", "OPENLOBSTER_SMOKE_NEO4J_DATABASE"},
	})
	if err := configurePlugin(client, cfg); err != nil {
		addSmokeFailure(report, options, "memory", err.Error(), file)
		return
	}

	const memoryStressWrites = 8
	for i := 0; i < memoryStressWrites; i++ {
		label := fmt.Sprintf("smoke-%d", i)
		content := fmt.Sprintf("openlobster smoke memory entry %d", i)
		_, err := client.callJSON("store", map[string]any{
			"config":      cfg,
			"user_id":     "smoke-user",
			"content":     content,
			"label":       label,
			"relation":    "HAS_FACT",
			"entity_type": "fact",
		})
		if err != nil {
			addSmokeFailure(report, options, "memory.store", err.Error(), file)
			return
		}
	}

	raw, err := client.callJSON("retrieve", map[string]any{"config": cfg, "query": "openlobster smoke memory entry", "limit": 64})
	if err != nil {
		addSmokeFailure(report, options, "memory.retrieve", err.Error(), file)
		return
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		addSmokeFailure(report, options, "memory.retrieve", "invalid JSON", file)
		return
	}
	if len(rows) < 3 {
		addSmokeFailure(report, options, "memory.retrieve", "insufficient results after stress writes", file)
		return
	}

	if err := assertMemoryCypherReturnsData(client, cfg, "MATCH (a)-[r]->(b) RETURN a, r, b"); err != nil {
		addSmokeFailure(report, options, "memory.cypher", err.Error(), file)
		return
	}

	if _, err := client.callJSON("store", map[string]any{
		"config":  cfg,
		"op":      "invalidate_cache",
		"user_id": "smoke-user",
	}); err != nil {
		addSmokeFailure(report, options, "memory.invalidate_cache", err.Error(), file)
		return
	}

	const relationStressWrites = 3
	relationTypes := make([]string, 0, relationStressWrites)
	peerIDs := make([]string, 0, relationStressWrites)
	for i := 0; i < relationStressWrites; i++ {
		relationType := fmt.Sprintf("SMOKE_REL_%d_%d", time.Now().UnixNano(), i)
		peerID := fmt.Sprintf("smoke-peer-%d", i)
		relationTypes = append(relationTypes, relationType)
		peerIDs = append(peerIDs, peerID)

		if _, err := client.callJSON("store", map[string]any{
			"config":   cfg,
			"op":       "add_relation",
			"from":     "smoke-user",
			"to":       peerID,
			"rel_type": relationType,
		}); err != nil {
			addSmokeFailure(report, options, "memory.add_relation", err.Error(), file)
			return
		}

		if err := assertMemoryRelationVisible(client, cfg, "smoke-user", "smoke-user", peerID, relationType); err != nil {
			addSmokeFailure(report, options, "memory.user_graph", err.Error(), file)
			return
		}
	}

	if err := deleteMemoryRelation(client, cfg, "smoke-user", peerIDs[0]); err != nil {
		addSmokeFailure(report, options, "memory.delete_relation", err.Error(), file)
		return
	}

	if err := assertMemoryRelationAbsent(client, cfg, "smoke-user", "smoke-user", peerIDs[0], relationTypes[0]); err != nil {
		addSmokeFailure(report, options, "memory.delete_relation", err.Error(), file)
	}
}

func assertMemoryRelationVisible(client *runtimePluginClient, cfg map[string]any, userID string, fromID string, toID string, relationType string) error {
	raw, err := client.callJSON("query", map[string]any{
		"config":  cfg,
		"op":      "user_graph",
		"user_id": userID,
	})
	if err != nil {
		return err
	}

	var graph struct {
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Label  string `json:"label"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(raw, &graph); err != nil {
		return fmt.Errorf("invalid user_graph JSON")
	}

	for _, edge := range graph.Edges {
		if strings.TrimSpace(edge.Label) != relationType {
			continue
		}
		if memoryNodeIDMatches(edge.Source, fromID) && memoryNodeIDMatches(edge.Target, toID) {
			return nil
		}
	}

	return fmt.Errorf("relation %s not visible in user_graph", relationType)
}

func deleteMemoryRelation(client *runtimePluginClient, cfg map[string]any, fromID string, toID string) error {
	prefixedFrom := fromID
	if !strings.HasPrefix(prefixedFrom, "user:") {
		prefixedFrom = "user:" + prefixedFrom
	}
	prefixedTo := toID
	if !strings.HasPrefix(prefixedTo, "user:") {
		prefixedTo = "user:" + prefixedTo
	}

	attempts := []struct {
		from string
		to   string
	}{
		{from: fromID, to: toID},
		{from: prefixedFrom, to: prefixedTo},
	}

	var lastErr error
	for _, attempt := range attempts {
		_, err := client.callJSON("store", map[string]any{
			"config": cfg,
			"op":     "delete_relation",
			"from":   attempt.from,
			"to":     attempt.to,
		})
		if err == nil {
			return nil
		}
		lastErr = err
	}

	if lastErr == nil {
		return fmt.Errorf("delete_relation failed")
	}
	return lastErr
}

func assertMemoryRelationAbsent(client *runtimePluginClient, cfg map[string]any, userID string, fromID string, toID string, relationType string) error {
	raw, err := client.callJSON("query", map[string]any{
		"config":  cfg,
		"op":      "user_graph",
		"user_id": userID,
	})
	if err != nil {
		return err
	}

	var graph struct {
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Label  string `json:"label"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(raw, &graph); err != nil {
		return fmt.Errorf("invalid user_graph JSON")
	}

	for _, edge := range graph.Edges {
		if strings.TrimSpace(edge.Label) != relationType {
			continue
		}
		if memoryNodeIDMatches(edge.Source, fromID) && memoryNodeIDMatches(edge.Target, toID) {
			return fmt.Errorf("relation %s still visible after deletion", relationType)
		}
	}

	return nil
}
func assertMemoryCypherReturnsData(client *runtimePluginClient, cfg map[string]any, cypher string) error {
	raw, err := client.callJSON("query", map[string]any{
		"config": cfg,
		"op":     "cypher",
		"cypher": cypher,
	})
	if err != nil {
		return err
	}

	var result struct {
		Data   []map[string]any `json:"data"`
		Errors []any            `json:"errors"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("invalid cypher JSON")
	}

	for _, queryErr := range result.Errors {
		msg := strings.TrimSpace(fmt.Sprintf("%v", queryErr))
		if msg != "" && msg != "<nil>" {
			return fmt.Errorf("cypher returned errors: %s", msg)
		}
	}

	if len(result.Data) == 0 {
		return fmt.Errorf("cypher returned empty data")
	}

	return nil
}

func memoryNodeIDMatches(actual string, expected string) bool {
	actualID := strings.TrimSpace(actual)
	expectedID := strings.TrimSpace(expected)
	if actualID == expectedID {
		return true
	}
	if strings.HasPrefix(actualID, "user:") && strings.TrimPrefix(actualID, "user:") == expectedID {
		return true
	}
	if strings.HasPrefix(expectedID, "user:") && strings.TrimPrefix(expectedID, "user:") == actualID {
		return true
	}
	return false
}

func runSecretsSmoke(client *runtimePluginClient, report *PluginReport, options ValidateOptions, file string, tmpDir string) {
	cfg := cloneConfigMap(options.SmokeConfig)
	ensureConfigValue(cfg, "path", filepath.Join(tmpDir, "secrets.json"))
	ensureConfigValue(cfg, "key", "smoke-key")
	fillMissingConfigFromEnv(cfg, map[string][]string{
		"url":   {"OPENLOBSTER_SMOKE_SECRETS_URL", "OPENLOBSTER_SMOKE_OPENBAO_URL"},
		"token": {"OPENLOBSTER_SMOKE_SECRETS_TOKEN", "OPENLOBSTER_SMOKE_OPENBAO_TOKEN"},
		"mount": {"OPENLOBSTER_SMOKE_SECRETS_MOUNT", "OPENLOBSTER_SMOKE_OPENBAO_MOUNT"},
	})
	if err := configurePlugin(client, cfg); err != nil {
		addSmokeFailure(report, options, "secrets", err.Error(), file)
		return
	}

	key := "smoke/key"
	value := "smoke-value"
	if _, err := client.callJSON("set", map[string]any{"config": cfg, "key": key, "value": value}); err != nil {
		addSmokeFailure(report, options, "secrets.set", err.Error(), file)
		return
	}
	getRaw, err := client.callJSON("get", map[string]any{"config": cfg, "key": key})
	if err != nil {
		addSmokeFailure(report, options, "secrets.get", err.Error(), file)
		return
	}
	var getResp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(getRaw, &getResp); err != nil || getResp.Value != value {
		addSmokeFailure(report, options, "secrets.get", "value mismatch", file)
		return
	}
	if _, err := client.callJSON("delete", map[string]any{"config": cfg, "key": key}); err != nil {
		addSmokeFailure(report, options, "secrets.delete", err.Error(), file)
	}
}

func runAudioSmoke(client *runtimePluginClient, report *PluginReport, options ValidateOptions, file string) {
	cfg := cloneConfigMap(options.SmokeConfig)
	apiKey := configString(cfg, "api_key")
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENLOBSTER_SMOKE_ELEVENLABS_API_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("ELEVENLABS_API_KEY"))
	}
	if apiKey == "" {
		addSmokeFailure(report, options, "audio", "missing api_key: provide via --config or OPENLOBSTER_SMOKE_ELEVENLABS_API_KEY", file)
		return
	}

	cfg["api_key"] = apiKey
	if err := configurePlugin(client, cfg); err != nil {
		addSmokeFailure(report, options, "audio", err.Error(), file)
		return
	}

	ttsRaw, err := client.callJSON("tts", map[string]any{"text": "OpenLobster smoke test audio", "config": cfg})
	if err != nil {
		addSmokeFailure(report, options, "audio.tts", err.Error(), file)
		return
	}
	var ttsResp struct {
		Audio  string `json:"audio"`
		Format string `json:"format"`
	}
	if err := json.Unmarshal(ttsRaw, &ttsResp); err != nil {
		addSmokeFailure(report, options, "audio.tts", "invalid JSON", file)
		return
	}
	if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ttsResp.Audio)); err != nil {
		addSmokeFailure(report, options, "audio.tts", "invalid base64 audio", file)
		return
	}

	sttRaw, err := client.callJSON("stt", map[string]any{"audio": ttsResp.Audio, "format": fallbackString(ttsResp.Format, "mp3"), "config": cfg})
	if err != nil {
		addSmokeFailure(report, options, "audio.stt", err.Error(), file)
		return
	}
	var sttResp struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(sttRaw, &sttResp); err != nil || strings.TrimSpace(sttResp.Text) == "" {
		addSmokeFailure(report, options, "audio.stt", "empty transcription", file)
	}
}

func runAISmoke(client *runtimePluginClient, report *PluginReport, options ValidateOptions, file string) {
	cfg := cloneConfigMap(options.SmokeConfig)
	fillMissingConfigFromEnv(cfg, map[string][]string{
		"api_key":  {"OPENLOBSTER_SMOKE_AI_API_KEY"},
		"base_url": {"OPENLOBSTER_SMOKE_AI_BASE_URL"},
		"endpoint": {"OPENLOBSTER_SMOKE_AI_ENDPOINT"},
		"model":    {"OPENLOBSTER_SMOKE_AI_MODEL"},
	})

	model := configString(cfg, "model")
	if model == "" {
		addSmokeFailure(report, options, "ai", "missing model: provide via --config or OPENLOBSTER_SMOKE_AI_MODEL", file)
		return
	}

	if err := configurePlugin(client, cfg); err != nil {
		addSmokeFailure(report, options, "ai", err.Error(), file)
		return
	}

	systemPrompt := "You are running an OpenLobster automated smoke test. Follow instructions exactly. Keep responses minimal. When asked to call a tool, emit a tool call with precise arguments."
	noToolsPrompt := "Smoke test step 1/3: return exactly OK."
	noToolsRaw, err := client.callJSON("chat", map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": noToolsPrompt,
			},
		},
		"max_tokens": 256,
		"config":     cfg,
	})
	if err != nil {
		addSmokeFailure(report, options, "ai.chat.no-tools", err.Error(), file)
		return
	}

	noToolsOut, err := parseAIChatOutput(noToolsRaw)
	if err != nil {
		addSmokeFailure(report, options, "ai.chat.no-tools", "invalid JSON", file)
		return
	}
	if strings.TrimSpace(noToolsOut.Error) != "" {
		addSmokeFailure(report, options, "ai.chat.no-tools", noToolsOut.Error, file)
		return
	}
	if !hasAIChatSignal(noToolsOut) {
		addSmokeFailure(report, options, "ai.chat.no-tools", "empty content and no tool_calls", file)
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
	toolPrompt := "Smoke test step 2/3: call the tool math:add with {\"a\":2,\"b\":3}. Do not provide final answer yet."

	withToolsRaw, err := client.callJSON("chat", map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": toolPrompt,
			},
		},
		"tools":      []map[string]any{tool},
		"max_tokens": 256,
		"config":     cfg,
	})
	if err != nil {
		addSmokeFailure(report, options, "ai.chat.with-tools", err.Error(), file)
		return
	}

	withToolsOut, err := parseAIChatOutput(withToolsRaw)
	if err != nil {
		addSmokeFailure(report, options, "ai.chat.with-tools", "invalid JSON", file)
		return
	}
	if strings.TrimSpace(withToolsOut.Error) != "" {
		addSmokeFailure(report, options, "ai.chat.with-tools", withToolsOut.Error, file)
		return
	}
	if !hasAIChatSignal(withToolsOut) {
		addSmokeFailure(report, options, "ai.chat.with-tools", "empty content and no tool_calls", file)
		return
	}

	call, ok := firstAIChatToolCall(withToolsOut.ToolCalls)
	if !ok {
		addSmokeFailure(report, options, "ai.chat.tool-history", "model did not emit a valid tool_call", file)
		return
	}

	historyRaw, err := client.callJSON("chat", map[string]any{
		"model": model,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": toolPrompt,
			},
			{
				"role":    "assistant",
				"content": withToolsOut.Content,
				"tool_calls": []map[string]any{{
					"id":   call.ID,
					"type": "function",
					"function": map[string]any{
						"name":      call.Name,
						"arguments": call.Arguments,
					},
				}},
			},
			{
				"role":         "tool",
				"tool_call_id": call.ID,
				"tool_name":    call.Name,
				"content":      toolResultForCall(call),
			},
			{
				"role":    "user",
				"content": "Smoke test step 3/3: use the tool result and answer with one short final sentence.",
			},
		},
		"tools":      []map[string]any{tool},
		"max_tokens": 256,
		"config":     cfg,
	})
	if err != nil {
		addSmokeFailure(report, options, "ai.chat.tool-history", err.Error(), file)
		return
	}

	historyOut, err := parseAIChatOutput(historyRaw)
	if err != nil {
		addSmokeFailure(report, options, "ai.chat.tool-history", "invalid JSON", file)
		return
	}
	if strings.TrimSpace(historyOut.Error) != "" {
		addSmokeFailure(report, options, "ai.chat.tool-history", historyOut.Error, file)
		return
	}
	if !hasAIChatSignal(historyOut) {
		addSmokeFailure(report, options, "ai.chat.tool-history", "empty follow-up content and no tool_calls", file)
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
func runMessagingSmoke(client *runtimePluginClient, report *PluginReport, options ValidateOptions, file string) {
	cfg := cloneConfigMap(options.SmokeConfig)
	ensureConfigValue(cfg, "default_chat_id", "123456789")
	ensureConfigValue(cfg, "default_channel_id", "123456789012345678")
	ensureConfigValue(cfg, "channel", "C1234567890")
	ensureConfigValue(cfg, "default_to_number", "+15550000001")

	_ = configurePlugin(client, cfg)

	if client.hasFunction("inbound_mode") {
		mode, err := client.callString("inbound_mode", nil)
		if err != nil {
			addSmokeFailure(report, options, "messaging.inbound_mode", err.Error(), file)
			return
		}
		if !isInboundModeValid(mode) {
			addSmokeFailure(report, options, "messaging.inbound_mode", "invalid mode", file)
			return
		}
	}

	if client.hasFunction("capabilities") {
		raw, err := client.callJSON("capabilities", nil)
		if err != nil {
			addSmokeFailure(report, options, "messaging.capabilities", err.Error(), file)
			return
		}
		var caps map[string]any
		if err := json.Unmarshal(raw, &caps); err != nil || len(caps) == 0 {
			addSmokeFailure(report, options, "messaging.capabilities", "invalid capabilities JSON", file)
		}
	}

	if client.hasFunction("resolve_channel_id") {
		channelID := fallbackString(configString(cfg, "channel_id"), configString(cfg, "default_channel_id"))
		channelID = fallbackString(channelID, configString(cfg, "default_chat_id"))
		channelID = fallbackString(channelID, configString(cfg, "channel"))
		channelID = fallbackString(channelID, "smoke-channel")
		payload := map[string]any{
			"config": cfg,
			"message": map[string]any{
				"channel_id": channelID,
				"metadata":   map[string]any{},
			},
		}
		resolved, err := client.callString("resolve_channel_id", payload)
		if err != nil || strings.TrimSpace(resolved) == "" {
			addSmokeFailure(report, options, "messaging.resolve_channel_id", "could not resolve channel", file)
		}
	}

}

func smokeManifest(client *runtimePluginClient, report *PluginReport) error {
	raw, err := client.callJSON(metadataExportName, nil)
	if err != nil {
		return err
	}
	var metadata struct {
		ID         string          `json:"id"`
		Type       string          `json:"type"`
		Schema     json.RawMessage `json:"schema"`
		Properties json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return fmt.Errorf("invalid metadata JSON")
	}
	metadata.ID = strings.TrimSpace(metadata.ID)
	metadata.Type = strings.TrimSpace(metadata.Type)
	if metadata.ID == "" {
		return fmt.Errorf("metadata id empty")
	}
	if strings.TrimSpace(report.ID) != "" && metadata.ID != strings.TrimSpace(report.ID) {
		return fmt.Errorf("metadata id mismatch")
	}
	if metadata.Type == "" {
		return fmt.Errorf("metadata type empty")
	}
	if strings.TrimSpace(report.Type) != "" && metadata.Type != strings.TrimSpace(report.Type) {
		return fmt.Errorf("metadata type mismatch")
	}
	if !json.Valid(metadata.Schema) {
		return fmt.Errorf("metadata schema invalid")
	}
	if !json.Valid(metadata.Properties) {
		return fmt.Errorf("metadata properties invalid")
	}

	var propertiesObj map[string]any
	if err := json.Unmarshal(metadata.Properties, &propertiesObj); err != nil {
		return fmt.Errorf("metadata properties root must be object")
	}
	return nil
}

func configurePlugin(client *runtimePluginClient, cfg map[string]any) error {
	if !client.hasFunction("configure") {
		return nil
	}
	_, err := client.callJSON("configure", map[string]any{"config": cfg})
	return err
}

func addSmokeFailure(report *PluginReport, _ ValidateOptions, name string, message string, file string) {
	report.addIssue(SeverityError, smokeFailRule, fmt.Sprintf("%s: %s", name, message), file)
}


func cloneConfigMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func ensureConfigValue(cfg map[string]any, key string, fallback any) {
	if cfg == nil {
		return
	}
	if value, ok := cfg[key]; ok {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return
			}
		default:
			if typed != nil {
				return
			}
		}
	}
	cfg[key] = fallback
}

func fillMissingConfigFromEnv(cfg map[string]any, envByKey map[string][]string) {
	if cfg == nil {
		return
	}
	for key, envKeys := range envByKey {
		if configString(cfg, key) != "" {
			continue
		}
		for _, envKey := range envKeys {
			value := strings.TrimSpace(os.Getenv(envKey))
			if value == "" {
				continue
			}
			cfg[key] = value
			break
		}
	}
}

func configString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	value, ok := cfg[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func fallbackString(primary string, fallback string) string {
	trimmed := strings.TrimSpace(primary)
	if trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}


func isInboundModeValid(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "polling", "gateway", "webhook", "disabled":
		return true
	default:
		return false
	}
}

type runtimePluginClient struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	grpcConn *grpc.ClientConn
	info     rpcGetInfoResponse
	exports  map[string]struct{}
}

func startRuntimePlugin(binaryPath string) (*runtimePluginClient, error) {
	hostConn, childFile, err := openSocketpair()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binaryPath)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		_ = hostConn.Close()
		_ = childFile.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = hostConn.Close()
		_ = childFile.Close()
		_ = stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.ExtraFiles = []*os.File{childFile}

	if err := cmd.Start(); err != nil {
		_ = hostConn.Close()
		_ = childFile.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("start plugin process: %w", err)
	}
	_ = childFile.Close()

	if err := writeHandshakeFrame(stdin, handshakeFrame{
		Type:      handshakeTypeRequest,
		Version:   handshakeVersion,
		Transport: "grpc_socketpair",
		FD:        3,
	}); err != nil {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("write handshake: %w", err)
	}

	ack, err := readHandshakeAck(stdout, 10*time.Second)
	if err != nil {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("read handshake ack: %w", err)
	}
	if ack.Type != handshakeTypeAck || !ack.OK {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("handshake rejected")
	}

	dialer := newSingleConnDialer(hostConn)
	dialCtx, cancelDial := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDial()
	grpcConn, err := grpc.DialContext(
		dialCtx,
		"passthrough:///openlobster-validator",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer.DialContext),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		_ = hostConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("grpc dial: %w", err)
	}

	infoCtx, cancelInfo := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelInfo()
	info := &rpcGetInfoResponse{}
	err = grpcConn.Invoke(infoCtx, rpcMethodGetInfo, &rpcGetInfoRequest{}, info)
	if err != nil {
		_ = grpcConn.Close()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return nil, fmt.Errorf("GetInfo failed: %w", err)
	}

	exports := map[string]struct{}{}
	for _, name := range info.Exports {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			exports[trimmed] = struct{}{}
		}
	}

	out := &runtimePluginClient{cmd: cmd, stdin: stdin, stdout: stdout, grpcConn: grpcConn, exports: exports}
	out.info = *info
	return out, nil
}

func (c *runtimePluginClient) hasFunction(function string) bool {
	_, ok := c.exports[strings.TrimSpace(function)]
	return ok
}

func (c *runtimePluginClient) callJSON(function string, payload any) ([]byte, error) {
	var input []byte
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		input = raw
	}
	callTimeout := 12 * time.Second
	switch function {
	case "chat", "tts", "stt", "send":
		callTimeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	resp := &rpcCallResponse{}
	if err := c.grpcConn.Invoke(ctx, rpcMethodCall, &rpcCallRequest{Function: function, Input: input}, resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Error) != "" {
		return nil, fmt.Errorf("%s", strings.TrimSpace(resp.Error))
	}
	return append([]byte(nil), resp.Output...), nil
}

func (c *runtimePluginClient) callString(function string, payload any) (string, error) {
	out, err := c.callJSON(function, payload)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *runtimePluginClient) Close() error {
	if c == nil {
		return nil
	}
	if c.grpcConn != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = c.grpcConn.Invoke(ctx, rpcMethodClose, &rpcCloseRequest{}, &rpcCloseResponse{})
		cancel()
	}
	if c.grpcConn != nil {
		_ = c.grpcConn.Close()
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	return nil
}

func buildPluginBinary(pluginDir string, outputPath string) error {
	cmd := exec.Command("go", "build", "-o", outputPath, ".")
	cmd.Dir = pluginDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		if len(message) > 240 {
			message = message[:240] + "..."
		}
		return fmt.Errorf("go build failed: %s", message)
	}
	return nil
}

func readHandshakeAck(stdout io.Reader, timeout time.Duration) (handshakeFrame, error) {
	reader := bufio.NewReader(stdout)
	type result struct {
		frame handshakeFrame
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		frame, err := readHandshakeFrame(reader)
		ch <- result{frame: frame, err: err}
	}()
	select {
	case got := <-ch:
		return got.frame, got.err
	case <-time.After(timeout):
		return handshakeFrame{}, fmt.Errorf("timeout waiting handshake ack")
	}
}

func openSocketpair() (net.Conn, *os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("socketpair: %w", err)
	}
	hostFile := os.NewFile(uintptr(fds[0]), "validator-host")
	childFile := os.NewFile(uintptr(fds[1]), "validator-child")
	hostConn, err := net.FileConn(hostFile)
	_ = hostFile.Close()
	if err != nil {
		_ = childFile.Close()
		return nil, nil, fmt.Errorf("socketpair host conn: %w", err)
	}
	return hostConn, childFile, nil
}

type singleConnDialer struct {
	once sync.Once
	conn net.Conn
}

func newSingleConnDialer(conn net.Conn) *singleConnDialer {
	return &singleConnDialer{conn: conn}
}

func (d *singleConnDialer) DialContext(context.Context, string) (net.Conn, error) {
	var out net.Conn
	d.once.Do(func() {
		out = d.conn
		d.conn = nil
	})
	if out == nil {
		return nil, fmt.Errorf("single connection already consumed")
	}
	return out, nil
}

const (
	rpcMethodGetInfo = "/openlobster.plugin.v1.PluginService/GetInfo"
	rpcMethodCall    = "/openlobster.plugin.v1.PluginService/Call"
	rpcMethodClose   = "/openlobster.plugin.v1.PluginService/Close"

	handshakeTypeRequest = "handshake_request"
	handshakeTypeAck     = "handshake_ack"
	handshakeVersion     = 1
)

type rpcGetInfoRequest struct{}

type rpcGetInfoResponse struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	Schema      []byte   `json:"schema,omitempty"`
	Exports     []string `json:"exports,omitempty"`
}

type rpcCallRequest struct {
	Function string `json:"function,omitempty"`
	Input    []byte `json:"input,omitempty"`
}

type rpcCallResponse struct {
	Output []byte `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

type rpcCloseRequest struct{}

type rpcCloseResponse struct{}

type handshakeFrame struct {
	Type      string `json:"type"`
	Version   int    `json:"version"`
	Transport string `json:"transport,omitempty"`
	FD        int    `json:"fd,omitempty"`
	OK        bool   `json:"ok,omitempty"`
	Error     string `json:"error,omitempty"`
}

type jsonCodec struct{}

func (jsonCodec) Name() string {
	return "json"
}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

func writeHandshakeFrame(w io.Writer, frame handshakeFrame) error {
	if w == nil {
		return fmt.Errorf("nil handshake writer")
	}
	enc := json.NewEncoder(w)
	return enc.Encode(frame)
}

func readHandshakeFrame(r *bufio.Reader) (handshakeFrame, error) {
	if r == nil {
		return handshakeFrame{}, fmt.Errorf("nil handshake reader")
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return handshakeFrame{}, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return handshakeFrame{}, fmt.Errorf("empty handshake frame")
	}
	var frame handshakeFrame
	if err := json.Unmarshal([]byte(line), &frame); err != nil {
		return handshakeFrame{}, err
	}
	return frame, nil
}
