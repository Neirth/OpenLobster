// ConfigUpdateAdapter persists GraphQL config mutations into viper + disk.
package dto

import (
	"context"
	"fmt"

	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/spf13/viper"
)

// ConfigUpdateAdapter persists UpdateConfigInput into viper and reloads channels.
// When provider/agent keys change, OnApplied receives providerTouched=true and
// must refresh ConfigSnapshot and perform a soft reboot (recreate AI provider).
type ConfigUpdateAdapter struct {
	ConfigPath    string
	ReloadChannel func(channelType string)
	ViperKeys     map[string]string
	OnApplied     func(providerTouched bool)
}

// Apply saves the input fields to viper, persists to disk, and triggers any
// required channel reloads or provider soft-reboots.
func (a *ConfigUpdateAdapter) Apply(ctx context.Context, input map[string]interface{}) ([]string, error) {
	var changedChannels []string
	channelTouched := make(map[string]bool)

	a.applyProviderKeys(input)

	if caps, ok := input["capabilities"].(map[string]interface{}); ok {
		for k, v := range caps {
			viper.Set("agent.capabilities."+k, v)
		}
	}

	for inputKey, val := range input {
		if inputKey == "capabilities" || a.isProviderInputKey(inputKey) {
			continue
		}
		viperKey, ok := a.ViperKeys[inputKey]
		if !ok {
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
	for k := range input {
		if a.isProviderInputKey(k) {
			providerTouched = true
			break
		}
	}

	if len(input) > 0 {
		if err := config.WriteEncryptedConfig(a.ConfigPath); err != nil {
			return nil, fmt.Errorf("persisting config to %s: %w", a.ConfigPath, err)
		}
		for _, ch := range changedChannels {
			a.ReloadChannel(ch)
		}
		if a.OnApplied != nil {
			a.OnApplied(providerTouched)
		}
	}
	return changedChannels, nil
}

func (a *ConfigUpdateAdapter) isProviderInputKey(k string) bool {
	switch k {
	case "provider", "model", "apiKey", "baseURL", "ollamaHost", "ollamaApiKey",
		"anthropicApiKey", "dockerModelRunnerEndpoint", "dockerModelRunnerModel":
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
		if v, ok := input["ollamaHost"].(string); ok && v != "" {
			viper.Set("providers.ollama.endpoint", v)
		}
		if v, ok := input["ollamaApiKey"].(string); ok && v != "" {
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
	case "openai-compatible":
		if v, ok := input["apiKey"].(string); ok && v != "" {
			viper.Set("providers.openaicompat.api_key", v)
		}
		if v, ok := input["baseURL"].(string); ok && v != "" {
			viper.Set("providers.openaicompat.base_url", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.openaicompat.model", v)
		}
	case "anthropic":
		if v, ok := input["anthropicApiKey"].(string); ok && v != "" {
			viper.Set("providers.anthropic.api_key", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.anthropic.model", v)
		}
	case "docker-model-runner":
		if v, ok := input["dockerModelRunnerEndpoint"].(string); ok && v != "" {
			viper.Set("providers.docker_model_runner.endpoint", v)
		}
		if v, ok := input["dockerModelRunnerModel"].(string); ok && v != "" {
			viper.Set("providers.docker_model_runner.default_model", v)
		}
	case "opencode-zen":
		if v, ok := input["apiKey"].(string); ok && v != "" {
			viper.Set("providers.opencode.api_key", v)
		}
		if v, ok := input["model"].(string); ok && v != "" {
			viper.Set("providers.opencode.model", v)
		}
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
		"loggingLevel":            "logging.level",
		"loggingPath":             "logging.path",
		"secretsBackend":          "secrets.backend",
		"secretsFilePath":         "secrets.file.path",
		"secretsOpenbaoURL":       "secrets.openbao.url",
		"secretsOpenbaoToken":     "secrets.openbao.token",
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
	}
}
