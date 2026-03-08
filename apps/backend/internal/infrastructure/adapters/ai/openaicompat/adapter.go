// Package openaicompat provides an adapter for any OpenAI-compatible AI provider.
//
// Use this adapter when the target service exposes an OpenAI Chat Completions
// endpoint at a custom URL (e.g. a self-hosted vLLM instance, LM Studio,
// Groq, Together AI, Mistral, etc.).
//
// The implementation delegates entirely to the unified aiopenai adapter,
// supplying the user-configured base URL.
//
// # License
// See LICENSE in the root of the repository.
package openaicompat

import (
	"github.com/neirth/openlobster/internal/domain/ports"
	aiopenai "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openai"
)

// Adapter is an alias for the unified OpenAI-compatible adapter.
// All method implementations are inherited from aiopenai.Adapter.
type Adapter = aiopenai.Adapter

// NewAdapter creates an Adapter that sends requests to the provided baseURL.
// The apiKey and model fields map directly to the Authorization header and
// the Model field of each ChatCompletion request.
func NewAdapter(baseURL, apiKey, model string, maxTokens int) *Adapter {
	return aiopenai.NewAdapterWithEndpoint(baseURL, apiKey, model, maxTokens)
}

var _ ports.AIProviderPort = (*Adapter)(nil)
