// Package openai provides a unified AI adapter backed by the official
// github.com/openai/openai-go/v3 SDK.
//
// Use [NewAdapter] for the standard OpenAI API endpoint or
// [NewAdapterWithEndpoint] for any OpenAI-compatible service (OpenRouter,
// LM Studio, vLLM, etc.).
//
// # License
// See LICENSE in the root of the repository.
package openai

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/neirth/openlobster/internal/domain/ports"
	goOpenAI "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// Adapter implements [ports.AIProviderPort] using the official OpenAI SDK.
// It supports any OpenAI-compatible endpoint via [NewAdapterWithEndpoint].
type Adapter struct {
	client         goOpenAI.Client
	model          string
	maxTokens      int
	reasoningLevel string
	contextWindow  int
}

// NewAdapter creates an Adapter targeting the standard OpenAI API endpoint.
func NewAdapter(apiKey, model string, maxTokens int, reasoningLevel string) *Adapter {
	return NewAdapterWithEndpoint("", apiKey, model, maxTokens, reasoningLevel)
}

// NewAdapterWithEndpoint creates an Adapter targeting an arbitrary
// OpenAI-compatible endpoint. Pass an empty baseURL to use the default
// OpenAI endpoint (api.openai.com).
func NewAdapterWithEndpoint(baseURL, apiKey, model string, maxTokens int, reasoningLevel string) *Adapter {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &Adapter{
		client:         goOpenAI.NewClient(opts...),
		model:          model,
		maxTokens:      maxTokens,
		reasoningLevel: reasoningLevel,
	}
}

// Chat sends a chat-completion request and returns the model's reply.
// Stop reason "tool_calls" is normalised to "tool_use" so upper layers remain
// provider-agnostic.
func (a *Adapter) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	params := goOpenAI.ChatCompletionNewParams{
		Model:               goOpenAI.ChatModel(a.model),
		Messages:            convertMessages(sanitizeMessages(req.Messages)),
		MaxCompletionTokens: goOpenAI.Int(int64(a.maxTokens)),
	}

	if a.reasoningLevel != "" && a.reasoningLevel != "none" {
		// ReasoningEffort is not yet supported by the vendored OpenAI SDK.
		// We preserve the level in the Adapter struct for future updates:
		// params.ReasoningEffort = goOpenAI.F(shared.ReasoningEffort(a.reasoningLevel))
		_ = a.reasoningLevel // satisfy linter
	}

	if len(req.Tools) > 0 {
		params.Tools = convertTools(req.Tools)
	}

	resp, err := a.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return ports.ChatResponse{}, err
	}

	if len(resp.Choices) == 0 {
		return ports.ChatResponse{StopReason: "no_response"}, nil
	}

	choice := resp.Choices[0]
	stopReason := choice.FinishReason
	// Normalise provider-specific stop reason to the internal convention.
	if stopReason == "tool_calls" {
		stopReason = "tool_use"
	}

	result := ports.ChatResponse{
		Content:    choice.Message.Content,
		StopReason: stopReason,
	}

	// The model's internal reasoning (e.g. reasoning_content) is handled by the model
	// to improve the final answer, but we do not expose or store it.

	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]ports.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = ports.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: ports.FunctionCall{
					// Restore qualified name: provider-safe __ → internal :
					Name:      strings.ReplaceAll(tc.Function.Name, "__", ":"),
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return result, nil
}

// ChatWithAudio processes a chat request that may include audio data.
// The audio is currently ignored; only text messages are forwarded.
func (a *Adapter) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return a.Chat(ctx, ports.ChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Tools:    req.Tools,
	})
}

// ChatToAudio sends a chat request and returns the response as text (audio
// synthesis is not natively supported by the Chat Completions endpoint used
// here).
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

// SupportsAudioInput reports whether the configured model can process audio.
func (a *Adapter) SupportsAudioInput() bool {
	return strings.Contains(a.model, "gpt-4o")
}

// SupportsAudioOutput reports whether the configured model can produce audio.
func (a *Adapter) SupportsAudioOutput() bool {
	return strings.Contains(a.model, "gpt-4o")
}

// GetMaxTokens returns the configured maximum token budget.
func (a *Adapter) GetMaxTokens() int {
	return a.maxTokens
}

// GetContextWindow returns the context window set via OverrideContextWindow, or
// 8192 as a safe fallback. OpenAI's API does not expose context window metadata,
// so callers must set providers.openai.context_window in config when the model
// supports more than 8192 input tokens.
func (a *Adapter) GetContextWindow() int {
	if a.contextWindow > 0 {
		return a.contextWindow
	}
	return 8192
}

// OverrideContextWindow explicitly sets the context window for this adapter.
func (a *Adapter) OverrideContextWindow(n int) {
	a.contextWindow = n
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// convertUserContentParts builds the OpenAI content-part slice for a multimodal
// user message. Text blocks become TextContentParts; image blocks become
// ImageContentParts. Audio URL-only blocks are skipped (not supported by the
// Chat Completions endpoint used here).
func convertUserContentParts(blocks []ports.ContentBlock, fallback string) []goOpenAI.ChatCompletionContentPartUnionParam {
	parts := make([]goOpenAI.ChatCompletionContentPartUnionParam, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case ports.ContentBlockText:
			if b.Text != "" {
				parts = append(parts, goOpenAI.TextContentPart(b.Text))
			}
		case ports.ContentBlockImage:
			if b.URL != "" {
				parts = append(parts, goOpenAI.ImageContentPart(goOpenAI.ChatCompletionContentPartImageImageURLParam{
					URL: b.URL,
				}))
			}
			// Data (base64) images not handled here; URL is the expected path for platform attachments.
		case ports.ContentBlockAudio:
			// Audio input via URL is not supported by the Chat Completions endpoint.
		}
	}
	if len(parts) == 0 {
		parts = append(parts, goOpenAI.TextContentPart(fallback))
	}
	return parts
}

// sanitizeMessages drops tool-role messages whose ToolCallID is empty or does
// not correspond to a tool_use block declared by a preceding assistant message,
// preventing the OpenAI API from receiving a tool_call_id that has no matching
// tool call.
func sanitizeMessages(msgs []ports.ChatMessage) []ports.ChatMessage {
	// Single forward pass: only IDs declared by a preceding assistant message
	// are valid for a subsequent tool message.
	validIDs := make(map[string]bool)
	out := make([]ports.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "tool" {
			if m.ToolCallID == "" {
				log.Printf("openai: dropping tool message with empty tool_call_id")
				continue
			}
			if !validIDs[m.ToolCallID] {
				log.Printf("openai: dropping orphan tool message (tool_call_id=%q)", m.ToolCallID)
				continue
			}
		}
		// Register any tool-call IDs this message declares so that subsequent
		// tool messages can be validated against them.
		for _, tc := range m.ToolCalls {
			if tc.ID != "" {
				validIDs[tc.ID] = true
			}
		}
		out = append(out, m)
	}
	return out
}

// convertMessages converts domain ChatMessages to the SDK union param slice.
func convertMessages(msgs []ports.ChatMessage) []goOpenAI.ChatCompletionMessageParamUnion {
	out := make([]goOpenAI.ChatCompletionMessageParamUnion, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "system":
			out = append(out, goOpenAI.SystemMessage(m.Content))

		case "user":
			if len(m.Blocks) == 0 {
				out = append(out, goOpenAI.UserMessage(m.Content))
			} else {
				out = append(out, goOpenAI.ChatCompletionMessageParamUnion{
					OfUser: &goOpenAI.ChatCompletionUserMessageParam{
						Content: goOpenAI.ChatCompletionUserMessageParamContentUnion{
							OfArrayOfContentParts: convertUserContentParts(m.Blocks, m.Content),
						},
					},
				})
			}

		case "assistant":
			if len(m.ToolCalls) > 0 {
				// Assistant message that triggered tool calls.
				toolCallParams := make([]goOpenAI.ChatCompletionMessageToolCallUnionParam, len(m.ToolCalls))
				for i, tc := range m.ToolCalls {
					name := strings.ReplaceAll(tc.Function.Name, ":", "__")
					toolCallParams[i] = goOpenAI.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &goOpenAI.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: goOpenAI.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      name,
								Arguments: tc.Function.Arguments,
							},
						},
					}
				}
				out = append(out, goOpenAI.ChatCompletionMessageParamUnion{
					OfAssistant: &goOpenAI.ChatCompletionAssistantMessageParam{
						Content:   goOpenAI.ChatCompletionAssistantMessageParamContentUnion{OfString: goOpenAI.String(m.Content)},
						ToolCalls: toolCallParams,
					},
				})
			} else {
				out = append(out, goOpenAI.AssistantMessage(m.Content))
			}

		case "tool":
			out = append(out, goOpenAI.ToolMessage(m.Content, m.ToolCallID))

		default:
			// Fallback: treat unknown roles as user messages.
			out = append(out, goOpenAI.UserMessage(m.Content))
		}
	}
	return out
}

// convertTools converts domain Tool definitions to SDK params, sanitising tool
// names by replacing the ":" namespace separator with "__" (the OpenAI API
// only allows alphanumeric characters, dashes, and underscores in names).
func convertTools(tools []ports.Tool) []goOpenAI.ChatCompletionToolUnionParam {
	out := make([]goOpenAI.ChatCompletionToolUnionParam, 0, len(tools))
	for _, t := range tools {
		if t.Function == nil {
			continue
		}
		name := strings.ReplaceAll(t.Function.Name, ":", "__")
		params := shared.FunctionParameters{}
		if t.Function.Parameters != nil {
			raw, err := json.Marshal(t.Function.Parameters)
			if err == nil {
				_ = json.Unmarshal(raw, &params)
			}
		}

		out = append(out, goOpenAI.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        name,
			Description: goOpenAI.String(t.Function.Description),
			Parameters:  params,
		}))
	}
	return out
}

var _ ports.AIProviderPort = (*Adapter)(nil)
