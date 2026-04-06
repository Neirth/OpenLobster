package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/neirth/openlobster/internal/domain/ports"
)

const (
	chatWithAudioFn = "chat_with_audio"
	chatToAudioFn   = "chat_to_audio"
	metadataFn      = "get_metadata"
)

// AIWrapper wraps a "ai"-type PluginPort and implements ports.AIProviderPort.
// The plugin must export openlobster_chat(). All other AIProviderPort methods
// have sensible no-op / default implementations.
type AIWrapper struct {
	plugin ports.PluginPort
	cfg    map[string]interface{} // per-plugin settings from config

	metadataOnce sync.Once
	metadata     aiMetadata
}

type aiMetadata struct {
	Properties map[string]any `json:"properties"`
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

type chatWithAudioPluginInput struct {
	Model     string                 `json:"model"`
	Messages  []ports.ChatMessage    `json:"messages"`
	AudioData []byte                 `json:"audio_data"`
	Tools     []ports.Tool           `json:"tools,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty"`
}

type chatToAudioPluginInput struct {
	Model     string                 `json:"model"`
	Messages  []ports.ChatMessage    `json:"messages"`
	Tools     []ports.Tool           `json:"tools,omitempty"`
	MaxTokens int                    `json:"max_tokens,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty"`
}

type chatToAudioPluginOutput struct {
	Content    string `json:"content"`
	Audio      string `json:"audio,omitempty"`
	AudioData  string `json:"audio_data,omitempty"`
	StopReason string `json:"stop_reason"`
	Error      string `json:"error,omitempty"`
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

func (w *AIWrapper) ChatWithAudio(_ context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	input := chatWithAudioPluginInput{
		Model:     req.Model,
		Messages:  req.Messages,
		AudioData: req.AudioData,
		Tools:     req.Tools,
		Config:    w.currentConfig(),
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ports.ChatResponse{}, fmt.Errorf("ai plugin %s: marshal chat_with_audio input: %w", w.plugin.ID(), err)
	}
	out, err := w.plugin.Call(chatWithAudioFn, raw)
	if err != nil {
		if isMissingPluginFunction(err, chatWithAudioFn) {
			return ports.ChatResponse{}, fmt.Errorf("ai plugin %s: audio input not supported", w.plugin.ID())
		}
		return ports.ChatResponse{}, err
	}
	var resp chatPluginOutput
	if err := json.Unmarshal(out, &resp); err != nil {
		return ports.ChatResponse{}, fmt.Errorf("ai plugin %s: unmarshal chat_with_audio output: %w", w.plugin.ID(), err)
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

func (w *AIWrapper) ChatToAudio(_ context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	input := chatToAudioPluginInput{
		Model:     req.Model,
		Messages:  req.Messages,
		Tools:     req.Tools,
		MaxTokens: req.MaxTokens,
		Config:    w.currentConfig(),
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ports.ChatResponseWithAudio{}, fmt.Errorf("ai plugin %s: marshal chat_to_audio input: %w", w.plugin.ID(), err)
	}
	out, err := w.plugin.Call(chatToAudioFn, raw)
	if err != nil {
		if isMissingPluginFunction(err, chatToAudioFn) {
			return ports.ChatResponseWithAudio{}, fmt.Errorf("ai plugin %s: audio output not supported", w.plugin.ID())
		}
		return ports.ChatResponseWithAudio{}, err
	}
	var resp chatToAudioPluginOutput
	if err := json.Unmarshal(out, &resp); err != nil {
		return ports.ChatResponseWithAudio{}, fmt.Errorf("ai plugin %s: unmarshal chat_to_audio output: %w", w.plugin.ID(), err)
	}
	if resp.Error != "" {
		return ports.ChatResponseWithAudio{}, fmt.Errorf("ai plugin %s: %s", w.plugin.ID(), resp.Error)
	}
	audioBase64 := strings.TrimSpace(resp.AudioData)
	if audioBase64 == "" {
		audioBase64 = strings.TrimSpace(resp.Audio)
	}
	audioData := []byte{}
	if audioBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(audioBase64)
		if err != nil {
			return ports.ChatResponseWithAudio{}, fmt.Errorf("ai plugin %s: decode chat_to_audio audio: %w", w.plugin.ID(), err)
		}
		audioData = decoded
	}
	return ports.ChatResponseWithAudio{
		Content:    resp.Content,
		AudioData:  audioData,
		StopReason: resp.StopReason,
	}, nil
}

func (w *AIWrapper) SupportsAudioInput() bool  { return w.metadataBoolProperty("supports_audio_input") }
func (w *AIWrapper) SupportsAudioOutput() bool { return w.metadataBoolProperty("supports_audio_output") }
func (w *AIWrapper) GetMaxTokens() int         { return ports.DefaultMaxTokens }
func (w *AIWrapper) GetContextWindow() int     { return 128000 }

func (w *AIWrapper) loadMetadata() aiMetadata {
	w.metadataOnce.Do(func() {
		w.metadata = aiMetadata{Properties: map[string]any{}}

		raw, err := w.plugin.Call(metadataFn, nil)
		if err != nil {
			return
		}

		var decoded aiMetadata
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return
		}
		if decoded.Properties == nil {
			decoded.Properties = map[string]any{}
		}

		w.metadata = decoded
	})

	return w.metadata
}

func (w *AIWrapper) metadataBoolProperty(key string) bool {
	md := w.loadMetadata()
	value, ok := md.Properties[key]
	if !ok {
		return false
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(typed))
		return trimmed == "true" || trimmed == "1" || trimmed == "yes"
	default:
		return false
	}
}
