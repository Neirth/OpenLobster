package models

type ChannelCapabilities struct {
	HasVoiceMessage  bool
	HasCallStream    bool
	HasTextStream    bool
	HasMediaSupport  bool
	MaxMessageSize   int64
	SupportedFormats []string
}

func (c *ChannelCapabilities) Supports(capability string) bool {
	switch capability {
	case "voice_message":
		return c.HasVoiceMessage
	case "call_stream":
		return c.HasCallStream
	case "text_stream":
		return c.HasTextStream
	case "media":
		return c.HasMediaSupport
	}
	return false
}

type ChannelConfig struct {
	Type         ChannelType         `json:"type"`
	Enabled      bool                `json:"enabled"`
	ChannelID    string              `json:"channel_id"`
	Capabilities ChannelCapabilities `json:"capabilities"`
}

var TelegramCapabilities = ChannelCapabilities{
	HasVoiceMessage:  true,
	HasCallStream:    false,
	HasTextStream:    true,
	HasMediaSupport:  true,
	MaxMessageSize:   4096,
	SupportedFormats: []string{"text", "photo", "video", "audio", "document", "sticker"},
}

var DiscordCapabilities = ChannelCapabilities{
	HasVoiceMessage:  true,
	HasCallStream:    true,
	HasTextStream:    true,
	HasMediaSupport:  true,
	MaxMessageSize:   2000,
	SupportedFormats: []string{"text", "image", "video", "audio", "file", "embed"},
}

var WhatsAppCapabilities = ChannelCapabilities{
	HasVoiceMessage:  true,
	HasCallStream:    true,
	HasTextStream:    true,
	HasMediaSupport:  true,
	MaxMessageSize:   4096,
	SupportedFormats: []string{"text", "image", "video", "audio", "document", "voice"},
}

var TwilioCapabilities = ChannelCapabilities{
	HasVoiceMessage:  true,
	HasCallStream:    true,
	HasTextStream:    true,
	HasMediaSupport:  true,
	MaxMessageSize:   1600,
	SupportedFormats: []string{"text", "audio", "mms"},
}

var SlackCapabilities = ChannelCapabilities{
	HasVoiceMessage:  true,
	HasCallStream:    false,
	HasTextStream:    true,
	HasMediaSupport:  true,
	MaxMessageSize:   40000,
	SupportedFormats: []string{"text", "image", "video", "audio", "file"},
}

func GetCapabilitiesForChannel(channelType ChannelType) ChannelCapabilities {
	switch channelType {
	case ChannelDiscord:
		return DiscordCapabilities
	case ChannelWhatsApp:
		return WhatsAppCapabilities
	case ChannelTwilio:
		return TwilioCapabilities
	case ChannelSlack:
		return SlackCapabilities
	default:
		return TelegramCapabilities
	}
}

type ChannelMessage struct {
	ID           string                 `json:"id"`
	ChannelID    string                 `json:"channel_id"`
	Content      string                 `json:"content"`
	Timestamp    int64                  `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ChannelType  ChannelType            `json:"channel_type"`
	Capabilities ChannelCapabilities    `json:"-"`
}

type MediaAttachment struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

type VoiceStream struct {
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
	BitDepth   int    `json:"bit_depth"`
	Data       []byte `json:"data"`
}
