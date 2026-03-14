// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: see LICENSE

package ollama

import (
	"encoding/base64"
	"encoding/json"
)

// ImageData holds raw image bytes. Marshals to a base64 JSON string for the Ollama API.
type ImageData []byte

func (d ImageData) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString(d))
}

// ToolCallFunctionArguments is a map of argument name to value.
type ToolCallFunctionArguments map[string]interface{}

// ToolCallFunction describes the function invoked in a tool call.
type ToolCallFunction struct {
	Index     int                      `json:"index,omitempty"`
	Name      string                   `json:"name"`
	Arguments ToolCallFunctionArguments `json:"arguments,omitempty"`
}

// ToolCall represents a single tool call from the model.
type ToolCall struct {
	ID       string          `json:"id,omitempty"`
	Function ToolCallFunction `json:"function"`
}

// Message is a chat message in the Ollama wire format.
type Message struct {
	Role       string      `json:"role"`
	Content    string      `json:"content"`
	Images     []ImageData `json:"images,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolName   string      `json:"tool_name,omitempty"`
}

// ChatRequest is the body sent to the Ollama /api/chat endpoint.
type ChatRequest struct {
	Model    string                 `json:"model"`
	Messages []Message              `json:"messages"`
	Tools    Tools                  `json:"tools,omitempty"`
	Stream   *bool                  `json:"stream,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

// ChatResponse is the body received from the Ollama /api/chat endpoint.
type ChatResponse struct {
	Model      string  `json:"model"`
	Message    Message `json:"message"`
	DoneReason string  `json:"done_reason"`
	Done       bool    `json:"done"`
}

// ToolFunctionParameters describes a tool function's JSON-schema parameters.
type ToolFunctionParameters struct {
	Type       string             `json:"type"`
	Required   []string           `json:"required,omitempty"`
	Properties *ToolPropertiesMap `json:"properties,omitempty"`
}

// ToolFunction describes the function part of a tool definition.
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  ToolFunctionParameters `json:"parameters"`
}

// Tool represents a callable tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// Tools is a slice of Tool.
type Tools []Tool

// PropertyType holds the JSON schema type of a tool property.
// It serializes as a bare string (e.g. "string") in the Ollama API.
type PropertyType struct {
	Type string
}

func (p PropertyType) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Type)
}

// ToolProperty describes a single property in a tool's parameter schema.
type ToolProperty struct {
	Type        PropertyType  `json:"type"`
	Description string        `json:"description,omitempty"`
	Enum        []interface{} `json:"enum,omitempty"`
}

// ToolPropertiesMap is an ordered map of property name → ToolProperty.
// It serializes as a plain JSON object.
type ToolPropertiesMap struct {
	keys   []string
	values map[string]ToolProperty
}

// NewToolPropertiesMap creates an empty ToolPropertiesMap.
func NewToolPropertiesMap() *ToolPropertiesMap {
	return &ToolPropertiesMap{values: make(map[string]ToolProperty)}
}

// Set adds or updates a property.
func (m *ToolPropertiesMap) Set(key string, value ToolProperty) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Get retrieves a property by key.
func (m *ToolPropertiesMap) Get(key string) (ToolProperty, bool) {
	v, ok := m.values[key]
	return v, ok
}

// MarshalJSON serializes the map as a plain JSON object.
func (m *ToolPropertiesMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	obj := make(map[string]ToolProperty, len(m.values))
	for k, v := range m.values {
		obj[k] = v
	}
	return json.Marshal(obj)
}
