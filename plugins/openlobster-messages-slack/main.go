package main

import (
"context"
"encoding/json"
"fmt"
"time"

slackgo "github.com/slack-go/slack"
pdk "github.com/extism/go-pdk"
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
pdk.OutputString(`{"type":"object","properties":{"bot_token":{"type":"string","title":"Bot Token (xoxb-)"},"channel":{"type":"string","title":"Default Channel (optional)"}},"required":["bot_token"]}`)
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
_, _, err := client.PostMessage(input.Message.ChannelID, slackgo.MsgOptionText(input.Message.Content, false))
if err != nil {
pdk.SetError(err)
return 1
}
return 0
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
lastTS[channelID] = m.Timestamp
emitMessage(&pluginMessage{
ChannelID:  channelID,
SenderID:   m.User,
SenderName: m.User,
Content:    m.Text,
IsGroup:    true,
Timestamp:  time.Now(),
Metadata:   map[string]interface{}{"channel_type": "slack", "ts": m.Timestamp},
})
}
}
time.Sleep(3 * time.Second)
}
}

func main() {}
