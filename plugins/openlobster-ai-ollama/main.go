package main

import (
"context"
"encoding/json"
"fmt"
"net/http"
"net/url"

pdk "github.com/extism/go-pdk"
ollamaAPI "github.com/ollama/ollama/api"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-ai-ollama"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
pdk.OutputString("Ollama local AI provider plugin for OpenLobster")
return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("ai"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
pdk.OutputString(`{"type":"object","properties":{"base_url":{"type":"string","title":"Base URL","default":"http://localhost:11434"},"default_model":{"type":"string","title":"Default Model","default":"llama3.2"}},"required":[]}`)
return 0
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type toolCall struct {
ID   string `json:"id"`
Type string `json:"type"`
Function struct {
Name      string `json:"name"`
Arguments string `json:"arguments"`
} `json:"function"`
}

type chatMessage struct {
Role      string     `json:"role"`
Content   string     `json:"content"`
ToolCalls []toolCall `json:"tool_calls,omitempty"`
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
BaseURL      string `json:"base_url"`
DefaultModel string `json:"default_model"`
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

baseURL := input.Config.BaseURL
if baseURL == "" {
baseURL = "http://localhost:11434"
}

model := input.Model
if model == "" {
model = input.Config.DefaultModel
}
if model == "" {
model = "llama3.2"
}

u, err := url.Parse(baseURL)
if err != nil {
pdk.SetError(fmt.Errorf("invalid base_url: %w", err))
return 1
}

client := ollamaAPI.NewClient(u, http.DefaultClient)

// Build ollama messages
var messages []ollamaAPI.Message
for _, m := range input.Messages {
messages = append(messages, ollamaAPI.Message{
Role:    m.Role,
Content: m.Content,
})
}

// Build ollama tools — use json round-trip to convert to the exact SDK type
var ollamaTools ollamaAPI.Tools
for _, t := range input.Tools {
toolJSON, _ := json.Marshal(map[string]interface{}{
"type": "function",
"function": map[string]interface{}{
"name":        t.Function.Name,
"description": t.Function.Description,
"parameters":  t.Function.Parameters,
},
})
var ot ollamaAPI.Tool
if err := json.Unmarshal(toolJSON, &ot); err == nil {
ollamaTools = append(ollamaTools, ot)
}
}

stream := false
req := &ollamaAPI.ChatRequest{
Model:    model,
Messages: messages,
Stream:   &stream,
Tools:    ollamaTools,
}
if input.MaxTokens > 0 {
req.Options = map[string]any{"num_predict": input.MaxTokens}
}

var resp ollamaAPI.ChatResponse
err = client.Chat(context.Background(), req, func(r ollamaAPI.ChatResponse) error {
resp = r
return nil
})
if err != nil {
out := outputPayload{Error: err.Error()}
_ = pdk.OutputJSON(out)
return 1
}

out := outputPayload{
Content:    resp.Message.Content,
StopReason: resp.DoneReason,
}
out.Usage.PromptTokens = resp.PromptEvalCount
out.Usage.CompletionTokens = resp.EvalCount

for i, tc := range resp.Message.ToolCalls {
argsJSON, _ := json.Marshal(tc.Function.Arguments)
out.ToolCalls = append(out.ToolCalls, toolCall{
ID:   fmt.Sprintf("call_%d", i),
Type: "function",
Function: struct {
Name      string `json:"name"`
Arguments string `json:"arguments"`
}{Name: tc.Function.Name, Arguments: string(argsJSON)},
})
}

if err := pdk.OutputJSON(out); err != nil {
pdk.SetError(err)
return 1
}
return 0
}

func main() {}
