package utils

import "encoding/json"

// PtrStr returns the string value of s, or "" if s is nil.
func PtrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// StringPtr returns a pointer to s.
func StringPtr(s string) *string {
	return &s
}

// ParseProperties parses a JSON string into map[string]string.
// Returns nil if s is nil, empty, or invalid JSON.
func ParseProperties(s *string) map[string]string {
	if s == nil || *s == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(*s), &m); err != nil {
		return nil
	}
	return m
}

// PropsToAny converts map[string]string to map[string]any.
func PropsToAny(m map[string]string) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
