package main

import (
"context"
"encoding/json"
"fmt"

anthropic "github.com/anthropics/anthropic-sdk-go"
"github.com/anthropics/anthropic-sdk-go/option"
pdk "github.com/extism/go-pdk"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-ai-anthropic"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
pdk.OutputString("Anthropic Claude AI provider plugin for OpenLobster")
return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("ai"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
pdk.OutputString(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key"},"model":{"type":"string","title":"Model","default":"claude-sonnet-4-5"}},"required":["api_key"]}`)
return 0
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type contentBlock struct {
Type     string `json:"type"`
Text     string `json:"text,omitempty"`
Data     string `json:"data,omitempty"`
MIMEType string `json:"mime_type,omitempty"`
}

type toolCall struct {
ID   string `json:"id"`
Type string `json:"type"`
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
APIKey string `json:"api_key"`
Model  string `json:"model,omitempty"`
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

// ---------------------------------------------------------------------------
// chat
// ---------------------------------------------------------------------------

//go:wasmexport chat
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

client := anthropic.NewClient(option.WithAPIKey(apiKey))

model := input.Model
if model == "" {
model = input.Config.Model
}
if model == "" {
model = "claude-sonnet-4-5"
}

maxTokens := int64(500)
if input.MaxTokens > 0 {
maxTokens = int64(input.MaxTokens)
}

// Separate system messages
var systemText string
var messages []anthropic.MessageParam
for _, m := range input.Messages {
switch m.Role {
case "system":
systemText = m.Content
case "assistant":
if len(m.ToolCalls) > 0 {
var blocks []anthropic.ContentBlockParamUnion
if m.Content != "" {
blocks = append(blocks, anthropic.NewTextBlock(m.Content))
}
for _, tc := range m.ToolCalls {
var inp map[string]interface{}
_ = json.Unmarshal([]byte(tc.Function.Arguments), &inp)
blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, inp, tc.Function.Name))
}
messages = append(messages, anthropic.NewAssistantMessage(blocks...))
} else {
messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
}
case "tool":
messages = append(messages, anthropic.NewUserMessage(
anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false),
))
default:
if len(m.Blocks) > 0 {
var parts []anthropic.ContentBlockParamUnion
for _, b := range m.Blocks {
switch b.Type {
case "text":
parts = append(parts, anthropic.NewTextBlock(b.Text))
case "image":
if b.Data != "" {
parts = append(parts, anthropic.NewImageBlockBase64(b.MIMEType, b.Data))
}
}
}
messages = append(messages, anthropic.NewUserMessage(parts...))
} else {
messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
}
}
}

// Build tools
var anthropicTools []anthropic.ToolUnionParam
for _, t := range input.Tools {
var props any
var required []string
if p, ok := t.Function.Parameters.(map[string]interface{}); ok {
props = p["properties"]
if r, ok := p["required"].([]interface{}); ok {
for _, rv := range r {
if s, ok := rv.(string); ok {
required = append(required, s)
}
}
}
}
anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
OfTool: &anthropic.ToolParam{
Name:        t.Function.Name,
Description: anthropic.String(t.Function.Description),
InputSchema: anthropic.ToolInputSchemaParam{
Properties: props,
Required:   required,
},
},
})
}

params := anthropic.MessageNewParams{
Model:     anthropic.Model(model),
MaxTokens: maxTokens,
Messages:  messages,
}
if systemText != "" {
params.System = []anthropic.TextBlockParam{{Type: "text", Text: systemText}}
}
if len(anthropicTools) > 0 {
params.Tools = anthropicTools
}

resp, err := client.Messages.New(context.Background(), params)
if err != nil {
out := outputPayload{Error: err.Error()}
_ = pdk.OutputJSON(out)
return 1
}

out := outputPayload{StopReason: string(resp.StopReason)}
out.Usage.PromptTokens = int(resp.Usage.InputTokens)
out.Usage.CompletionTokens = int(resp.Usage.OutputTokens)

for _, block := range resp.Content {
switch block.Type {
case "text":
tb := block.AsText()
out.Content += tb.Text
case "tool_use":
tu := block.AsToolUse()
argsJSON, _ := json.Marshal(tu.Input)
out.ToolCalls = append(out.ToolCalls, toolCall{
ID:   tu.ID,
Type: "function",
Function: struct {
Name      string `json:"name"`
Arguments string `json:"arguments"`
}{Name: tu.Name, Arguments: string(argsJSON)},
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
