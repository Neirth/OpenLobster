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
func LoadPlugins(ctx context.Context, dir string, onMessage func([]byte), builtins []string, callTimeout time.Duration) ([]ports.PluginPort, error) {
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
		if _, exists := loadedIDs[adapter.ID()]; exists {
			_ = adapter.Close()
			log.Printf("plugins: skip %s — duplicate ID already loaded", adapter.ID())
			return false
		}

		markBuiltin := false
		if _, ok := builtinSet[adapter.ID()]; ok {
			markBuiltin = true
		}
		if markBuiltin {
			if stateSetter, ok := interface{}(adapter).(ports.PluginStateSetterPort); ok {
				stateSetter.SetBuiltin(true)
			}
		}

		pluginType := strings.TrimSpace(adapter.Type())
		switch pluginType {
		case "ai", "messaging", "memory", "audio", "secrets":
		default:
			_ = adapter.Close()
			if state, ok := adapter.(ports.PluginStatePort); ok {
				if lastErr := strings.TrimSpace(state.LastError()); lastErr != "" {
					log.Printf("plugins: skip %s — unsupported type %q (last_error=%s)", adapter.ID(), pluginType, lastErr)
					return false
				}
			}
			log.Printf("plugins: skip %s — unsupported type %q", adapter.ID(), pluginType)
			return false
		}

		if pluginType == "messaging" {
			if err := validateMessagingPluginABI(adapter); err != nil {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — invalid messaging ABI: %v", adapter.ID(), err)
				return false
			}
		}

		if pluginType == "ai" {
			if err := validateAIPluginABI(adapter); err != nil {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — invalid AI ABI: %v", adapter.ID(), err)
				return false
			}
		}

		plugins = append(plugins, adapter)
		loadedIDs[adapter.ID()] = struct{}{}
		log.Printf("plugins: loaded %s (%s v%s) from %s", adapter.ID(), adapter.Name(), adapter.Version(), source)
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

func runtimeOnMessage(pluginID string, onMessage func([]byte)) func([]byte) {
	channelType := pluginChannelType(pluginID)
	return func(payload []byte) {
		if len(payload) == 0 {
			return
		}
		payloadCopy := append([]byte(nil), payload...)
		threads.PublishPluginMessage(pluginID, channelType, payloadCopy)
		if onMessage != nil {
			go onMessage(payloadCopy)
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
		return nil // Basic adapters might not support introspection
	}
	if !introspector.HasFunction("chat") {
		return fmt.Errorf("AI plugin must export %q function", "chat")
	}
	return nil
}
