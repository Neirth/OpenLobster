package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkdownToHTML_Plain(t *testing.T) {
	result := markdownToHTML("hello world")
	assert.Equal(t, "hello world", result)
}

func TestMarkdownToHTML_Bold(t *testing.T) {
	result := markdownToHTML("**bold**")
	assert.Contains(t, result, "<b>")
	assert.Contains(t, result, "bold")
}

func TestMarkdownToHTML_InlineCode(t *testing.T) {
	result := markdownToHTML("`code`")
	assert.Contains(t, result, "<code>")
	assert.Contains(t, result, "code")
}

func TestMarkdownToHTML_FencedCodeBlock(t *testing.T) {
	result := markdownToHTML("```\ncode block\n```")
	assert.Contains(t, result, "<pre>")
	assert.Contains(t, result, "code block")
}

func TestMarkdownToHTML_FencedCodeBlockWithLang(t *testing.T) {
	result := markdownToHTML("```go\npackage main\n```")
	assert.Contains(t, result, "language-go")
	assert.Contains(t, result, "package main")
}

func TestMarkdownToHTML_Link(t *testing.T) {
	result := markdownToHTML("[text](https://example.com)")
	assert.Contains(t, result, "<a href=")
	assert.Contains(t, result, "https://example.com")
}

// TestMarkdownToHTML_NestedBoldItalic ensures we don't produce invalid HTML nesting
// (e.g. <b>...<i>...</b>...</i>) which Telegram rejects with "expected </i>, found </b>".
func TestMarkdownToHTML_NestedBoldItalic(t *testing.T) {
	// Bold contains italic: **bold *italic* text** → <b>bold <i>italic</i> text</b>
	result := markdownToHTML("**bold *italic* text**")
	assert.Contains(t, result, "<b>bold <i>italic</i> text</b>")

	// Problematic case: **bold *italic** text* - italic must NOT match across </b>
	// (would produce <b>bold <i>italic</b> text</i>). With the fix, italic content
	// excludes < and >, so we get <b>bold *italic</b> <i>text</i> (valid nesting).
	result2 := markdownToHTML("**bold *italic** text*")
	assert.Contains(t, result2, "<b>")
	assert.Contains(t, result2, "</b>")
	// Must not contain invalid nesting: </b> between <i> and </i>
	assert.NotRegexp(t, `<i>[^<]*</b>`, result2)
}
