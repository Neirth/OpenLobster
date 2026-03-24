// BuildConfigSnapshot converts the infrastructure Config into the AppConfigSnapshot
// DTO consumed by the GraphQL config resolver.
package dto

import (
	"github.com/neirth/openlobster/internal/infrastructure/config"
)

// BuildConfigSnapshot builds an AppConfigSnapshot from a loaded Config.
// It is called at startup and again on every config save so the resolver
// always reflects the current state without a process restart.
func BuildConfigSnapshot(cfg *config.Config, providerNameFn func(*config.Config) string) *AppConfigSnapshot {
	provider := providerNameFn(cfg)
	var apiKey, baseURL, ollamaHost, ollamaApiKey, anthropicApiKey, model string
	switch provider {
	case "openrouter":
		apiKey = cfg.Providers.OpenRouter.APIKey
		model = cfg.Providers.OpenRouter.DefaultModel
	case "ollama":
		ollamaHost = cfg.Providers.Ollama.Endpoint
		ollamaApiKey = cfg.Providers.Ollama.APIKey
		model = cfg.Providers.Ollama.DefaultModel
	case "openai":
		apiKey = cfg.Providers.OpenAI.APIKey
		model = cfg.Providers.OpenAI.Model
	case "opencode-zen":
		apiKey = cfg.Providers.OpenCode.APIKey
		model = cfg.Providers.OpenCode.Model
	case "openai-compatible":
		apiKey = cfg.Providers.OpenAICompat.APIKey
		baseURL = cfg.Providers.OpenAICompat.BaseURL
		model = cfg.Providers.OpenAICompat.Model
	case "anthropic":
		anthropicApiKey = cfg.Providers.Anthropic.APIKey
		model = cfg.Providers.Anthropic.Model
	case "docker-model-runner":
		ollamaHost = cfg.Providers.DockerModelRunner.Endpoint
		model = cfg.Providers.DockerModelRunner.DefaultModel
	}
	return &AppConfigSnapshot{
		Agent: &AgentConfigSnapshot{
			Name:                      cfg.Agent.Name,
			SystemPrompt:              cfg.Agent.SystemPrompt,
			Provider:                  provider,
			Model:                     model,
			APIKey:                    apiKey,
			BaseURL:                   baseURL,
			OllamaHost:                ollamaHost,
			OllamaApiKey:              ollamaApiKey,
			AnthropicApiKey:           anthropicApiKey,
			DockerModelRunnerEndpoint: cfg.Providers.DockerModelRunner.Endpoint,
			DockerModelRunnerModel:    cfg.Providers.DockerModelRunner.DefaultModel,
			ReasoningLevel:            cfg.Agent.ReasoningLevel,
		},
		Capabilities: &CapabilitiesSnapshot{
			Browser: cfg.Agent.Capabilities.Browser, Terminal: cfg.Agent.Capabilities.Terminal,
			Subagents: cfg.Agent.Capabilities.Subagents, Memory: cfg.Agent.Capabilities.Memory,
			MCP:        cfg.Agent.Capabilities.MCP,
			Filesystem: cfg.Agent.Capabilities.Filesystem, Sessions: cfg.Agent.Capabilities.Sessions,
		},
		Database: &DatabaseConfigSnapshot{
			Driver: cfg.Database.Driver, DSN: cfg.Database.DSN,
			MaxOpenConns: cfg.Database.MaxOpenConns, MaxIdleConns: cfg.Database.MaxIdleConns,
		},
		Memory: &MemoryConfigSnapshot{
			Backend: string(cfg.Memory.Backend), FilePath: cfg.Memory.File.Path,
			Neo4j:    &Neo4jConfigSnapshot{URI: cfg.Memory.Neo4j.URI, User: cfg.Memory.Neo4j.User, Password: cfg.Memory.Neo4j.Password},
			Postgres: &PostgresConfigSnapshot{DSN: cfg.Memory.Postgres.DSN},
		},
		Subagents: &SubagentsConfigSnapshot{
			MaxConcurrent: cfg.SubAgents.MaxConcurrent, DefaultTimeout: cfg.SubAgents.DefaultTimeout.String(),
		},
		GraphQL:   &GraphQLConfigSnapshot{Enabled: cfg.GraphQL.Enabled, Port: cfg.GraphQL.Port, Host: cfg.GraphQL.Host, BaseURL: cfg.GraphQL.BaseURL},
		Logging:   &LoggingConfigSnapshot{Level: cfg.Logging.Level, Path: cfg.Logging.Path},
		Scheduler: &SchedulerConfigSnapshot{Enabled: cfg.Scheduler.Enabled, MemoryEnabled: cfg.Scheduler.MemoryEnabled, MemoryInterval: cfg.Scheduler.MemoryInterval.String()},
		Secrets: &SecretsConfigSnapshot{
			Backend: cfg.Secrets.Backend,
			File:    &FileSecretsSnapshot{Path: cfg.Secrets.File.Path},
			Openbao: func() *OpenbaoSecretsSnapshot {
				if cfg.Secrets.Openbao == nil {
					return nil
				}
				return &OpenbaoSecretsSnapshot{URL: cfg.Secrets.Openbao.URL, Token: cfg.Secrets.Openbao.Token}
			}(),
		},
		ChannelSecrets: &ChannelSecretsSnapshot{
			TelegramEnabled:  cfg.Channels.Telegram.Enabled,
			TelegramToken:    cfg.Channels.Telegram.BotToken,
			DiscordEnabled:   cfg.Channels.Discord.Enabled,
			DiscordToken:     cfg.Channels.Discord.BotToken,
			WhatsAppEnabled:  cfg.Channels.WhatsApp.Enabled,
			WhatsAppPhoneId:  cfg.Channels.WhatsApp.PhoneID,
			WhatsAppApiToken: cfg.Channels.WhatsApp.APIToken,
			TwilioEnabled:    cfg.Channels.Twilio.Enabled,
			TwilioAccountSid: cfg.Channels.Twilio.AccountSID,
			TwilioAuthToken:  cfg.Channels.Twilio.AuthToken,
			TwilioFromNumber: cfg.Channels.Twilio.FromNumber,
			SlackEnabled:     cfg.Channels.Slack.Enabled,
			SlackBotToken:    cfg.Channels.Slack.BotToken,
			SlackAppToken:    cfg.Channels.Slack.AppToken,
		},
		WizardCompleted: cfg.Wizard.Completed,
	}
}
