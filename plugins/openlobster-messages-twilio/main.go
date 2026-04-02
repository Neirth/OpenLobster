package main

import (
"encoding/json"
"fmt"
"io"
"net/http"
"time"

twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
twilio "github.com/twilio/twilio-go"
pdk "github.com/extism/go-pdk"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport get_name
func getName() int32 { pdk.OutputString("openlobster-messages-twilio"); return 0 }

//go:wasmexport get_version
func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

//go:wasmexport get_description
func getDescription() int32 {
pdk.OutputString("Twilio SMS messaging plugin for OpenLobster")
return 0
}

//go:wasmexport get_type
func getType() int32 { pdk.OutputString("messaging"); return 0 }

//go:wasmexport get_schema
func getSchema() int32 {
pdk.OutputString(`{"type":"object","properties":{"account_sid":{"type":"string","title":"Account SID"},"auth_token":{"type":"string","title":"Auth Token"},"from_number":{"type":"string","title":"From Phone Number"},"webhook_port":{"type":"integer","title":"Webhook Listen Port","default":8089}},"required":["account_sid","auth_token","from_number"]}`)
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

accountSID, _ := input.Config["account_sid"].(string)
authToken, _ := input.Config["auth_token"].(string)
fromNumber, _ := input.Config["from_number"].(string)

if accountSID == "" || authToken == "" {
pdk.SetError(fmt.Errorf("account_sid and auth_token required"))
return 1
}

client := twilio.NewRestClientWithParams(twilio.ClientParams{
Username:   accountSID,
Password:   authToken,
AccountSid: accountSID,
})

params := &twilioApi.CreateMessageParams{}
params.SetTo(input.Message.ChannelID)
params.SetFrom(fromNumber)
params.SetBody(input.Message.Content)

if _, err := client.Api.CreateMessage(params); err != nil {
pdk.SetError(err)
return 1
}
return 0
}

// ---------------------------------------------------------------------------
// start — HTTP webhook server for inbound Twilio SMS
// ---------------------------------------------------------------------------

//go:wasmexport start
func start() int32 {
var cfg map[string]interface{}
if err := pdk.InputJSON(&cfg); err != nil {
pdk.SetError(err)
return 1
}

port := 8089
if p, ok := cfg["webhook_port"].(float64); ok && p > 0 {
port = int(p)
}

mux := http.NewServeMux()
mux.HandleFunc("/sms", func(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
return
}
if err := r.ParseForm(); err != nil {
return
}
from := r.FormValue("From")
body := r.FormValue("Body")
if from == "" {
return
}
emitMessage(&pluginMessage{
ChannelID:  from,
SenderID:   from,
SenderName: from,
Content:    body,
IsGroup:    false,
Timestamp:  time.Now(),
Metadata:   map[string]interface{}{"channel_type": "twilio"},
})
w.Header().Set("Content-Type", "text/xml")
io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Response></Response>`)
})

http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
return 0
}

func main() {}
