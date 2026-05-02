// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package smoke runs exhaustive bidirectional smoke tests against a pre-built
// OpenLobster plugin binary. Each plugin type has a dedicated runner. The
// entry point is ValidatePluginBinary.
package smoke

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/neirth/openlobster/utils/validation_layer/src/manifest"
	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
)

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

// ValidatePluginBinary runs a complete smoke validation against a pre-built
// plugin binary using the bidirectional JSON-RPC 2.0 / STDIO protocol.
func ValidatePluginBinary(binaryPath string, opts types.ValidateOptions) (types.PluginReport, error) {
	report := types.PluginReport{Binary: binaryPath}

	info, err := os.Stat(binaryPath)
	if err != nil {
		return report, fmt.Errorf("binary not found: %w", err)
	}
	if info.IsDir() {
		return report, fmt.Errorf("%q is a directory, not a plugin binary", binaryPath)
	}
	if info.Mode()&0o111 == 0 {
		return report, fmt.Errorf("%q is not executable", binaryPath)
	}

	client, err := protocol.StartRuntimePlugin(binaryPath)
	if err != nil {
		addSmokeFailure(&report, "runtime", err.Error(), binaryPath)
		return report, nil
	}
	defer client.Close()

	// Populate name from GetInfo; fall back to binary basename.
	pluginInfo := client.Info()
	report.Name = strings.TrimSpace(pluginInfo.ID)
	if report.Name == "" {
		report.Name = binaryPath[strings.LastIndexByte(binaryPath, '/')+1:]
	}
	report.Version = strings.TrimSpace(pluginInfo.Version)

	// Validate manifest (calls get_metadata, checks schema structure).
	if err := manifest.SmokeManifest(client, &report); err != nil {
		addSmokeFailure(&report, "manifest", err.Error(), binaryPath)
	}

// Verify required exports.
	exportSet := make(map[string]struct{}, len(report.Exports))
	for _, exp := range report.Exports {
		exportSet[exp] = struct{}{}
	}
	for _, req := range manifest.BaseRequiredExports {
		if _, ok := exportSet[req]; !ok {
			addSmokeFailure(&report, "missing-export",
				fmt.Sprintf("required export %q not found in runtime exports", req), binaryPath)
		}
	}

	// Initial configure with smoke config.
	if client.HasFunction("configure") {
		_ = configurePlugin(client, cloneMap(opts.SmokeConfig))
	}

	// Determine plugin type: use opts.PluginType if provided, otherwise use report.Type, or detect from binary name
	pluginType := opts.PluginType
	if pluginType == "" {
		pluginType = report.Type
	}
	if pluginType == "" {
		// Fallback: detect from binary name
		lowerPath := strings.ToLower(binaryPath)
		if strings.Contains(lowerPath, "memory") {
			pluginType = "memory"
		} else if strings.Contains(lowerPath, "secrets") {
			pluginType = "secrets"
		} else if strings.Contains(lowerPath, "ai") {
			pluginType = "ai"
		} else if strings.Contains(lowerPath, "messaging") || strings.Contains(lowerPath, "message") {
			pluginType = "messaging"
		}
	}

	// Run smoke tests only if explicitly requested OR if this is the old behavior (no type specified)
	shouldRunSmoke := opts.RunSmokeTest || opts.PluginType == ""

	// Type-specific happy-path smoke tests, only if shouldRunSmoke is true
	if shouldRunSmoke {
		switch pluginType {
		case "memory":
			tmpDir, _ := os.MkdirTemp("", "openlobster-smoke-")
			defer os.RemoveAll(tmpDir)
			runMemorySmoke(client, &report, opts, binaryPath, tmpDir)
		case "secrets":
			tmpDir, _ := os.MkdirTemp("", "openlobster-smoke-")
			defer os.RemoveAll(tmpDir)
			runSecretsSmoke(client, &report, opts, binaryPath, tmpDir)
		case "audio":
			runAudioSmoke(client, &report, opts, binaryPath)
		case "ai":
			runAISmoke(client, &report, opts, binaryPath)
		case "messaging":
			runMessagingSmoke(client, &report, opts, binaryPath)
		}
	}

	// Exhaustive error-path: every exported function must survive nil input.
	runErrorPathSmoke(client, &report, binaryPath)

	return report, nil
}

// ---------------------------------------------------------------------------
// Error-path smoke
// ---------------------------------------------------------------------------

// runErrorPathSmoke calls every exported function with nil input and verifies
// the plugin does not crash or hang. Returning a plugin-level error is
// acceptable — only a transport failure (timeout, EOF) is treated as a failure.
func runErrorPathSmoke(client protocol.PluginClient, report *types.PluginReport, file string) {
	skip := map[string]struct{}{
		"configure":    {},
		"get_metadata": {},
		"close":        {},
		"start":        {},
		"stop":         {},
	}
	exports := make([]string, len(report.Exports))
	copy(exports, report.Exports)
	sort.Strings(exports)

	for _, fn := range exports {
		if _, ok := skip[fn]; ok {
			continue
		}
		_, err := client.CallJSON(fn, nil)
		if err == nil {
			continue
		}
		if isTransportFailure(err.Error()) {
			addSmokeFailure(report, "error-path",
				fmt.Sprintf("%s: crashed or hung on nil input: %v", fn, err), file)
			return
		}
		// Plugin returned an error string — expected for missing required fields.
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func addSmokeFailure(report *types.PluginReport, name, message, file string) {
	report.AddIssue(types.SeverityError, types.SmokeFailRule,
		fmt.Sprintf("%s: %s", name, message), file)
}

func configurePlugin(client protocol.PluginClient, cfg map[string]any) error {
	if !client.HasFunction("configure") {
		return nil
	}
	// Merge cfg at root level, not wrapped in "config"
	payload := map[string]any{}
	for k, v := range cfg {
		payload[k] = v
	}
	_, err := client.CallJSON("configure", payload)
	return err
}

// isTransportFailure returns true when the error indicates the plugin process
// crashed or timed out (as opposed to returning a plugin-level error string).
func isTransportFailure(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "eof") ||
		strings.Contains(lower, "broken pipe") ||
		strings.Contains(lower, "read response")
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
