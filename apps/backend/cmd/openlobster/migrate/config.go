// # License
// See LICENSE in the root of the repository.
package migrate

import (
	"fmt"
	"slices"
	"strings"
)

// migrateConfig sends an updateConfig mutation with every field found in the
// OpenClaw config that has a known OpenLobster equivalent.
func migrateConfig(cfg viperReader, c *gqlClient) error {
	input := map[string]any{}

	// Agent name — try both OpenClaw key layouts.
	for _, key := range []string{"agent.name", "agents.defaults.name"} {
		if v := cfg.GetString(key); v != "" {
			input["agentName"] = v
			break
		}
	}

	// Model — detect provider from the model string.
	if model := cfg.GetString("agents.defaults.model.primary"); model != "" {
		provider, modelID := detectProvider(model)
		if provider != "" {
			input["provider"] = provider
		}
		input["model"] = modelID
	}

	// System prompt.
	for _, key := range []string{"agent.systemPrompt", "agents.defaults.systemPrompt", "agents.defaults.soul"} {
		if v := cfg.GetString(key); v != "" {
			input["systemPrompt"] = v
			break
		}
	}

	// Base URL for the model provider (custom endpoints, OpenRouter, LiteLLM, etc.)
	for _, key := range []string{
		"agents.defaults.model.baseURL",
		"agents.defaults.model.base_url",
		"models.providers.openai.baseURL",
	} {
		if v := cfg.GetString(key); v != "" {
			input["baseURL"] = v
			break
		}
	}

	// Ollama host — if model is ollama-based, also set ollamaHost.
	for _, key := range []string{
		"agents.defaults.model.ollamaHost",
		"models.providers.ollama.baseURL",
		"models.providers.ollama.host",
	} {
		if v := cfg.GetString(key); v != "" {
			input["ollamaHost"] = v
			break
		}
	}

	// API keys injected from ~/.openclaw/.env by enrichWithEnv.
	if v := cfg.GetString("env.anthropic_api_key"); v != "" {
		input["anthropicApiKey"] = v
	}
	if v := cfg.GetString("env.ollama_api_key"); v != "" {
		input["ollamaApiKey"] = v
	}
	// For OpenAI / OpenRouter, use the generic apiKey field (only when not anthropic).
	if _, isAnthropic := input["anthropicApiKey"]; !isAnthropic {
		for _, key := range []string{"env.openai_api_key", "env.openrouter_api_key"} {
			if v := cfg.GetString(key); v != "" {
				input["apiKey"] = v
				break
			}
		}
	}

	// Logging.
	for _, key := range []string{"logging.level", "log.level"} {
		if v := cfg.GetString(key); v != "" {
			input["loggingLevel"] = v
			break
		}
	}
	for _, key := range []string{"logging.file", "logging.path", "log.path"} {
		if v := cfg.GetString(key); v != "" {
			input["loggingPath"] = v
			break
		}
	}

	// Scheduler.
	if v := cfg.Get("scheduler.enabled"); v != nil {
		input["schedulerEnabled"] = v
	}

	// Mark setup wizard as completed — migration constitutes setup.
	input["wizardCompleted"] = true

	// Channel field mappings: OpenClaw key → OpenLobster GraphQL input field.
	type fieldMap struct{ src, dst string }
	channelFields := []fieldMap{
		// Telegram
		{"channels.telegram.enabled", "channelTelegramEnabled"},
		{"channels.telegram.botToken", "channelTelegramToken"},
		{"channels.telegram.bot_token", "channelTelegramToken"},
		// Discord
		{"channels.discord.enabled", "channelDiscordEnabled"},
		{"channels.discord.token", "channelDiscordToken"},
		{"channels.discord.botToken", "channelDiscordToken"},
		// Slack
		{"channels.slack.enabled", "channelSlackEnabled"},
		{"channels.slack.botToken", "channelSlackBotToken"},
		{"channels.slack.bot_token", "channelSlackBotToken"},
		{"channels.slack.appToken", "channelSlackAppToken"},
		{"channels.slack.app_token", "channelSlackAppToken"},
		// WhatsApp — credentials are incompatible; enabled flag only.
		{"channels.whatsapp.enabled", "channelWhatsAppEnabled"},
		// Twilio
		{"channels.twilio.enabled", "channelTwilioEnabled"},
		{"channels.twilio.accountSid", "channelTwilioAccountSid"},
		{"channels.twilio.account_sid", "channelTwilioAccountSid"},
		{"channels.twilio.authToken", "channelTwilioAuthToken"},
		{"channels.twilio.auth_token", "channelTwilioAuthToken"},
		{"channels.twilio.fromNumber", "channelTwilioFromNumber"},
		{"channels.twilio.from_number", "channelTwilioFromNumber"},
	}

	for _, m := range channelFields {
		val := cfg.Get(m.src)
		if val == nil {
			continue
		}
		if s, ok := val.(string); ok && isPlaceholder(s) {
			continue
		}
		// Avoid overwriting a dst field already set by an earlier alias.
		if _, exists := input[m.dst]; !exists {
			input[m.dst] = val
		}
	}

	if len(input) == 0 {
		fmt.Println("config: no mappable fields found — skipping")
		return nil
	}

	fmt.Printf("config: %d field(s) to migrate\n", len(input))
	for k, v := range input {
		fmt.Printf("  %-40s = %v\n", k, maskSecret(k, v))
	}

	if c.dryRun {
		return nil
	}

	const mutation = `mutation UpdateConfig($input: UpdateConfigInput!) {
		updateConfig(input: $input) { success error }
	}`

	var result struct {
		UpdateConfig struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		} `json:"updateConfig"`
	}
	if err := c.do(mutation, map[string]any{"input": input}, &result); err != nil {
		return err
	}
	if !result.UpdateConfig.Success {
		return fmt.Errorf("%s", result.UpdateConfig.Error)
	}
	fmt.Println("  saved")
	return nil
}

// detectProvider infers the OpenLobster provider name from an OpenClaw model string.
func detectProvider(model string) (provider, modelID string) {
	lower := strings.ToLower(model)
	switch {
	case strings.HasPrefix(lower, "claude-"):
		return "anthropic", model
	case strings.HasPrefix(lower, "gpt-"),
		strings.HasPrefix(lower, "o1"), strings.HasPrefix(lower, "o3"), strings.HasPrefix(lower, "o4"):
		return "openai", model
	case strings.HasPrefix(lower, "gemini-"):
		return "openai", model // OpenLobster uses openai-compatible endpoint for Gemini
	default:
		return "", model
	}
}

// maskSecret redacts sensitive values when printing to stdout.
func maskSecret(key string, val any) any {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "token") || strings.Contains(lower, "key") ||
		strings.Contains(lower, "secret") || strings.Contains(lower, "password") ||
		strings.Contains(lower, "sid") {
		if s, ok := val.(string); ok && len(s) > 4 {
			return s[:4] + strings.Repeat("*", len(s)-4)
		}
	}
	return val
}

func isPlaceholder(s string) bool {
	return slices.Contains([]string{
		"", "YOUR_API_KEY_HERE", "YOUR_BOT_TOKEN_HERE",
		"YOUR_ACCOUNT_SID", "YOUR_AUTH_TOKEN", "YOUR_API_TOKEN_HERE",
	}, s)
}
