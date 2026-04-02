package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pdk "github.com/extism/go-pdk"
	slackgo "github.com/slack-go/slack"
	_ "github.com/stealthrocket/net/http"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-messages-slack"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
	pdk.OutputString("Slack Bot messaging plugin for OpenLobster")
	return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("messaging"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"bot_token":{"type":"string","title":"Bot Token (xoxb-)","description":"Slack Bot User OAuth token (xoxb-...)"},"channel":{"type":"string","title":"Default Channel (optional)","description":"Fallback channel when outgoing message does not specify channel_id"}},"required":["bot_token"]}`)
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
		"HasVoiceMessage": false,
		"HasCallStream":   false,
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

func resolveSlackChannelID(input resolveChannelIDInput) (string, error) {
	channelID := strings.TrimSpace(input.Message.ChannelID)
	if channelID != "" && !strings.EqualFold(channelID, "slack") {
		return channelID, nil
	}

	if input.Message.Metadata != nil {
		if v, ok := input.Message.Metadata["slack_channel_id"].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s, nil
			}
		}
		if v, ok := input.Message.Metadata["platform_channel_id"].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s, nil
			}
		}
	}

	if fallback, _ := input.Config["channel"].(string); strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback), nil
	}

	return "", fmt.Errorf("slack resolve_channel_id: missing platform channel id (set message.metadata.slack_channel_id or config.channel)")
}

//go:wasmexport resolve_channel_id
func resolveChannelID() int32 {
	var input resolveChannelIDInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	channelID, err := resolveSlackChannelID(input)
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

	token, _ := input.Config["bot_token"].(string)
	if token == "" {
		pdk.SetError(fmt.Errorf("slack bot_token required"))
		return 1
	}

	client := slackgo.New(token)

	resolveInput := resolveChannelIDInput{Config: input.Config}
	resolveInput.Message.ChannelID = input.Message.ChannelID
	resolvedChannelID, err := resolveSlackChannelID(resolveInput)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	_, _, err = client.PostMessage(resolvedChannelID, slackgo.MsgOptionText(input.Message.Content, false))
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

//go:wasmexport handle_webhook
func handleWebhook() int32 {
	pdk.SetError(fmt.Errorf("webhook_not_supported: slack plugin uses polling"))
	return 1
}

// ---------------------------------------------------------------------------
// start — polls conversations.history for new messages
// ---------------------------------------------------------------------------

//go:wasmexport start
func start() int32 {
	var cfg map[string]interface{}
	if err := pdk.InputJSON(&cfg); err != nil {
		pdk.SetError(err)
		return 1
	}

	token, _ := cfg["bot_token"].(string)
	if token == "" {
		pdk.SetError(fmt.Errorf("slack bot_token required"))
		return 1
	}
	defaultChannel, _ := cfg["channel"].(string)

	client := slackgo.New(token)
	ctx := context.Background()
	auth, _ := client.AuthTestContext(ctx)
	botUserID := ""
	if auth != nil {
		botUserID = auth.UserID
	}

	// Gather channels to poll
	var channelIDs []string
	if defaultChannel != "" {
		channelIDs = append(channelIDs, defaultChannel)
	} else {
		resp, cursor := (*slackgo.GetConversationsParameters)(nil), ""
		_ = resp
		params := &slackgo.GetConversationsParameters{Limit: 200}
		for {
			chans, nextCursor, err := client.GetConversationsContext(ctx, params)
			if err != nil {
				break
			}
			for _, ch := range chans {
				channelIDs = append(channelIDs, ch.ID)
			}
			if nextCursor == "" {
				break
			}
			cursor = nextCursor
			params.Cursor = cursor
		}
	}

	lastTS := make(map[string]string)

	for {
		for _, channelID := range channelIDs {
			histParams := &slackgo.GetConversationHistoryParameters{
				ChannelID: channelID,
				Limit:     10,
			}
			if ts, ok := lastTS[channelID]; ok {
				histParams.Oldest = ts
			}
			hist, err := client.GetConversationHistoryContext(ctx, histParams)
			if err != nil {
				continue
			}
			// messages are newest-first; process in reverse
			for i := len(hist.Messages) - 1; i >= 0; i-- {
				m := hist.Messages[i]
				if ts, ok := lastTS[channelID]; ok && m.Timestamp == ts {
					continue
				}
				if m.BotID != "" || (botUserID != "" && m.User == botUserID) {
					continue
				}
				if m.SubType != "" {
					continue
				}
				lastTS[channelID] = m.Timestamp
				isGroup := !strings.HasPrefix(channelID, "D")
				isMentioned := !isGroup
				if isGroup && botUserID != "" {
					isMentioned = strings.Contains(m.Text, "<@"+botUserID+">")
				}
				emitMessage(&pluginMessage{
					ChannelID:   channelID,
					SenderID:    m.User,
					SenderName:  m.User,
					Content:     m.Text,
					IsGroup:     isGroup,
					IsMentioned: isMentioned,
					Timestamp:   time.Now(),
					Metadata:    map[string]interface{}{"channel_type": "slack", "ts": m.Timestamp},
				})
			}
		}
		time.Sleep(3 * time.Second)
	}
}

func main() {}
