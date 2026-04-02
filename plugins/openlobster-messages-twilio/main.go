package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	pdk "github.com/extism/go-pdk"
	_ "github.com/stealthrocket/net/http"
	twilio "github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
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
	pdk.OutputString(`{"type":"object","properties":{"account_sid":{"type":"string","title":"Account SID","description":"Twilio Account SID"},"auth_token":{"type":"string","title":"Auth Token","description":"Twilio Auth Token"},"from_number":{"type":"string","title":"From Phone Number","description":"Twilio sender number in E.164 format"},"default_to_number":{"type":"string","title":"Default Recipient Number (optional)","description":"Fallback recipient number used when message.channel_id is the logical channel slug"}},"required":["account_sid","auth_token","from_number"]}`)
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

type resolveChannelIDInput struct {
	Config  map[string]interface{} `json:"config"`
	Message struct {
		ChannelID string                 `json:"channel_id"`
		Metadata  map[string]interface{} `json:"metadata,omitempty"`
	} `json:"message"`
}

type webhookInput struct {
	Config  map[string]interface{} `json:"config"`
	Request struct {
		Method  string              `json:"method"`
		Path    string              `json:"path"`
		Query   map[string][]string `json:"query,omitempty"`
		Headers map[string][]string `json:"headers,omitempty"`
		Body    string              `json:"body,omitempty"`
	} `json:"request"`
}

func resolveTwilioChannelID(input resolveChannelIDInput) (string, error) {
	channelID := strings.TrimSpace(input.Message.ChannelID)
	if channelID != "" && !strings.EqualFold(channelID, "twilio") {
		return channelID, nil
	}

	if input.Message.Metadata != nil {
		if v, ok := input.Message.Metadata["twilio_to"].(string); ok {
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

	if fallback, _ := input.Config["default_to_number"].(string); strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback), nil
	}

	return "", fmt.Errorf("twilio resolve_channel_id: missing recipient number (set message.metadata.twilio_to or config.default_to_number)")
}

//go:wasmexport resolve_channel_id
func resolveChannelID() int32 {
	var input resolveChannelIDInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	channelID, err := resolveTwilioChannelID(input)
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

	resolveInput := resolveChannelIDInput{Config: input.Config}
	resolveInput.Message.ChannelID = input.Message.ChannelID
	resolvedChannelID, err := resolveTwilioChannelID(resolveInput)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	params := &twilioApi.CreateMessageParams{}
	params.SetTo(resolvedChannelID)
	params.SetFrom(fromNumber)
	params.SetBody(input.Message.Content)

	if _, err := client.Api.CreateMessage(params); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// handle_webhook — inbound Twilio webhook payload processor
// ---------------------------------------------------------------------------

//go:wasmexport handle_webhook
func handleWebhook() int32 {
	var input webhookInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	if strings.ToUpper(strings.TrimSpace(input.Request.Method)) != "POST" {
		pdk.SetError(fmt.Errorf("invalid webhook method: %s", input.Request.Method))
		return 1
	}

	values, err := url.ParseQuery(input.Request.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("invalid twilio webhook body: %w", err))
		return 1
	}

	from := strings.TrimSpace(values.Get("From"))
	if from == "" {
		pdk.OutputString("")
		return 0
	}
	bodyText := values.Get("Body")
	senderName := strings.TrimSpace(values.Get("ProfileName"))
	if senderName == "" {
		senderName = from
	}

	msg := pluginMessage{
		ChannelID:  from,
		SenderID:   from,
		SenderName: senderName,
		Content:    bodyText,
		IsGroup:    false,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"channel_type":        "twilio",
			"platform_channel_id": from,
		},
	}

	if err := pdk.OutputJSON(msg); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// start — no-op (webhooks are handled by the host route /webhook/{channel_id})
// ---------------------------------------------------------------------------

//go:wasmexport start
func start() int32 {
	return 0
}

func main() {}
