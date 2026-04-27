// Package plugin provides the plugin loader and registry for OpenLobster plugins.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/plugin/runtime/subprocess"
	"github.com/neirth/openlobster/internal/infrastructure/plugin/runtime/threads"
)

// LoadPlugins scans dir for plugins and returns the loaded collection.
//
// Plugins are native subprocess binaries from plugins dir.
func LoadPlugins(ctx context.Context, dir string, onMessage func(pluginID string, channelType string, payload []byte), builtins []string, callTimeout time.Duration) ([]ports.PluginPort, error) {
	// Ensure bundled plugins are extracted/updated before loading
	if err := ExtractEmbeddedPlugins(dir); err != nil {
		log.Printf("plugins: extraction failed: %v", err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("plugins: mkdir %s: %w", dir, err)
	}

	nativeEntries, err := discoverNativePluginPaths(dir)
	if err != nil {
		return nil, fmt.Errorf("plugins: discover native binaries in %s: %w", dir, err)
	}

	builtinSet := make(map[string]struct{}, len(builtins))
	for _, id := range builtins {
		if id = strings.TrimSpace(id); id != "" {
			builtinSet[id] = struct{}{}
		}
	}

	loadedIDs := make(map[string]struct{})
	plugins := make([]ports.PluginPort, 0)
	nativeCount := 0

	loadAndAppend := func(adapter ports.PluginPort, source string) bool {
		if adapter == nil {
			return false
		}
		pluginType := strings.TrimSpace(adapter.Type())
		pluginID := adapter.ID()
		compositeID := fmt.Sprintf("%s:%s", pluginType, pluginID)

		if _, exists := loadedIDs[compositeID]; exists {
			_ = adapter.Close()
			log.Printf("plugins: skip %s %s — duplicate ID already loaded", pluginType, pluginID)
			return false
		}

		markBuiltin := false
		if _, ok := builtinSet[pluginID]; ok {
			markBuiltin = true
		}
		if markBuiltin {
			if stateSetter, ok := interface{}(adapter).(ports.PluginStateSetterPort); ok {
				stateSetter.SetBuiltin(true)
			}
		}

		switch pluginType {
		case "ai", "messaging", "memory", "audio", "secrets":
		default:
			_ = adapter.Close()
			if state, ok := adapter.(ports.PluginStatePort); ok {
				if lastErr := strings.TrimSpace(state.LastError()); lastErr != "" {
					log.Printf("plugins: skip %s — unsupported type %q (last_error=%s)", compositeID, pluginType, lastErr)
					return false
				}
			}
			log.Printf("plugins: skip %s — unsupported type %q", compositeID, pluginType)
			return false
		}

		if pluginType == "messaging" {
			if err := validateMessagingPluginABI(adapter); err != nil {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — invalid messaging ABI: %v", compositeID, err)
				return false
			}
		}

		if pluginType == "ai" {
			if err := validateAIPluginABI(adapter); err != nil {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — invalid AI ABI: %v", compositeID, err)
				return false
			}
		}

		if pluginType == "memory" {
			if err := validateMemoryPluginABI(adapter); err != nil {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — invalid memory ABI: %v", compositeID, err)
				return false
			}
		}

		if pluginType == "audio" {
			if err := validateAudioPluginABI(adapter); err != nil {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — invalid audio ABI: %v", compositeID, err)
				return false
			}
		}

		if pluginType == "secrets" {
			if err := validateSecretsPluginABI(adapter); err != nil {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — invalid secrets ABI: %v", compositeID, err)
				return false
			}
		}

		plugins = append(plugins, adapter)
		loadedIDs[compositeID] = struct{}{}
		log.Printf("plugins: loaded %s (v%s) from %s", compositeID, adapter.Version(), source)
		return true
	}

	sort.Strings(nativeEntries)
	for _, path := range nativeEntries {
		pluginID := pluginIDFromPath(path)
		if pluginID == "" {
			continue
		}
		if _, exists := loadedIDs[pluginID]; exists {
			continue
		}

		adapter, createErr := subprocess.NewAdapter(ctx, path, runtimeOnMessage(pluginID, onMessage), callTimeout)
		if createErr != nil {
			log.Printf("plugins: skip native %s — %v", filepath.Base(path), createErr)
			continue
		}
		if loadAndAppend(adapter, path) {
			nativeCount++
		}
	}

	log.Printf(
		"plugins: %d plugin(s) loaded (%d native from %s)",
		len(plugins),
		nativeCount,
		dir,
	)
	return plugins, nil
}

func discoverNativePluginPaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		lowerName := strings.ToLower(name)
		if !strings.HasPrefix(lowerName, "openlobster-") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}

		if runtime.GOOS == "windows" {
			if strings.HasSuffix(lowerName, ".exe") {
				out = append(out, fullPath)
			}
			continue
		}

		if info.Mode()&0o111 == 0 {
			continue
		}
		out = append(out, fullPath)
	}

	sort.Strings(out)
	return out, nil
}

func pluginIDFromPath(path string) string {
	base := filepath.Base(path)
	if base == "" {
		return ""
	}
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}

func pluginChannelType(pluginID string) string {
	id := strings.TrimSpace(strings.ToLower(pluginID))
	const prefix = "openlobster-messages-"
	if strings.HasPrefix(id, prefix) {
		return strings.TrimSpace(id[len(prefix):])
	}
	return ""
}

func runtimeOnMessage(pluginID string, onMessage func(string, string, []byte)) func([]byte) {
	channelType := pluginChannelType(pluginID)
	return func(payload []byte) {
		if len(payload) == 0 {
			return
		}
		payloadCopy := append([]byte(nil), payload...)
		threads.PublishPluginMessage(pluginID, channelType, payloadCopy)
		if onMessage != nil {
			go onMessage(pluginID, channelType, payloadCopy)
		}
	}
}

func validateMessagingPluginABI(p ports.PluginPort) error {
	raw := p.Properties()
	if len(raw) == 0 {
		return fmt.Errorf("plugin properties are empty (required for messaging ABI check)")
	}

	var props struct {
		InboundMode string `json:"inbound_mode"`
	}
	if err := json.Unmarshal(raw, &props); err != nil {
		return fmt.Errorf("parse plugin properties (inbound_mode): %w", err)
	}

	inboundMode, parseErr := parseInboundModeOutput([]byte(props.InboundMode))
	if parseErr != nil {
		return parseErr
	}

	introspector, ok := p.(ports.PluginFunctionIntrospectionPort)
	if !ok {
		return fmt.Errorf("messaging plugin %s: adapter does not support ABI function introspection", p.ID())
	}

	if !introspector.HasFunction("send") {
		return fmt.Errorf("messaging plugin %s: missing required function %q", p.ID(), "send")
	}

	if !introspector.HasFunction("capabilities") {
		return fmt.Errorf("messaging plugin %s: missing required function %q", p.ID(), "capabilities")
	}

	// Check required exports based on capabilities
	var caps struct {
		Capabilities ports.ChannelCapabilities `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &caps); err == nil {
		if caps.Capabilities.HasVoiceMessage {
			if !introspector.HasFunction("send_voice") {
				return fmt.Errorf("messaging plugin %s claims voice support but lacks required %q export", p.ID(), "send_voice")
			}
			if !introspector.HasFunction("speaking") {
				return fmt.Errorf("messaging plugin %s claims voice support but lacks required %q indicator export", p.ID(), "speaking")
			}
		}
	}

	if !introspector.HasFunction("typing") {
		return fmt.Errorf("messaging plugin %s: missing required function %q", p.ID(), "typing")
	}

	hasStart := introspector.HasFunction("start")
	hasHandleWebhook := introspector.HasFunction(handleWebhookFn)
	if err := validateMessagingInboundContract(inboundMode, hasStart, hasHandleWebhook); err != nil {
		return err
	}

	probeInput := map[string]interface{}{
		"config": map[string]interface{}{},
		"message": map[string]interface{}{
			"channel_id": "__abi_probe_channel__",
			"content":    "",
		},
	}

	probeRaw, err := json.Marshal(probeInput)
	if err != nil {
		return fmt.Errorf("marshal probe: %w", err)
	}

	out, err := p.Call(resolveChannelIDFn, probeRaw)
	if err != nil {
		return fmt.Errorf("required function %q failed: %w", resolveChannelIDFn, err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("required function %q returned empty channel_id", resolveChannelIDFn)
	}

	return nil
}

func validateMessagingInboundContract(inboundMode string, hasStart bool, hasHandleWebhook bool) error {
	switch inboundMode {
	case ports.InboundModePolling, ports.InboundModeGateway:
		if !hasStart {
			return fmt.Errorf("inbound_mode=%s requires exported function %q", inboundMode, "start")
		}
	case ports.InboundModeWebhook:
		if !hasHandleWebhook {
			return fmt.Errorf("inbound_mode=%s requires exported function %q", inboundMode, handleWebhookFn)
		}
		if hasStart {
			return fmt.Errorf("inbound_mode=%s forbids exported function %q; remove no-op start and rely on host webhook ingress", inboundMode, "start")
		}
	case ports.InboundModeDisabled:
		if hasStart || hasHandleWebhook {
			return fmt.Errorf("inbound_mode=%s forbids exported functions %q and %q", inboundMode, "start", handleWebhookFn)
		}
	default:
		return fmt.Errorf("invalid %s value %q", inboundModeFn, inboundMode)
	}

	return nil
}

func validateAIPluginABI(p ports.PluginPort) error {
	introspector, ok := p.(ports.PluginFunctionIntrospectionPort)
	if !ok {
		return nil
	}
	if !introspector.HasFunction("chat") {
		return fmt.Errorf("AI plugin must export %q function", "chat")
	}

	// Check for multimodal audio exports if properties claim support
	raw := p.Properties()
	if len(raw) > 0 {
		var props struct {
			SupportsAudioInput  bool `json:"supports_audio_input"`
			SupportsAudioOutput bool `json:"supports_audio_output"`
		}
		if err := json.Unmarshal(raw, &props); err == nil {
			if props.SupportsAudioInput && !introspector.HasFunction("chat_with_audio") {
				return fmt.Errorf("AI plugin claims audio input support but lacks %q export", "chat_with_audio")
			}
			if props.SupportsAudioOutput && !introspector.HasFunction("chat_to_audio") {
				return fmt.Errorf("AI plugin claims audio output support but lacks %q export", "chat_to_audio")
			}
		}
	}

	// Functional probe: call chat with minimal input to verify JSON-RPC compliance
	probeInput := map[string]interface{}{
		"model": "abi-probe-model",
		"messages": []map[string]string{
			{"role": "user", "content": ""},
		},
	}
	probeRaw, _ := json.Marshal(probeInput)
	_, err := p.Call("chat", probeRaw)
	if err != nil && strings.Contains(err.Error(), "broken pipe") {
		return fmt.Errorf("AI plugin %q: functional probe failed (broken pipe)", p.ID())
	}

	return nil
}

func validateMemoryPluginABI(p ports.PluginPort) error {
	introspector, ok := p.(ports.PluginFunctionIntrospectionPort)
	if !ok {
		return nil
	}
	required := []string{"store", "retrieve", "query"}
	for _, fn := range required {
		if !introspector.HasFunction(fn) {
			return fmt.Errorf("memory plugin must export %q function", fn)
		}
	}

	// Functional probe: query with empty filter
	probeInput := map[string]interface{}{
		"filter": map[string]interface{}{},
	}
	probeRaw, _ := json.Marshal(probeInput)
	_, err := p.Call("query", probeRaw)
	if err != nil && strings.Contains(err.Error(), "broken pipe") {
		return fmt.Errorf("memory plugin %q: functional probe failed (broken pipe)", p.ID())
	}

	return nil
}

func validateAudioPluginABI(p ports.PluginPort) error {
	introspector, ok := p.(ports.PluginFunctionIntrospectionPort)
	if !ok {
		return nil
	}
	required := []string{"tts", "stt"}
	for _, fn := range required {
		if !introspector.HasFunction(fn) {
			return fmt.Errorf("audio plugin must export %q function", fn)
		}
	}

	// Functional probe: tts with minimal text
	probeInput := map[string]interface{}{
		"text": "",
	}
	probeRaw, _ := json.Marshal(probeInput)
	_, err := p.Call("tts", probeRaw)
	if err != nil && strings.Contains(err.Error(), "broken pipe") {
		return fmt.Errorf("audio plugin %q: functional probe failed (broken pipe)", p.ID())
	}

	return nil
}

func validateSecretsPluginABI(p ports.PluginPort) error {
	introspector, ok := p.(ports.PluginFunctionIntrospectionPort)
	if !ok {
		return nil
	}
	required := []string{"get", "set", "delete", "list"}
	for _, fn := range required {
		if !introspector.HasFunction(fn) {
			return fmt.Errorf("secrets plugin must export %q function", fn)
		}
	}

	// Functional probe: list
	probeInput := map[string]interface{}{}
	probeRaw, _ := json.Marshal(probeInput)
	_, err := p.Call("list", probeRaw)
	if err != nil && strings.Contains(err.Error(), "broken pipe") {
		return fmt.Errorf("secrets plugin %q: functional probe failed (broken pipe)", p.ID())
	}

	return nil
}
