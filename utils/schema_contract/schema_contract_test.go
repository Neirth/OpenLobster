// Copyright (c) OpenLobster contributors. See LICENSE for details.

package schemacontract

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ValidateSchemaStructure tests
// These mirror the Rust SDK's validation.rs test suite to ensure both
// implementations stay in sync on the structural rules.
// ---------------------------------------------------------------------------

func TestValidateSchemaStructure_NullOrEmpty(t *testing.T) {
	if err := ValidateSchemaStructure(nil); err != nil {
		t.Fatalf("nil schema: unexpected error: %v", err)
	}
	if err := ValidateSchemaStructure([]byte("")); err != nil {
		t.Fatalf("empty schema: unexpected error: %v", err)
	}
}

func TestValidateSchemaStructure_EmptyObject(t *testing.T) {
	if err := ValidateSchemaStructure([]byte(`{}`)); err != nil {
		t.Fatalf("empty object: unexpected error: %v", err)
	}
}

func TestValidateSchemaStructure_ObjectTypeIsValid(t *testing.T) {
	if err := ValidateSchemaStructure([]byte(`{"type":"object"}`)); err != nil {
		t.Fatalf(`{"type":"object"}: unexpected error: %v`, err)
	}
}

func TestValidateSchemaStructure_NonObjectRootTypeRejected(t *testing.T) {
	err := ValidateSchemaStructure([]byte(`{"type":"string"}`))
	if err == nil {
		t.Fatal("expected error for non-object root type")
	}
}

func TestValidateSchemaStructure_RequiredMissingFromProperties(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {"api_key": {"type": "string"}},
		"required": ["api_key", "missing_field"]
	}`)
	err := ValidateSchemaStructure(schema)
	if err == nil {
		t.Fatal("expected error for required field missing from properties")
	}
}

func TestValidateSchemaStructure_ValidFullSchema(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"api_key":    {"type": "string"},
			"max_tokens": {"type": "integer"},
			"enabled":    {"type": "boolean"}
		},
		"required": ["api_key"]
	}`)
	if err := ValidateSchemaStructure(schema); err != nil {
		t.Fatalf("full valid schema: unexpected error: %v", err)
	}
}

func TestValidateSchemaStructure_InvalidPropertyType(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {"count": {"type": "float"}}
	}`)
	err := ValidateSchemaStructure(schema)
	if err == nil {
		t.Fatal("expected error for invalid property type 'float'")
	}
}

func TestValidateSchemaStructure_AllValidPrimitiveTypes(t *testing.T) {
	for typ := range ValidSchemaTypes {
		schema := []byte(`{"type":"object","properties":{"x":{"type":"` + typ + `"}}}`)
		if err := ValidateSchemaStructure(schema); err != nil {
			t.Fatalf("type %q should be valid: %v", typ, err)
		}
	}
}

func TestValidateSchemaStructure_NonObjectJSON(t *testing.T) {
	err := ValidateSchemaStructure([]byte(`"not an object"`))
	if err == nil {
		t.Fatal("expected error for non-object JSON schema")
	}
}

// ---------------------------------------------------------------------------
// ValidateConfigSchema tests
// These mirror the backend's abi/schema_validation tests.
// ---------------------------------------------------------------------------

func TestValidateConfigSchema_EmptySchema(t *testing.T) {
	if err := ValidateConfigSchema(nil, map[string]interface{}{"key": "val"}); err != nil {
		t.Fatalf("nil schema: unexpected error: %v", err)
	}
	if err := ValidateConfigSchema([]byte(""), map[string]interface{}{}); err != nil {
		t.Fatalf("empty schema: unexpected error: %v", err)
	}
}

func TestValidateConfigSchema_MissingRequired(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"api_key":{"type":"string"}},"required":["api_key"]}`)
	err := ValidateConfigSchema(schema, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing required property")
	}
}

func TestValidateConfigSchema_EmptyRequiredString(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"api_key":{"type":"string"}},"required":["api_key"]}`)
	err := ValidateConfigSchema(schema, map[string]interface{}{"api_key": ""})
	if err == nil {
		t.Fatal("expected error for empty required string property")
	}
}

func TestValidateConfigSchema_ValidConfig(t *testing.T) {
	schema := []byte(`{
		"type": "object",
		"properties": {
			"api_key": {"type": "string"},
			"enabled": {"type": "boolean"}
		},
		"required": ["api_key"]
	}`)
	cfg := map[string]interface{}{
		"api_key": "sk-test",
		"enabled": true,
	}
	if err := ValidateConfigSchema(schema, cfg); err != nil {
		t.Fatalf("valid config: unexpected error: %v", err)
	}
}

func TestValidateConfigSchema_TypeMismatch(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"count":{"type":"integer"}}}`)
	cfg := map[string]interface{}{"count": "not-an-integer"}
	err := ValidateConfigSchema(schema, cfg)
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
}

func TestValidateConfigSchema_AbsentOptionalPropertySkipped(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"count":{"type":"integer"}}}`)
	// count is not required and not present — must pass.
	if err := ValidateConfigSchema(schema, map[string]interface{}{}); err != nil {
		t.Fatalf("absent optional: unexpected error: %v", err)
	}
}
