// Package anthropic provides an AI adapter backed by the official
// github.com/anthropics/anthropic-sdk-go SDK.
//
// The adapter implements [ports.AIProviderPort] and translates the internal
// domain types to Anthropic API types, including tool-name encoding
// (":" ↔ "__") so that tool names remain compatible with both providers.
//
// # License
// See LICENSE in the root of the repository.
package anthropic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/neirth/openlobster/internal/domain/ports"
)

const defaultMaxTokens = 4096

// Adapter implements [ports.AIProviderPort] using the official Anthropic SDK.
type Adapter struct {
	client    anthropic.Client
	model     string
	maxTokens int
}

// NewAdapter creates an Adapter that targets the Anthropic Messages API.
func NewAdapter(apiKey, model string, maxTokens int) *Adapter {
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	return &Adapter{
		client:    anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:     model,
		maxTokens: maxTokens,
	}
}

// NewAdapterWithBaseURL creates an Adapter that sends requests to a custom
// base URL (e.g. an Anthropic-compatible gateway). The SDK appends /v1/messages
// to the provided baseURL.
func NewAdapterWithBaseURL(baseURL, apiKey, model string, maxTokens int) *Adapter {
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	return &Adapter{
		client:    anthropic.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL)),
		model:     model,
		maxTokens: maxTokens,
	}
}

// Chat sends a chat request to the Anthropic API and returns the model's
// response. The stop reason "end_turn" is normalised to "stop" so upper
// layers remain provider-agnostic.
func (a *Adapter) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = a.model
	}

	systemBlocks, messages := convertMessages(req.Messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(a.maxTokens),
		Messages:  messages,
	}
	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}
	if len(req.Tools) > 0 {
		params.Tools = convertTools(req.Tools)
	}

	resp, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return ports.ChatResponse{}, fmt.Errorf("anthropic chat: %w", err)
	}

	return parseResponse(resp), nil
}

// ChatWithAudio delegates to Chat; Anthropic does not support audio input in
// the Messages API.
func (a *Adapter) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	textReq := ports.ChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Tools:    req.Tools,
	}
	return a.Chat(ctx, textReq)
}

// ChatToAudio delegates to Chat; Anthropic does not support audio output in
// the Messages API.
func (a *Adapter) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	resp, err := a.Chat(ctx, req)
	if err != nil {
		return ports.ChatResponseWithAudio{}, err
	}
	return ports.ChatResponseWithAudio{
		Content:    resp.Content,
		StopReason: resp.StopReason,
	}, nil
}

// SupportsAudioInput always returns false.
func (a *Adapter) SupportsAudioInput() bool { return false }

// SupportsAudioOutput always returns false.
func (a *Adapter) SupportsAudioOutput() bool { return false }

// GetMaxTokens returns the configured maximum token limit.
func (a *Adapter) GetMaxTokens() int { return a.maxTokens }

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// encodeToolName replaces ":" with "__" so tool names are valid identifiers
// for the Anthropic API (which does not allow colons in tool names).
func encodeToolName(name string) string {
	return strings.ReplaceAll(name, ":", "__")
}

// decodeToolName restores "__" back to ":" after receiving tool calls from the
// Anthropic API.
func decodeToolName(name string) string {
	return strings.ReplaceAll(name, "__", ":")
}

// convertUserBlocks builds the content block slice for a user-role message.
// When the message carries multimodal Blocks, each is rendered appropriately;
// otherwise a single text block is returned.
func convertUserBlocks(m ports.ChatMessage) []anthropic.ContentBlockParamUnion {
	if len(m.Blocks) == 0 {
		return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(m.Content)}
	}
	out := make([]anthropic.ContentBlockParamUnion, 0, len(m.Blocks))
	for _, b := range m.Blocks {
		switch b.Type {
		case ports.ContentBlockText:
			if b.Text != "" {
				out = append(out, anthropic.NewTextBlock(b.Text))
			}
		case ports.ContentBlockImage:
			if b.URL != "" {
				out = append(out, anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: b.URL}))
			} else if len(b.Data) > 0 {
				out = append(out, anthropic.NewImageBlock(anthropic.Base64ImageSourceParam{
					MediaType: anthropic.Base64ImageSourceMediaType(b.MIMEType),
					Data:      base64.StdEncoding.EncodeToString(b.Data),
				}))
			}
		case ports.ContentBlockAudio:
			// Anthropic does not support audio input in the Messages API; skip silently.
		}
	}
	if len(out) == 0 {
		out = append(out, anthropic.NewTextBlock(m.Content))
	}
	return out
}

// convertMessages splits system-role messages out into TextBlockParam slices
// and converts the remaining messages to []anthropic.MessageParam.
//
// Consecutive messages with the same role are preserved as-is because the
// Anthropic API merges them automatically.
func convertMessages(msgs []ports.ChatMessage) ([]anthropic.TextBlockParam, []anthropic.MessageParam) {
	var systemBlocks []anthropic.TextBlockParam
	var params []anthropic.MessageParam

	for _, m := range msgs {
		switch m.Role {
		case "system":
			if m.Content != "" {
				systemBlocks = append(systemBlocks, anthropic.TextBlockParam{Text: m.Content})
			}

		case "user":
			params = append(params, anthropic.NewUserMessage(convertUserBlocks(m)...))

		case "tool":
			// Tool result: a user-turn message with OfToolResult content.
			// When the tool returned multimodal Blocks (e.g. an image from read_file),
			// include them in the tool_result content; otherwise use a text block.
			var resultContent []anthropic.ToolResultBlockParamContentUnion
			if len(m.Blocks) > 0 {
				for _, b := range m.Blocks {
					switch b.Type {
					case ports.ContentBlockImage:
						if b.URL != "" {
							resultContent = append(resultContent, anthropic.ToolResultBlockParamContentUnion{
								OfImage: &anthropic.ImageBlockParam{
									Source: anthropic.ImageBlockParamSourceUnion{
										OfURL: &anthropic.URLImageSourceParam{URL: b.URL},
									},
								},
							})
						} else if len(b.Data) > 0 {
							resultContent = append(resultContent, anthropic.ToolResultBlockParamContentUnion{
								OfImage: &anthropic.ImageBlockParam{
									Source: anthropic.ImageBlockParamSourceUnion{
										OfBase64: &anthropic.Base64ImageSourceParam{
											MediaType: anthropic.Base64ImageSourceMediaType(b.MIMEType),
											Data:      base64.StdEncoding.EncodeToString(b.Data),
										},
									},
								},
							})
						}
						if b.Text != "" {
							resultContent = append(resultContent, anthropic.ToolResultBlockParamContentUnion{
								OfText: &anthropic.TextBlockParam{Text: b.Text},
							})
						}
					case ports.ContentBlockAudio:
						// Anthropic does not support audio in tool results; fall back to text.
						if b.Text != "" {
							resultContent = append(resultContent, anthropic.ToolResultBlockParamContentUnion{
								OfText: &anthropic.TextBlockParam{Text: b.Text},
							})
						}
					case ports.ContentBlockText:
						if b.Text != "" {
							resultContent = append(resultContent, anthropic.ToolResultBlockParamContentUnion{
								OfText: &anthropic.TextBlockParam{Text: b.Text},
							})
						}
					}
				}
			}
			if len(resultContent) == 0 {
				resultContent = []anthropic.ToolResultBlockParamContentUnion{
					{OfText: &anthropic.TextBlockParam{Text: m.Content}},
				}
			}
			params = append(params, anthropic.NewUserMessage(
				anthropic.ContentBlockParamUnion{
					OfToolResult: &anthropic.ToolResultBlockParam{
						ToolUseID: m.ToolCallID,
						Content:   resultContent,
					},
				},
			))

		case "assistant":
			var blocks []anthropic.ContentBlockParamUnion
			// Append text content if non-empty.
			if m.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(m.Content))
			}
			// Append tool-use blocks for each tool call.
			for _, tc := range m.ToolCalls {
				inputRaw := json.RawMessage(tc.Function.Arguments)
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    tc.ID,
						Name:  encodeToolName(tc.Function.Name),
						Input: inputRaw,
					},
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, anthropic.NewTextBlock(""))
			}
			params = append(params, anthropic.NewAssistantMessage(blocks...))
		}
	}

	return systemBlocks, params
}

// convertTools converts the provider-agnostic tool definitions to the
// Anthropic SDK parameter type.
func convertTools(tools []ports.Tool) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		if t.Function == nil {
			continue
		}

		// Extract just the "properties" map from the JSON-Schema object.
		var properties interface{}
		if props, ok := t.Function.Parameters["properties"]; ok {
			properties = props
		} else {
			properties = map[string]interface{}{}
		}

		result = append(result, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        encodeToolName(t.Function.Name),
				Description: anthropic.String(t.Function.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: properties,
				},
			},
		})
	}
	return result
}

// parseResponse converts an Anthropic Message response to the internal
// ChatResponse type.
func parseResponse(resp *anthropic.Message) ports.ChatResponse {
	var textParts []string
	var toolCalls []ports.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				textParts = append(textParts, block.Text)
			}
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, ports.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ports.FunctionCall{
					Name:      decodeToolName(block.Name),
					Arguments: string(argsJSON),
				},
			})
		}
	}

	stopReason := string(resp.StopReason)
	// Normalise Anthropic stop reasons to the internal convention.
	switch stopReason {
	case "end_turn":
		stopReason = "stop"
	case "max_tokens":
		stopReason = "max_tokens"
		// "tool_use", "stop_sequence" are passed through unchanged.
	}

	return ports.ChatResponse{
		Content:    strings.Join(textParts, ""),
		ToolCalls:  toolCalls,
		StopReason: stopReason,
	}
}

var _ ports.AIProviderPort = (*Adapter)(nil)
