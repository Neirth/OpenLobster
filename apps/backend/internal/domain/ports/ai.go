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
	GetMaxTokens() int
}

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	// ToolCalls is populated for assistant messages that triggered tool use.
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID links a tool-role result message back to the originating call.
	ToolCallID string     `json:"tool_call_id,omitempty"`
	// ToolName is the name of the tool for tool-role messages (Ollama Cloud expects tool_name).
	ToolName   string     `json:"tool_name,omitempty"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
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

type ChatResponse struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason"`
	Audio      []byte     `json:"audio,omitempty"`
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
