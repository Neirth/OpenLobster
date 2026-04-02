// Copyright (C) 2024 OpenLobster contributors
// SPDX-License-Identifier: see LICENSE
package discord

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/google/uuid"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// Adapter implements ports.MessagingPort for Discord via arikawa/v3.
type Adapter struct {
	s     *state.State
	token string
}

// NewAdapter creates a new Discord adapter. The gateway connection is opened
// by Start.
func NewAdapter(token string) (*Adapter, error) {
	s := state.New("Bot " + token)
	return &Adapter{s: s, token: token}, nil
}

// ChannelType returns this adapter's logical channel type.
func (a *Adapter) ChannelType() string {
	return "discord"
}

// SendTyping triggers the typing indicator in a Discord channel.
func (a *Adapter) SendTyping(ctx context.Context, channelID string) error {
	_ = ctx
	chID, err := parseChannelID(channelID)
	if err != nil {
		return nil // best effort
	}
	return a.s.Typing(chID)
}

// SendMessage sends a plain text message to a Discord channel.
func (a *Adapter) SendMessage(ctx context.Context, msg *models.Message) error {
	_ = ctx
	if msg == nil {
		return nil
	}
	channelID, err := parseChannelID(msg.ChannelID)
	if err != nil {
		return fmt.Errorf("discord send message: invalid channel id %q: %w", msg.ChannelID, err)
	}
	_, err = a.s.SendMessage(channelID, msg.Content)
	if err != nil {
		return fmt.Errorf("discord send message: %w", err)
	}
	return nil
}

// SendMedia sends media content by embedding the URL when available.
func (a *Adapter) SendMedia(ctx context.Context, media *ports.Media) error {
	_ = ctx
	if media == nil {
		return nil
	}
	channelID, err := parseChannelID(media.ChatID)
	if err != nil {
		return fmt.Errorf("discord send media: invalid channel id %q: %w", media.ChatID, err)
	}
	if media.URL != "" {
		embed := discord.Embed{Image: &discord.EmbedImage{URL: media.URL}}
		if media.Caption != "" {
			embed.Description = media.Caption
		}
		_, err = a.s.SendMessage(channelID, "", embed)
	} else {
		_, err = a.s.SendMessage(channelID, media.Caption)
	}
	if err != nil {
		return fmt.Errorf("discord send media: %w", err)
	}
	return nil
}

// HandleWebhook is a no-op: Discord traffic is handled via gateway events.
func (a *Adapter) HandleWebhook(_ context.Context, _ []byte) (*models.Message, error) {
	return nil, nil
}

// GetUserInfo retrieves basic user information by Discord user ID.
func (a *Adapter) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	_ = ctx
	sf, err := discord.ParseSnowflake(userID)
	if err != nil {
		return &ports.UserInfo{ID: userID, Username: userID, DisplayName: userID}, nil
	}
	user, err := a.s.User(discord.UserID(sf))
	if err != nil {
		return &ports.UserInfo{ID: userID, Username: userID, DisplayName: userID}, nil
	}
	return &ports.UserInfo{ID: userID, Username: user.Username, DisplayName: user.DisplayOrUsername()}, nil
}

// React adds an emoji reaction to a Discord message.
// messageID must be in the format "channelID-messageID".
func (a *Adapter) React(ctx context.Context, messageID string, emoji string) error {
	_ = ctx
	channelIDStr, msgIDStr := splitMessageID(messageID)
	chSF, err := discord.ParseSnowflake(channelIDStr)
	if err != nil {
		return fmt.Errorf("discord react: invalid channel id: %w", err)
	}
	msgSF, err := discord.ParseSnowflake(msgIDStr)
	if err != nil {
		return fmt.Errorf("discord react: invalid message id: %w", err)
	}
	return a.s.React(discord.ChannelID(chSF), discord.MessageID(msgSF), discord.APIEmoji(emoji))
}

// GetCapabilities returns capability flags for Discord.
func (a *Adapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{
		HasVoiceMessage: true,
		HasCallStream:   true,
		HasTextStream:   true,
		HasMediaSupport: true,
	}
}

// Start opens Discord gateway and forwards inbound messages through onMessage.
func (a *Adapter) Start(ctx context.Context, onMessage func(context.Context, *models.Message)) error {
	if onMessage == nil {
		onMessage = func(context.Context, *models.Message) {}
	}

	botUser, err := a.s.Me()
	if err != nil {
		botUser = nil
	}

	a.s.AddHandler(func(e *gateway.MessageCreateEvent) {
		if e == nil || e.Author.Bot {
			return
		}
		if botUser != nil && e.Author.ID == botUser.ID {
			return
		}

		isGroup := e.GuildID.IsValid()
		isMentioned := !isGroup
		if isGroup && botUser != nil {
			isMentioned = false
			for _, u := range e.Mentions {
				if u.ID == botUser.ID {
					isMentioned = true
					break
				}
			}
			if !isMentioned && e.ReferencedMessage != nil {
				isMentioned = e.ReferencedMessage.Author.ID == botUser.ID
			}
			if !isMentioned {
				idStr := botUser.ID.String()
				isMentioned = strings.Contains(e.Content, "<@"+idStr+">") ||
					strings.Contains(e.Content, "<@!"+idStr+">")
			}
		}

		attachments := make([]models.Attachment, 0, len(e.Attachments))
		for _, att := range e.Attachments {
			attType := "document"
			mimeType := att.ContentType
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			if strings.HasPrefix(mimeType, "image/") {
				attType = "image"
			} else if strings.HasPrefix(mimeType, "audio/") {
				attType = "audio"
			} else if strings.HasPrefix(mimeType, "video/") {
				attType = "video"
			}

			var data []byte
			if resp, getErr := http.Get(att.URL); getErr == nil { //nolint:noctx
				data, _ = io.ReadAll(resp.Body)
				resp.Body.Close()
			}

			attachments = append(attachments, models.Attachment{
				Type:     attType,
				Filename: att.Filename,
				Size:     int64(att.Size),
				MIMEType: mimeType,
				Data:     data,
			})
		}

		msg := &models.Message{
			ID:          uuid.New(),
			ChannelID:   e.ChannelID.String(),
			SenderName:  e.Author.Username,
			SenderID:    e.Author.ID.String(),
			IsGroup:     isGroup,
			IsMentioned: isMentioned,
			Content:     e.Content,
			Timestamp:   e.Timestamp.Time(),
			Metadata: map[string]interface{}{
				"channel_type":        "discord",
				"platform_channel_id": e.ChannelID.String(),
			},
			Attachments: attachments,
		}
		onMessage(ctx, msg)
	})

	a.s.AddIntents(
		gateway.IntentGuildMessages |
			gateway.IntentDirectMessages |
			gateway.IntentMessageContent,
	)

	if err := a.s.Open(ctx); err != nil {
		return fmt.Errorf("discord gateway open: %w", err)
	}

	if botUser == nil {
		if u, meErr := a.s.Me(); meErr == nil {
			botUser = u
		}
	}

	go func() {
		<-ctx.Done()
		a.s.Close()
	}()
	return nil
}

// ConvertAudioForPlatform currently returns audio unchanged and marks it as OGG.
func (a *Adapter) ConvertAudioForPlatform(_ context.Context, audioData []byte, _ string) ([]byte, string, error) {
	return audioData, "ogg", nil
}

func parseChannelID(s string) (discord.ChannelID, error) {
	sf, err := discord.ParseSnowflake(s)
	if err != nil {
		return 0, err
	}
	return discord.ChannelID(sf), nil
}

func splitMessageID(s string) (string, string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' {
			return s[:i], s[i+1:]
		}
	}
	return s, s
}

var _ ports.MessagingPort = (*Adapter)(nil)
