// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package backcompat provides compatibility checks between the current plugin
// JSON schema and the configuration keys expected in release/0.3.0 (when
// plugins did not exist and all config lived in Viper/YAML).
//
// For each built-in plugin, the 030 reference defines the minimum set of
// properties that must still be present in the plugin's JSON schema.
package backcompat

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RefProperty describes a single property expected in the JSON schema.
type RefProperty struct {
	Type     string // expected JSON Schema type ("string", "boolean", "integer")
	Required bool   // was it required in 0.3.0?
}

// RefSchema is the minimum JSON Schema shape a plugin must satisfy to be
// backwards-compatible with OpenLobster 0.3.0 deployments.
type RefSchema struct {
	Properties map[string]RefProperty
}

// --- 0.3.0 reference: derived from Viper mapstructure tags in ---
// --- apps/backend/internal/infrastructure/config/config.go (tag: release/0.3.0)

var ref030 = map[string]RefSchema{

	// --- MEMORY: Neo4j ---
	// Viper 0.3.0 keys  → Schema props
	// memory.neo4j.uri      → uri        (string, required)
	// memory.neo4j.user     → user       (string, required) — canonical, "username" is alias
	// memory.neo4j.password → password   (string, required)
	// Source: config.go line 246-249 — type MemoryNeo4jConfig struct
	"neo4j": {
		Properties: map[string]RefProperty{
			"uri":      {Type: "string", Required: true},
			"user":     {Type: "string", Required: true},
			"password": {Type: "string", Required: true},
		},
	},

	// --- SECRETS: OpenBao ---
	// Viper 0.3.0 keys        → Schema props
	// secrets.openbao.url      → url        (string, required)
	// secrets.openbao.token    → token      (string, required)
	// secrets.openbao.mount    → mount      (string, default "secret") — NEW in current, not in 0.3.0
	// Source: config.go line 422-425 — type OpenbaoSecretsConfig struct
	"openbao": {
		Properties: map[string]RefProperty{
			"url":   {Type: "string", Required: true},
			"token": {Type: "string", Required: true},
			"mount": {Type: "string", Required: false},
		},
	},

// --- MEMORY: GML (file-based graph) ---
// Viper 0.3.0 keys  → Schema props
// memory.file.path   → path       (string, required)
// Source: config.go line 242-243 — type MemoryFileConfig struct
// Plugin info.id = "memory:file"
"memory:file": {
    Properties: map[string]RefProperty{
        "path": {Type: "string", Required: true},
    },
},

// --- SECRETS: JSON file ---
// Viper 0.3.0 keys   → Schema props
// secrets.file.path   → path       (string, required)
// Source: config.go line 418-419 — type SecretsFileConfig struct
// Plugin info.id = "file"
"file": {
    Properties: map[string]RefProperty{
        "path": {Type: "string", Required: true},
    },
},

	// --- AI: Ollama ---
	// Viper 0.3.0 keys                → Schema props
	// providers.ollama.endpoint        → endpoint       (string)
	// providers.ollama.default_model   → default_model  (string)
	// providers.ollama.api_key         → api_key        (string)
	// Source: config.go line 274-278 — type OllamaConfig struct
	"ollama": {
		Properties: map[string]RefProperty{
			"endpoint":      {Type: "string", Required: false},
			"default_model": {Type: "string", Required: false},
			"api_key":       {Type: "string", Required: false},
		},
	},

	// --- AI: OpenAI ---
	// Viper 0.3.0 keys       → Schema props
	// providers.openai.api_key  → api_key     (string)
	// providers.openai.model    → model       (string)
	// providers.openai.base_url → base_url    (string)
	// Source: config.go line 288-292 — type OpenAIConfig struct
	"openai": {
		Properties: map[string]RefProperty{
			"api_key":  {Type: "string", Required: false},
			"model":    {Type: "string", Required: false},
			"base_url": {Type: "string", Required: false},
		},
	},

	// --- AI: Anthropic ---
	// Viper 0.3.0 keys          → Schema props
	// providers.anthropic.api_key  → api_key    (string)
	// providers.anthropic.model    → model      (string)
	// Source: config.go line 308-310 — type AnthropicConfig struct
	"anthropic": {
		Properties: map[string]RefProperty{
			"api_key": {Type: "string", Required: false},
			"model":   {Type: "string", Required: false},
		},
	},

	// --- MESSAGING: Telegram ---
	// Viper 0.3.0 keys           → Schema props
	// channels.telegram.bot_token  → bot_token  (string, required)
	// Source: config.go line 331-333 — type TelegramConfig struct
	"telegram": {
		Properties: map[string]RefProperty{
			"bot_token": {Type: "string", Required: true},
		},
	},

	// --- MESSAGING: Discord ---
	// Viper 0.3.0 keys         → Schema props
	// channels.discord.bot_token  → bot_token  (string, required)
	// Source: config.go line 336-338 — type DiscordConfig struct
	"discord": {
		Properties: map[string]RefProperty{
			"bot_token": {Type: "string", Required: true},
		},
	},

	// --- MESSAGING: Slack ---
	// Viper 0.3.0 keys       → Schema props
	// channels.slack.bot_token  → bot_token  (string, required)
	// channels.slack.app_token  → app_token  (string, required)
	// Source: config.go line 358-361 — type SlackConfig struct
	"slack": {
		Properties: map[string]RefProperty{
			"bot_token": {Type: "string", Required: true},
			"app_token": {Type: "string", Required: true},
		},
	},

	// --- AUDIO: ElevenLabs — NOT in 0.3.0 ---
	// There was no audio.* section in the Config struct in 0.3.0.
	// OpenLobster 0.3.0 did not support audio plugins.
	// No backcompat reference is needed.
}

// KnownBuiltins maps short plugin IDs to their canonical identifiers.
// These map to the builtins that existed as Viper config sections in 0.3.0.
//
// User-facing name → plugin info.id (reported by get_info):
//   GML          → "memory:file"
//   Secrets-file → "file"
//   Neo4j        → "neo4j"
//   OpenBao      → "openbao"
//   Telegram     → "telegram"
//   Discord      → "discord"
//   Slack        → "slack"
//   Anthropic    → "anthropic"
//   Ollama       → "ollama"
//   OpenAI       → "openai"
var KnownBuiltins = map[string]bool{
	"memory:file": true,
	"file":        true,
	"neo4j":       true,
	"openbao":     true,
	"ollama":      true,
	"openai":      true,
	"anthropic":   true,
	"telegram":    true,
	"discord":     true,
	"slack":       true,
}

// CheckReport runs the backwards-compatibility check for a plugin and reports
// any issues found.  Returns nil if the plugin is not a known built-in or has
// no 030 reference.
func CheckReport(pluginID string, currentSchema json.RawMessage) ([]BackCompatIssue, error) {
	ref, ok := ref030[pluginID]
	if !ok {
		return nil, nil // no reference to validate against
	}

	if len(currentSchema) == 0 {
		return nil, fmt.Errorf("plugin %q: no JSON schema to validate backcompat against", pluginID)
	}

	var current struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(currentSchema, &current); err != nil {
		return nil, fmt.Errorf("plugin %q: invalid JSON schema: %w", pluginID, err)
	}

	requiredSet := make(map[string]struct{}, len(current.Required))
	for _, r := range current.Required {
		requiredSet[strings.TrimSpace(r)] = struct{}{}
	}

	var issues []BackCompatIssue

	for propName, refProp := range ref.Properties {
		currentRaw, exists := current.Properties[propName]
		if !exists {
			// Also try common aliases
			found := false
			for _, alias := range Aliases(propName) {
				if _, ok := current.Properties[alias]; ok {
					found = true
					break
				}
			}
			if !found {
				issues = append(issues, BackCompatIssue{
					Property: propName,
					Message:  fmt.Sprintf("property %q from 0.3.0 is missing from current JSON schema", propName),
				})
				continue
			}
			// Use the alias'd property for type checking
			for _, alias := range Aliases(propName) {
				if raw, ok := current.Properties[alias]; ok {
					currentRaw = raw
					break
				}
			}
		}

		// Type compatibility check
		var cp struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(currentRaw, &cp); err == nil {
			ct := strings.TrimSpace(cp.Type)
			if ct != "" && ct != refProp.Type {
				issues = append(issues, BackCompatIssue{
					Property: propName,
					Message:  fmt.Sprintf("property %q: type changed from %q (0.3.0) to %q (current)", propName, refProp.Type, ct),
				})
			}
		}

		// Required status check
		if refProp.Required {
			if _, ok := requiredSet[propName]; !ok {
				// Check aliases too
				aliasRequired := false
				for _, alias := range Aliases(propName) {
					if _, ok := requiredSet[alias]; ok {
						aliasRequired = true
						break
					}
				}
				if !aliasRequired {
					issues = append(issues, BackCompatIssue{
						Property: propName,
						Message:  fmt.Sprintf("property %q was required in 0.3.0 but is not marked required in current schema", propName),
					})
				}
			}
		}
	}

	return issues, nil
}

// Aliases returns common property name aliases for 0.3.0 → current mapping.
func Aliases(name string) []string {
	// Some Viper keys had different names than the plugin schema property.
	aliases := map[string][]string{
		"user":      {"username"},
		"username":  {"user"},
		"bot_token": {"token"},
		"token":     {"bot_token"},
		"app_token": {"app_token", "token"},
	}
	if a, ok := aliases[name]; ok {
		return a
	}
	return nil
}

// BackCompatIssue describes a single backcompat finding.
type BackCompatIssue struct {
	Property string `json:"property"`
	Message  string `json:"message"`
}
