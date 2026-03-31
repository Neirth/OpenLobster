package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"unsafe"

	_ "github.com/stealthrocket/net/wasip1"
)

var (
	inputBuf  []byte
	resultBuf []byte
)

//go:wasmexport openlobster_alloc_input
func allocInput(size uint32) uint32 {
	inputBuf = make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&inputBuf[0])))
}

//go:wasmexport openlobster_result_ptr
func resultPtr() uint32 {
	if len(resultBuf) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
}

//go:wasmexport openlobster_result_len
func resultLen() uint32 {
	return uint32(len(resultBuf))
}

func writeResult(v interface{}) int32 {
	b, err := json.Marshal(v)
	if err != nil {
		resultBuf = []byte(`{"error":"marshal failed"}`)
		return 1
	}
	resultBuf = b
	return 0
}

func writeStringResult(s string) int64 {
	resultBuf = []byte(s)
	ptr := uint32(uintptr(unsafe.Pointer(&resultBuf[0])))
	return int64(ptr)<<32 | int64(len(resultBuf))
}

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_get_name
func getName() int64 { return writeStringResult("openlobster-messages-whatsapp") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("WhatsApp Business Cloud API messaging plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("messaging") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"access_token":{"type":"string","title":"Access Token"},"phone_number_id":{"type":"string","title":"Phone Number ID"},"verify_token":{"type":"string","title":"Webhook Verify Token"}},"required":["access_token","phone_number_id"]}`)
}

// ---------------------------------------------------------------------------
// Host function
// ---------------------------------------------------------------------------

//go:wasmimport openlobster host_emit_message
func hostEmitMessage(ptr uint32, size uint32)

func emitMessage(msg map[string]interface{}) {
	b, _ := json.Marshal(msg)
	if len(b) == 0 {
		return
	}
	hostEmitMessage(uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
}

// ---------------------------------------------------------------------------
// WhatsApp Cloud API types
// ---------------------------------------------------------------------------

const waAPI = "https://graph.facebook.com/v18.0"

// openlobster_start: WhatsApp uses webhooks, so this function starts an HTTP
// server to receive webhook events from Meta.
//
//go:wasmexport openlobster_start
func start() int32 {
	var input struct {
		Config struct {
			AccessToken   string `json:"access_token"`
			PhoneNumberID string `json:"phone_number_id"`
			VerifyToken   string `json:"verify_token"`
			WebhookPort   int    `json:"webhook_port,omitempty"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}

	port := input.Config.WebhookPort
	if port == 0 {
		port = 8088
	}
	verifyToken := input.Config.VerifyToken

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Webhook verification
			mode := r.URL.Query().Get("hub.mode")
			token := r.URL.Query().Get("hub.verify_token")
			challenge := r.URL.Query().Get("hub.challenge")
			if mode == "subscribe" && token == verifyToken {
				w.Write([]byte(challenge))
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if r.Method != http.MethodPost {
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}

		var payload struct {
			Entry []struct {
				Changes []struct {
					Value struct {
						Messages []struct {
							From string `json:"from"`
							ID   string `json:"id"`
							Text struct {
								Body string `json:"body"`
							} `json:"text"`
							Type string `json:"type"`
						} `json:"messages"`
						Contacts []struct {
							WaID    string `json:"wa_id"`
							Profile struct {
								Name string `json:"name"`
							} `json:"profile"`
						} `json:"contacts"`
					} `json:"value"`
				} `json:"changes"`
			} `json:"entry"`
		}

		if err := json.Unmarshal(body, &payload); err != nil {
			return
		}

		contactNames := make(map[string]string)
		for _, entry := range payload.Entry {
			for _, change := range entry.Changes {
				for _, contact := range change.Value.Contacts {
					contactNames[contact.WaID] = contact.Profile.Name
				}
				for _, msg := range change.Value.Messages {
					if msg.Type != "text" {
						continue
					}
					senderName := contactNames[msg.From]
					if senderName == "" {
						senderName = msg.From
					}
					emitMessage(map[string]interface{}{
						"channel_id":   msg.From,
						"sender_id":    msg.From,
						"sender_name":  senderName,
						"content":      msg.Text.Body,
						"is_group":     false,
						"is_mentioned": false,
						"metadata":     map[string]string{"channel_type": "whatsapp"},
					})
				}
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	return 0
}

// ---------------------------------------------------------------------------
// openlobster_send
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_send
func send() int32 {
	var input struct {
		ChannelID string `json:"channel_id"` // recipient phone number
		Content   string `json:"content"`
		Config    struct {
			AccessToken   string `json:"access_token"`
			PhoneNumberID string `json:"phone_number_id"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}

	apiURL := fmt.Sprintf("%s/%s/messages", waAPI, input.Config.PhoneNumberID)
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                input.ChannelID,
		"type":              "text",
		"text":              map[string]string{"body": input.Content},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+input.Config.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	defer resp.Body.Close()
	return writeResult(map[string]bool{"ok": true})
}

//go:wasmexport openlobster_configure
func configure() int32 {
	return writeResult(map[string]bool{"ok": true})
}

func main() {}
