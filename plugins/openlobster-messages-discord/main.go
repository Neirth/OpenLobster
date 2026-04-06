//go:build !tinygo

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
	_ "github.com/stealthrocket/net/http"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func getName() int32 { pdk.OutputString("openlobster-messages-discord"); return 0 }

func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

func getDescription() int32 {
	pdk.OutputString("Discord Bot messaging plugin for OpenLobster")
	return 0
}

func getType() int32 { pdk.OutputString("messaging"); return 0 }

func inboundMode() int32 { pdk.OutputString("gateway"); return 0 }

func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token","description":"Discord bot token with permissions to send and read messages"},"guild_id":{"type":"string","title":"Guild ID (optional)","description":"Optional guild scope for startup checks and routing"},"default_channel_id":{"type":"string","title":"Default Channel ID (optional)","description":"Fallback Discord channel id used when message.channel_id is the logical channel slug"},"default_recipient_id":{"type":"string","title":"Default Recipient User ID (optional)","description":"Fallback Discord user id for private DM delivery when no channel id is available"}},"required":["token"]}`)
	return 0
}

func getMetadata() int32 {
	metadata := map[string]interface{}{
		"id":          "openlobster-messages-discord",
		"name":        "openlobster-messages-discord",
		"version":     "0.1.0",
		"description": "Discord Bot messaging plugin for OpenLobster",
		"type":        "messaging",
		"schema":      json.RawMessage(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token","description":"Discord bot token with permissions to send and read messages"},"guild_id":{"type":"string","title":"Guild ID (optional)","description":"Optional guild scope for startup checks and routing"},"default_channel_id":{"type":"string","title":"Default Channel ID (optional)","description":"Fallback Discord channel id used when message.channel_id is the logical channel slug"},"default_recipient_id":{"type":"string","title":"Default Recipient User ID (optional)","description":"Fallback Discord user id for private DM delivery when no channel id is available"}},"required":["token"]}`),
		"properties":  json.RawMessage(`{}`),
	}
	if err := pdk.OutputJSON(metadata); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func getManifest() int32 {
	pdk.OutputString(`{"id":"openlobster-messages-discord","name":"openlobster-messages-discord","version":"0.1.0","description":"Discord Bot messaging plugin for OpenLobster","type":"messaging","schema":{"type":"object","properties":{"token":{"type":"string","title":"Bot Token","description":"Discord bot token with permissions to send and read messages"},"guild_id":{"type":"string","title":"Guild ID (optional)","description":"Optional guild scope for startup checks and routing"},"default_channel_id":{"type":"string","title":"Default Channel ID (optional)","description":"Fallback Discord channel id used when message.channel_id is the logical channel slug"},"default_recipient_id":{"type":"string","title":"Default Recipient User ID (optional)","description":"Fallback Discord user id for private DM delivery when no channel id is available"}},"required":["token"]}}`)
	return 0
}

// ---------------------------------------------------------------------------
// Host emit_message
// ---------------------------------------------------------------------------

func hostEmitMessage(offset uint64) {
	pdk.EmitAllocated(offset)
}

type pluginMessage struct {
	ChannelID   string                 `json:"channel_id"`
	SenderID    string                 `json:"sender_id,omitempty"`
	SenderName  string                 `json:"sender_name,omitempty"`
	Content     string                 `json:"content"`
	IsGroup     bool                   `json:"is_group,omitempty"`
	IsMentioned bool                   `json:"is_mentioned,omitempty"`
	GroupName   string                 `json:"group_name,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func emitMessage(msg *pluginMessage) {
	b, _ := json.Marshal(msg)
	mem := pdk.AllocateBytes(b)
	hostEmitMessage(mem.Offset())
}

// ---------------------------------------------------------------------------
// capabilities
// ---------------------------------------------------------------------------

func capabilities() int32 {
	_ = pdk.OutputJSON(map[string]bool{
		"HasVoiceMessage": true,
		"HasCallStream":   true,
		"HasTextStream":   true,
		"HasMediaSupport": true,
	})
	return 0
}

// ---------------------------------------------------------------------------
// send
// ---------------------------------------------------------------------------

type sendInput struct {
	Config  map[string]interface{} `json:"config"`
	Message struct {
		ChannelID string                 `json:"channel_id"`
		SenderID  string                 `json:"sender_id,omitempty"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
		Content   string                 `json:"content"`
	} `json:"message"`
}

type resolveChannelIDInput struct {
	Config  map[string]interface{} `json:"config"`
	Message struct {
		ChannelID string                 `json:"channel_id"`
		SenderID  string                 `json:"sender_id,omitempty"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	} `json:"message"`
}

type discordDestination struct {
	ChannelID   string
	RecipientID string
}

func resolveDiscordDestination(input resolveChannelIDInput) (discordDestination, error) {
	channelID := strings.TrimSpace(input.Message.ChannelID)
	if channelID != "" && !strings.EqualFold(channelID, "discord") {
		return discordDestination{ChannelID: channelID, RecipientID: extractDiscordRecipientID(input)}, nil
	}

	if input.Message.Metadata != nil {
		if s := readStringMap(input.Message.Metadata, "platform_channel_id"); s != "" {
			return discordDestination{ChannelID: s, RecipientID: extractDiscordRecipientID(input)}, nil
		}
	}

	if fallback, _ := input.Config["default_channel_id"].(string); strings.TrimSpace(fallback) != "" {
		return discordDestination{ChannelID: strings.TrimSpace(fallback), RecipientID: extractDiscordRecipientID(input)}, nil
	}

	recipientID := extractDiscordRecipientID(input)
	if recipientID != "" {
		return discordDestination{RecipientID: recipientID}, nil
	}

	return discordDestination{}, fmt.Errorf("discord resolve_channel_id: missing destination (set message.channel_id, message.sender_id, message.metadata.platform_channel_id, message.metadata.platform_user_id, config.default_channel_id, or config.default_recipient_id)")
}

func extractDiscordRecipientID(input resolveChannelIDInput) string {
	if s := strings.TrimSpace(input.Message.SenderID); s != "" {
		return s
	}
	if input.Message.Metadata != nil {
		if s := readStringMap(input.Message.Metadata, "platform_user_id"); s != "" {
			return s
		}
		if s := readStringMap(input.Message.Metadata, "recipient_id"); s != "" {
			return s
		}
		if s := readStringMap(input.Message.Metadata, "sender_id"); s != "" {
			return s
		}
	}
	if fallback, _ := input.Config["default_recipient_id"].(string); strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return ""
}

func readStringMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func resolveDiscordChannelID(input resolveChannelIDInput) (string, error) {
	destination, err := resolveDiscordDestination(input)
	if err != nil {
		return "", err
	}
	if destination.ChannelID != "" {
		return destination.ChannelID, nil
	}
	if destination.RecipientID != "" {
		return destination.RecipientID, nil
	}
	return "", fmt.Errorf("discord resolve_channel_id: missing destination")
}

func resolveChannelID() int32 {
	var input resolveChannelIDInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	channelID, err := resolveDiscordChannelID(input)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	pdk.OutputString(channelID)
	return 0
}

func send() int32 {
	var input sendInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	token, _ := input.Config["token"].(string)
	if token == "" {
		pdk.SetError(fmt.Errorf("discord token required"))
		return 1
	}

	client := api.NewClient("Bot " + token)

	resolveInput := resolveChannelIDInput{Config: input.Config}
	resolveInput.Message.ChannelID = input.Message.ChannelID
	resolveInput.Message.SenderID = input.Message.SenderID
	resolveInput.Message.Metadata = input.Message.Metadata
	destination, err := resolveDiscordDestination(resolveInput)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	if destination.ChannelID != "" {
		if err := sendDiscordMessageToChannel(client, destination.ChannelID, input.Message.Content); err == nil {
			return 0
		} else {
			fallbackRecipient := strings.TrimSpace(destination.RecipientID)
			if fallbackRecipient == "" && isDiscordUnknownChannelError(err) {
				fallbackRecipient = strings.TrimSpace(destination.ChannelID)
			}
			if fallbackRecipient != "" {
				dmChannelID, dmErr := ensureDiscordDMChannelID(client, fallbackRecipient)
				if dmErr == nil {
					if sendErr := sendDiscordMessageToChannel(client, dmChannelID, input.Message.Content); sendErr == nil {
						return 0
					} else {
						pdk.SetError(sendErr)
						return 1
					}
				}
			}
			pdk.SetError(err)
			return 1
		}
	}

	if destination.RecipientID != "" {
		dmChannelID, dmErr := ensureDiscordDMChannelID(client, destination.RecipientID)
		if dmErr != nil {
			pdk.SetError(dmErr)
			return 1
		}
		if sendErr := sendDiscordMessageToChannel(client, dmChannelID, input.Message.Content); sendErr != nil {
			pdk.SetError(sendErr)
			return 1
		}
		return 0
	}

	pdk.SetError(fmt.Errorf("discord send: missing destination"))
	return 1
}

func sendDiscordMessageToChannel(client *api.Client, channelIDRaw, content string) error {
	sf, err := discord.ParseSnowflake(strings.TrimSpace(channelIDRaw))
	if err != nil {
		return fmt.Errorf("invalid channel_id: %w", err)
	}
	channelID := discord.ChannelID(sf)

	_, err = client.SendMessageComplex(channelID, api.SendMessageData{
		Content: content,
	})
	if err != nil {
		return err
	}
	return nil
}

func ensureDiscordDMChannelID(client *api.Client, recipientIDRaw string) (string, error) {
	sf, err := discord.ParseSnowflake(strings.TrimSpace(recipientIDRaw))
	if err != nil {
		return "", fmt.Errorf("invalid recipient_id: %w", err)
	}
	recipientID := discord.UserID(sf)
	channel, err := client.CreatePrivateChannel(recipientID)
	if err != nil {
		return "", err
	}
	return channel.ID.String(), nil
}

func isDiscordUnknownChannelError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unknown channel")
}

func handleWebhook() int32 {
	pdk.SetError(fmt.Errorf("webhook_not_supported: discord uses gateway events"))
	return 1
}

// ---------------------------------------------------------------------------
// start — gateway loop (Discord WebSocket API)
// ---------------------------------------------------------------------------

func start() int32 {
	var cfg map[string]interface{}
	if err := pdk.InputJSON(&cfg); err != nil {
		pdk.SetError(err)
		return 1
	}

	token, _ := cfg["token"].(string)
	if token == "" {
		pdk.SetError(fmt.Errorf("discord token required"))
		return 1
	}

	botID := discord.UserID(0)
	if me, err := api.NewClient("Bot " + token).Me(); err == nil {
		botID = me.ID
	}

	s := state.New("Bot " + token)
	inbound := make(chan *pluginMessage, 256)
	s.AddIntents(
		gateway.IntentGuildMessages |
			gateway.IntentDirectMessages |
			gateway.IntentMessageContent,
	)

	s.AddHandler(func(e *gateway.MessageCreateEvent) {
		if e == nil {
			return
		}
		if e.Author.Bot {
			return
		}
		if botID.IsValid() && e.Author.ID == botID {
			return
		}

		isGroup := e.GuildID.IsValid()
		isMentioned := !isGroup
		if isGroup && botID.IsValid() {
			for _, u := range e.Mentions {
				if u.ID == botID {
					isMentioned = true
					break
				}
			}
			if !isMentioned && e.ReferencedMessage != nil {
				isMentioned = e.ReferencedMessage.Author.ID == botID
			}
			if !isMentioned {
				idStr := botID.String()
				isMentioned = strings.Contains(e.Content, "<@"+idStr+">") ||
					strings.Contains(e.Content, "<@!"+idStr+">")
			}
		}

		msg := &pluginMessage{
			ChannelID:   e.ChannelID.String(),
			SenderID:    e.Author.ID.String(),
			SenderName:  e.Author.Username,
			Content:     e.Content,
			IsGroup:     isGroup,
			IsMentioned: isMentioned,
			Timestamp:   e.Timestamp.Time(),
			Metadata:    map[string]interface{}{"channel_type": "discord"},
		}

		select {
		case inbound <- msg:
		default:
			// Drop on saturation instead of blocking gateway callbacks.
		}
	})

	ctx := context.Background()
	if err := s.Open(ctx); err != nil {
		pdk.SetError(fmt.Errorf("discord gateway open failed: %w", err))
		return 1
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-inbound:
			emitMessage(msg)
		case <-ticker.C:
		}
	}
}

func main() {
	pdk.MustRun(pdk.Plugin{
		ID: "openlobster-messages-discord",
		Exports: map[string]pdk.Function{
			"get_metadata":       getMetadata,
			"inbound_mode":       inboundMode,
			"capabilities":       capabilities,
			"resolve_channel_id": resolveChannelID,
			"configure":          configureHot,
			"send":               send,
			"handle_webhook":     handleWebhook,
			"start":              start,
		},
	})
}
