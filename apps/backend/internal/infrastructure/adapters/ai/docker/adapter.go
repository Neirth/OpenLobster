// Package dockermodelrunner provides an adapter for Docker Desktop's Model Runner.
//
// Docker Desktop ships a built-in GPU inference engine that exposes an
// OpenAI-compatible Chat Completions API at:
//
//	http://host.docker.internal:12434/engines/v1
//
// Models are referenced by their Docker Hub path, e.g. "ai/mistral-nemo".
// The implementation delegates entirely to the unified aiopenai adapter.
//
// NOTE: The model configured for Docker Model Runner MUST support tool/function
// calling. Models without tool support will return errors when the agent tries
// to use tools. See the Docker Hub model page for capability information.
//
// # License
// See LICENSE in the root of the repository.
package dockermodelrunner

import (
	"log/slog"

	"github.com/neirth/openlobster/internal/domain/ports"
	aiopenai "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openai"
)

// DefaultEndpoint is the Docker Desktop Model Runner inference endpoint.
// Uses host.docker.internal so the backend container can reach the host's Docker Desktop.
const DefaultEndpoint = "http://host.docker.internal:12434/engines/v1"

// Adapter is an alias for the unified OpenAI-compatible adapter.
type Adapter = aiopenai.Adapter

// NewAdapter creates an Adapter targeting the Docker Model Runner endpoint.
// If endpoint is empty, DefaultEndpoint is used. The model field should be a
// Docker Hub model path such as "ai/mistral-nemo".
//
// A warning is logged at startup because Docker Model Runner models must
// explicitly support tool/function calling. Using a model without tool support
// will cause runtime errors when the agent attempts to call tools.
func NewAdapter(endpoint, model string, maxTokens int, reasoningLevel string) *Adapter {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	slog.Warn("docker model runner: the configured model must support tool/function calling — "+
		"models without tool support will fail at runtime when the agent tries to use tools",
		"endpoint", endpoint,
		"model", model,
	)
	// Docker Model Runner does not require an API key.
	return aiopenai.NewAdapterWithEndpoint(endpoint, "not-needed", model, maxTokens, reasoningLevel)
}

var _ ports.AIProviderPort = (*Adapter)(nil)
