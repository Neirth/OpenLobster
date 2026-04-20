package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewToolResultSanitizer(t *testing.T) {
	s := NewToolResultSanitizer()

	assert.NotNil(t, s)
	assert.Equal(t, 100000, s.MaxLength)
	assert.True(t, s.StripControlChars)
}

func TestToolResultSanitizer_sanitize(t *testing.T) {
	s := NewToolResultSanitizer()

	result := s.sanitize("Hello World")
	assert.Equal(t, "Hello World", result)
}

func TestToolResultSanitizer_sanitize_TruncatesLongContent(t *testing.T) {
	s := NewToolResultSanitizer()
	s.MaxLength = 30

	longContent := "This is a very long string that should be truncated"
	result := s.sanitize(longContent)

	assert.True(t, len(result) < len(longContent))
	assert.Contains(t, result, "[truncated]")
}

func TestToolResultSanitizer_sanitize_StripsControlChars(t *testing.T) {
	s := NewToolResultSanitizer()
	s.StripControlChars = true

	content := "Hello\x00World\x1FTest"
	result := s.sanitize(content)

	assert.NotContains(t, result, "\x00")
	assert.NotContains(t, result, "\x1F")
	assert.Equal(t, "HelloWorldTest", result)
}

func TestContainsNO_REPLY(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"Empty string", "", true},
		{"Whitespace only", "   ", true},
		{"NO_REPLY signal", "NO_REPLY", true},
		{"Normal text", "Hello world", false},
		{"NO_REPLY with whitespace", "  NO_REPLY  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsNO_REPLY(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNoReplySignal_Constant(t *testing.T) {
	assert.Equal(t, "NO_REPLY", NoReplySignal)
}
