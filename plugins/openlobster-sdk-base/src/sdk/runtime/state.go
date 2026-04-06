// Copyright (c) OpenLobster contributors. See LICENSE for details.

package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// Function is a no-arg plugin export compatible with the previous go-pdk style.
type Function func() int32

// Plugin defines a native plugin and its exported function handlers.
type Plugin struct {
	ID       string
	Metadata Metadata
	Exports  map[string]Function
}

// Metadata describes plugin identity and contract surfaced to the core.
type Metadata struct {
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name,omitempty"`
	Version     string          `json:"version,omitempty"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Properties  json.RawMessage `json:"properties,omitempty"`
}

// MetadataExport returns a standard metadata export function.
func MetadataExport(meta Metadata) Function {
	return func() int32 {
		if err := OutputJSON(meta.normalize("")); err != nil {
			SetError(err)
			return 1
		}
		return 0
	}
}

func parseMetadataJSON(raw []byte) (Metadata, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return Metadata{}, fmt.Errorf("runtime: empty metadata payload")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return Metadata{}, fmt.Errorf("runtime: decode metadata: %w", err)
	}

	var out Metadata
	out.ID = decodeMetadataString(fields["id"])
	out.Name = decodeMetadataString(fields["name"])
	out.Version = decodeMetadataString(fields["version"])
	out.Description = decodeMetadataString(fields["description"])
	out.Type = decodeMetadataString(fields["type"])
	out.Schema = decodeMetadataSchema(fields["schema"])
	out.Properties = decodeMetadataProperties(fields["properties"])

	return out, nil
}

func decodeMetadataString(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func decodeMetadataSchema(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}

	if trimmed[0] == '"' {
		var schemaString string
		if err := json.Unmarshal(trimmed, &schemaString); err != nil {
			return nil
		}
		trimmed = bytes.TrimSpace([]byte(schemaString))
		if len(trimmed) == 0 {
			return nil
		}
	}

	return append(json.RawMessage(nil), trimmed...)
}

func decodeMetadataProperties(raw json.RawMessage) json.RawMessage {
	trimmed := decodeMetadataSchema(raw)
	if len(trimmed) == 0 {
		return nil
	}

	var obj map[string]any
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil
	}
	normalized, err := json.Marshal(obj)
	if err != nil {
		return nil
	}

	return append(json.RawMessage(nil), normalized...)
}

func normalizeSchema(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage("{}")
	}
	if !json.Valid(trimmed) {
		return json.RawMessage("{}")
	}
	return append(json.RawMessage(nil), trimmed...)
}

func normalizeProperties(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage("{}")
	}
	if json.Valid(trimmed) == false {
		return json.RawMessage("{}")
	}

	var obj map[string]any
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return json.RawMessage("{}")
	}
	normalized, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage("{}")
	}

	return append(json.RawMessage(nil), normalized...)
}

func (m Metadata) normalize(defaultID string) Metadata {
	out := Metadata{
		ID:          strings.TrimSpace(m.ID),
		Name:        strings.TrimSpace(m.Name),
		Version:     strings.TrimSpace(m.Version),
		Description: strings.TrimSpace(m.Description),
		Type:        strings.TrimSpace(m.Type),
		Schema:      normalizeSchema(m.Schema),
		Properties:  normalizeProperties(m.Properties),
	}

	if out.ID == "" {
		out.ID = strings.TrimSpace(defaultID)
	}
	if out.ID == "" {
		out.ID = "unknown-plugin"
	}
	if out.Name == "" {
		out.Name = out.ID
	}

	return out
}

func coalesceMetadata(primary, fallback Metadata) Metadata {
	out := fallback
	if strings.TrimSpace(primary.ID) != "" {
		out.ID = strings.TrimSpace(primary.ID)
	}
	if strings.TrimSpace(primary.Name) != "" {
		out.Name = strings.TrimSpace(primary.Name)
	}
	if strings.TrimSpace(primary.Version) != "" {
		out.Version = strings.TrimSpace(primary.Version)
	}
	if strings.TrimSpace(primary.Description) != "" {
		out.Description = strings.TrimSpace(primary.Description)
	}
	if strings.TrimSpace(primary.Type) != "" {
		out.Type = strings.TrimSpace(primary.Type)
	}
	if len(bytes.TrimSpace(primary.Schema)) != 0 {
		out.Schema = append(json.RawMessage(nil), bytes.TrimSpace(primary.Schema)...)
	}
	if len(bytes.TrimSpace(primary.Properties)) != 0 {
		out.Properties = append(json.RawMessage(nil), bytes.TrimSpace(primary.Properties)...)
	}
	return out
}

// LogLevel values used by Log.
type LogLevel string

const (
	LogInfo  LogLevel = "info"
	LogError LogLevel = "error"
)

type allocatedBytes struct {
	offset uint64
}

// Offset returns the in-call allocation identifier.
func (a allocatedBytes) Offset() uint64 {
	return a.offset
}

type callScope struct {
	input    []byte
	output   []byte
	callErr  error
	emitFn   func([]byte)
	logFn    func(level, message string)
	nextOff  uint64
	allocMap map[uint64][]byte
}

var (
	scopeMu sync.Mutex
	scope   *callScope
)

func currentScope() (*callScope, error) {
	if scope == nil {
		return nil, fmt.Errorf("runtime: no active call scope")
	}
	return scope, nil
}

// InputJSON decodes the current function input into v.
func InputJSON(v any) error {
	s, err := currentScope()
	if err != nil {
		return err
	}
	if len(s.input) == 0 {
		return io.EOF
	}
	return json.Unmarshal(s.input, v)
}

// OutputString sets the current function output as a plain UTF-8 string.
func OutputString(v string) {
	s, err := currentScope()
	if err != nil {
		return
	}
	s.output = []byte(v)
}

// OutputJSON sets the current function output as marshaled JSON bytes.
func OutputJSON(v any) error {
	s, err := currentScope()
	if err != nil {
		return err
	}
	b, marshalErr := json.Marshal(v)
	if marshalErr != nil {
		return marshalErr
	}
	s.output = b
	return nil
}

// SetError marks the current function call as failed.
func SetError(err error) {
	s, scopeErr := currentScope()
	if scopeErr != nil {
		return
	}
	s.callErr = err
}

// AllocateBytes stores bytes in call-local memory and returns a handle.
// This is a compatibility helper for previous offset-based emit helpers.
func AllocateBytes(data []byte) allocatedBytes {
	s, err := currentScope()
	if err != nil {
		return allocatedBytes{}
	}
	if s.allocMap == nil {
		s.allocMap = make(map[uint64][]byte)
	}
	s.nextOff++
	copyData := append([]byte(nil), data...)
	s.allocMap[s.nextOff] = copyData
	return allocatedBytes{offset: s.nextOff}
}

// EmitAllocated emits bytes previously stored with AllocateBytes.
func EmitAllocated(offset uint64) {
	s, err := currentScope()
	if err != nil || s.emitFn == nil {
		return
	}
	if payload, ok := s.allocMap[offset]; ok {
		s.emitFn(append([]byte(nil), payload...))
	}
}

// EmitMessage marshals v as JSON and publishes it as an emit_message event.
func EmitMessage(v any) {
	s, err := currentScope()
	if err != nil || s.emitFn == nil {
		return
	}
	b, marshalErr := json.Marshal(v)
	if marshalErr != nil {
		return
	}
	s.emitFn(b)
}

// Log emits plugin log events to the host stream.
func Log(level LogLevel, message string) {
	s, err := currentScope()
	if err != nil || s.logFn == nil {
		return
	}
	lvl := strings.TrimSpace(strings.ToLower(string(level)))
	if lvl == "" {
		lvl = string(LogInfo)
	}
	s.logFn(lvl, strings.TrimSpace(message))
}

func invokeWithScope(input []byte, emitFn func([]byte), logFn func(level, message string), fn Function) ([]byte, error) {
	if fn == nil {
		return nil, fmt.Errorf("runtime: nil function")
	}

	scopeMu.Lock()
	defer scopeMu.Unlock()

	s := &callScope{
		input:    append([]byte(nil), input...),
		emitFn:   emitFn,
		logFn:    logFn,
		nextOff:  0,
		allocMap: make(map[uint64][]byte),
	}
	scope = s
	defer func() { scope = nil }()

	code := fn()
	if s.callErr != nil {
		return nil, s.callErr
	}
	if code != 0 {
		return nil, fmt.Errorf("runtime: export returned non-zero status: %d", code)
	}
	return append([]byte(nil), s.output...), nil
}
