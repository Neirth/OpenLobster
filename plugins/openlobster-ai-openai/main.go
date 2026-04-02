package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/extism/go-pdk"
	goOpenAI "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type InputPayload struct {
	Model     string        `json:"model"`
	Messages  []ChatMessage `json:"messages"`
	Tools     []Tool        `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Config    struct {
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url,omitempty"`
	} `json:"config"`
}

type OutputToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

type OutputUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type OutputPayload struct {
	Content    string           `json:"content"`
	ToolCalls  []OutputToolCall `json:"tool_calls,omitempty"`
	StopReason string           `json:"stop_reason"`
	Usage      OutputUsage      `json:"usage"`
}

//go:wasmexport get_name
func getName() int32 {
	pdk.OutputString("openlobster-ai-openai")
	return 0
}

//go:wasmexport get_version
func getVersion() int32 {
	pdk.OutputString("0.1.0")
	return 0
}

//go:wasmexport get_description
func getDescription() int32 {
	pdk.OutputString("OpenAI API plugin for OpenLobster")
	return 0
}

//go:wasmexport get_type
func getType() int32 {
	pdk.OutputString("ai")
	return 0
}

//go:wasmexport get_schema
func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key"},"base_url":{"type":"string","title":"Base URL (optional)","default":"https://api.openai.com/v1"},"default_model":{"type":"string","title":"Default Model","default":"gpt-4o"}},"required":["api_key"]}`)
	return 0
}

func convertMessages(in []ChatMessage) []goOpenAI.ChatCompletionMessageParamUnion {
	out := make([]goOpenAI.ChatCompletionMessageParamUnion, 0, len(in))
	for _, m := range in {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		content := ""
		switch v := m.Content.(type) {
		case string:
			content = v
		default:
			b, _ := json.Marshal(v)
			content = string(b)
		}
		switch role {
		case "system":
			out = append(out, goOpenAI.SystemMessage(content))
		case "assistant":
			out = append(out, goOpenAI.AssistantMessage(content))
		case "tool":
			if m.ToolCallID != "" {
				out = append(out, goOpenAI.ToolMessage(content, m.ToolCallID))
			}
		default:
			out = append(out, goOpenAI.UserMessage(content))
		}
	}
	return out
}

func convertTools(in []Tool) []goOpenAI.ChatCompletionToolParam {
	out := make([]goOpenAI.ChatCompletionToolParam, 0, len(in))
	for _, t := range in {
		if strings.ToLower(strings.TrimSpace(t.Type)) != "function" || t.Function.Name == "" {
			continue
		}
		fn := goOpenAI.FunctionDefinitionParam{
			Name:        strings.ReplaceAll(t.Function.Name, ":", "__"),
			Description: goOpenAI.String(t.Function.Description),
		}
		if t.Function.Parameters != nil {
			if raw, err := json.Marshal(t.Function.Parameters); err == nil {
				fn.Parameters = raw
			}
		}
		out = append(out, goOpenAI.ChatCompletionToolParam{
			Type:     "function",
			Function: fn,
		})
	}
	return out
}

//go:wasmexport chat
func chat() int32 {
	var input InputPayload
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}
	if strings.TrimSpace(input.Config.APIKey) == "" {
		pdk.SetErrorString("api_key required")
		return 1
	}

	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = "gpt-4o"
	}

	opts := []option.RequestOption{option.WithAPIKey(input.Config.APIKey)}
	if base := strings.TrimSpace(input.Config.BaseURL); base != "" {
		opts = append(opts, option.WithBaseURL(base))
	}
	client := goOpenAI.NewClient(opts...)

	params := goOpenAI.ChatCompletionNewParams{
		Model:    goOpenAI.ChatModel(model),
		Messages: convertMessages(input.Messages),
	}
	if input.MaxTokens > 0 {
		params.MaxCompletionTokens = goOpenAI.Int(int64(input.MaxTokens))
	}
	if tools := convertTools(input.Tools); len(tools) > 0 {
		params.Tools = tools
	}

	resp, err := client.Chat.Completions.New(context.Background(), params)
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	if len(resp.Choices) == 0 {
		pdk.SetErrorString("no choices in response")
		return 1
	}

	choice := resp.Choices[0]
	out := OutputPayload{
		Content:    choice.Message.Content,
		StopReason: choice.FinishReason,
	}
	if out.StopReason == "tool_calls" {
		out.StopReason = "tool_use"
	}
	out.Usage = OutputUsage{
		PromptTokens:     int(resp.Usage.PromptTokens),
		CompletionTokens: int(resp.Usage.CompletionTokens),
	}
	for _, tc := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, OutputToolCall{
			ID:    tc.ID,
			Name:  strings.ReplaceAll(tc.Function.Name, "__", ":"),
			Input: tc.Function.Arguments,
		})
	}

	if err := pdk.OutputJSON(out); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func main() {}
