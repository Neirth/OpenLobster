// Copyright (c) OpenLobster contributors. See LICENSE for details.

package discord

import (
	"context"
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// parseChannelID — table-driven
// ---------------------------------------------------------------------------

func TestParseChannelID_TableDriven(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid snowflake", "123456789012345678", false},
		{"another valid", "987654321098765432", false},
		{"invalid text", "not-a-snowflake", true},
		{"empty string", "", true},
		{"zero string", "0", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseChannelID(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitMessageID — table-driven
// ---------------------------------------------------------------------------

func TestSplitMessageID_TableDriven(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantCh  string
		wantMsg string
	}{
		{"with separator", "111-222", "111", "222"},
		{"no separator", "single", "single", "single"},
		{"trailing dash", "abc-", "abc", ""},
		{"leading dash", "-xyz", "", "xyz"},
		{"multiple dashes", "a-b-c", "a-b", "c"}, // splits on last dash
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch, msg := splitMessageID(tc.input)
			assert.Equal(t, tc.wantCh, ch)
			assert.Equal(t, tc.wantMsg, msg)
		})
	}
}

// ---------------------------------------------------------------------------
// NewAdapter
// ---------------------------------------------------------------------------

func TestNewAdapter_CreatesState(t *testing.T) {
	a, err := NewAdapter("Bot test-token")
	require.NoError(t, err)
	require.NotNil(t, a)
	assert.NotNil(t, a.s)
}

// ---------------------------------------------------------------------------
// HandleWebhook — always nil, nil
// ---------------------------------------------------------------------------

func TestAdapter_HandleWebhook_NilPayload(t *testing.T) {
	a, _ := NewAdapter("t")
	msg, err := a.HandleWebhook(context.Background(), nil)
	assert.NoError(t, err)
	assert.Nil(t, msg)
}

func TestAdapter_HandleWebhook_NonNilPayload(t *testing.T) {
	a, _ := NewAdapter("t")
	msg, err := a.HandleWebhook(context.Background(), []byte(`{"event":"message"}`))
	assert.NoError(t, err)
	assert.Nil(t, msg)
}

// ---------------------------------------------------------------------------
// GetCapabilities
// ---------------------------------------------------------------------------

func TestAdapter_GetCapabilities_AllFlags(t *testing.T) {
	a, _ := NewAdapter("t")
	caps := a.GetCapabilities()
	assert.True(t, caps.HasVoiceMessage)
	assert.True(t, caps.HasCallStream)
	assert.True(t, caps.HasTextStream)
	assert.True(t, caps.HasMediaSupport)
}

// ---------------------------------------------------------------------------
// ConvertAudioForPlatform — passthrough
// ---------------------------------------------------------------------------

func TestAdapter_ConvertAudioForPlatform_PassThrough(t *testing.T) {
	a, _ := NewAdapter("t")
	data := []byte{1, 2, 3, 4}
	out, format, err := a.ConvertAudioForPlatform(context.Background(), data, "wav")
	assert.NoError(t, err)
	assert.Equal(t, data, out)
	assert.Equal(t, "ogg", format)
}

func TestAdapter_ConvertAudioForPlatform_EmptyData(t *testing.T) {
	a, _ := NewAdapter("t")
	out, format, err := a.ConvertAudioForPlatform(context.Background(), []byte{}, "ogg")
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, out)
	assert.Equal(t, "ogg", format)
}

// ---------------------------------------------------------------------------
// SendMessage — invalid channel ID path (no real gateway connection needed)
// ---------------------------------------------------------------------------

func TestAdapter_SendMessage_InvalidChannelID(t *testing.T) {
	a, _ := NewAdapter("t")
	msg := &models.Message{ChannelID: "INVALID", Content: "hello"}
	err := a.SendMessage(context.Background(), msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel id")
}

// ---------------------------------------------------------------------------
// SendMedia — invalid channel ID path
// ---------------------------------------------------------------------------

func TestAdapter_SendMedia_InvalidChannelID(t *testing.T) {
	a, _ := NewAdapter("t")
	media := &ports.Media{ChatID: "INVALID", URL: "https://example.com/img.png"}
	err := a.SendMedia(context.Background(), media)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel id")
}

// ---------------------------------------------------------------------------
// SendTyping — invalid channel ID returns nil (best-effort)
// ---------------------------------------------------------------------------

func TestAdapter_SendTyping_InvalidChannelID(t *testing.T) {
	a, _ := NewAdapter("t")
	// Invalid channel ID: best-effort, should not error.
	err := a.SendTyping(context.Background(), "INVALID")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// GetUserInfo — invalid snowflake falls back gracefully
// ---------------------------------------------------------------------------

func TestAdapter_GetUserInfo_InvalidSnowflake(t *testing.T) {
	a, _ := NewAdapter("t")
	info, err := a.GetUserInfo(context.Background(), "not-a-snowflake")
	// Should return a degraded UserInfo, not an error.
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "not-a-snowflake", info.ID)
	assert.Equal(t, "not-a-snowflake", info.Username)
}

// ---------------------------------------------------------------------------
// React — invalid channel/message IDs
// ---------------------------------------------------------------------------

func TestAdapter_React_InvalidChannelID(t *testing.T) {
	a, _ := NewAdapter("t")
	err := a.React(context.Background(), "INVALID-12345", "👍")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel id")
}

func TestAdapter_React_InvalidMessageID(t *testing.T) {
	a, _ := NewAdapter("t")
	// Valid channel snowflake but invalid message part.
	err := a.React(context.Background(), "123456789012345678-INVALID", "👍")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid message id")
}
