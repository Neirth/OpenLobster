package main

import (
"encoding/json"
"fmt"
"time"

wapiClient "github.com/wapikit/wapi.go/pkg/client"
wapiComponents "github.com/wapikit/wapi.go/pkg/components"
wapiEvents "github.com/wapikit/wapi.go/pkg/events"
pdk "github.com/extism/go-pdk"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-messages-whatsapp"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
pdk.OutputString("WhatsApp Cloud API messaging plugin for OpenLobster")
return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("messaging"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
pdk.OutputString(`{"type":"object","properties":{"api_access_token":{"type":"string","title":"API Access Token"},"business_account_id":{"type":"string","title":"Business Account ID"},"phone_number_id":{"type":"string","title":"Phone Number ID"},"webhook_secret":{"type":"string","title":"Webhook Secret"},"webhook_path":{"type":"string","title":"Webhook Path","default":"/webhook"},"webhook_port":{"type":"integer","title":"Webhook Port","default":8090}},"required":["api_access_token","phone_number_id"]}`)
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

apiToken, _ := input.Config["api_access_token"].(string)
businessAccountID, _ := input.Config["business_account_id"].(string)
phoneNumberID, _ := input.Config["phone_number_id"].(string)
webhookSecret, _ := input.Config["webhook_secret"].(string)

if apiToken == "" || phoneNumberID == "" {
pdk.SetError(fmt.Errorf("api_access_token and phone_number_id required"))
return 1
}
if webhookSecret == "" {
webhookSecret = "placeholder"
}

client := wapiClient.New(&wapiClient.ClientConfig{
BusinessAccountId: businessAccountID,
ApiAccessToken:    apiToken,
WebhookSecret:     webhookSecret,
})

mc := client.NewMessagingClient(phoneNumberID)

textMsg, err := wapiComponents.NewTextMessage(wapiComponents.TextMessageConfigs{
Text: input.Message.Content,
})
if err != nil {
pdk.SetError(fmt.Errorf("create text message: %w", err))
return 1
}

if _, err := mc.Message.Send(textMsg, input.Message.ChannelID); err != nil {
pdk.SetError(err)
return 1
}
return 0
}

// ---------------------------------------------------------------------------
// start — webhook server using wapi.go
// ---------------------------------------------------------------------------

//go:wasmexport start
func start() int32 {
var cfg map[string]interface{}
if err := pdk.InputJSON(&cfg); err != nil {
pdk.SetError(err)
return 1
}

apiToken, _ := cfg["api_access_token"].(string)
businessAccountID, _ := cfg["business_account_id"].(string)
webhookSecret, _ := cfg["webhook_secret"].(string)
webhookPath, _ := cfg["webhook_path"].(string)
if webhookPath == "" {
webhookPath = "/webhook"
}
webhookPort := 8090
if p, ok := cfg["webhook_port"].(float64); ok && p > 0 {
webhookPort = int(p)
}
if webhookSecret == "" {
webhookSecret = "placeholder"
}

client := wapiClient.New(&wapiClient.ClientConfig{
BusinessAccountId: businessAccountID,
ApiAccessToken:    apiToken,
WebhookSecret:     webhookSecret,
WebhookPath:       webhookPath,
WebhookServerPort: webhookPort,
})

client.On(wapiEvents.TextMessageEventType, func(event wapiEvents.BaseEvent) {
textEvent, ok := event.(*wapiEvents.TextMessageEvent)
if !ok {
return
}
emitMessage(&pluginMessage{
ChannelID:  textEvent.From,
SenderID:   textEvent.From,
SenderName: textEvent.From,
Content:    textEvent.Text,
Timestamp:  time.Now(),
Metadata:   map[string]interface{}{"channel_type": "whatsapp"},
})
})

client.Initiate()
return 0
}

func main() {}
