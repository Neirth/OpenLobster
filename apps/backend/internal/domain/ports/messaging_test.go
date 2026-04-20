package ports

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCapabilitiesForType_Telegram(t *testing.T) {
	caps := GetCapabilitiesForType("telegram")
	assert.True(t, caps.HasVoiceMessage)
	assert.False(t, caps.HasCallStream)
	assert.True(t, caps.HasTextStream)
	assert.True(t, caps.HasMediaSupport)
}

func TestGetCapabilitiesForType_Discord(t *testing.T) {
	caps := GetCapabilitiesForType("discord")
	assert.True(t, caps.HasVoiceMessage)
	assert.True(t, caps.HasCallStream)
	assert.True(t, caps.HasTextStream)
	assert.True(t, caps.HasMediaSupport)
}

func TestGetCapabilitiesForType_WhatsApp(t *testing.T) {
	caps := GetCapabilitiesForType("whatsapp")
	assert.True(t, caps.HasVoiceMessage)
	assert.True(t, caps.HasCallStream)
	assert.True(t, caps.HasTextStream)
	assert.True(t, caps.HasMediaSupport)
}

func TestGetCapabilitiesForType_Twilio(t *testing.T) {
	caps := GetCapabilitiesForType("twilio")
	assert.True(t, caps.HasVoiceMessage)
	assert.True(t, caps.HasCallStream)
	assert.True(t, caps.HasTextStream)
	assert.True(t, caps.HasMediaSupport)
}

func TestGetCapabilitiesForType_Unknown(t *testing.T) {
	caps := GetCapabilitiesForType("unknown")
	assert.False(t, caps.HasVoiceMessage)
	assert.False(t, caps.HasCallStream)
	assert.True(t, caps.HasTextStream)
	assert.False(t, caps.HasMediaSupport)
}

func TestChannelCapabilities_Empty(t *testing.T) {
	caps := ChannelCapabilities{}
	assert.False(t, caps.HasVoiceMessage)
	assert.False(t, caps.HasCallStream)
	assert.False(t, caps.HasTextStream)
	assert.False(t, caps.HasMediaSupport)
}
