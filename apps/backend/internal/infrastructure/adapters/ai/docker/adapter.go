// Package dockermodelrunner provides an adapter for Docker Desktop's Model Runner.
//
// Docker Desktop ships a built-in GPU inference engine that exposes an
// OpenAI-compatible Chat Completions API at:
//
//	http://localhost:12434/engines/v1
//
// Models are referenced by their Docker Hub path, e.g. "ai/mistral-nemo".
// The implementation delegates entirely to the unified aiopenai adapter.
//
// # License
// See LICENSE in the root of the repository.
package dockermodelrunner

import (
	"github.com/neirth/openlobster/internal/domain/ports"
	aiopenai "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openai"
)

// DefaultEndpoint is the Docker Desktop Model Runner inference endpoint.
const DefaultEndpoint = "http://localhost:12434/engines/v1"

// Adapter is an alias for the unified OpenAI-compatible adapter.
type Adapter = aiopenai.Adapter

// NewAdapter creates an Adapter targeting the Docker Model Runner endpoint.
// If endpoint is empty, DefaultEndpoint is used. The model field should be a
// Docker Hub model path such as "ai/mistral-nemo".
func NewAdapter(endpoint, model string, maxTokens int) *Adapter {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	// Docker Model Runner does not require an API key.
	return aiopenai.NewAdapterWithEndpoint(endpoint, "ignored", model, maxTokens)
}

var _ ports.AIProviderPort = (*Adapter)(nil)
