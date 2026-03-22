// Package factory creates AI provider adapters from application configuration.
package factory

import (
	aianthropicadapter "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/anthropic"
	aidockermodelrunner "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/docker"
	aiollama "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/ollama"
	aiopenai "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openai"
	aiopenaicompat "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openaicompat"
	aiopenrouter "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openrouter"
	aizenadapter "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/zen"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/infrastructure/config"
)

// MaxOutputTokens is the fixed output-token limit for AI completions (~2000 chars, fits Discord).
const MaxOutputTokens = 500

// BuildFromConfig creates the AIProviderPort that matches cfg.Agent.Provider
// when explicitly set, falling back to the first provider with valid credentials.
// Returns nil if no provider is configured.
func BuildFromConfig(cfg *config.Config) ports.AIProviderPort {
	switch ProviderName(cfg) {
	case "openai":
		model := cfg.Providers.OpenAI.Model
		if model == "" {
			model = "gpt-4o"
		}
		var a *aiopenai.Adapter
		if baseURL := cfg.Providers.OpenAI.BaseURL; baseURL != "" {
			a = aiopenai.NewAdapterWithEndpoint(baseURL, cfg.Providers.OpenAI.APIKey, model, MaxOutputTokens, cfg.Agent.ReasoningLevel)
		} else {
			a = aiopenai.NewAdapter(cfg.Providers.OpenAI.APIKey, model, MaxOutputTokens, cfg.Agent.ReasoningLevel)
		}
		if cfg.Providers.OpenAI.ContextWindow > 0 {
			a.OverrideContextWindow(cfg.Providers.OpenAI.ContextWindow)
		}
		return a
	case "openrouter":
		model := cfg.Providers.OpenRouter.DefaultModel
		if model == "" {
			model = "openai/gpt-4o"
		}
		a := aiopenrouter.NewAdapter(cfg.Providers.OpenRouter.APIKey, model, MaxOutputTokens)
		if cfg.Providers.OpenRouter.ContextWindow > 0 {
			a.OverrideContextWindow(cfg.Providers.OpenRouter.ContextWindow)
		}
		return a
	case "openai-compatible":
		model := cfg.Providers.OpenAICompat.Model
		if model == "" {
			model = "default"
		}
		a := aiopenaicompat.NewAdapter(
			cfg.Providers.OpenAICompat.BaseURL,
			cfg.Providers.OpenAICompat.APIKey,
			model,
			MaxOutputTokens,
		)
		if cfg.Providers.OpenAICompat.ContextWindow > 0 {
			a.OverrideContextWindow(cfg.Providers.OpenAICompat.ContextWindow)
		}
		return a
	case "ollama":
		model := cfg.Providers.Ollama.DefaultModel
		if model == "" {
			model = "llama3"
		}
		a := aiollama.NewAdapterWithOptions(cfg.Providers.Ollama.Endpoint, cfg.Providers.Ollama.APIKey, model, MaxOutputTokens, cfg.Logging.Level)
		if cfg.Providers.Ollama.ContextWindow > 0 {
			a.OverrideContextWindow(cfg.Providers.Ollama.ContextWindow)
		}
		return a
	case "anthropic":
		model := cfg.Providers.Anthropic.Model
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		a := aianthropicadapter.NewAdapter(cfg.Providers.Anthropic.APIKey, model, MaxOutputTokens, cfg.Agent.ReasoningLevel)
		if cfg.Providers.Anthropic.ContextWindow > 0 {
			a.OverrideContextWindow(cfg.Providers.Anthropic.ContextWindow)
		}
		return a
	case "opencode-zen":
		model := cfg.Providers.OpenCode.Model
		if model == "" {
			model = "kimi-k2.5"
		}
		return aizenadapter.NewAdapter(cfg.Providers.OpenCode.APIKey, model, MaxOutputTokens, cfg.Agent.ReasoningLevel)
	case "docker-model-runner":
		model := cfg.Providers.DockerModelRunner.DefaultModel
		if model == "" {
			model = "ai/mistral-nemo"
		}
		a := aidockermodelrunner.NewAdapter(cfg.Providers.DockerModelRunner.Endpoint, model, MaxOutputTokens)
		if cfg.Providers.DockerModelRunner.ContextWindow > 0 {
			a.OverrideContextWindow(cfg.Providers.DockerModelRunner.ContextWindow)
		}
		return a
	}
	return nil
}

// ProviderName returns the active AI provider name. When cfg.Agent.Provider is
// explicitly set it is honoured; otherwise the first provider with valid
// credentials is returned.
func ProviderName(cfg *config.Config) string {
	explicit := cfg.Agent.Provider
	switch explicit {
	case "openai":
		if cfg.Providers.OpenAI.APIKey != "" && cfg.Providers.OpenAI.APIKey != "YOUR_API_KEY_HERE" {
			return "openai"
		}
	case "openrouter":
		if cfg.Providers.OpenRouter.APIKey != "" && cfg.Providers.OpenRouter.APIKey != "YOUR_API_KEY_HERE" {
			return "openrouter"
		}
	case "openai-compatible":
		if cfg.Providers.OpenAICompat.BaseURL != "" {
			return "openai-compatible"
		}
	case "ollama":
		if cfg.Providers.Ollama.Endpoint != "" {
			return "ollama"
		}
	case "anthropic":
		if cfg.Providers.Anthropic.APIKey != "" && cfg.Providers.Anthropic.APIKey != "YOUR_API_KEY_HERE" {
			return "anthropic"
		}
	case "opencode-zen":
		if cfg.Providers.OpenCode.APIKey != "" && cfg.Providers.OpenCode.APIKey != "YOUR_API_KEY_HERE" {
			return "opencode-zen"
		}
	case "docker-model-runner":
		if cfg.Providers.DockerModelRunner.Endpoint != "" {
			return "docker-model-runner"
		}
	}
	// Fallback: first provider with valid credentials.
	switch {
	case cfg.Providers.OpenAI.APIKey != "" && cfg.Providers.OpenAI.APIKey != "YOUR_API_KEY_HERE":
		return "openai"
	case cfg.Providers.OpenRouter.APIKey != "" && cfg.Providers.OpenRouter.APIKey != "YOUR_API_KEY_HERE":
		return "openrouter"
	case cfg.Providers.OpenAICompat.APIKey != "" &&
		cfg.Providers.OpenAICompat.APIKey != "YOUR_API_KEY_HERE" &&
		cfg.Providers.OpenAICompat.BaseURL != "":
		return "openai-compatible"
	case cfg.Providers.Ollama.Endpoint != "":
		return "ollama"
	case cfg.Providers.Anthropic.APIKey != "" && cfg.Providers.Anthropic.APIKey != "YOUR_API_KEY_HERE":
		return "anthropic"
	case cfg.Providers.OpenCode.APIKey != "" && cfg.Providers.OpenCode.APIKey != "YOUR_API_KEY_HERE":
		return "opencode-zen"
	case cfg.Providers.DockerModelRunner.Endpoint != "":
		return "docker-model-runner"
	default:
		return "none"
	}
}
