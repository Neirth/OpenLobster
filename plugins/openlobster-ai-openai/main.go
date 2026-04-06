//go:build !tinygo

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
	goOpenAI "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
	_ "github.com/stealthrocket/net/http"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func getName() int32 { pdk.OutputString("openlobster-ai-openai"); return 0 }

func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

func getDescription() int32 {
	pdk.OutputString("OpenAI AI provider plugin for OpenLobster")
	return 0
}

func getType() int32 { pdk.OutputString("ai"); return 0 }

func supportsAudioInput() int32 { pdk.OutputString("true"); return 0 }

func supportsAudioOutput() int32 { pdk.OutputString("false"); return 0 }

func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key","description":"Provider API key used for authentication"},"model":{"type":"string","title":"Model","default":"gpt-4o","description":"Default model when a request does not specify one"},"endpoint":{"type":"string","title":"Endpoint","description":"Select the provider endpoint by name","default":"OpenAI","enum":["OpenAI","OpenRouter","Docker Model Runner","OpenCode Zen","Groq","Together AI","Fireworks AI","DeepSeek","Perplexity","Mistral","xAI","Custom"]},"base_url":{"type":"string","title":"Base URL (Custom)","description":"Required when endpoint is Custom"}},"required":["api_key"]}`)
	return 0
}

func getMetadata() int32 {
	metadata := map[string]interface{}{
		"id":          "openlobster-ai-openai",
		"name":        "openlobster-ai-openai",
		"version":     "0.1.0",
		"description": "OpenAI AI provider plugin for OpenLobster",
		"type":        "ai",
		"schema":      json.RawMessage(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key","description":"Provider API key used for authentication"},"model":{"type":"string","title":"Model","default":"gpt-4o","description":"Default model when a request does not specify one"},"endpoint":{"type":"string","title":"Endpoint","description":"Select the provider endpoint by name","default":"OpenAI","enum":["OpenAI","OpenRouter","Docker Model Runner","OpenCode Zen","Groq","Together AI","Fireworks AI","DeepSeek","Perplexity","Mistral","xAI","Custom"]},"base_url":{"type":"string","title":"Base URL (Custom)","description":"Required when endpoint is Custom"}},"required":["api_key"]}`),
		"properties":  json.RawMessage(`{"supports_audio_input":true,"supports_audio_output":false}`),
	}
	if err := pdk.OutputJSON(metadata); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type contentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	URL      string `json:"url,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	Blocks     []contentBlock `json:"blocks,omitempty"`
	ToolCalls  []toolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolName   string         `json:"tool_name,omitempty"`
}

type toolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
	} `json:"function"`
}

type inputPayload struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	Tools     []toolDef     `json:"tools,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Config    struct {
		APIKey   string `json:"api_key"`
		Endpoint string `json:"endpoint,omitempty"`
		BaseURL  string `json:"base_url,omitempty"`
		Model    string `json:"model,omitempty"`
	} `json:"config"`
}

type outputPayload struct {
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason"`
	Usage      struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error string `json:"error,omitempty"`
}

func encodeToolName(name string) string {
	return strings.ReplaceAll(name, ":", "__")
}

func decodeToolName(name string) string {
	return strings.ReplaceAll(name, "__", ":")
}

func openAIImageURLFromBlock(block contentBlock) string {
	if url := strings.TrimSpace(block.URL); url != "" {
		return url
	}
	if len(block.Data) == 0 {
		return ""
	}
	mimeType := strings.ToLower(strings.TrimSpace(block.MIMEType))
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(block.Data))
}

func openAIInputAudioFormat(mimeType string) string {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(normalized, ";"); idx >= 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}

	switch normalized {
	case "wav", "audio/wav", "audio/x-wav", "audio/wave", "audio/vnd.wave":
		return "wav"
	case "mp3", "audio/mp3", "audio/mpeg", "audio/mpeg3", "audio/x-mp3", "audio/x-mpeg", "audio/x-mpeg-3":
		return "mp3"
	default:
		return ""
	}
}

func openAIMessageContentWithFallback(message goOpenAI.ChatCompletionMessage) string {
	if strings.TrimSpace(message.Content) != "" {
		return message.Content
	}

	raw := strings.TrimSpace(message.RawJSON())
	if raw == "" {
		return message.Content
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return message.Content
	}

	if reasoning, ok := payload["reasoning_content"].(string); ok {
		if strings.TrimSpace(reasoning) != "" {
			return reasoning
		}
	}
	if thinking, ok := payload["thinking"].(string); ok {
		if strings.TrimSpace(thinking) != "" {
			return thinking
		}
	}

	return message.Content
}

func sanitizeMessages(messages []chatMessage) []chatMessage {
	validIDs := make(map[string]struct{})
	out := make([]chatMessage, 0, len(messages))

	for _, m := range messages {
		if m.Role == "tool" {
			id := strings.TrimSpace(m.ToolCallID)
			if id == "" {
				continue
			}
			if _, ok := validIDs[id]; !ok {
				continue
			}
		}

		for _, tc := range m.ToolCalls {
			if strings.TrimSpace(tc.ID) == "" {
				continue
			}
			validIDs[tc.ID] = struct{}{}
		}

		out = append(out, m)
	}

	return out
}

func resolveEndpointBaseURL(endpointName string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(endpointName))
	if name == "" || name == "openai" {
		return "", nil
	}

	switch name {
	case "openrouter":
		return "https://openrouter.ai/api/v1", nil
	case "docker model runner", "docker-model-runner":
		return "http://localhost:12434/engines/v1", nil
	case "opencode zen", "opencode-zen", "opencode":
		return "https://opencode.ai/zen/v1", nil
	case "groq":
		return "https://api.groq.com/openai/v1", nil
	case "together ai", "together":
		return "https://api.together.xyz/v1", nil
	case "fireworks ai", "fireworks":
		return "https://api.fireworks.ai/inference/v1", nil
	case "deepseek":
		return "https://api.deepseek.com/v1", nil
	case "perplexity":
		return "https://api.perplexity.ai", nil
	case "mistral":
		return "https://api.mistral.ai/v1", nil
	case "xai", "x.ai":
		return "https://api.x.ai/v1", nil
	case "custom":
		return "", fmt.Errorf("base_url required when endpoint is Custom")
	default:
		return "", fmt.Errorf("unsupported endpoint %q", endpointName)
	}
}

// ---------------------------------------------------------------------------
// chat
// ---------------------------------------------------------------------------

func chat() int32 {
	var input inputPayload
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	apiKey := input.Config.APIKey
	if apiKey == "" {
		pdk.SetError(fmt.Errorf("api_key required"))
		return 1
	}

	baseURL := strings.TrimSpace(input.Config.BaseURL)
	if baseURL == "" {
		resolvedURL, err := resolveEndpointBaseURL(input.Config.Endpoint)
		if err != nil {
			pdk.SetError(err)
			return 1
		}
		baseURL = resolvedURL
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := goOpenAI.NewClient(opts...)

	model := input.Model
	if model == "" {
		model = input.Config.Model
	}
	if model == "" {
		model = "gpt-4o"
	}

	// Build messages
	var messages []goOpenAI.ChatCompletionMessageParamUnion
	for _, m := range sanitizeMessages(input.Messages) {
		switch m.Role {
		case "system":
			messages = append(messages, goOpenAI.SystemMessage(m.Content))
		case "assistant":
			if len(m.ToolCalls) > 0 {
				var tcs []goOpenAI.ChatCompletionMessageToolCallUnionParam
				for _, tc := range m.ToolCalls {
					tcs = append(tcs, goOpenAI.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &goOpenAI.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: goOpenAI.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      encodeToolName(tc.Function.Name),
								Arguments: tc.Function.Arguments,
							},
						},
					})
				}
				messages = append(messages, goOpenAI.ChatCompletionMessageParamUnion{
					OfAssistant: &goOpenAI.ChatCompletionAssistantMessageParam{
						Content:   goOpenAI.ChatCompletionAssistantMessageParamContentUnion{OfString: goOpenAI.String(m.Content)},
						ToolCalls: tcs,
					},
				})
			} else {
				messages = append(messages, goOpenAI.AssistantMessage(m.Content))
			}
		case "tool":
			messages = append(messages, goOpenAI.ToolMessage(m.Content, m.ToolCallID))
		default:
			if len(m.Blocks) > 0 {
				var parts []goOpenAI.ChatCompletionContentPartUnionParam
				for _, b := range m.Blocks {
					switch b.Type {
					case "text":
						parts = append(parts, goOpenAI.TextContentPart(b.Text))
					case "image":
						if imageURL := openAIImageURLFromBlock(b); imageURL != "" {
							parts = append(parts, goOpenAI.ImageContentPart(
								goOpenAI.ChatCompletionContentPartImageImageURLParam{URL: imageURL},
							))
						}
					case "audio":
						if len(b.Data) > 0 {
							if audioFormat := openAIInputAudioFormat(b.MIMEType); audioFormat != "" {
								parts = append(parts, goOpenAI.InputAudioContentPart(
									goOpenAI.ChatCompletionContentPartInputAudioInputAudioParam{
										Data:   base64.StdEncoding.EncodeToString(b.Data),
										Format: audioFormat,
									},
								))
							}
						}
					}
				}
				messages = append(messages, goOpenAI.UserMessage(parts))
			} else {
				messages = append(messages, goOpenAI.UserMessage(m.Content))
			}
		}
	}

	// Build tools
	var openAITools []goOpenAI.ChatCompletionToolUnionParam
	for _, t := range input.Tools {
		if t.Function.Parameters == nil {
			t.Function.Parameters = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		paramsJSON, _ := json.Marshal(t.Function.Parameters)
		var params shared.FunctionParameters
		_ = json.Unmarshal(paramsJSON, &params)
		openAITools = append(openAITools, goOpenAI.ChatCompletionToolUnionParam{
			OfFunction: &goOpenAI.ChatCompletionFunctionToolParam{
				Function: shared.FunctionDefinitionParam{
					Name:        encodeToolName(t.Function.Name),
					Description: goOpenAI.String(t.Function.Description),
					Parameters:  params,
				},
			},
		})
	}

	params := goOpenAI.ChatCompletionNewParams{
		Model:    goOpenAI.ChatModel(model),
		Messages: messages,
	}
	if len(openAITools) > 0 {
		params.Tools = openAITools
	}
	if input.MaxTokens > 0 {
		params.MaxTokens = goOpenAI.Int(int64(input.MaxTokens))
	}

	resp, err := client.Chat.Completions.New(context.Background(), params)
	if err != nil {
		pdk.SetError(fmt.Errorf("openai chat request failed: %w", err))
		out := outputPayload{Error: err.Error()}
		_ = pdk.OutputJSON(out)
		return 1
	}

	out := outputPayload{}
	if len(resp.Choices) > 0 {
		out.Content = openAIMessageContentWithFallback(resp.Choices[0].Message)
		out.StopReason = string(resp.Choices[0].FinishReason)
		if out.StopReason == "tool_calls" {
			out.StopReason = "tool_use"
		}
		for _, tc := range resp.Choices[0].Message.ToolCalls {
			call := tc.AsFunction()
			out.ToolCalls = append(out.ToolCalls, toolCall{
				ID:   call.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: decodeToolName(call.Function.Name), Arguments: call.Function.Arguments},
			})
		}
	}
	out.Usage.PromptTokens = int(resp.Usage.PromptTokens)
	out.Usage.CompletionTokens = int(resp.Usage.CompletionTokens)

	if err := pdk.OutputJSON(out); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func main() {
	pdk.MustRun(pdk.Plugin{
		ID: "openlobster-ai-openai",
		Exports: map[string]pdk.Function{
			"get_metadata": getMetadata,
			"configure":    configureHot,
			"chat":         chat,
		},
	})
}
