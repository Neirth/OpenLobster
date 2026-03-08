package discord

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAdapter(t *testing.T) {
	a, err := NewAdapter("test-token")
	assert.NoError(t, err)
	assert.NotNil(t, a)
}

func TestParseChannelID(t *testing.T) {
	// Valid snowflake
	id, err := parseChannelID("123456789012345678")
	assert.NoError(t, err)
	assert.NotZero(t, id)
}

func TestParseChannelID_Invalid(t *testing.T) {
	_, err := parseChannelID("invalid")
	assert.Error(t, err)
}

func TestSplitMessageID(t *testing.T) {
	ch, msg := splitMessageID("111-222")
	assert.Equal(t, "111", ch)
	assert.Equal(t, "222", msg)
}

func TestSplitMessageID_NoSeparator(t *testing.T) {
	ch, msg := splitMessageID("single")
	assert.Equal(t, "single", ch)
	assert.Equal(t, "single", msg)
}

func TestAdapter_HandleWebhook(t *testing.T) {
	a, _ := NewAdapter("x")
	msg, err := a.HandleWebhook(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, msg)
}

func TestAdapter_GetCapabilities(t *testing.T) {
	a, _ := NewAdapter("x")
	caps := a.GetCapabilities()
	assert.True(t, caps.HasTextStream)
	assert.True(t, caps.HasVoiceMessage)
	assert.True(t, caps.HasMediaSupport)
}

func TestAdapter_ConvertAudioForPlatform(t *testing.T) {
	a, _ := NewAdapter("x")
	data := []byte{1, 2, 3}
	out, fmt, err := a.ConvertAudioForPlatform(nil, data, "raw")
	assert.NoError(t, err)
	assert.Equal(t, data, out)
	assert.Equal(t, "ogg", fmt)
}
