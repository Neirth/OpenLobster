package main

import (
"context"
"encoding/json"
"fmt"
"strconv"
"strings"
"time"

tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
pdk "github.com/extism/go-pdk"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-messages-telegram"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
pdk.OutputString("Telegram Bot API messaging plugin for OpenLobster")
return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("messaging"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
pdk.OutputString(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token"}},"required":["token"]}`)
return 0
}

// ---------------------------------------------------------------------------
// Host emit_message (Extism PDK offset-based)
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

//go:wasmexport capabilities
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
ChannelID string `json:"channel_id"`
Content   string `json:"content"`
} `json:"message"`
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
pdk.SetError(fmt.Errorf("telegram token required"))
return 1
}

bot, err := tgbotapi.NewBotAPI(token)
if err != nil {
pdk.SetError(fmt.Errorf("bot init: %w", err))
return 1
}

chatID, err := strconv.ParseInt(input.Message.ChannelID, 10, 64)
if err != nil {
pdk.SetError(fmt.Errorf("invalid channel_id: %w", err))
return 1
}

msg := tgbotapi.NewMessage(chatID, input.Message.Content)
msg.ParseMode = tgbotapi.ModeHTML
if _, err := bot.Send(msg); err != nil {
pdk.SetError(err)
return 1
}
return 0
}

// ---------------------------------------------------------------------------
// start — long-polling loop
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
pdk.SetError(fmt.Errorf("telegram token required"))
return 1
}

bot, err := tgbotapi.NewBotAPI(token)
if err != nil {
pdk.SetError(fmt.Errorf("bot init: %w", err))
return 1
}

u := tgbotapi.NewUpdate(0)
u.Timeout = 60
updates := bot.GetUpdatesChan(u)

ctx := context.Background()
for {
select {
case <-ctx.Done():
bot.StopReceivingUpdates()
return 0
case update, ok := <-updates:
if !ok {
return 0
}
if update.Message == nil {
continue
}
// Skip messages from the bot itself
if from := update.Message.From; from != nil && from.ID == bot.Self.ID {
continue
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

emitMessage(&pluginMessage{
ChannelID:  strconv.FormatInt(tgMsg.Chat.ID, 10),
SenderID:   senderID,
SenderName: senderName,
Content:    content,
IsGroup:    isGroup,
GroupName:  groupName,
Timestamp:  tgMsg.Time(),
Metadata:   map[string]interface{}{"channel_type": "telegram"},
})
}
}
}

func main() {}
