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
	pdk "github.com/extism/go-pdk"
	_ "github.com/stealthrocket/net/http"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-messages-discord"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
	pdk.OutputString("Discord Bot messaging plugin for OpenLobster")
	return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("messaging"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token","description":"Discord bot token with permissions to send and read messages"},"guild_id":{"type":"string","title":"Guild ID (optional)","description":"Optional guild scope for startup checks and routing"},"default_channel_id":{"type":"string","title":"Default Channel ID (optional)","description":"Fallback Discord channel id used when message.channel_id is the logical channel slug"}},"required":["token"]}`)
	return 0
}

// ---------------------------------------------------------------------------
// Host emit_message
// ---------------------------------------------------------------------------

//go:wasmimport openlobster emit_message
func hostEmitMessage(offset uint64)

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

//go:wasmexport capabilities
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
		ChannelID string `json:"channel_id"`
		Content   string `json:"content"`
	} `json:"message"`
}

type resolveChannelIDInput struct {
	Config  map[string]interface{} `json:"config"`
	Message struct {
		ChannelID string                 `json:"channel_id"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	} `json:"message"`
}

func resolveDiscordChannelID(input resolveChannelIDInput) (string, error) {
	channelID := strings.TrimSpace(input.Message.ChannelID)
	if channelID != "" && !strings.EqualFold(channelID, "discord") {
		return channelID, nil
	}

	if input.Message.Metadata != nil {
		if v, ok := input.Message.Metadata["platform_channel_id"].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s, nil
			}
		}
	}

	if fallback, _ := input.Config["default_channel_id"].(string); strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback), nil
	}

	return "", fmt.Errorf("discord resolve_channel_id: missing platform channel id (set message.metadata.platform_channel_id or config.default_channel_id)")
}

//go:wasmexport resolve_channel_id
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

//go:wasmexport send
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
	resolvedChannelID, err := resolveDiscordChannelID(resolveInput)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	sf, err := discord.ParseSnowflake(resolvedChannelID)
	if err != nil {
		pdk.SetError(fmt.Errorf("invalid channel_id: %w", err))
		return 1
	}
	channelID := discord.ChannelID(sf)

	_, err = client.SendMessageComplex(channelID, api.SendMessageData{
		Content: input.Message.Content,
	})
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

//go:wasmexport handle_webhook
func handleWebhook() int32 {
	pdk.SetError(fmt.Errorf("webhook_not_supported: discord uses gateway events"))
	return 1
}

// ---------------------------------------------------------------------------
// start — gateway loop (Discord WebSocket API)
// ---------------------------------------------------------------------------

//go:wasmexport start
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

func main() {}
