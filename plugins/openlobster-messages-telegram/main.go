package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
	_ "github.com/stealthrocket/net/http"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func getName() int32 { pdk.OutputString("openlobster-messages-telegram"); return 0 }

func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

func getDescription() int32 {
	pdk.OutputString("Telegram Bot API messaging plugin for OpenLobster")
	return 0
}

func getType() int32 { pdk.OutputString("messaging"); return 0 }

func inboundMode() int32 { pdk.OutputString("polling"); return 0 }

func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token","description":"Telegram bot token from @BotFather"},"default_chat_id":{"type":"string","title":"Default Chat ID (optional)","description":"Fallback Telegram chat id used when message.channel_id is the logical channel slug"}},"required":["token"]}`)
	return 0
}

func getMetadata() int32 {
	metadata := map[string]interface{}{
		"id":          "openlobster-messages-telegram",
		"name":        "openlobster-messages-telegram",
		"version":     "0.1.0",
		"description": "Telegram Bot API messaging plugin for OpenLobster",
		"type":        "messaging",
		"schema":      json.RawMessage(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token","description":"Telegram bot token from @BotFather"},"default_chat_id":{"type":"string","title":"Default Chat ID (optional)","description":"Fallback Telegram chat id used when message.channel_id is the logical channel slug"}},"required":["token"]}`),
		"properties":  json.RawMessage(`{}`),
	}
	if err := pdk.OutputJSON(metadata); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Host emit_message compatibility bridge (offset-based)
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
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	mem := pdk.AllocateBytes(b)
	hostEmitMessage(mem.Offset())
}

// ---------------------------------------------------------------------------
// capabilities
// ---------------------------------------------------------------------------

func capabilities() int32 {
	_ = pdk.OutputJSON(map[string]bool{
		"HasVoiceMessage": true,
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
		ChannelID string                 `json:"channel_id"`
		Content   string                 `json:"content"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	} `json:"message"`
}

const telegramChannelID = "telegram"
const telegramHTTPTimeout = 20 * time.Second

type resolveChannelIDInput struct {
	Config  map[string]interface{} `json:"config"`
	Message struct {
		ChannelID string                 `json:"channel_id"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	} `json:"message"`
}

func resolveTelegramChannelID(input resolveChannelIDInput) (string, error) {
	channelID := strings.TrimSpace(input.Message.ChannelID)
	if channelID != "" && !strings.EqualFold(channelID, telegramChannelID) {
		return channelID, nil
	}

	if input.Message.Metadata != nil {
		if v, ok := input.Message.Metadata["telegram_chat_id"].(string); ok {
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

	if fallback, _ := input.Config["default_chat_id"].(string); strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback), nil
	}

	return "", fmt.Errorf("telegram resolve_channel_id: missing chat id (set message.channel_id, message.metadata.telegram_chat_id, or config.default_chat_id)")
}

func resolveChannelID() int32 {
	var input resolveChannelIDInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	channelID, err := resolveTelegramChannelID(input)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	pdk.OutputString(channelID)
	return 0
}

func parseTelegramChatID(input sendInput) (int64, error) {
	resolveInput := resolveChannelIDInput{Config: input.Config}
	resolveInput.Message.ChannelID = input.Message.ChannelID
	resolveInput.Message.Metadata = input.Message.Metadata
	chatIDRaw, err := resolveTelegramChannelID(resolveInput)
	if err != nil {
		return 0, err
	}
	chatID, err := strconv.ParseInt(chatIDRaw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid channel_id: %w", err)
	}
	return chatID, nil
}

func newTelegramBot(token string) (*tgbotapi.BotAPI, error) {
	if token == "" {
		return nil, fmt.Errorf("telegram token required")
	}

	bot := &tgbotapi.BotAPI{
		Token:  token,
		Buffer: 100,
		Client: &http.Client{Timeout: telegramHTTPTimeout},
	}
	bot.SetAPIEndpoint(tgbotapi.APIEndpoint)
	return bot, nil
}

func sendTelegramMarkdown(bot *tgbotapi.BotAPI, chatID int64, content string) error {
	msg := tgbotapi.NewMessage(chatID, content)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := bot.Send(msg); err != nil {
		// Keep delivery resilient when AI output contains markdown entities Telegram cannot parse.
		if strings.Contains(strings.ToLower(err.Error()), "can't parse entities") {
			fallback := tgbotapi.NewMessage(chatID, content)
			_, retryErr := bot.Send(fallback)
			return retryErr
		}
		return err
	}
	return nil
}

func send() int32 {
	var input sendInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	token, _ := input.Config["token"].(string)
	bot, err := newTelegramBot(token)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	chatID, err := parseTelegramChatID(input)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	if err := sendTelegramMarkdown(bot, chatID, input.Message.Content); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func typing() int32 {
	var input sendInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	token, _ := input.Config["token"].(string)
	bot, err := newTelegramBot(token)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	chatID, err := parseTelegramChatID(input)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	if _, err := bot.Request(action); err != nil {
		pdk.SetError(err)
		return 1
	}

	return 0
}

func handleWebhook() int32 {
	pdk.SetError(fmt.Errorf("webhook_not_supported: telegram plugin uses long polling"))
	return 1
}

// ---------------------------------------------------------------------------
// start — long-polling loop
// ---------------------------------------------------------------------------

func start() int32 {
	var cfg map[string]interface{}
	if err := pdk.InputJSON(&cfg); err != nil {
		pdk.SetError(err)
		return 1
	}

	token, _ := cfg["token"].(string)
	bot, err := newTelegramBot(token)
	if err != nil {
		pdk.SetError(err)
		return 1
	}
	pdk.Log(pdk.LogInfo, "telegram start: long-polling loop initialized")

	u := tgbotapi.NewUpdate(0)
	// Use short polling (timeout=0) because some WASI runtimes stall on
	// long-held HTTP responses.
	u.Timeout = 0
	u.Limit = 10
	botUsername := strings.ToLower(strings.TrimSpace(bot.Self.UserName))
	idleCycles := 0

	for {
		updates, err := bot.GetUpdates(u)
		if err != nil {
			pdk.Log(pdk.LogError, fmt.Sprintf("telegram polling error: offset=%d err=%v", u.Offset, err))
			pdk.SetError(fmt.Errorf("telegram polling get_updates failed (offset=%d): %w", u.Offset, err))
			return 1
		}

		if len(updates) == 0 {
			idleCycles++
			if idleCycles%30 == 0 {
				pdk.Log(pdk.LogInfo, fmt.Sprintf("telegram polling idle: cycles=%d offset=%d", idleCycles, u.Offset))
			}
			continue
		}

		idleCycles = 0
		firstUpdateID := updates[0].UpdateID
		lastUpdateID := updates[len(updates)-1].UpdateID
		pdk.Log(pdk.LogInfo, fmt.Sprintf("telegram polling batch: count=%d update_id_range=%d-%d offset=%d", len(updates), firstUpdateID, lastUpdateID, u.Offset))

		for _, update := range updates {
			if update.UpdateID >= u.Offset {
				u.Offset = update.UpdateID + 1
			}

			if update.Message == nil {
				continue
			}
			// Skip messages from the bot itself when self id is known.
			if bot.Self.ID != 0 {
				if from := update.Message.From; from != nil && from.ID == bot.Self.ID {
					continue
				}
			}

			tgMsg := update.Message
			senderID := ""
			senderName := ""
			if from := tgMsg.From; from != nil {
				senderID = strconv.FormatInt(from.ID, 10)
				if from.UserName != "" {
					senderName = "@" + from.UserName
				} else {
					senderName = strings.TrimSpace(from.FirstName + " " + from.LastName)
				}
			}

			isGroup := tgMsg.Chat.IsGroup() || tgMsg.Chat.IsSuperGroup() || tgMsg.Chat.IsChannel()
			groupName := ""
			if isGroup {
				groupName = tgMsg.Chat.Title
			}

			content := tgMsg.Text
			if content == "" && tgMsg.Caption != "" {
				content = tgMsg.Caption
			}

			isMentioned := !isGroup
			if isGroup {
				if botUsername != "" {
					isMentioned = strings.Contains(strings.ToLower(content), "@"+botUsername)
				}
				if !isMentioned && bot.Self.ID != 0 && tgMsg.ReplyToMessage != nil && tgMsg.ReplyToMessage.From != nil {
					isMentioned = tgMsg.ReplyToMessage.From.ID == bot.Self.ID
				}
			}

			emitMessage(&pluginMessage{
				ChannelID:   strconv.FormatInt(tgMsg.Chat.ID, 10),
				SenderID:    senderID,
				SenderName:  senderName,
				Content:     content,
				IsGroup:     isGroup,
				IsMentioned: isMentioned,
				GroupName:   groupName,
				Timestamp:   tgMsg.Time(),
				Metadata:    map[string]interface{}{"channel_type": "telegram"},
			})
		}
	}

}

func main() {
	pdk.MustRun(pdk.Plugin{
		ID: "openlobster-messages-telegram",
		Exports: map[string]pdk.Function{
			"get_metadata":       getMetadata,
			"inbound_mode":       inboundMode,
			"capabilities":       capabilities,
			"resolve_channel_id": resolveChannelID,
			"configure":          configureHot,
			"send":               send,
			"typing":             typing,
			"handle_webhook":     handleWebhook,
			"start":              start,
		},
	})
}
