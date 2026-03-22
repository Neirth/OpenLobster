// Package zen provides an AI adapter for the OpenCode Zen gateway.
//
// OpenCode Zen (https://opencode.ai/docs/zen/) is a curated AI gateway that
// exposes multiple model families through different endpoint formats:
//
//   - claude-*  models → Anthropic Messages API at https://opencode.ai/zen/v1/messages
//   - gpt-*     models → OpenAI chat/completions at https://opencode.ai/zen/v1/chat/completions
//   - all other models → OpenAI-compatible at https://opencode.ai/zen/v1/chat/completions
//     (covers kimi-*, minimax-*, glm-*, qwen3-*, big-pickle, gemini-*, etc.)
//
// The adapter selects the underlying implementation automatically based on
// the model name prefix, so callers only need to supply the API key and model.
//
// # License
// See LICENSE in the root of the repository.
package zen

import (
	"context"
	"strings"

	"github.com/neirth/openlobster/internal/domain/ports"
	aianthropicadapter "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/anthropic"
	aiopenai "github.com/neirth/openlobster/internal/infrastructure/adapters/ai/openai"
)

// Adapter routes requests to the correct Zen endpoint based on the model name.
type Adapter struct {
	underlying ports.AIProviderPort
	maxTokens  int
}

// NewAdapter returns an Adapter configured for the given model.
// Routing logic:
//   - "claude-*"  → Anthropic Messages API (https://opencode.ai/zen/v1/messages)
//   - everything else → OpenAI-compatible chat/completions
func NewAdapter(apiKey, model string, maxTokens int, reasoningLevel string) *Adapter {
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	var underlying ports.AIProviderPort

	switch {
	case strings.HasPrefix(model, "claude-"):
		underlying = aianthropicadapter.NewAdapterWithBaseURL("https://opencode.ai/zen/v1", apiKey, model, maxTokens, reasoningLevel)
	default:
		// GPT-5 Responses API, Gemini, Kimi, MiniMax, GLM, Qwen3, Big Pickle …
		// all exposed via the OpenAI-compatible chat/completions shim.
		underlying = aiopenai.NewAdapterWithEndpoint("https://opencode.ai/zen/v1", apiKey, model, maxTokens, reasoningLevel)
	}

	return &Adapter{underlying: underlying, maxTokens: maxTokens}
}

// Chat delegates the request to the selected underlying adapter.
func (a *Adapter) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	return a.underlying.Chat(ctx, req)
}

// ChatWithAudio delegates to the underlying adapter.
func (a *Adapter) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return a.underlying.ChatWithAudio(ctx, req)
}

// ChatToAudio delegates to the underlying adapter (audio output not supported
// by Zen — falls back to text response).
func (a *Adapter) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return a.underlying.ChatToAudio(ctx, req)
}

// SupportsAudioInput reports whether the underlying provider supports audio input.
func (a *Adapter) SupportsAudioInput() bool {
	return a.underlying.SupportsAudioInput()
}

// SupportsAudioOutput reports whether the underlying provider supports audio output.
func (a *Adapter) SupportsAudioOutput() bool {
	return a.underlying.SupportsAudioOutput()
}

// GetMaxTokens returns the configured maximum token limit.
func (a *Adapter) GetMaxTokens() int {
	return a.maxTokens
}

// GetContextWindow delegates to the underlying adapter which already has the
// correct model-specific context window for OpenAI or Anthropic models.
func (a *Adapter) GetContextWindow() int {
	return a.underlying.GetContextWindow()
}

var _ ports.AIProviderPort = (*Adapter)(nil)
