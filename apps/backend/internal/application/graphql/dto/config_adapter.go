// ConfigUpdateAdapter persists GraphQL config mutations into viper + disk.
package dto

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/spf13/viper"
)

// ConfigUpdateAdapter persists UpdateConfigInput into viper and reconciles
// runtime state through callbacks.
// When provider/agent keys change, OnApplied receives providerTouched=true and
// must refresh ConfigSnapshot and perform a soft reboot (recreate AI provider).
type ConfigUpdateAdapter struct {
	mu            sync.Mutex
	ConfigPath    string
	ReloadChannel func(channelType string)
	ViperKeys     map[string]string
	OnApplied     func(providerTouched bool)
}

// Apply saves the input fields to viper, persists to disk, and triggers any
// required channel reloads or provider soft-reboots.
func (a *ConfigUpdateAdapter) Apply(ctx context.Context, input map[string]interface{}) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var changedChannels []string
	channelTouched := make(map[string]bool)

	a.applyProviderKeys(input)

	if caps, ok := input["capabilities"].(map[string]interface{}); ok {
		// Read the full current capabilities map and merge incoming changes into
		// it, then set the parent key as a whole. Setting sub-keys individually
		// (e.g. "agent.capabilities.browser") can interact poorly with viper's
		// nested-map merging when AllSettings() is called for serialisation —
		// setting the parent map atomically avoids the issue.
		merged := map[string]interface{}{
			"browser":    viper.GetBool("agent.capabilities.browser"),
			"terminal":   viper.GetBool("agent.capabilities.terminal"),
			"subagents":  viper.GetBool("agent.capabilities.subagents"),
			"memory":     viper.GetBool("agent.capabilities.memory"),
			"mcp":        viper.GetBool("agent.capabilities.mcp"),
			"filesystem": viper.GetBool("agent.capabilities.filesystem"),
			"sessions":   viper.GetBool("agent.capabilities.sessions"),
		}
		for k, v := range caps {
			merged[k] = v
		}
		viper.Set("agent.capabilities", merged)
	}

	for inputKey, val := range input {
		if inputKey == "capabilities" || a.isProviderInputKey(inputKey) {
			continue
		}

		// Standard field mapping via ViperKeys
		viperKey, ok := a.ViperKeys[inputKey]
		if !ok {
			// If it's not a mapped field, but it's a dotted key (likely from a dynamic plugin),
			// allow it to be set directly in Viper.
			if strings.Contains(inputKey, ".") {
				viper.Set(inputKey, val)
			}
			continue
		}
		viper.Set(viperKey, val)
		switch inputKey {
		case "channelTelegramEnabled", "channelTelegramToken":
			channelTouched["telegram"] = true
		case "channelDiscordEnabled", "channelDiscordToken":
			channelTouched["discord"] = true
		case "channelSlackEnabled", "channelSlackBotToken", "channelSlackAppToken":
			channelTouched["slack"] = true
		case "channelWhatsAppEnabled", "channelWhatsAppPhoneId", "channelWhatsAppApiToken":
			channelTouched["whatsapp"] = true
		case "channelTwilioEnabled", "channelTwilioAccountSid", "channelTwilioAuthToken", "channelTwilioFromNumber":
			channelTouched["twilio"] = true
		}
	}
	for ch := range channelTouched {
		changedChannels = append(changedChannels, ch)
	}

	providerTouched := false
	memoryTouched := false
	secretsTouched := false

	for k := range input {
		if a.isProviderInputKey(k) || strings.HasPrefix(k, "providers.") {
			providerTouched = true
		}
		if strings.HasPrefix(k, "memory.") {
			memoryTouched = true
		}
		if strings.HasPrefix(k, "secrets.") {
			secretsTouched = true
		}
	}

	if len(input) > 0 {
		if err := config.WriteEncryptedConfig(a.ConfigPath); err != nil {
			return nil, fmt.Errorf("persisting config to %s: %w", a.ConfigPath, err)
		}

		// Ensure Viper state is flushed and re-synced with disk through our decryption layer.
		data, err := config.ReadConfigBytes(a.ConfigPath)
		if err == nil {
			viper.SetConfigType("yaml")
			if err := viper.ReadConfig(bytes.NewReader(data)); err != nil {
				log.Printf("adapter: warning: re-parsing config after write failed: %v", err)
			}
		} else {
			log.Printf("adapter: warning: decrypting config after write failed: %v", err)
		}

		// Keep a single runtime reconciliation path when OnApplied is available.
		if a.OnApplied != nil {
			a.OnApplied(providerTouched || memoryTouched || secretsTouched)
		} else if a.ReloadChannel != nil {
			for _, ch := range changedChannels {
				a.ReloadChannel(ch)
			}
		}
	}
	return changedChannels, nil
}
func (a *ConfigUpdateAdapter) isProviderInputKey(k string) bool {
	switch k {
	case "provider", "model", "apiKey", "baseURL", "ollamaHost", "ollamaApiKey",
		"anthropicApiKey", "dockerModelRunnerEndpoint",
		"reasoningLevel":
		return true
	}
	return false
}

func (a *ConfigUpdateAdapter) applyProviderKeys(input map[string]interface{}) {
	provider, _ := input["provider"].(string)
	if provider == "" {
		provider = viper.GetString("agent.provider")
	}
	if p, ok := input["provider"].(string); ok && p != "" {
		viper.Set("agent.provider", p)
	}
	switch provider {
	case "openrouter":
		if v, ok := input["apiKey"].(string); ok && v != "" {
			viper.Set("providers.openrouter.api_key", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.openrouter.default_model", v)
		}
	case "ollama":
		if v, ok := input["baseURL"].(string); ok && v != "" {
			viper.Set("providers.ollama.endpoint", v)
		} else if v, ok := input["ollamaHost"].(string); ok && v != "" {
			viper.Set("providers.ollama.endpoint", v)
		}
		if v, ok := input["apiKey"].(string); ok && v != "" {
			viper.Set("providers.ollama.api_key", v)
		} else if v, ok := input["ollamaApiKey"].(string); ok && v != "" {
			viper.Set("providers.ollama.api_key", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.ollama.default_model", v)
		}
	case "openai":
		if v, ok := input["apiKey"].(string); ok && v != "" {
			viper.Set("providers.openai.api_key", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.openai.model", v)
		}
		if v, ok := input["baseURL"].(string); ok && v != "" {
			viper.Set("providers.openai.base_url", v)
		}
	case "anthropic":
		if v, ok := input["apiKey"].(string); ok && v != "" {
			viper.Set("providers.anthropic.api_key", v)
		} else if v, ok := input["anthropicApiKey"].(string); ok && v != "" {
			viper.Set("providers.anthropic.api_key", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.anthropic.model", v)
		}
	case "openai-compatible":
		if v, ok := input["apiKey"].(string); ok && v != "" {
			viper.Set("providers.openaicompat.api_key", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.openaicompat.model", v)
		}
		if v, ok := input["baseURL"].(string); ok && v != "" {
			viper.Set("providers.openaicompat.base_url", v)
		}
	case "opencode-zen":
		if v, ok := input["apiKey"].(string); ok && v != "" {
			viper.Set("providers.opencode.api_key", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.opencode.model", v)
		}
	case "docker-model-runner":
		if v, ok := input["dockerModelRunnerEndpoint"].(string); ok && v != "" {
			viper.Set("providers.docker_model_runner.endpoint", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.docker_model_runner.default_model", v)
		}
	default:
		// Dynamic plugins are now handled via direct dotted keys in the main Apply loop.
	}

	if v, ok := input["reasoningLevel"].(string); ok && v != "" {
		viper.Set("agent.reasoning_level", v)
	}
}

// InputToViperKeyMap returns the mapping from GraphQL input field names to
// their corresponding viper config keys.
func InputToViperKeyMap() map[string]string {
	return map[string]string{
		"agentName":               "agent.name",
		"systemPrompt":            "agent.system_prompt",
		"databaseDriver":          "database.driver",
		"databaseDSN":             "database.dsn",
		"databaseMaxOpenConns":    "database.max_open_conns",
		"databaseMaxIdleConns":    "database.max_idle_conns",
		"memoryBackend":           "memory.backend",
		"memoryFilePath":          "memory.file.path",
		"memoryNeo4jURI":          "memory.neo4j.uri",
		"memoryNeo4jUser":         "memory.neo4j.user",
		"memoryNeo4jPassword":     "memory.neo4j.password",
		"subagentsMaxConcurrent":  "subagents.max_concurrent",
		"subagentsDefaultTimeout": "subagents.default_timeout",
		"graphqlEnabled":          "graphql.enabled",
		"graphqlPort":             "graphql.port",
		"graphqlHost":             "graphql.host",
		"graphqlBaseUrl":          "graphql.base_url",
		"webEnabled":              "web.enabled",
		"loggingLevel":            "logging.level",
		"loggingPath":             "logging.path",
		"secretsBackend":          "secrets.backend",
		"secretsFilePath":         "secrets.file.path",
		"secretsOpenbaoURL":       "secrets.openbao.url",
		"secretsOpenbaoToken":     "secrets.openbao.token",
		"pluginDefaultMemory":     "memory.backend",
		"pluginDefaultSecrets":    "secrets.backend",
		"pluginDefaultAudio":      "audio.backend",
		"pluginDefaultAi":         "agent.provider",
		"a2aEnabled":              "a2a.enabled",
		"schedulerEnabled":        "scheduler.enabled",
		"schedulerMemoryEnabled":  "scheduler.memory_enabled",
		"schedulerMemoryInterval": "scheduler.memory_interval",
		"channelTelegramEnabled":  "channels.telegram.enabled",
		"channelTelegramToken":    "channels.telegram.bot_token",
		"channelDiscordEnabled":   "channels.discord.enabled",
		"channelDiscordToken":     "channels.discord.bot_token",
		"channelWhatsAppEnabled":  "channels.whatsapp.enabled",
		"channelWhatsAppPhoneId":  "channels.whatsapp.phone_id",
		"channelWhatsAppApiToken": "channels.whatsapp.api_token",
		"channelTwilioEnabled":    "channels.twilio.enabled",
		"channelTwilioAccountSid": "channels.twilio.account_sid",
		"channelTwilioAuthToken":  "channels.twilio.auth_token",
		"channelTwilioFromNumber": "channels.twilio.from_number",
		"channelSlackEnabled":     "channels.slack.enabled",
		"channelSlackBotToken":    "channels.slack.bot_token",
		"channelSlackAppToken":    "channels.slack.app_token",
		"wizardCompleted":         "wizard.completed",
		"reasoningLevel":          "agent.reasoning_level",
	}
}
