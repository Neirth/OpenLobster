package ports

import (
	"context"
)

type AIProviderPort interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ChatWithAudio(ctx context.Context, req ChatRequestWithAudio) (ChatResponse, error)
	ChatToAudio(ctx context.Context, req ChatRequest) (ChatResponseWithAudio, error)
	SupportsAudioInput() bool
	SupportsAudioOutput() bool
	// GetMaxTokens returns the maximum number of output tokens the adapter is
	// configured to generate per response.
	GetMaxTokens() int
	// GetContextWindow returns the model's total context window in tokens
	// (input + output). Used by the memory consolidation pipeline to size
	// message chunks so they fit within the model's input budget.
	GetContextWindow() int
}

// ContentBlockType identifies the kind of content in a multimodal message part.
type ContentBlockType string

const (
	ContentBlockText  ContentBlockType = "text"
	ContentBlockImage ContentBlockType = "image"
	ContentBlockAudio ContentBlockType = "audio"
)

// ContentBlock is a single part of a multimodal user message.
// For text blocks only Text is set. For image blocks, MIMEType and Data (base64
// encoded bytes) or URL must be set. For audio blocks, MIMEType and Data
// (raw PCM or encoded bytes) must be set.
type ContentBlock struct {
	Type     ContentBlockType `json:"type"`
	Text     string           `json:"text,omitempty"`
	URL      string           `json:"url,omitempty"`
	Data     []byte           `json:"data,omitempty"`
	MIMEType string           `json:"mime_type,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// Blocks carries multimodal content (images, audio). When non-empty it takes
	// precedence over Content for user messages; adapters must render all blocks.
	Blocks []ContentBlock `json:"blocks,omitempty"`
	// ToolCalls is populated for assistant messages that triggered tool use.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID links a tool-role result message back to the originating call.
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolName is the name of the tool for tool-role messages (Ollama Cloud expects tool_name).
	ToolName string `json:"tool_name,omitempty"`
}

type ChatRequest struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	Tools     []Tool        `json:"tools,omitempty"`
	// MaxTokens limits the number of tokens in the response. Zero means use the
	// adapter default configured at startup.
	MaxTokens int `json:"max_tokens,omitempty"`
}

type Tool struct {
	Type     string        `json:"type"`
	Function *FunctionTool `json:"function,omitempty"`
}

type FunctionTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// TokenUsage reports the number of tokens consumed by a chat call.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
}

// Total returns the sum of prompt and completion tokens.
func (u TokenUsage) Total() int { return u.PromptTokens + u.CompletionTokens }

type ChatResponse struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason"`
	Audio      []byte     `json:"audio,omitempty"`
	Usage      TokenUsage `json:"usage"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatRequestWithAudio struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	AudioData []byte        `json:"audio_data"`
	Tools     []Tool        `json:"tools,omitempty"`
}

type ChatResponseWithAudio struct {
	Content    string `json:"content"`
	AudioData  []byte `json:"audio_data"`
	StopReason string `json:"stop_reason"`
}
