// Package plugin provides the plugin loader and registry for OpenLobster WASM plugins.
package plugin

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/plugin/wasm"
)

// LoadPlugins scans dir for *.wasm files, loads each one into a new Adapter,
// and returns the collection. The directory is created if it does not exist.
// onMessage is forwarded to the WASM runtime for messaging plugins.
func LoadPlugins(ctx context.Context, dir string, onMessage func([]byte), builtins []string, callTimeout time.Duration, dataDir string) ([]ports.PluginPort, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("plugins: mkdir %s: %w", dir, err)
	}

	rt, err := wasm.NewRuntime(ctx, onMessage)
	if err != nil {
		return nil, fmt.Errorf("plugins: create wasm runtime: %w", err)
	}

	entries, err := filepath.Glob(filepath.Join(dir, "*.wasm"))
	if err != nil {
		return nil, fmt.Errorf("plugins: glob %s: %w", dir, err)
	}

	builtinSet := make(map[string]struct{}, len(builtins))
	for _, id := range builtins {
		if id = strings.TrimSpace(id); id != "" {
			builtinSet[id] = struct{}{}
		}
	}

	var plugins []ports.PluginPort
	for _, path := range entries {
		allowFS := strings.Contains(path, "-memory-") || strings.Contains(path, "-secrets-")
		adapter, err := wasm.NewAdapter(ctx, rt, path, callTimeout, allowFS, dataDir)
		if err != nil {
			log.Printf("plugins: skip %s — %v", filepath.Base(path), err)
			continue
		}
		if len(builtinSet) > 0 {
			if _, ok := builtinSet[adapter.ID()]; !ok {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — not in builtin catalog", adapter.ID())
				continue
			}
			if stateSetter, ok := interface{}(adapter).(ports.PluginStateSetterPort); ok {
				stateSetter.SetBuiltin(true)
			}
		}
		switch adapter.Type() {
		case "ai", "messaging", "memory", "audio", "secrets":
		default:
			_ = adapter.Close()
			log.Printf("plugins: skip %s — unsupported type %q", adapter.ID(), adapter.Type())
			continue
		}
		plugins = append(plugins, adapter)
		log.Printf("plugins: loaded %s (%s v%s)", adapter.ID(), adapter.Name(), adapter.Version())
	}

	log.Printf("plugins: %d plugin(s) loaded from %s", len(plugins), dir)
	return plugins, nil
}
