package main

import (
	"context"
	"encoding/json"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/extism/go-pdk"
)

type InputMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

type InputPayload struct {
	Model     string         `json:"model"`
	Messages  []InputMessage `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
	MaxTokens int            `json:"max_tokens,omitempty"`
	Config    struct {
		APIKey       string `json:"api_key"`
		DefaultModel string `json:"default_model,omitempty"`
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
	pdk.OutputString("openlobster-ai-anthropic")
	return 0
}

//go:wasmexport get_version
func getVersion() int32 {
	pdk.OutputString("0.1.0")
	return 0
}

//go:wasmexport get_description
func getDescription() int32 {
	pdk.OutputString("Anthropic Claude API plugin for OpenLobster")
	return 0
}

//go:wasmexport get_type
func getType() int32 {
	pdk.OutputString("ai")
	return 0
}

//go:wasmexport get_schema
func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key"},"default_model":{"type":"string","title":"Default Model","default":"claude-opus-4-6"}},"required":["api_key"]}`)
	return 0
}

func encodeToolName(name string) string { return strings.ReplaceAll(name, ":", "__") }
func decodeToolName(name string) string { return strings.ReplaceAll(name, "__", ":") }

func convertMessages(in []InputMessage) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(in))
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
		msgRole := anthropic.MessageParamRoleUser
		if role == "assistant" {
			msgRole = anthropic.MessageParamRoleAssistant
		}
		out = append(out, anthropic.MessageParam{
			Role: msgRole,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock(content),
			},
		})
	}
	return out
}

func convertTools(in []ToolDefinition) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(in))
	for _, t := range in {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		tool := anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        encodeToolName(name),
				Description: anthropic.String(t.Description),
			},
		}
		if t.InputSchema != nil {
			if raw, err := json.Marshal(t.InputSchema); err == nil {
				tool.OfTool.InputSchema = raw
			}
		}
		out = append(out, tool)
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
		model = strings.TrimSpace(input.Config.DefaultModel)
	}
	if model == "" {
		model = "claude-opus-4-6"
	}
	maxTokens := input.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	client := anthropic.NewClient(option.WithAPIKey(input.Config.APIKey))
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  convertMessages(input.Messages),
	}
	if tools := convertTools(input.Tools); len(tools) > 0 {
		params.Tools = tools
	}

	resp, err := client.Messages.New(context.Background(), params)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	out := OutputPayload{StopReason: resp.StopReason}
	if out.StopReason == "end_turn" {
		out.StopReason = "stop"
	}
	if out.StopReason == "tool_use" {
		out.StopReason = "tool_use"
	}
	out.Usage = OutputUsage{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
	}

	for _, block := range resp.Content {
		switch block.AsAny().(type) {
		case anthropic.TextBlock:
			tb := block.OfText
			out.Content += tb.Text
		case anthropic.ToolUseBlock:
			tool := block.OfToolUse
			inputBytes, _ := json.Marshal(tool.Input)
			out.ToolCalls = append(out.ToolCalls, OutputToolCall{
				ID:    tool.ID,
				Name:  decodeToolName(tool.Name),
				Input: string(inputBytes),
			})
		}
	}

	if err := pdk.OutputJSON(out); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func main() {}
