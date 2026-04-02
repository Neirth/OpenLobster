package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	pdk "github.com/extism/go-pdk"
	_ "github.com/stealthrocket/net/http"
	wapiClient "github.com/wapikit/wapi.go/pkg/client"
	wapiComponents "github.com/wapikit/wapi.go/pkg/components"
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
	pdk.OutputString(`{"type":"object","properties":{"api_access_token":{"type":"string","title":"API Access Token","description":"WhatsApp Cloud API access token"},"business_account_id":{"type":"string","title":"Business Account ID","description":"Meta Business Account ID (optional but recommended)"},"phone_number_id":{"type":"string","title":"Phone Number ID","description":"WhatsApp phone number ID used for sends"},"default_to_number":{"type":"string","title":"Default Recipient Number (optional)","description":"Fallback recipient number used when message.channel_id is the logical channel slug"},"webhook_secret":{"type":"string","title":"Webhook Secret","description":"Secret used to validate incoming webhook signatures"}},"required":["api_access_token","phone_number_id"]}`)
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

type whatsAppWebhookPayload struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Metadata struct {
					PhoneNumberID string `json:"phone_number_id"`
				} `json:"metadata"`
				Contacts []struct {
					Profile struct {
						Name string `json:"name"`
					} `json:"profile"`
				} `json:"contacts"`
				Messages []struct {
					From      string `json:"from"`
					Timestamp string `json:"timestamp"`
					Type      string `json:"type"`
					Text      struct {
						Body string `json:"body"`
					} `json:"text"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}

func resolveWhatsAppChannelID(input resolveChannelIDInput) (string, error) {
	channelID := strings.TrimSpace(input.Message.ChannelID)
	if channelID != "" && !strings.EqualFold(channelID, "whatsapp") {
		return channelID, nil
	}

	if input.Message.Metadata != nil {
		if v, ok := input.Message.Metadata["whatsapp_to"].(string); ok {
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

	return "", fmt.Errorf("whatsapp resolve_channel_id: missing recipient number (set message.metadata.whatsapp_to or config.default_to_number)")
}

//go:wasmexport resolve_channel_id
func resolveChannelID() int32 {
	var input resolveChannelIDInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	channelID, err := resolveWhatsAppChannelID(input)
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

	resolveInput := resolveChannelIDInput{Config: input.Config}
	resolveInput.Message.ChannelID = input.Message.ChannelID
	resolvedChannelID, err := resolveWhatsAppChannelID(resolveInput)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	if _, err := mc.Message.Send(textMsg, resolvedChannelID); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// handle_webhook — inbound WhatsApp webhook payload processor
// ---------------------------------------------------------------------------

func parseWhatsAppTimestamp(raw string) time.Time {
	ts, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || ts <= 0 {
		return time.Now()
	}
	return time.Unix(ts, 0).UTC()
}

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

	var payload whatsAppWebhookPayload
	if err := json.Unmarshal([]byte(input.Request.Body), &payload); err != nil {
		pdk.SetError(fmt.Errorf("invalid whatsapp webhook body: %w", err))
		return 1
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			senderName := ""
			if len(change.Value.Contacts) > 0 {
				senderName = strings.TrimSpace(change.Value.Contacts[0].Profile.Name)
			}
			for _, inMsg := range change.Value.Messages {
				from := strings.TrimSpace(inMsg.From)
				if from == "" {
					continue
				}

				if senderName == "" {
					senderName = from
				}

				msg := pluginMessage{
					ChannelID:  from,
					SenderID:   from,
					SenderName: senderName,
					Content:    strings.TrimSpace(inMsg.Text.Body),
					Timestamp:  parseWhatsAppTimestamp(inMsg.Timestamp),
					Metadata: map[string]interface{}{
						"channel_type":        "whatsapp",
						"platform_channel_id": from,
						"phone_number_id":     change.Value.Metadata.PhoneNumberID,
						"message_type":        inMsg.Type,
					},
				}

				if err := pdk.OutputJSON(msg); err != nil {
					pdk.SetError(err)
					return 1
				}
				return 0
			}
		}
	}

	pdk.OutputString("")
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
