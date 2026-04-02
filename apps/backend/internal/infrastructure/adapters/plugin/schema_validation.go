package plugin

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type schemaFragment struct {
	Type       string                    `json:"type"`
	Required   []string                  `json:"required"`
	Properties map[string]schemaProperty `json:"properties"`
}

type schemaProperty struct {
	Type string `json:"type"`
}

// ValidateConfigSchema validates config using a plugin-provided JSON schema
// fragment (type/properties/required).
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
	for _, req := range schema.Required {
		v, ok := cfg[req]
		if !ok {
			return fmt.Errorf("missing required property %q", req)
		}
		if s, ok := v.(string); ok && s == "" {
			return fmt.Errorf("required property %q is empty", req)
		}
	}
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

