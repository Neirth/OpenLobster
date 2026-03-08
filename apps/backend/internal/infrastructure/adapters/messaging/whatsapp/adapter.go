// Copyright (C) 2024 OpenLobster contributors
// SPDX-License-Identifier: see LICENSE
package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/google/uuid"

	wapi "github.com/wapikit/wapi.go/pkg/client"
	"github.com/wapikit/wapi.go/pkg/components"
	"github.com/wapikit/wapi.go/pkg/messaging"
)

// Adapter implements ports.MessagingPort for the WhatsApp Cloud API.
type Adapter struct {
	messagingClient *messaging.MessagingClient
	phoneID         string
}

// NewAdapter creates a new WhatsApp adapter backed by the official wapi.go SDK.
func NewAdapter(phoneID, apiToken string) (*Adapter, error) {
	wapiClient := wapi.New(&wapi.ClientConfig{
		ApiAccessToken: apiToken,
		WebhookSecret:  "placeholder",
	})
	mc := wapiClient.NewMessagingClient(phoneID)
	return &Adapter{
		messagingClient: mc,
		phoneID:         phoneID,
	}, nil
}

// SendTyping is a no-op for WhatsApp (typing not implemented).
func (a *Adapter) SendTyping(_ context.Context, _ string) error { return nil }

// SendMessage sends a plain text message via the WhatsApp Cloud API.
func (a *Adapter) SendMessage(ctx context.Context, msg *models.Message) error {
	textMsg, err := components.NewTextMessage(components.TextMessageConfigs{
		Text: msg.Content,
	})
	if err != nil {
		return fmt.Errorf("whatsapp build text message: %w", err)
	}
	_, err = a.messagingClient.Message.Send(textMsg, msg.ChannelID)
	if err != nil {
		return fmt.Errorf("whatsapp send message: %w", err)
	}
	return nil
}

// SendMedia sends an image or document message via the WhatsApp Cloud API.
func (a *Adapter) SendMedia(ctx context.Context, media *ports.Media) error {
	imgMsg, err := components.NewImageMessage(components.ImageMessageConfigs{
		Link:    media.URL,
		Caption: media.Caption,
	})
	if err != nil {
		return fmt.Errorf("whatsapp build image message: %w", err)
	}
	_, err = a.messagingClient.Message.Send(imgMsg, media.ChatID)
	if err != nil {
		return fmt.Errorf("whatsapp send media: %w", err)
	}
	return nil
}

func (a *Adapter) HandleWebhook(ctx context.Context, payload []byte) (*models.Message, error) {
	var webhook WhatsAppWebhook
	if err := json.Unmarshal(payload, &webhook); err != nil {
		return nil, err
	}

	if len(webhook.Entry) == 0 || len(webhook.Entry[0].Changes) == 0 {
		return nil, nil
	}

	change := webhook.Entry[0].Changes[0]
	if len(change.Value.Messages) == 0 {
		return nil, nil
	}

	wam := change.Value.Messages[0]

	msg := &models.Message{
		ID:        uuid.New(),
		ChannelID: wam.From,
		Timestamp: time.Now(),
	}

	if wam.Text != nil {
		msg.Content = wam.Text.Body
	} else if wam.Image != nil {
		msg.Content = wam.Image.Caption
		if msg.Content == "" {
			msg.Content = "[Image]"
		}
	} else if wam.Document != nil {
		msg.Content = wam.Document.Caption
		if msg.Content == "" {
			msg.Content = "[Document: " + wam.Document.Filename + "]"
		}
	} else if wam.Audio != nil {
		msg.Content = "[Audio]"
	} else if wam.Video != nil {
		msg.Content = wam.Video.Caption
		if msg.Content == "" {
			msg.Content = "[Video]"
		}
	} else if wam.Location != nil {
		msg.Content = "[Location]"
	}

	return msg, nil
}

func (a *Adapter) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	return &ports.UserInfo{
		ID:          userID,
		Username:    userID,
		DisplayName: userID,
	}, nil
}

func (a *Adapter) React(ctx context.Context, messageID string, emoji string) error {
	return nil
}

type WhatsAppWebhook struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Text      *struct {
						Body string `json:"body"`
					} `json:"text,omitempty"`
					Image *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						URL      string `json:"url"`
						Caption  string `json:"caption,omitempty"`
					} `json:"image,omitempty"`
					Document *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						URL      string `json:"url"`
						Filename string `json:"filename"`
						Caption  string `json:"caption,omitempty"`
					} `json:"document,omitempty"`
					Audio *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						URL      string `json:"url"`
					} `json:"audio,omitempty"`
					Video *struct {
						ID       string `json:"id"`
						MimeType string `json:"mime_type"`
						URL      string `json:"url"`
						Caption  string `json:"caption,omitempty"`
					} `json:"video,omitempty"`
					Location *struct {
						Latitude  float64 `json:"latitude"`
						Longitude float64 `json:"longitude"`
						Name      string  `json:"name,omitempty"`
					} `json:"location,omitempty"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

func ParsePhone(phone string) string {
	result := ""
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			result += string(c)
		}
	}
	if len(result) > 0 && result[0] == '0' {
		result = result[1:]
	}
	return result
}

func FormatPhone(phone string) string {
	parsed := ParsePhone(phone)
	if len(parsed) > 0 {
		return parsed + "@c.us"
	}
	return phone
}

func ExtractPhoneFromJID(jid string) string {
	if idx := len(jid) - 1; idx > 0 {
		return jid[:idx]
	}
	return jid
}

func ExtractMessageID(resp map[string]interface{}) string {
	if msgs, ok := resp["messages"].([]map[string]interface{}); ok && len(msgs) > 0 {
		if id, ok := msgs[0]["id"].(string); ok {
			return id
		}
	}
	return ""
}

var _ ports.MessagingPort = (*Adapter)(nil)

func (a *Adapter) ConvertAudioForPlatform(ctx context.Context, audioData []byte, format string) ([]byte, string, error) {
	return audioData, "ogg", nil
}

// Start is a no-op for WhatsApp: messages arrive via the incoming webhook endpoint.
func (a *Adapter) Start(_ context.Context, _ func(context.Context, *models.Message)) error {
	return nil
}

func (a *Adapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{
		HasVoiceMessage: true,
		HasCallStream:   true,
		HasTextStream:   true,
		HasMediaSupport: true,
	}
}
