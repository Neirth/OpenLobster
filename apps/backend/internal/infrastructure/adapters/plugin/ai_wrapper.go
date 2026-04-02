package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/neirth/openlobster/internal/domain/ports"
)

// AIWrapper wraps a "ai"-type PluginPort and implements ports.AIProviderPort.
// The plugin must export openlobster_chat(). All other AIProviderPort methods
// have sensible no-op / default implementations.
type AIWrapper struct {
	plugin ports.PluginPort
	cfg    map[string]interface{} // per-plugin settings from config
}

func (w *AIWrapper) currentConfig() map[string]interface{} {
	live := liveConfigForPlugin(w.plugin.ID(), w.cfg)
	out := make(map[string]interface{}, len(live)+1)
	for k, v := range live {
		out[k] = v
	}
	// Internal hint for WASI plugins that need a writable HOME.
	out["__plugin_home"] = "plugin-home"
	return out
}

// NewAIWrapper returns an AIWrapper backed by p.
func NewAIWrapper(p ports.PluginPort, cfg map[string]interface{}) *AIWrapper {
	return &AIWrapper{plugin: p, cfg: cfg}
}

type chatPluginInput struct {
	Model     string                 `json:"model"`
	Messages  []ports.ChatMessage    `json:"messages"`
	Tools     []ports.Tool           `json:"tools,omitempty"`
	MaxTokens int                    `json:"max_tokens,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty"`
}

type chatPluginOutput struct {
	Content    string           `json:"content"`
	ToolCalls  []ports.ToolCall `json:"tool_calls,omitempty"`
	StopReason string           `json:"stop_reason"`
	Usage      struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error string `json:"error,omitempty"`
}

func (w *AIWrapper) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	input := chatPluginInput{
		Model:     req.Model,
		Messages:  req.Messages,
		Tools:     req.Tools,
		MaxTokens: req.MaxTokens,
		Config:    w.currentConfig(),
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ports.ChatResponse{}, fmt.Errorf("ai plugin %s: marshal input: %w", w.plugin.ID(), err)
	}
	out, err := w.plugin.Call("chat", raw)
	if err != nil {
		return ports.ChatResponse{}, err
	}
	var resp chatPluginOutput
	if err := json.Unmarshal(out, &resp); err != nil {
		return ports.ChatResponse{}, fmt.Errorf("ai plugin %s: unmarshal output: %w", w.plugin.ID(), err)
	}
	if resp.Error != "" {
		return ports.ChatResponse{}, fmt.Errorf("ai plugin %s: %s", w.plugin.ID(), resp.Error)
	}
	return ports.ChatResponse{
		Content:    resp.Content,
		ToolCalls:  resp.ToolCalls,
		StopReason: resp.StopReason,
		Usage: ports.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
		},
	}, nil
}

func (w *AIWrapper) ChatWithAudio(_ context.Context, _ ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, fmt.Errorf("ai plugin %s: audio input not supported", w.plugin.ID())
}

func (w *AIWrapper) ChatToAudio(_ context.Context, _ ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, fmt.Errorf("ai plugin %s: audio output not supported", w.plugin.ID())
}

func (w *AIWrapper) SupportsAudioInput() bool  { return false }
func (w *AIWrapper) SupportsAudioOutput() bool { return false }
func (w *AIWrapper) GetMaxTokens() int         { return ports.DefaultMaxTokens }
func (w *AIWrapper) GetContextWindow() int     { return 128000 }
