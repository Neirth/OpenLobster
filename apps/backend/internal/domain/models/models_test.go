package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage("ch1", "hello")
	assert.NotNil(t, msg)
	assert.Equal(t, "ch1", msg.ChannelID)
	assert.Equal(t, "hello", msg.Content)
	assert.NotEqual(t, uuid.Nil, msg.ID)
	assert.NotNil(t, msg.Metadata)
}

func TestMessage_SetReplyTo(t *testing.T) {
	msg := NewMessage("ch", "reply")
	id := uuid.New()
	msg.SetReplyTo(id)
	assert.True(t, msg.IsReply)
	assert.NotNil(t, msg.ReplyToID)
	assert.Equal(t, id, *msg.ReplyToID)
}

func TestMessage_IsSystemMessage(t *testing.T) {
	msg := NewMessage("ch", "x")
	assert.False(t, msg.IsSystemMessage())
	msg.Role = "system"
	assert.True(t, msg.IsSystemMessage())
}

func TestMessage_IsEmpty(t *testing.T) {
	msg := NewMessage("ch", "content")
	assert.False(t, msg.IsEmpty())
	msg.Content = ""
	assert.True(t, msg.IsEmpty())
}

func TestNewSession(t *testing.T) {
	s := NewSession("user1")
	assert.NotNil(t, s)
	assert.Equal(t, SessionTypeDM, s.Type)
	assert.Equal(t, "user1", s.UserID)
	assert.True(t, s.IsActive)
	assert.NotNil(t, s.Messages)
}

func TestNewGroupSession(t *testing.T) {
	groupID := uuid.New()
	s := NewGroupSession(ChannelDiscord, "ch1", groupID)
	assert.NotNil(t, s)
	assert.Equal(t, SessionTypeGroup, s.Type)
	assert.Equal(t, "ch1", s.ChannelID)
	assert.Equal(t, ChannelDiscord, s.ChannelType)
	assert.Equal(t, groupID, *s.GroupID)
}

func TestSession_AddMessage(t *testing.T) {
	s := NewSession("u1")
	msg := Message{Content: "hi"}
	s.AddMessage(msg)
	assert.Len(t, s.Messages, 1)
	assert.Equal(t, "hi", s.Messages[0].Content)
}

func TestSession_MarkInactive(t *testing.T) {
	s := NewSession("u1")
	assert.True(t, s.IsActive)
	s.MarkInactive()
	assert.False(t, s.IsActive)
}

func TestNewUser(t *testing.T) {
	u := NewUser("primary-1")
	assert.NotNil(t, u)
	assert.Equal(t, "primary-1", u.PrimaryID)
	assert.NotNil(t, u.Memory)
	assert.NotNil(t, u.Memory.Facts)
	assert.NotNil(t, u.Memory.Preferences)
}

func TestUser_AddChannel(t *testing.T) {
	u := NewUser("p1")
	ch := UserChannel{ChannelType: ChannelTelegram, Username: "alice"}
	u.AddChannel(ch)
	assert.Len(t, u.Channels, 1)
	assert.Equal(t, ChannelTelegram, u.Channels[0].ChannelType)
}

func TestUser_AddFact(t *testing.T) {
	u := NewUser("p1")
	u.AddFact("likes pizza", 0.9, "conversation")
	assert.Len(t, u.Memory.Facts, 1)
	assert.Equal(t, "likes pizza", u.Memory.Facts[0].Statement)
	assert.Equal(t, 0.9, u.Memory.Facts[0].Confidence)
}

func TestChannelType_Constants(t *testing.T) {
	assert.Equal(t, ChannelType("discord"), ChannelDiscord)
	assert.Equal(t, ChannelType("telegram"), ChannelTelegram)
	assert.Equal(t, ChannelType("loopback"), ChannelLoopback)
}

func TestChannelCapabilities_Supports(t *testing.T) {
	caps := ChannelCapabilities{HasVoiceMessage: true, HasTextStream: true}
	assert.True(t, caps.Supports("voice_message"))
	assert.True(t, caps.Supports("text_stream"))
	assert.False(t, caps.Supports("call_stream"))
	assert.False(t, caps.Supports("unknown"))
}

func TestTelegramCapabilities(t *testing.T) {
	assert.True(t, TelegramCapabilities.HasVoiceMessage)
	assert.True(t, TelegramCapabilities.HasTextStream)
}

func TestDiscordCapabilities(t *testing.T) {
	assert.True(t, DiscordCapabilities.HasCallStream)
	assert.True(t, DiscordCapabilities.HasMediaSupport)
}

func TestGetCapabilitiesForChannel(t *testing.T) {
	assert.Equal(t, DiscordCapabilities, GetCapabilitiesForChannel(ChannelDiscord))
	assert.Equal(t, WhatsAppCapabilities, GetCapabilitiesForChannel(ChannelWhatsApp))
	assert.Equal(t, TwilioCapabilities, GetCapabilitiesForChannel(ChannelTwilio))
	assert.Equal(t, SlackCapabilities, GetCapabilitiesForChannel(ChannelSlack))
	assert.Equal(t, TelegramCapabilities, GetCapabilitiesForChannel(ChannelTelegram))
	assert.Equal(t, TelegramCapabilities, GetCapabilitiesForChannel(ChannelLoopback))
}

func TestChannelCapabilities_Supports_Media(t *testing.T) {
	caps := ChannelCapabilities{HasMediaSupport: true}
	assert.True(t, caps.Supports("media"))
	caps.HasMediaSupport = false
	assert.False(t, caps.Supports("media"))
}
