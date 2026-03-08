package ports

import (
	"context"

	"github.com/neirth/openlobster/internal/domain/models"
)

type contextKey string

// ContextKeyChannelType is the context key for passing channel_type when
// routing React, GetUserInfo, ConvertAudioForPlatform through messagingRouter.
// Example: ctx = context.WithValue(ctx, ports.ContextKeyChannelType, "telegram")
var ContextKeyChannelType contextKey = "channel_type"

type ChannelCapabilities struct {
	HasVoiceMessage bool
	HasCallStream   bool
	HasTextStream   bool
	HasMediaSupport bool
}

type MessagingPort interface {
	SendMessage(ctx context.Context, msg *models.Message) error
	SendMedia(ctx context.Context, media *Media) error
	// SendTyping shows a typing indicator to the user. No-op if not supported.
	// Used before sending a delayed response to give feedback.
	SendTyping(ctx context.Context, channelID string) error
	HandleWebhook(ctx context.Context, payload []byte) (*models.Message, error)
	GetUserInfo(ctx context.Context, userID string) (*UserInfo, error)
	React(ctx context.Context, messageID string, emoji string) error
	GetCapabilities() ChannelCapabilities
	ConvertAudioForPlatform(ctx context.Context, audioData []byte, format string) ([]byte, string, error)
	// Start connects to the messaging platform and calls onMessage for every
	// incoming message. Adapters that use incoming webhooks (WhatsApp, Twilio)
	// implement this as a no-op. Blocking adapters must run their loop in a
	// goroutine and return immediately.
	Start(ctx context.Context, onMessage func(context.Context, *models.Message)) error
}

func GetCapabilitiesForType(channelType string) ChannelCapabilities {
	switch channelType {
	case "telegram":
		return ChannelCapabilities{
			HasVoiceMessage: true,
			HasCallStream:   false,
			HasTextStream:   true,
			HasMediaSupport: true,
		}
	case "discord":
		return ChannelCapabilities{
			HasVoiceMessage: true,
			HasCallStream:   true,
			HasTextStream:   true,
			HasMediaSupport: true,
		}
	case "whatsapp":
		return ChannelCapabilities{
			HasVoiceMessage: true,
			HasCallStream:   true,
			HasTextStream:   true,
			HasMediaSupport: true,
		}
	case "twilio":
		return ChannelCapabilities{
			HasVoiceMessage: true,
			HasCallStream:   true,
			HasTextStream:   true,
			HasMediaSupport: true,
		}
	default:
		return ChannelCapabilities{
			HasTextStream:   true,
			HasMediaSupport: false,
		}
	}
}

type Media struct {
	ChatID      string
	URL         string
	Caption     string
	FileName    string
	ContentType string
	// ChannelType is used by messagingRouter to route SendMedia to the correct
	// adapter. If empty, the router cannot route.
	ChannelType string
}

type UserInfo struct {
	ID          string
	Username    string
	DisplayName string
}

type VoicePort interface {
	AcceptCall(ctx context.Context, callID string) (*VoiceCall, error)
	EndCall(ctx context.Context, callID string) error
	StartStream(ctx context.Context, callID string) (*VoiceStream, error)
	Interrupt(ctx context.Context, callID string) error
	SendTone(ctx context.Context, callID string, tone ToneType) error
	SupportsVoiceCalls() bool
}

type VoiceCall struct {
	ID        string
	UserID    string
	Status    CallStatus
	StartTime int64
}

type CallStatus string

const (
	CallStatusRinging CallStatus = "ringing"
	CallStatusActive  CallStatus = "active"
	CallStatusOnHold  CallStatus = "on_hold"
	CallStatusEnded   CallStatus = "ended"
)

type VoiceStream struct {
	Input     <-chan AudioChunk
	Output    chan<- AudioChunk
	Interrupt chan struct{}
	Mute      chan bool
}

type AudioChunk struct {
	Data []byte
}

type ToneType string

const (
	ToneThinking ToneType = "thinking"
	ToneTools    ToneType = "tools"
	ToneEncoding ToneType = "encoding"
)
