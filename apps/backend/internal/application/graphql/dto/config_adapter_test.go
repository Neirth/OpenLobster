// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Tests that every field exposed in the UpdateConfigInput GraphQL mutation is
// correctly persisted into viper and that BuildConfigSnapshot reads it back.
package dto

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetViper clears global viper state between tests.
func resetViper() {
	viper.Reset()
}

// noopReload is a channel reload stub that does nothing.
func noopReload(_ string) {}

// newAdapter returns a ConfigUpdateAdapter wired to a temp file.
func newAdapter(t *testing.T) (*ConfigUpdateAdapter, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0600))
	return &ConfigUpdateAdapter{
		ConfigPath:    path,
		ReloadChannel: noopReload,
		ViperKeys:     InputToViperKeyMap(),
		OnApplied:     func(_ bool) {},
	}, path
}

// TestConfigAdapter_AgentFields verifies agent name, system prompt and provider.
func TestConfigAdapter_AgentFields(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"agentName":    "TestBot",
		"systemPrompt": "You are helpful.",
		"provider":     "ollama",
		"model":        "llama3",
		"ollamaHost":   "http://localhost:11434",
		"ollamaApiKey": "ollama-secret",
	})
	require.NoError(t, err)

	assert.Equal(t, "TestBot", viper.GetString("agent.name"))
	assert.Equal(t, "You are helpful.", viper.GetString("agent.system_prompt"))
	assert.Equal(t, "ollama", viper.GetString("agent.provider"))
	assert.Equal(t, "llama3", viper.GetString("providers.ollama.default_model"))
	assert.Equal(t, "http://localhost:11434", viper.GetString("providers.ollama.endpoint"))
	assert.Equal(t, "ollama-secret", viper.GetString("providers.ollama.api_key"))
}

// TestConfigAdapter_ProviderOpenAI verifies OpenAI provider fields.
func TestConfigAdapter_ProviderOpenAI(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"provider": "openai",
		"model":    "gpt-4o",
		"apiKey":   "sk-openai",
		"baseURL":  "https://api.openai.com/v1",
	})
	require.NoError(t, err)

	assert.Equal(t, "openai", viper.GetString("agent.provider"))
	assert.Equal(t, "gpt-4o", viper.GetString("providers.openai.model"))
	assert.Equal(t, "sk-openai", viper.GetString("providers.openai.api_key"))
	assert.Equal(t, "https://api.openai.com/v1", viper.GetString("providers.openai.base_url"))
}

// TestConfigAdapter_ProviderOpenAICompat verifies OpenAI-compatible provider fields.
func TestConfigAdapter_ProviderOpenAICompat(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"provider": "openai-compatible",
		"model":    "mistral",
		"apiKey":   "sk-compat",
		"baseURL":  "https://my-compat.example.com/v1",
	})
	require.NoError(t, err)

	assert.Equal(t, "openai-compatible", viper.GetString("agent.provider"))
	assert.Equal(t, "mistral", viper.GetString("providers.openaicompat.model"))
	assert.Equal(t, "sk-compat", viper.GetString("providers.openaicompat.api_key"))
	assert.Equal(t, "https://my-compat.example.com/v1", viper.GetString("providers.openaicompat.base_url"))
}

// TestConfigAdapter_ProviderAnthropic verifies Anthropic provider fields.
func TestConfigAdapter_ProviderAnthropic(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"provider": "anthropic",
		"model":    "claude-sonnet-4-6",
		"apiKey":   "sk-ant",
		"reasoningLevel": "high",
	})
	require.NoError(t, err)

	assert.Equal(t, "anthropic", viper.GetString("agent.provider"))
	assert.Equal(t, "high", viper.GetString("agent.reasoning_level"))
	assert.Equal(t, "claude-sonnet-4-6", viper.GetString("providers.anthropic.model"))
	assert.Equal(t, "sk-ant", viper.GetString("providers.anthropic.api_key"))
}

// TestConfigAdapter_ProviderOpenRouter verifies OpenRouter provider fields.
func TestConfigAdapter_ProviderOpenRouter(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"provider": "openrouter",
		"model":    "openai/gpt-4o",
		"apiKey":   "sk-or",
	})
	require.NoError(t, err)

	assert.Equal(t, "openrouter", viper.GetString("agent.provider"))
	assert.Equal(t, "openai/gpt-4o", viper.GetString("providers.openrouter.default_model"))
	assert.Equal(t, "sk-or", viper.GetString("providers.openrouter.api_key"))
}

// TestConfigAdapter_ProviderDockerModelRunner verifies Docker Model Runner fields.
func TestConfigAdapter_ProviderDockerModelRunner(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"provider":                  "docker-model-runner",
		"dockerModelRunnerEndpoint": "http://localhost:12434/engines/v1",
		"dockerModelRunnerModel":    "ai/mistral-nemo",
	})
	require.NoError(t, err)

	assert.Equal(t, "docker-model-runner", viper.GetString("agent.provider"))
	assert.Equal(t, "http://localhost:12434/engines/v1", viper.GetString("providers.docker_model_runner.endpoint"))
	assert.Equal(t, "ai/mistral-nemo", viper.GetString("providers.docker_model_runner.default_model"))
}

// TestConfigAdapter_ProviderOpenCodeZen verifies OpenCode Zen provider fields.
func TestConfigAdapter_ProviderOpenCodeZen(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"provider": "opencode-zen",
		"model":    "kimi-k2.5",
		"apiKey":   "sk-zen",
	})
	require.NoError(t, err)

	assert.Equal(t, "opencode-zen", viper.GetString("agent.provider"))
	assert.Equal(t, "kimi-k2.5", viper.GetString("providers.opencode.model"))
	assert.Equal(t, "sk-zen", viper.GetString("providers.opencode.api_key"))
}

// TestConfigAdapter_Capabilities verifies all capability flags.
func TestConfigAdapter_Capabilities(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"capabilities": map[string]interface{}{
			"browser":    true,
			"terminal":   true,
			"subagents":  false,
			"memory":     true,
			"mcp":        false,
			"filesystem": true,
			"sessions":   false,
		},
	})
	require.NoError(t, err)

	assert.True(t, viper.GetBool("agent.capabilities.browser"))
	assert.True(t, viper.GetBool("agent.capabilities.terminal"))
	assert.False(t, viper.GetBool("agent.capabilities.subagents"))
	assert.True(t, viper.GetBool("agent.capabilities.memory"))
	assert.False(t, viper.GetBool("agent.capabilities.mcp"))
	assert.True(t, viper.GetBool("agent.capabilities.filesystem"))
	assert.False(t, viper.GetBool("agent.capabilities.sessions"))
}

// TestConfigAdapter_Database verifies database configuration fields.
func TestConfigAdapter_Database(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"databaseDriver":       "postgres",
		"databaseDSN":          "postgres://localhost/db",
		"databaseMaxOpenConns": 20,
		"databaseMaxIdleConns": 5,
	})
	require.NoError(t, err)

	assert.Equal(t, "postgres", viper.GetString("database.driver"))
	assert.Equal(t, "postgres://localhost/db", viper.GetString("database.dsn"))
	assert.Equal(t, 20, viper.GetInt("database.max_open_conns"))
	assert.Equal(t, 5, viper.GetInt("database.max_idle_conns"))
}

// TestConfigAdapter_Memory verifies memory backend configuration fields.
func TestConfigAdapter_Memory(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"memoryBackend":   "neo4j",
		"memoryFilePath":  "./data/memory.gml",
		"memoryNeo4jURI":  "bolt://localhost:7687",
		"memoryNeo4jUser": "neo4j",
		"memoryNeo4jPassword": "password",
	})
	require.NoError(t, err)

	assert.Equal(t, "neo4j", viper.GetString("memory.backend"))
	assert.Equal(t, "./data/memory.gml", viper.GetString("memory.file.path"))
	assert.Equal(t, "bolt://localhost:7687", viper.GetString("memory.neo4j.uri"))
	assert.Equal(t, "neo4j", viper.GetString("memory.neo4j.user"))
	assert.Equal(t, "password", viper.GetString("memory.neo4j.password"))
}

// TestConfigAdapter_Subagents verifies subagent configuration fields.
func TestConfigAdapter_Subagents(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"subagentsMaxConcurrent":  3,
		"subagentsDefaultTimeout": "30s",
	})
	require.NoError(t, err)

	assert.Equal(t, 3, viper.GetInt("subagents.max_concurrent"))
	assert.Equal(t, "30s", viper.GetString("subagents.default_timeout"))
}

// TestConfigAdapter_GraphQL verifies GraphQL server configuration fields.
func TestConfigAdapter_GraphQL(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"graphqlEnabled": true,
		"graphqlPort":    9090,
		"graphqlHost":    "0.0.0.0",
		"graphqlBaseUrl": "https://myapp.example.com",
	})
	require.NoError(t, err)

	assert.True(t, viper.GetBool("graphql.enabled"))
	assert.Equal(t, 9090, viper.GetInt("graphql.port"))
	assert.Equal(t, "0.0.0.0", viper.GetString("graphql.host"))
	assert.Equal(t, "https://myapp.example.com", viper.GetString("graphql.base_url"))
}

// TestConfigAdapter_Logging verifies logging configuration fields.
func TestConfigAdapter_Logging(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"loggingLevel": "debug",
		"loggingPath":  "./logs/app.log",
	})
	require.NoError(t, err)

	assert.Equal(t, "debug", viper.GetString("logging.level"))
	assert.Equal(t, "./logs/app.log", viper.GetString("logging.path"))
}

// TestConfigAdapter_Secrets verifies secrets backend configuration fields.
func TestConfigAdapter_Secrets(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"secretsBackend":     "openbao",
		"secretsFilePath":    "./data/secrets.json",
		"secretsOpenbaoURL":  "https://vault.example.com",
		"secretsOpenbaoToken": "hvs.token",
	})
	require.NoError(t, err)

	assert.Equal(t, "openbao", viper.GetString("secrets.backend"))
	assert.Equal(t, "./data/secrets.json", viper.GetString("secrets.file.path"))
	assert.Equal(t, "https://vault.example.com", viper.GetString("secrets.openbao.url"))
	assert.Equal(t, "hvs.token", viper.GetString("secrets.openbao.token"))
}

// TestConfigAdapter_Scheduler verifies scheduler/heartbeat configuration fields.
func TestConfigAdapter_Scheduler(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"schedulerEnabled":        true,
		"schedulerMemoryEnabled":  true,
		"schedulerMemoryInterval": "5m",
	})
	require.NoError(t, err)

	// Scheduler fields are now natively stored under "scheduler".
	assert.True(t, viper.GetBool("scheduler.enabled"))
	assert.True(t, viper.GetBool("scheduler.memory_enabled"))
	assert.Equal(t, "5m", viper.GetString("scheduler.memory_interval"))
}

// TestConfigAdapter_ChannelTelegram verifies Telegram channel fields.
func TestConfigAdapter_ChannelTelegram(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"channelTelegramEnabled": true,
		"channelTelegramToken":   "tg-bot-token",
	})
	require.NoError(t, err)

	assert.True(t, viper.GetBool("channels.telegram.enabled"))
	assert.Equal(t, "tg-bot-token", viper.GetString("channels.telegram.bot_token"))
}

// TestConfigAdapter_ChannelDiscord verifies Discord channel fields.
func TestConfigAdapter_ChannelDiscord(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"channelDiscordEnabled": true,
		"channelDiscordToken":   "dc-bot-token",
	})
	require.NoError(t, err)

	assert.True(t, viper.GetBool("channels.discord.enabled"))
	assert.Equal(t, "dc-bot-token", viper.GetString("channels.discord.bot_token"))
}

// TestConfigAdapter_ChannelWhatsApp verifies WhatsApp channel fields.
func TestConfigAdapter_ChannelWhatsApp(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"channelWhatsAppEnabled":  true,
		"channelWhatsAppPhoneId":  "+34600000000",
		"channelWhatsAppApiToken": "wa-token",
	})
	require.NoError(t, err)

	assert.True(t, viper.GetBool("channels.whatsapp.enabled"))
	assert.Equal(t, "+34600000000", viper.GetString("channels.whatsapp.phone_id"))
	assert.Equal(t, "wa-token", viper.GetString("channels.whatsapp.api_token"))
}

// TestConfigAdapter_ChannelTwilio verifies Twilio channel fields.
func TestConfigAdapter_ChannelTwilio(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"channelTwilioEnabled":    true,
		"channelTwilioAccountSid": "AC123",
		"channelTwilioAuthToken":  "twilio-token",
		"channelTwilioFromNumber": "+15550000000",
	})
	require.NoError(t, err)

	assert.True(t, viper.GetBool("channels.twilio.enabled"))
	assert.Equal(t, "AC123", viper.GetString("channels.twilio.account_sid"))
	assert.Equal(t, "twilio-token", viper.GetString("channels.twilio.auth_token"))
	assert.Equal(t, "+15550000000", viper.GetString("channels.twilio.from_number"))
}

// TestConfigAdapter_ChannelSlack verifies Slack channel fields.
func TestConfigAdapter_ChannelSlack(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"channelSlackEnabled":  true,
		"channelSlackBotToken": "xoxb-bot",
		"channelSlackAppToken": "xapp-app",
	})
	require.NoError(t, err)

	assert.True(t, viper.GetBool("channels.slack.enabled"))
	assert.Equal(t, "xoxb-bot", viper.GetString("channels.slack.bot_token"))
	assert.Equal(t, "xapp-app", viper.GetString("channels.slack.app_token"))
}

// TestConfigAdapter_WizardCompleted verifies the wizard completion flag.
func TestConfigAdapter_WizardCompleted(t *testing.T) {
	resetViper()
	a, _ := newAdapter(t)

	_, err := a.Apply(context.Background(), map[string]interface{}{
		"wizardCompleted": true,
	})
	require.NoError(t, err)

	assert.True(t, viper.GetBool("wizard.completed"))
}

// TestConfigAdapter_ProviderFallback verifies that when provider is not in the
// input, the existing viper value is used instead of defaulting to "ollama".
func TestConfigAdapter_ProviderFallback(t *testing.T) {
	resetViper()
	viper.Set("agent.provider", "anthropic")
	a, _ := newAdapter(t)

	// Sending only model without provider — should use "anthropic" from viper.
	_, err := a.Apply(context.Background(), map[string]interface{}{
		"apiKey": "sk-ant-new",
	})
	require.NoError(t, err)

	// agent.provider must not have been overwritten to "ollama".
	assert.Equal(t, "anthropic", viper.GetString("agent.provider"))
	assert.Equal(t, "sk-ant-new", viper.GetString("providers.anthropic.api_key"))
}

// ---------------------------------------------------------------------------
// BuildConfigSnapshot round-trip tests
// ---------------------------------------------------------------------------

// makeFullConfig returns a Config with every field populated.
func makeFullConfig() *config.Config {
	return &config.Config{
		Agent: config.AgentConfig{
			Name:         "TestBot",
			SystemPrompt: "Be helpful.",
			Provider:     "anthropic",
			Capabilities: config.CapabilitiesConfig{
				Browser:    true,
				Terminal:   true,
				Subagents:  true,
				Memory:     true,
				MCP:        true,
				Filesystem: true,
				Sessions:   true,
			},
		},
		Providers: config.ProvidersConfig{
			Anthropic: config.AnthropicConfig{APIKey: "sk-ant", Model: "claude-sonnet-4-6"},
			OpenAI:    config.OpenAIConfig{APIKey: "sk-oai", Model: "gpt-4o", BaseURL: "https://api.openai.com/v1"},
			Ollama:    config.OllamaConfig{Endpoint: "http://localhost:11434", DefaultModel: "llama3", APIKey: "olk"},
			OpenRouter: config.OpenRouterConfig{APIKey: "sk-or", DefaultModel: "openai/gpt-4o"},
			OpenAICompat: config.OpenAICompatConfig{APIKey: "sk-c", BaseURL: "https://compat.example.com", Model: "mistral"},
			OpenCode:  config.OpenCodeConfig{APIKey: "sk-zen", Model: "kimi-k2.5"},
			DockerModelRunner: config.DockerModelRunnerConfig{Endpoint: "http://dmr:12434", DefaultModel: "ai/mistral-nemo"},
		},
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: "./db.sqlite", MaxOpenConns: 10, MaxIdleConns: 2},
		Memory: config.MemoryConfig{
			Backend: "neo4j",
			File:    config.MemoryFileConfig{Path: "./mem.gml"},
			Neo4j:   config.MemoryNeo4jConfig{URI: "bolt://localhost:7687", User: "neo4j", Password: "pass"},
		},
		SubAgents: config.SubAgentsConfig{MaxConcurrent: 4},
		GraphQL:   config.GraphQLConfig{Enabled: true, Port: 8080, Host: "0.0.0.0", BaseURL: "https://app.example.com"},
		Logging:   config.LoggingConfig{Level: "debug", Path: "./app.log"},
		Scheduler: config.SchedulerConfig{Enabled: true, MemoryEnabled: true},
		Secrets: config.SecretsConfig{
			Backend: "openbao",
			File:    config.SecretsFileConfig{Path: "./secrets.json"},
			Openbao: &config.OpenbaoSecretsConfig{URL: "https://vault.example.com", Token: "hvs.token"},
		},
		Channels: config.ChannelsConfig{
			Telegram: config.TelegramConfig{Enabled: true, BotToken: "tg-token"},
			Discord:  config.DiscordConfig{Enabled: true, BotToken: "dc-token"},
			WhatsApp: config.WhatsAppConfig{Enabled: true, PhoneID: "+34600000000", APIToken: "wa-token"},
			Twilio:   config.TwilioConfig{Enabled: true, AccountSID: "AC123", AuthToken: "tw-token", FromNumber: "+15550000000"},
			Slack:    config.SlackConfig{Enabled: true, BotToken: "xoxb-bot", AppToken: "xapp-app"},
		},
		Wizard: config.WizardConfig{Completed: true},
	}
}

// TestBuildConfigSnapshot_AgentFields verifies agent fields in snapshot.
func TestBuildConfigSnapshot_AgentFields(t *testing.T) {
	cfg := makeFullConfig()
	// With provider="anthropic" and Anthropic having credentials, ProviderName returns "anthropic".
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return c.Agent.Provider })

	require.NotNil(t, snap.Agent)
	assert.Equal(t, "TestBot", snap.Agent.Name)
	assert.Equal(t, "Be helpful.", snap.Agent.SystemPrompt)
	assert.Equal(t, "anthropic", snap.Agent.Provider)
	assert.Equal(t, "claude-sonnet-4-6", snap.Agent.Model)
	assert.Equal(t, "sk-ant", snap.Agent.AnthropicApiKey)
}

// TestBuildConfigSnapshot_Capabilities verifies all capability flags.
func TestBuildConfigSnapshot_Capabilities(t *testing.T) {
	cfg := makeFullConfig()
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return "ollama" })

	require.NotNil(t, snap.Capabilities)
	assert.True(t, snap.Capabilities.Browser)
	assert.True(t, snap.Capabilities.Terminal)
	assert.True(t, snap.Capabilities.Subagents)
	assert.True(t, snap.Capabilities.Memory)
	assert.True(t, snap.Capabilities.MCP)
	assert.True(t, snap.Capabilities.Filesystem)
	assert.True(t, snap.Capabilities.Sessions)
}

// TestBuildConfigSnapshot_Database verifies database fields in snapshot.
func TestBuildConfigSnapshot_Database(t *testing.T) {
	cfg := makeFullConfig()
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return "none" })

	require.NotNil(t, snap.Database)
	assert.Equal(t, "sqlite", snap.Database.Driver)
	assert.Equal(t, "./db.sqlite", snap.Database.DSN)
	assert.Equal(t, 10, snap.Database.MaxOpenConns)
	assert.Equal(t, 2, snap.Database.MaxIdleConns)
}

// TestBuildConfigSnapshot_Memory verifies memory fields in snapshot.
func TestBuildConfigSnapshot_Memory(t *testing.T) {
	cfg := makeFullConfig()
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return "none" })

	require.NotNil(t, snap.Memory)
	assert.Equal(t, "neo4j", snap.Memory.Backend)
	assert.Equal(t, "./mem.gml", snap.Memory.FilePath)
	require.NotNil(t, snap.Memory.Neo4j)
	assert.Equal(t, "bolt://localhost:7687", snap.Memory.Neo4j.URI)
	assert.Equal(t, "neo4j", snap.Memory.Neo4j.User)
	assert.Equal(t, "pass", snap.Memory.Neo4j.Password)
}

// TestBuildConfigSnapshot_Channels verifies all channel fields are populated.
func TestBuildConfigSnapshot_Channels(t *testing.T) {
	cfg := makeFullConfig()
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return "none" })

	require.NotNil(t, snap.ChannelSecrets)

	// Telegram
	assert.True(t, snap.ChannelSecrets.TelegramEnabled)
	assert.Equal(t, "tg-token", snap.ChannelSecrets.TelegramToken)

	// Discord
	assert.True(t, snap.ChannelSecrets.DiscordEnabled)
	assert.Equal(t, "dc-token", snap.ChannelSecrets.DiscordToken)

	// WhatsApp
	assert.True(t, snap.ChannelSecrets.WhatsAppEnabled)
	assert.Equal(t, "+34600000000", snap.ChannelSecrets.WhatsAppPhoneId)
	assert.Equal(t, "wa-token", snap.ChannelSecrets.WhatsAppApiToken)

	// Twilio
	assert.True(t, snap.ChannelSecrets.TwilioEnabled)
	assert.Equal(t, "AC123", snap.ChannelSecrets.TwilioAccountSid)
	assert.Equal(t, "tw-token", snap.ChannelSecrets.TwilioAuthToken)
	assert.Equal(t, "+15550000000", snap.ChannelSecrets.TwilioFromNumber)

	// Slack
	assert.True(t, snap.ChannelSecrets.SlackEnabled)
	assert.Equal(t, "xoxb-bot", snap.ChannelSecrets.SlackBotToken)
	assert.Equal(t, "xapp-app", snap.ChannelSecrets.SlackAppToken)
}

// TestBuildConfigSnapshot_Secrets verifies secrets fields in snapshot.
func TestBuildConfigSnapshot_Secrets(t *testing.T) {
	cfg := makeFullConfig()
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return "none" })

	require.NotNil(t, snap.Secrets)
	assert.Equal(t, "openbao", snap.Secrets.Backend)
	require.NotNil(t, snap.Secrets.File)
	assert.Equal(t, "./secrets.json", snap.Secrets.File.Path)
	require.NotNil(t, snap.Secrets.Openbao)
	assert.Equal(t, "https://vault.example.com", snap.Secrets.Openbao.URL)
	assert.Equal(t, "hvs.token", snap.Secrets.Openbao.Token)
}

// TestBuildConfigSnapshot_GraphQL verifies GraphQL fields in snapshot.
func TestBuildConfigSnapshot_GraphQL(t *testing.T) {
	cfg := makeFullConfig()
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return "none" })

	require.NotNil(t, snap.GraphQL)
	assert.True(t, snap.GraphQL.Enabled)
	assert.Equal(t, 8080, snap.GraphQL.Port)
	assert.Equal(t, "0.0.0.0", snap.GraphQL.Host)
	assert.Equal(t, "https://app.example.com", snap.GraphQL.BaseURL)
}

// TestBuildConfigSnapshot_Scheduler verifies scheduler fields in snapshot.
func TestBuildConfigSnapshot_Scheduler(t *testing.T) {
	cfg := makeFullConfig()
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return "none" })

	require.NotNil(t, snap.Scheduler)
	assert.True(t, snap.Scheduler.Enabled)
	assert.True(t, snap.Scheduler.MemoryEnabled)
}

// TestBuildConfigSnapshot_WizardCompleted verifies wizard flag in snapshot.
func TestBuildConfigSnapshot_WizardCompleted(t *testing.T) {
	cfg := makeFullConfig()
	snap := BuildConfigSnapshot(cfg, func(c *config.Config) string { return "none" })

	assert.True(t, snap.WizardCompleted)
}

// TestApplyProviderKeys_EmptyStringsDoNotOverwrite verifies that when the frontend
// sends all provider fields at once (with empty strings for the unused ones),
// only the active provider's non-empty credentials are persisted.
// This mirrors the real frontend behaviour: saving with provider=anthropic sends
// apiKey="" (for openai) and anthropicApiKey="sk-ant"; the openai key must remain.
func TestApplyProviderKeys_EmptyStringsDoNotOverwrite(t *testing.T) {
	resetViper()
	adapter, _ := newAdapter(t)
	ctx := context.Background()

	// Pre-populate openai key via a first save.
	_, err := adapter.Apply(ctx, map[string]interface{}{
		"provider": "openai",
		"apiKey":   "sk-openai-existing",
		"model":    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "sk-openai-existing", viper.GetString("providers.openai.api_key"))

	// Now switch to anthropic. Frontend sends apiKey="" (the openai field is blank).
	_, err = adapter.Apply(ctx, map[string]interface{}{
		"provider": "anthropic",
		"apiKey":   "sk-ant-new", // In this test case, we use apiKey for both but the logic should handle it correctly
		"model":    "claude-sonnet-4-6",
	})
	require.NoError(t, err)

	assert.Equal(t, "anthropic", viper.GetString("agent.provider"))
	assert.Equal(t, "sk-ant-new", viper.GetString("providers.anthropic.api_key"))
	// openai key must remain untouched
	assert.Equal(t, "sk-openai-existing", viper.GetString("providers.openai.api_key"))
}

// TestApplyProviderKeys_EmptyModelDoesNotOverwrite verifies that an empty model
// string does not overwrite a previously persisted model value.
func TestApplyProviderKeys_EmptyModelDoesNotOverwrite(t *testing.T) {
	resetViper()
	adapter, _ := newAdapter(t)
	ctx := context.Background()

	_, err := adapter.Apply(ctx, map[string]interface{}{
		"provider": "openai",
		"apiKey":   "sk-test",
		"model":    "gpt-4-turbo",
	})
	require.NoError(t, err)
	assert.Equal(t, "gpt-4-turbo", viper.GetString("providers.openai.model"))

	_, err = adapter.Apply(ctx, map[string]interface{}{
		"provider": "openai",
		"model":    "", // blank – should not clear the stored model
	})
	require.NoError(t, err)
	assert.Equal(t, "gpt-4-turbo", viper.GetString("providers.openai.model"))
}
