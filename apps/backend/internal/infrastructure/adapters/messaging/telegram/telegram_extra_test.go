// Copyright (c) OpenLobster contributors. See LICENSE for details.

package telegram

import (
	"context"
	"encoding/json"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// GetCapabilities
// ---------------------------------------------------------------------------

func TestAdapter_GetCapabilities(t *testing.T) {
	a := &Adapter{}
	caps := a.GetCapabilities()
	assert.True(t, caps.HasVoiceMessage)
	assert.False(t, caps.HasCallStream)
	assert.True(t, caps.HasTextStream)
	assert.True(t, caps.HasMediaSupport)
}

// ---------------------------------------------------------------------------
// GetUserInfo — always returns stub
// ---------------------------------------------------------------------------

func TestAdapter_GetUserInfo(t *testing.T) {
	a := &Adapter{}
	info, err := a.GetUserInfo(context.Background(), "user123")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "user123", info.ID)
	assert.Equal(t, "user123", info.Username)
	assert.Equal(t, "user123", info.DisplayName)
}

// ---------------------------------------------------------------------------
// React — always no-op
// ---------------------------------------------------------------------------

func TestAdapter_React_NoOp(t *testing.T) {
	a := &Adapter{}
	err := a.React(context.Background(), "msg123", "👍")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// ConvertAudioForPlatform — passthrough
// ---------------------------------------------------------------------------

func TestAdapter_ConvertAudioForPlatform(t *testing.T) {
	a := &Adapter{}
	data := []byte{1, 2, 3}
	out, format, err := a.ConvertAudioForPlatform(context.Background(), data, "wav")
	assert.NoError(t, err)
	assert.Equal(t, data, out)
	assert.Equal(t, "ogg", format)
}

// ---------------------------------------------------------------------------
// HandleWebhook — full coverage of all paths
// ---------------------------------------------------------------------------

func TestAdapter_HandleWebhook_InvalidJSON(t *testing.T) {
	a := &Adapter{}
	_, err := a.HandleWebhook(context.Background(), []byte("NOT JSON"))
	assert.Error(t, err)
}

func TestAdapter_HandleWebhook_NilMessage(t *testing.T) {
	a := &Adapter{}
	// Valid JSON Update but Message is nil.
	payload, _ := json.Marshal(tgbotapi.Update{UpdateID: 1})
	msg, err := a.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	assert.Nil(t, msg)
}

func TestAdapter_HandleWebhook_DirectMessage(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "testbot"}

	update := tgbotapi.Update{
		UpdateID: 1,
		Message: &tgbotapi.Message{
			MessageID: 42,
			From:      &tgbotapi.User{ID: 123, UserName: "alice"},
			Chat:      &tgbotapi.Chat{ID: 123, Type: "private"},
			Text:      "hello bot",
		},
	}
	payload, _ := json.Marshal(update)
	msg, err := a.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "@alice", msg.SenderName)
	assert.Equal(t, "hello bot", msg.Content)
	assert.False(t, msg.IsGroup)
	assert.False(t, msg.IsMentioned)
}

func TestAdapter_HandleWebhook_GroupMessage_WithMention(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "testbot"}

	update := tgbotapi.Update{
		UpdateID: 2,
		Message: &tgbotapi.Message{
			MessageID: 55,
			From:      &tgbotapi.User{ID: 456, FirstName: "Bob", LastName: "Smith"},
			Chat:      &tgbotapi.Chat{ID: -100, Type: "group", Title: "Test Group"},
			Text:      "Hey @testbot, how are you?",
		},
	}
	payload, _ := json.Marshal(update)
	msg, err := a.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.True(t, msg.IsGroup)
	assert.True(t, msg.IsMentioned)
	assert.Equal(t, "Test Group", msg.GroupName)
	assert.Equal(t, "Bob Smith", msg.SenderName)
}

func TestAdapter_HandleWebhook_SenderNoUsername(t *testing.T) {
	a := &Adapter{}
	update := tgbotapi.Update{
		UpdateID: 3,
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 789, FirstName: "Carol", LastName: ""},
			Chat: &tgbotapi.Chat{ID: 789, Type: "private"},
			Text: "hi",
		},
	}
	payload, _ := json.Marshal(update)
	msg, err := a.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	assert.Equal(t, "Carol", msg.SenderName)
}

func TestAdapter_HandleWebhook_NoFrom(t *testing.T) {
	a := &Adapter{}
	update := tgbotapi.Update{
		UpdateID: 4,
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 111, Type: "private"},
			Text: "anonymous",
		},
	}
	payload, _ := json.Marshal(update)
	msg, err := a.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "", msg.SenderName)
	assert.Equal(t, "", msg.SenderID)
}

func TestAdapter_HandleWebhook_ChannelPost(t *testing.T) {
	a := &Adapter{}
	update := tgbotapi.Update{
		UpdateID: 5,
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 1, UserName: "chan"},
			Chat: &tgbotapi.Chat{ID: -200, Type: "channel", Title: "My Channel"},
			Text: "channel post",
		},
	}
	payload, _ := json.Marshal(update)
	msg, err := a.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.True(t, msg.IsGroup) // channels are IsGroup
	assert.Equal(t, "My Channel", msg.GroupName)
}

// ---------------------------------------------------------------------------
// isMentioned
// ---------------------------------------------------------------------------

func TestIsMentioned_ReplyToBotMessage(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "testbot"}
	msg := &tgbotapi.Message{
		ReplyToMessage: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 999},
		},
	}
	assert.True(t, a.isMentioned(msg))
}

func TestIsMentioned_TextMention(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "testbot"}
	msg := &tgbotapi.Message{
		Text: "Hey @testbot please help",
	}
	assert.True(t, a.isMentioned(msg))
}

func TestIsMentioned_CaptionMention(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "mybot"}
	msg := &tgbotapi.Message{
		Caption: "@mybot look at this",
	}
	assert.True(t, a.isMentioned(msg))
}

func TestIsMentioned_NoMention(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "testbot"}
	msg := &tgbotapi.Message{
		Text: "Hello everyone",
	}
	assert.False(t, a.isMentioned(msg))
}

func TestIsMentioned_EmptyUsername_NeverMentioned(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: ""}
	msg := &tgbotapi.Message{
		Text: "@whatever text",
	}
	assert.False(t, a.isMentioned(msg))
}

func TestIsMentioned_EntityMention(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "testbot"}
	msg := &tgbotapi.Message{
		Text: "@testbot go",
		Entities: []tgbotapi.MessageEntity{
			{Type: "mention", Offset: 0, Length: 8},
		},
	}
	assert.True(t, a.isMentioned(msg))
}

func TestIsMentioned_EntityOtherBot(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "mybot"}
	msg := &tgbotapi.Message{
		Text: "@otherbot go",
		Entities: []tgbotapi.MessageEntity{
			{Type: "mention", Offset: 0, Length: 9},
		},
	}
	assert.False(t, a.isMentioned(msg))
}

func TestIsMentioned_ReplyToNonBot(t *testing.T) {
	a := &Adapter{botUserID: 999, botUsername: "testbot"}
	msg := &tgbotapi.Message{
		ReplyToMessage: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 1234}, // not the bot
		},
	}
	assert.False(t, a.isMentioned(msg))
}

// ---------------------------------------------------------------------------
// buildAttachments — document with audio/video MIME type
// ---------------------------------------------------------------------------

func TestBuildAttachments_Document_AudioMIME(t *testing.T) {
	msg := &tgbotapi.Message{
		Document: &tgbotapi.Document{FileID: "doc100", MimeType: "audio/mp3"},
	}
	_, atts := buildAttachments(msg, noopResolve)
	require.Len(t, atts, 1)
	assert.Equal(t, "audio", atts[0].Type)
}

func TestBuildAttachments_Document_VideoMIME(t *testing.T) {
	msg := &tgbotapi.Message{
		Document: &tgbotapi.Document{FileID: "doc101", MimeType: "video/mp4"},
	}
	_, atts := buildAttachments(msg, noopResolve)
	require.Len(t, atts, 1)
	assert.Equal(t, "audio", atts[0].Type) // video is grouped under "audio" in the adapter
}

func TestBuildAttachments_AllMediaTypes(t *testing.T) {
	msg := &tgbotapi.Message{
		Photo: []tgbotapi.PhotoSize{{FileID: "ph", Width: 100, Height: 100}},
		Voice: &tgbotapi.Voice{FileID: "v", FileSize: 512},
	}
	_, atts := buildAttachments(msg, noopResolve)
	assert.Len(t, atts, 2)
}

// ---------------------------------------------------------------------------
// markdownToHTML — additional inline formatting
// ---------------------------------------------------------------------------

func TestMarkdownToHTML_Italic(t *testing.T) {
	result := markdownToHTML("*italic text*")
	assert.Contains(t, result, "<i>")
	assert.Contains(t, result, "italic text")
}

func TestMarkdownToHTML_Strikethrough(t *testing.T) {
	result := markdownToHTML("~~strike~~")
	assert.Contains(t, result, "<s>")
	assert.Contains(t, result, "strike")
}

func TestMarkdownToHTML_Heading(t *testing.T) {
	result := markdownToHTML("# Heading One")
	assert.Contains(t, result, "<b>Heading One</b>")
}

func TestMarkdownToHTML_HorizontalRule(t *testing.T) {
	result := markdownToHTML("---")
	// Horizontal rules are stripped.
	assert.NotContains(t, result, "---")
}

func TestMarkdownToHTML_HtmlSpecialChars(t *testing.T) {
	result := markdownToHTML("a & b < c > d")
	assert.Contains(t, result, "&amp;")
	assert.Contains(t, result, "&lt;")
	assert.Contains(t, result, "&gt;")
}

func TestMarkdownToHTML_BoldUnderscore(t *testing.T) {
	result := markdownToHTML("__bold__")
	assert.Contains(t, result, "<b>bold</b>")
}

func TestMarkdownToHTML_EmptyString(t *testing.T) {
	result := markdownToHTML("")
	assert.Equal(t, "", result)
}

func TestMarkdownToHTML_InlineCodeHtmlEscaped(t *testing.T) {
	result := markdownToHTML("`<html>`")
	assert.Contains(t, result, "&lt;html&gt;")
	assert.Contains(t, result, "<code>")
}

func TestMarkdownToHTML_FencedCodeBlockHtmlEscaped(t *testing.T) {
	result := markdownToHTML("```\n<html>\n```")
	assert.Contains(t, result, "&lt;html&gt;")
}

// ---------------------------------------------------------------------------
// htmlEscapeExceptPlaceholders — direct tests
// ---------------------------------------------------------------------------

func TestHtmlEscapeExceptPlaceholders_NoSpecialChars(t *testing.T) {
	result := htmlEscapeExceptPlaceholders("plain text")
	assert.Equal(t, "plain text", result)
}

func TestHtmlEscapeExceptPlaceholders_AmpersandLtGt(t *testing.T) {
	result := htmlEscapeExceptPlaceholders("&<>")
	assert.Equal(t, "&amp;&lt;&gt;", result)
}

func TestHtmlEscapeExceptPlaceholders_Placeholder(t *testing.T) {
	// Placeholder bytes should be passed through unchanged.
	s := "\x00CODE0\x00"
	result := htmlEscapeExceptPlaceholders(s)
	assert.Equal(t, s, result)
}

func TestHtmlEscapeExceptPlaceholders_UnclosedPlaceholder(t *testing.T) {
	// NUL byte without a closing NUL — should not panic and handle gracefully.
	s := "\x00unclosed"
	result := htmlEscapeExceptPlaceholders(s)
	assert.NotEmpty(t, result)
}
