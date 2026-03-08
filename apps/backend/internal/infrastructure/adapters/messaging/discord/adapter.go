// Copyright (C) 2024 OpenLobster contributors
// SPDX-License-Identifier: see LICENSE
package discord

import (
	"context"
	"fmt"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/google/uuid"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
)

// Adapter implements ports.MessagingPort for Discord via the arikawa/v3 SDK.
type Adapter struct {
	s     *state.State
	token string
}

// NewAdapter creates a new Discord adapter. The gateway connection is not
// opened here; call Start to connect.
func NewAdapter(token string) (*Adapter, error) {
	s := state.New("Bot " + token)
	return &Adapter{s: s, token: token}, nil
}

// SendTyping triggers the typing indicator in a Discord channel.
func (a *Adapter) SendTyping(ctx context.Context, channelID string) error {
	chID, err := parseChannelID(channelID)
	if err != nil {
		return nil // best-effort, don't fail
	}
	return a.s.Typing(chID)
}

// SendMessage sends a plain text message to a Discord channel.
func (a *Adapter) SendMessage(ctx context.Context, msg *models.Message) error {
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

// SendMedia sends a message with an embedded image URL to a Discord channel.
func (a *Adapter) SendMedia(ctx context.Context, media *ports.Media) error {
	channelID, err := parseChannelID(media.ChatID)
	if err != nil {
		return fmt.Errorf("discord send media: invalid channel id %q: %w", media.ChatID, err)
	}
	if media.URL != "" {
		embed := discord.Embed{
			Image: &discord.EmbedImage{URL: media.URL},
		}
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

// HandleWebhook is a no-op for Discord: messages arrive via the WebSocket
// gateway, so this path is never invoked.
func (a *Adapter) HandleWebhook(_ context.Context, _ []byte) (*models.Message, error) {
	return nil, nil
}

// GetUserInfo retrieves basic user information by Discord user ID.
func (a *Adapter) GetUserInfo(ctx context.Context, userID string) (*ports.UserInfo, error) {
	sf, err := discord.ParseSnowflake(userID)
	if err != nil {
		return &ports.UserInfo{ID: userID, Username: userID, DisplayName: userID}, nil
	}
	user, err := a.s.User(discord.UserID(sf))
	if err != nil {
		return &ports.UserInfo{ID: userID, Username: userID, DisplayName: userID}, nil
	}
	       return &ports.UserInfo{
		       ID:          userID,
		       Username:    user.Username,
		       DisplayName: user.DisplayOrUsername(),
	       }, nil
}

// React adds an emoji reaction to a Discord message. messageID must be in the
// format "channelID-messageID" so both IDs can be extracted.
func (a *Adapter) React(ctx context.Context, messageID string, emoji string) error {
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

// GetCapabilities returns capability flags for the Discord channel.
func (a *Adapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{
		HasVoiceMessage: true,
		HasCallStream:   true,
		HasTextStream:   true,
		HasMediaSupport: true,
	}
}

// Start opens the Discord WebSocket gateway and calls onMessage for every
// incoming guild or direct message. The connection is closed when ctx is
// cancelled.
func (a *Adapter) Start(ctx context.Context, onMessage func(context.Context, *models.Message)) error {
	botUser, err := a.s.Me()
	if err != nil {
		// Me() may fail before the gateway is open; ignore and allow the
		// handler to perform a best-effort check.
		botUser = nil
	}

	a.s.AddHandler(func(e *gateway.MessageCreateEvent) {
		// Ignore messages sent by the bot itself.
		if botUser != nil && e.Author.ID == botUser.ID {
			return
		}

		// IsGroup is true for guild (server) channels; DMs have no GuildID.
		isGroup := e.GuildID.IsValid()

		// IsMentioned: the message explicitly mentions the bot user, or it is
		// a reply to one of the bot's own messages.
		isMentioned := false
		if isGroup && botUser != nil {
			for _, u := range e.Mentions {
				if u.ID == botUser.ID {
					isMentioned = true
					break
				}
			}
			if !isMentioned && e.ReferencedMessage != nil {
				isMentioned = e.ReferencedMessage.Author.ID == botUser.ID
			}
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

	// If Me() failed before Open, retry now that the gateway is connected.
	if botUser == nil {
		if u, err := a.s.Me(); err == nil {
			botUser = u
		}
	}

	go func() {
		<-ctx.Done()
		a.s.Close()
	}()
	return nil
}

// ConvertAudioForPlatform returns the audio data unchanged with the ogg format.
func (a *Adapter) ConvertAudioForPlatform(_ context.Context, audioData []byte, _ string) ([]byte, string, error) {
	return audioData, "ogg", nil
}

// parseChannelID parses a Discord snowflake channel ID from a string.
func parseChannelID(s string) (discord.ChannelID, error) {
	sf, err := discord.ParseSnowflake(s)
	if err != nil {
		return 0, err
	}
	return discord.ChannelID(sf), nil
}

// splitMessageID splits a compound "channelID-messageID" string.
func splitMessageID(s string) (string, string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' {
			return s[:i], s[i+1:]
		}
	}
	return s, s
}

// timestampToTime converts a discord.Timestamp to time.Time.
// This is compiled away if unused.
var _ = time.Time{}

var _ ports.MessagingPort = (*Adapter)(nil)

