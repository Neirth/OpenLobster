// Package openrouter provides an OpenRouter AI provider adapter.
//
// OpenRouter exposes an OpenAI-compatible Chat Completions endpoint.
// This package is a thin wrapper around the unified openai adapter,
// pre-configured with the OpenRouter base URL.
//
// # License
// See LICENSE in the root of the repository.
package openrouter

import (
	"github.com/neirth/openlobster/internal/domain/ports"
	aiopenai "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openai"
)

const baseURL = "https://openrouter.ai/api/v1"

// Adapter is an alias for the unified OpenAI-compatible adapter with the
// OpenRouter endpoint pre-configured.
//
// All method implementations are inherited from aiopenai.Adapter.
type Adapter = aiopenai.Adapter

// NewAdapter creates an Adapter targeting the OpenRouter API.
func NewAdapter(apiKey, model string, maxTokens int) *Adapter {
	return aiopenai.NewAdapterWithEndpoint(baseURL, apiKey, model, maxTokens)
}

var _ ports.AIProviderPort = (*Adapter)(nil)
