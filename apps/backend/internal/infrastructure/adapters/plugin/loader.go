// Package plugin provides the plugin loader and registry for OpenLobster WASM plugins.
package plugin

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/plugin/wasm"
)

// LoadPlugins scans dir for *.wasm files, loads each one into a new Adapter,
// and returns the collection. The directory is created if it does not exist.
// onMessage is forwarded to the WASM runtime for messaging plugins.
func LoadPlugins(ctx context.Context, dir string, onMessage func([]byte)) ([]ports.PluginPort, error) {
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

	var plugins []ports.PluginPort
	for _, path := range entries {
		adapter, err := wasm.NewAdapter(ctx, rt, path)
		if err != nil {
			log.Printf("plugins: skip %s — %v", filepath.Base(path), err)
			continue
		}
		plugins = append(plugins, adapter)
		log.Printf("plugins: loaded %s (%s v%s)", adapter.ID(), adapter.Name(), adapter.Version())
	}

	log.Printf("plugins: %d plugin(s) loaded from %s", len(plugins), dir)
	return plugins, nil
}
