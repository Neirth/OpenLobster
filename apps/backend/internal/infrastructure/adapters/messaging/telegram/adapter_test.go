package telegram

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopResolve is a file resolver stub that returns nil bytes without making
// any network calls, suitable for unit tests.
func noopResolve(_ string) []byte {
	return nil
}

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

func TestBuildAttachments_PlainText(t *testing.T) {
	msg := &tgbotapi.Message{Text: "hello"}
	text, atts := buildAttachments(msg, noopResolve)
	assert.Equal(t, "hello", text)
	assert.Empty(t, atts)
}

func TestBuildAttachments_CaptionWithPhoto(t *testing.T) {
	msg := &tgbotapi.Message{
		Caption: "look at this",
		Photo: []tgbotapi.PhotoSize{
			{FileID: "small", Width: 100, Height: 100},
			{FileID: "large", Width: 800, Height: 600},
		},
	}
	text, atts := buildAttachments(msg, noopResolve)
	assert.Equal(t, "look at this", text)
	require.Len(t, atts, 1)
	assert.Equal(t, "image", atts[0].Type)
	assert.Equal(t, "image/jpeg", atts[0].MIMEType)
}

func TestBuildAttachments_Voice(t *testing.T) {
	msg := &tgbotapi.Message{
		Voice: &tgbotapi.Voice{FileID: "voice123", FileSize: 4096},
	}
	text, atts := buildAttachments(msg, noopResolve)
	assert.Equal(t, "", text)
	require.Len(t, atts, 1)
	assert.Equal(t, "audio", atts[0].Type)
	assert.Equal(t, "audio/ogg", atts[0].MIMEType)
	assert.Equal(t, int64(4096), atts[0].Size)
}

func TestBuildAttachments_Audio(t *testing.T) {
	msg := &tgbotapi.Message{
		Audio: &tgbotapi.Audio{FileID: "audio456", MimeType: "audio/mpeg", FileName: "song.mp3", FileSize: 1024},
	}
	_, atts := buildAttachments(msg, noopResolve)
	require.Len(t, atts, 1)
	assert.Equal(t, "audio", atts[0].Type)
	assert.Equal(t, "audio/mpeg", atts[0].MIMEType)
	assert.Equal(t, "song.mp3", atts[0].Filename)
}

func TestBuildAttachments_AudioDefaultMIME(t *testing.T) {
	msg := &tgbotapi.Message{
		Audio: &tgbotapi.Audio{FileID: "audio789", MimeType: ""},
	}
	_, atts := buildAttachments(msg, noopResolve)
	require.Len(t, atts, 1)
	assert.Equal(t, "audio/mpeg", atts[0].MIMEType)
}

func TestBuildAttachments_Document_PDF(t *testing.T) {
	msg := &tgbotapi.Message{
		Document: &tgbotapi.Document{FileID: "doc001", MimeType: "application/pdf", FileName: "report.pdf"},
	}
	_, atts := buildAttachments(msg, noopResolve)
	require.Len(t, atts, 1)
	assert.Equal(t, "document", atts[0].Type)
	assert.Equal(t, "application/pdf", atts[0].MIMEType)
	assert.Equal(t, "report.pdf", atts[0].Filename)
}

func TestBuildAttachments_Document_ImageMIME(t *testing.T) {
	msg := &tgbotapi.Message{
		Document: &tgbotapi.Document{FileID: "doc002", MimeType: "image/png", FileName: "photo.png"},
	}
	_, atts := buildAttachments(msg, noopResolve)
	require.Len(t, atts, 1)
	assert.Equal(t, "image", atts[0].Type)
}

func TestBuildAttachments_Document_DefaultMIME(t *testing.T) {
	msg := &tgbotapi.Message{
		Document: &tgbotapi.Document{FileID: "doc003", MimeType: ""},
	}
	_, atts := buildAttachments(msg, noopResolve)
	require.Len(t, atts, 1)
	assert.Equal(t, "application/octet-stream", atts[0].MIMEType)
}

func TestBuildAttachments_ResolverReturnsNilData(t *testing.T) {
	// If the resolver fails (empty fileID), the attachment still appears but Data is nil.
	msg := &tgbotapi.Message{
		Photo: []tgbotapi.PhotoSize{{FileID: ""}},
	}
	_, atts := buildAttachments(msg, noopResolve)
	require.Len(t, atts, 1)
	assert.Nil(t, atts[0].Data)
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
