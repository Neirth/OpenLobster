// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package schemacontract defines the canonical rules for validating OpenLobster
// plugin JSON schemas and runtime configurations.
//
// This is the single source of truth for schema validation shared across:
//   - apps/backend (Go host-side runtime validation)
//   - utils/validation_layer (standalone smoke-test validator)
//
// The Rust SDK (plugins/openlobster-sdk-base/src/validation.rs) implements the
// same contract independently; see that file for the cross-language mirror.
package schemacontract

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ValidSchemaTypes are the JSON Schema primitive types accepted by the
// OpenLobster plugin protocol.
var ValidSchemaTypes = map[string]struct{}{
	"string":  {},
	"number":  {},
	"integer": {},
	"boolean": {},
	"object":  {},
	"array":   {},
}

// schemaFragment is used to unmarshal the subset of JSON Schema fields
// relevant to plugin configuration schemas.
type schemaFragment struct {
	Type       string                    `json:"type"`
	Required   []string                  `json:"required"`
	Properties map[string]schemaProperty `json:"properties"`
}

type schemaProperty struct {
	Type string `json:"type"`
}

// ValidateSchemaStructure validates the structural validity of a plugin-provided
// JSON Schema fragment (the value of PluginInfo.schema).
//
// Rules:
//  1. Root type, if present, must be "object".
//  2. Every entry in required[] must appear as a key in properties{}.
//  3. Every property that declares a type must use a valid JSON Schema type.
//
// Returns nil when the schema is valid, or an error describing the first
// violation found. An empty or nil schemaJSON is considered valid (treated as
// an empty object schema).
func ValidateSchemaStructure(schemaJSON []byte) error {
	if len(schemaJSON) == 0 {
		return nil
	}

	var schema schemaFragment
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return fmt.Errorf("root must be a JSON object")
	}

	// Rule 1: root type, if present, must be "object".
	if schema.Type != "" && schema.Type != "object" {
		return fmt.Errorf("root type must be \"object\", got %q", schema.Type)
	}

	// Rule 2: every required field must exist in properties.
	for _, field := range schema.Required {
		if _, ok := schema.Properties[field]; !ok {
			return fmt.Errorf("required field %q is not declared in properties", field)
		}
	}

	// Rule 3: validate property types.
	for key, prop := range schema.Properties {
		if prop.Type == "" {
			continue
		}
		if _, ok := ValidSchemaTypes[prop.Type]; !ok {
			return fmt.Errorf("property %q has invalid type %q", key, prop.Type)
		}
	}

	return nil
}

// ValidateConfigSchema validates a runtime configuration map against a
// plugin-provided JSON Schema fragment (type/properties/required).
//
// In addition to structural validation it also checks that:
//   - All required properties are present in cfg.
//   - Required string properties are not empty.
//   - Present properties whose schema declares a type have a matching Go type.
//
// An empty schemaJSON is considered valid (no constraints on cfg).
func ValidateConfigSchema(schemaJSON []byte, cfg map[string]interface{}) error {
	if len(schemaJSON) == 0 {
		return nil
	}

	var schema schemaFragment
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return fmt.Errorf("invalid plugin schema json: %w", err)
	}

	if schema.Type != "" && schema.Type != "object" {
		return fmt.Errorf("unsupported root schema type %q", schema.Type)
	}

	// Check required fields.
	for _, req := range schema.Required {
		v, ok := cfg[req]
		if !ok {
			return fmt.Errorf("missing required property %q", req)
		}
		if s, ok := v.(string); ok && s == "" {
			return fmt.Errorf("required property %q is empty", req)
		}
	}

	// Validate types of present properties.
	for key, prop := range schema.Properties {
		v, ok := cfg[key]
		if !ok || prop.Type == "" {
			continue
		}
		if !matchesJSONType(v, prop.Type) {
			return fmt.Errorf("property %q expected type %q", key, prop.Type)
		}
	}

	return nil
}

// matchesJSONType reports whether the Go value v matches the given JSON Schema
// primitive type string.
func matchesJSONType(v interface{}, typ string) bool {
	switch typ {
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		switch v.(type) {
		case float64, float32, int, int32, int64, uint, uint32, uint64:
			return true
		default:
			return false
		}
	case "integer":
		switch v.(type) {
		case int, int32, int64, uint, uint32, uint64:
			return true
		default:
			return false
		}
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "object":
		_, ok := v.(map[string]interface{})
		return ok
	case "array":
		return reflect.TypeOf(v) != nil && reflect.TypeOf(v).Kind() == reflect.Slice
	default:
		return true
	}
}
