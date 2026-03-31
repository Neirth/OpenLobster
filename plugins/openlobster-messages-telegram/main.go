package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
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
func getName() int64 { return writeStringResult("openlobster-messages-telegram") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("Telegram Bot API messaging plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("messaging") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token"}},"required":["token"]}`)
}

// ---------------------------------------------------------------------------
// Host function: emit message to the OpenLobster runtime
// ---------------------------------------------------------------------------

//go:wasmimport openlobster host_emit_message
func hostEmitMessage(ptr uint32, size uint32)

func emitMessage(msg map[string]interface{}) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	if len(b) == 0 {
		return
	}
	hostEmitMessage(uint32(uintptr(unsafe.Pointer(&b[0]))), uint32(len(b)))
}

// ---------------------------------------------------------------------------
// Telegram types
// ---------------------------------------------------------------------------

type tgUpdate struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int `json:"message_id"`
		From      *struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

type tgGetUpdatesResponse struct {
	OK     bool       `json:"ok"`
	Result []tgUpdate `json:"result"`
}

// ---------------------------------------------------------------------------
// openlobster_start — long-polling loop (blocks until context cancelled)
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_start
func start() int32 {
	var input struct {
		Config struct {
			Token string `json:"token"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}
	token := input.Config.Token
	if token == "" {
		resultBuf = []byte(`{"error":"telegram token required"}`)
		return 1
	}

	baseURL := "https://api.telegram.org/bot" + token
	offset := 0

	for {
		apiURL := fmt.Sprintf("%s/getUpdates?timeout=30&offset=%d", baseURL, offset)
		resp, err := http.Get(apiURL)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		var updates tgGetUpdatesResponse
		if err := json.Unmarshal(body, &updates); err != nil || !updates.OK {
			time.Sleep(5 * time.Second)
			continue
		}

		for _, upd := range updates.Result {
			offset = upd.UpdateID + 1
			if upd.Message == nil {
				continue
			}
			msg := upd.Message
			senderID := ""
			senderName := ""
			if msg.From != nil {
				senderID = strconv.FormatInt(msg.From.ID, 10)
				senderName = msg.From.FirstName
				if msg.From.LastName != "" {
					senderName += " " + msg.From.LastName
				}
				if msg.From.Username != "" {
					senderName = "@" + msg.From.Username
				}
			}
			emitMessage(map[string]interface{}{
				"channel_id":   strconv.FormatInt(msg.Chat.ID, 10),
				"sender_id":    senderID,
				"sender_name":  senderName,
				"content":      msg.Text,
				"is_group":     msg.Chat.Type != "private",
				"is_mentioned": false,
				"metadata":     map[string]string{"channel_type": "telegram"},
			})
		}
	}
}

// ---------------------------------------------------------------------------
// openlobster_send
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_send
func send() int32 {
	var input struct {
		ChannelID string `json:"channel_id"`
		Content   string `json:"content"`
		Config    struct {
			Token string `json:"token"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", input.Config.Token)
	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id": {input.ChannelID},
		"text":    {input.Content},
	})
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
