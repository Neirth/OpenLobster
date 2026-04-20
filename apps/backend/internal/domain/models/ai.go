package models

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

type AIProviderConfig struct {
	Type      ModelProvider `json:"type"`
	APIKey    string        `json:"api_key"`
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Endpoint  string        `json:"endpoint,omitempty"`
}

func NewAIProviderConfig(provider ModelProvider, apiKey, model string, maxTokens int) *AIProviderConfig {
	return &AIProviderConfig{
		Type:      provider,
		APIKey:    apiKey,
		Model:     model,
		MaxTokens: maxTokens,
	}
}
