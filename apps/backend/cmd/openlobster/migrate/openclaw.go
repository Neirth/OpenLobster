// # License
// See LICENSE in the root of the repository.
package migrate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// viperReader is the subset of viper.Viper used across migration steps,
// allowing test doubles to be injected without importing viper directly.
type viperReader interface {
	Get(key string) any
	GetString(key string) string
}

// cronJob represents a cron job entry from an OpenClaw config.
type cronJob struct {
	ID       string
	Name     string
	Schedule string
	Prompt   string
	Enabled  bool
}

// defaultOpenClawHome returns the default OpenClaw home directory (~/.openclaw),
// falling back to ./.openclaw if the home directory cannot be determined.
func defaultOpenClawHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./.openclaw"
	}
	return filepath.Join(home, ".openclaw")
}

// readOpenClaw reads the OpenClaw config file from the given home directory
// into a Viper instance. It looks for openclaw.json (JSON5 without comments)
// and falls back to openclaw.yaml / openclaw.yml.
func readOpenClaw(homeDir string) *viper.Viper {
	candidates := []struct {
		name     string
		cfgType  string
	}{
		{"openclaw.json", "json"},
		{"openclaw.yaml", "yaml"},
		{"openclaw.yml", "yaml"},
	}

	for _, c := range candidates {
		path := filepath.Join(homeDir, c.name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		v := viper.New()
		v.SetConfigType(c.cfgType)
		if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
			fmt.Fprintf(os.Stderr, "migrate: cannot parse %s: %v\n", path, err)
			os.Exit(1)
		}
		return v
	}

	fmt.Fprintf(os.Stderr, "migrate: no config file found in %s\n", homeDir)
	os.Exit(1)
	return nil
}

// readCronJobs extracts cron job definitions from the OpenClaw config.
// OpenClaw may store jobs under several different key paths.
func readCronJobs(cfg viperReader) []cronJob {
	candidates := []string{"scheduler.jobs", "cron.jobs", "gateway.cron.jobs"}
	for _, key := range candidates {
		raw := cfg.Get(key)
		if raw == nil {
			continue
		}
		list, ok := raw.([]any)
		if !ok || len(list) == 0 {
			continue
		}
		var jobs []cronJob
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			job := cronJob{
				ID:       stringField(m, "id"),
				Name:     stringField(m, "name"),
				Schedule: stringField(m, "schedule"),
				Prompt:   firstStringField(m, "prompt", "message", "task"),
				Enabled:  boolField(m, "enabled", true),
			}
			if !job.Enabled {
				continue
			}
			jobs = append(jobs, job)
		}
		if len(jobs) > 0 {
			return jobs
		}
	}
	return nil
}

// knownEnvKeys maps OpenClaw / provider environment variable names to the
// viper keys used internally during migration.
var knownEnvKeys = map[string]string{
	"ANTHROPIC_API_KEY":  "env.anthropic_api_key",
	"OPENAI_API_KEY":     "env.openai_api_key",
	"OPENROUTER_API_KEY": "env.openrouter_api_key",
	"OLLAMA_API_KEY":     "env.ollama_api_key",
	// OpenClaw live-override pattern: OPENCLAW_LIVE_<PROVIDER>_KEY
	"OPENCLAW_LIVE_ANTHROPIC_KEY":  "env.anthropic_api_key",
	"OPENCLAW_LIVE_OPENAI_KEY":     "env.openai_api_key",
	"OPENCLAW_LIVE_OPENROUTER_KEY": "env.openrouter_api_key",
	"OPENCLAW_LIVE_OLLAMA_KEY":     "env.ollama_api_key",
}

// enrichWithEnv populates the viper instance with provider API keys from two
// sources, in order of increasing priority:
//
//  1. ~/.openclaw/.env file (persistent daemon credentials)
//  2. Current process environment variables (live overrides)
func enrichWithEnv(v *viper.Viper, homeDir string) {
	// 1. Read the .env file first (lowest priority).
	if data, err := os.ReadFile(filepath.Join(homeDir, ".env")); err == nil {
		for envKey, viperKey := range knownEnvKeys {
			if val, ok := parseEnvFile(data)[envKey]; ok && val != "" {
				v.Set(viperKey, val)
			}
		}
	}

	// 2. Live environment variables override the file.
	for envKey, viperKey := range knownEnvKeys {
		if val := os.Getenv(envKey); val != "" {
			v.Set(viperKey, val)
		}
	}
}

// parseEnvFile parses a .env file (KEY=VALUE lines, # comments) into a map.
func parseEnvFile(data []byte) map[string]string {
	result := map[string]string{}
	for _, line := range splitLines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip surrounding quotes.
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		result[key] = val
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// stringField extracts a string value from a map by key.
func stringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// firstStringField returns the first non-empty string found for any of the given keys.
func firstStringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := stringField(m, key); s != "" {
			return s
		}
	}
	return ""
}

// boolField extracts a bool value from a map, returning def if absent.
func boolField(m map[string]any, key string, def bool) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return def
}
