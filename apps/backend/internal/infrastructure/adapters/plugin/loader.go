// Package plugin provides the plugin loader and registry for OpenLobster WASM plugins.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/adapters/plugin/ipc"
)

// LoadPlugins scans dir for *.wasm files, loads each one into a new Adapter,
// and returns the collection. It also loads embedded *.wasm files from
// embeddedPlugins, skipping embedded entries when an external plugin with the
// same ID already exists (external override).
//
// The external directory is created if it does not exist. onMessage is
// forwarded to the WASM runtime for messaging plugins.
//
// The builtin catalog is used to select which embedded plugins are allowed to
// load. External plugins are never filtered by this catalog.
func LoadPlugins(ctx context.Context, dir string, embeddedPlugins fs.FS, onMessage func([]byte), builtins []string, callTimeout time.Duration, dataDir string) ([]ports.PluginPort, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("plugins: mkdir %s: %w", dir, err)
	}
	_ = ctx

	helperPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("plugins: resolve helper binary path: %w", err)
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

	loadedIDs := make(map[string]struct{})
	plugins := make([]ports.PluginPort, 0)
	externalCount := 0
	embeddedCount := 0
	const maxStartupParallelism = 4

	loadAndAppend := func(adapter ports.PluginPort, source string, embedded bool) {
		if _, exists := loadedIDs[adapter.ID()]; exists {
			_ = adapter.Close()
			log.Printf("plugins: skip %s — duplicate ID already loaded", adapter.ID())
			return
		}

		markBuiltin := embedded
		if !markBuiltin {
			if _, ok := builtinSet[adapter.ID()]; ok {
				markBuiltin = true
			}
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
					return
				}
			}
			log.Printf("plugins: skip %s — unsupported type %q", adapter.ID(), pluginType)
			return
		}

		if pluginType == "messaging" {
			if err := validateMessagingPluginABI(adapter); err != nil {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — invalid messaging ABI: %v", adapter.ID(), err)
				return
			}
		}

		plugins = append(plugins, adapter)
		loadedIDs[adapter.ID()] = struct{}{}
		if embedded {
			embeddedCount++
		} else {
			externalCount++
		}
		log.Printf("plugins: loaded %s (%s v%s) from %s", adapter.ID(), adapter.Name(), adapter.Version(), source)
	}

	type externalCandidate struct {
		path     string
		pluginID string
		allowFS  bool
	}
	type externalResult struct {
		path     string
		pluginID string
		adapter  ports.PluginPort
		err      error
	}

	sort.Strings(entries)
	externalCandidates := make([]externalCandidate, 0, len(entries))
	seenExternalIDs := make(map[string]struct{}, len(entries))
	for _, path := range entries {
		pluginID := pluginIDFromPath(path)
		if pluginID == "" {
			continue
		}
		if _, exists := seenExternalIDs[pluginID]; exists {
			log.Printf("plugins: skip %s — duplicate external plugin", pluginID)
			continue
		}
		seenExternalIDs[pluginID] = struct{}{}
		externalCandidates = append(externalCandidates, externalCandidate{
			path:     path,
			pluginID: pluginID,
			allowFS:  pluginAllowsFS(pluginID),
		})
	}

	externalResults := make([]externalResult, len(externalCandidates))
	var externalWG sync.WaitGroup
	externalSem := make(chan struct{}, maxStartupParallelism)
	for i, candidate := range externalCandidates {
		externalWG.Add(1)
		go func(index int, c externalCandidate) {
			defer externalWG.Done()
			externalSem <- struct{}{}
			defer func() { <-externalSem }()

			adapter, createErr := ipc.NewExternalAdapter(helperPath, c.path, callTimeout, c.allowFS, dataDir, onMessage)
			externalResults[index] = externalResult{
				path:     c.path,
				pluginID: c.pluginID,
				adapter:  adapter,
				err:      createErr,
			}
		}(i, candidate)
	}
	externalWG.Wait()

	for _, result := range externalResults {
		if result.err != nil {
			log.Printf("plugins: skip %s — %v", filepath.Base(result.path), result.err)
			continue
		}
		adapter := result.adapter
		if adapter == nil {
			log.Printf("plugins: skip %s — adapter unavailable", filepath.Base(result.path))
			continue
		}
		if adapter.ID() != result.pluginID {
			if _, exists := loadedIDs[adapter.ID()]; exists {
				_ = adapter.Close()
				log.Printf("plugins: skip %s — duplicate ID already loaded", adapter.ID())
				continue
			}
		}
		loadAndAppend(adapter, result.path, false)
	}

	embeddedEntries, err := discoverEmbeddedWASMPaths(embeddedPlugins)
	if err != nil {
		return nil, fmt.Errorf("plugins: walk embedded wasm: %w", err)
	}

	type embeddedCandidate struct {
		embeddedPath string
		allowFS      bool
	}
	type embeddedResult struct {
		embeddedPath string
		adapter      ports.PluginPort
		err          error
	}

	embeddedCandidates := make([]embeddedCandidate, 0, len(embeddedEntries))
	for _, embeddedPath := range embeddedEntries {
		pluginID := pluginIDFromPath(embeddedPath)
		if pluginID == "" {
			continue
		}
		if len(builtinSet) > 0 {
			if _, ok := builtinSet[pluginID]; !ok {
				continue
			}
		}
		if _, exists := loadedIDs[pluginID]; exists {
			log.Printf("plugins: skip embedded %s — overridden by external plugin", pluginID)
			continue
		}
		embeddedCandidates = append(embeddedCandidates, embeddedCandidate{
			embeddedPath: embeddedPath,
			allowFS:      pluginAllowsFS(pluginID),
		})
	}

	embeddedResults := make([]embeddedResult, len(embeddedCandidates))
	var embeddedWG sync.WaitGroup
	embeddedSem := make(chan struct{}, maxStartupParallelism)
	for i, candidate := range embeddedCandidates {
		embeddedWG.Add(1)
		go func(index int, c embeddedCandidate) {
			defer embeddedWG.Done()
			embeddedSem <- struct{}{}
			defer func() { <-embeddedSem }()

			adapter, createErr := ipc.NewEmbeddedAdapter(helperPath, c.embeddedPath, callTimeout, c.allowFS, dataDir, onMessage)
			embeddedResults[index] = embeddedResult{
				embeddedPath: c.embeddedPath,
				adapter:      adapter,
				err:          createErr,
			}
		}(i, candidate)
	}
	embeddedWG.Wait()

	for _, result := range embeddedResults {
		if result.err != nil {
			log.Printf("plugins: skip embedded %s — %v", result.embeddedPath, result.err)
			continue
		}
		if result.adapter == nil {
			log.Printf("plugins: skip embedded %s — adapter unavailable", result.embeddedPath)
			continue
		}
		loadAndAppend(result.adapter, "embedded:"+result.embeddedPath, true)
	}

	log.Printf("plugins: %d plugin(s) loaded (%d external from %s, %d embedded)", len(plugins), externalCount, dir, embeddedCount)
	return plugins, nil
}

func discoverEmbeddedWASMPaths(bundle fs.FS) ([]string, error) {
	if bundle == nil {
		return nil, nil
	}

	entries := make([]string, 0)
	err := fs.WalkDir(bundle, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".wasm") {
			entries = append(entries, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)
	return entries, nil
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

func pluginAllowsFS(pluginID string) bool {
	return strings.Contains(pluginID, "-memory-") || strings.Contains(pluginID, "-secrets-")
}

func validateMessagingPluginABI(p ports.PluginPort) error {
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
