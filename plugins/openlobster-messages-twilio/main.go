package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
func getName() int64 { return writeStringResult("openlobster-messages-twilio") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("Twilio SMS messaging plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("messaging") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"account_sid":{"type":"string","title":"Account SID"},"auth_token":{"type":"string","title":"Auth Token"},"from_number":{"type":"string","title":"From Phone Number"}},"required":["account_sid","auth_token","from_number"]}`)
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
// openlobster_start — HTTP webhook server for inbound Twilio SMS
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_start
func start() int32 {
	var input struct {
		Config struct {
			AccountSID  string `json:"account_sid"`
			AuthToken   string `json:"auth_token"`
			FromNumber  string `json:"from_number"`
			WebhookPort int    `json:"webhook_port,omitempty"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}

	port := input.Config.WebhookPort
	if port == 0 {
		port = 8089
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
		emitMessage(map[string]interface{}{
			"channel_id":   from,
			"sender_id":    from,
			"sender_name":  from,
			"content":      body,
			"is_group":     false,
			"is_mentioned": false,
			"metadata":     map[string]string{"channel_type": "twilio"},
		})
		// Return TwiML
		w.Header().Set("Content-Type", "text/xml")
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Response></Response>`)
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
			AccountSID string `json:"account_sid"`
			AuthToken  string `json:"auth_token"`
			FromNumber string `json:"from_number"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", input.Config.AccountSID)
	data := url.Values{
		"From": {input.Config.FromNumber},
		"To":   {input.ChannelID},
		"Body": {input.Content},
	}
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(input.Config.AccountSID, input.Config.AuthToken)

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
