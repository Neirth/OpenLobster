// Copyright (c) OpenLobster contributors. See LICENSE for details.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePluginDirMissingConfigureExport(t *testing.T) {
	dir := writeTempPlugin(t, "openlobster-ai-test", `package main
import pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
func getMetadata() { pdk.OutputString(`+"`"+`{"id":"openlobster-ai-test","name":"test","version":"0.1.0","description":"test","type":"ai","schema":{"type":"object","properties":{}},"properties":{}}`+"`"+`) }
func main() {
	pdk.MustRun(pdk.Plugin{ID: "openlobster-ai-test", Exports: map[string]pdk.Function{
		"get_metadata": getMetadata,
	}})
}`)

	report, err := ValidatePluginDir(dir)
	if err != nil {
		t.Fatalf("ValidatePluginDir() error = %v", err)
	}
	if !issueContains(report, SeverityError, "missing-export", `required export "configure" is missing`) {
		t.Fatalf("expected missing configure issue, got %#v", report.Issues)
	}
}

func TestValidatePluginDirMissingGetMetadataExport(t *testing.T) {
	dir := writeTempPlugin(t, "openlobster-secrets-test", `package main
import pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
func configureHot() {}
func main() {
	pdk.MustRun(pdk.Plugin{ID: "openlobster-secrets-test", Exports: map[string]pdk.Function{
		"configure": configureHot,
	}})
}`)

	report, err := ValidatePluginDir(dir)
	if err != nil {
		t.Fatalf("ValidatePluginDir() error = %v", err)
	}
	if !issueContains(report, SeverityError, "missing-export", `required export "get_metadata" is missing`) {
		t.Fatalf("expected missing get_metadata issue, got %#v", report.Issues)
	}
}

func TestValidatePluginDirLegacyMetadataExportsForbidden(t *testing.T) {
	dir := writeTempPlugin(t, "openlobster-secrets-test", `package main
import pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
func getMetadata() { pdk.OutputString(`+"`"+`{"id":"openlobster-secrets-test","name":"test","version":"0.1.0","description":"test","type":"secrets","schema":{"type":"object","properties":{}},"properties":{}}`+"`"+`) }
func configureHot() {}
func main() {
	pdk.MustRun(pdk.Plugin{ID: "openlobster-secrets-test", Exports: map[string]pdk.Function{
		"get_metadata": getMetadata,
		"configure":    configureHot,
		"get_name":     getMetadata,
	}})
}`)

	report, err := ValidatePluginDir(dir)
	if err != nil {
		t.Fatalf("ValidatePluginDir() error = %v", err)
	}
	if !issueContains(report, SeverityError, "metadata-export", "legacy metadata exports are forbidden") {
		t.Fatalf("expected legacy export error, got %#v", report.Issues)
	}
}

func writeTempPlugin(t *testing.T, name string, source string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	return dir
}

func issueContains(report PluginReport, severity Severity, rule string, substring string) bool {
	for _, issue := range report.Issues {
		if issue.Severity != severity || issue.Rule != rule {
			continue
		}
		if strings.Contains(issue.Message, substring) {
			return true
		}
	}
	return false
}

func TestResolvePluginTargetsSelectorPath(t *testing.T) {
	pluginsDir := t.TempDir()
	path := filepath.Join(pluginsDir, "openlobster-messages-telegram")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir plugin dir: %v", err)
	}

	targets, err := resolvePluginTargets(pluginsDir, path)
	if err != nil {
		t.Fatalf("resolvePluginTargets(path) error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1", len(targets))
	}
	if filepath.Clean(targets[0]) != filepath.Clean(path) {
		t.Fatalf("target = %q, want %q", targets[0], path)
	}
}

func TestResolvePluginTargetsSelectorSubstring(t *testing.T) {
	pluginsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pluginsDir, "openlobster-messages-telegram"), 0o755); err != nil {
		t.Fatalf("mkdir telegram dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginsDir, "openlobster-ai-openai"), 0o755); err != nil {
		t.Fatalf("mkdir ai dir: %v", err)
	}

	targets, err := resolvePluginTargets(pluginsDir, "telegram")
	if err != nil {
		t.Fatalf("resolvePluginTargets(substring) error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("len(targets) = %d, want 1", len(targets))
	}
	if filepath.Base(targets[0]) != "openlobster-messages-telegram" {
		t.Fatalf("base target = %q, want openlobster-messages-telegram", filepath.Base(targets[0]))
	}
}

func TestResolvePluginTargetsSelectorNoMatch(t *testing.T) {
	pluginsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pluginsDir, "openlobster-messages-telegram"), 0o755); err != nil {
		t.Fatalf("mkdir telegram dir: %v", err)
	}

	if _, err := resolvePluginTargets(pluginsDir, "does-not-exist"); err == nil {
		t.Fatalf("expected no-match error")
	}
}
