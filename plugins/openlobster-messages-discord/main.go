package main

import (
"context"
"encoding/json"
"fmt"
"strconv"
"time"

"github.com/diamondburned/arikawa/v3/api"
"github.com/diamondburned/arikawa/v3/discord"
pdk "github.com/extism/go-pdk"
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
pdk.OutputString(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token"},"guild_id":{"type":"string","title":"Guild ID (optional)"}},"required":["token"]}`)
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

sf, err := discord.ParseSnowflake(input.Message.ChannelID)
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

// ---------------------------------------------------------------------------
// start — REST polling loop (Discord REST API, no WebSocket needed)
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

guildIDStr, _ := cfg["guild_id"].(string)

client := api.NewClient("Bot " + token)

// Discover text channels to poll
var channelIDs []discord.ChannelID
if guildIDStr != "" {
sf, err := discord.ParseSnowflake(guildIDStr)
if err == nil {
channels, err := client.Channels(discord.GuildID(sf))
if err == nil {
for _, ch := range channels {
if ch.Type == discord.GuildText {
channelIDs = append(channelIDs, ch.ID)
}
}
}
}
}

lastIDs := make(map[discord.ChannelID]discord.MessageID)
_ = context.Background()

for {
for _, chID := range channelIDs {
msgs, err := client.Messages(chID, 10)
if err != nil {
continue
}
// arikawa returns messages newest-first
for i := len(msgs) - 1; i >= 0; i-- {
m := msgs[i]
if last, ok := lastIDs[chID]; ok && m.ID <= last {
continue
}
lastIDs[chID] = m.ID
emitMessage(&pluginMessage{
ChannelID:  strconv.FormatUint(uint64(m.ChannelID), 10),
SenderID:   strconv.FormatUint(uint64(m.Author.ID), 10),
SenderName: m.Author.Username,
Content:    m.Content,
IsGroup:    true,
Timestamp:  m.Timestamp.Time(),
Metadata:   map[string]interface{}{"channel_type": "discord"},
})
}
}
time.Sleep(2 * time.Second)
}
}

func main() {}
