package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
func getName() int64 { return writeStringResult("openlobster-messages-slack") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("Slack Bot messaging plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("messaging") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"bot_token":{"type":"string","title":"Bot Token (xoxb-)"},"app_token":{"type":"string","title":"App Token (xapp-) for Socket Mode"}},"required":["bot_token"]}`)
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
// Slack types
// ---------------------------------------------------------------------------

const slackAPI = "https://slack.com/api"

func slackGet(token, method string, params map[string]string) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("%s/%s", slackAPI, method)
	req, _ := http.NewRequest("GET", apiURL, nil)
	q := req.URL.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// ---------------------------------------------------------------------------
// openlobster_start — polls conversations.history for new messages
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_start
func start() int32 {
	var input struct {
		Config struct {
			BotToken string `json:"bot_token"`
			Channel  string `json:"channel,omitempty"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}
	token := input.Config.BotToken
	if token == "" {
		resultBuf = []byte(`{"error":"slack bot_token required"}`)
		return 1
	}

	// Get list of channels if none specified
	channels := []string{}
	if input.Config.Channel != "" {
		channels = append(channels, input.Config.Channel)
	} else {
		result, err := slackGet(token, "conversations.list", map[string]string{"limit": "200"})
		if err == nil {
			if chans, ok := result["channels"].([]interface{}); ok {
				for _, ch := range chans {
					if chMap, ok := ch.(map[string]interface{}); ok {
						if id, ok := chMap["id"].(string); ok {
							channels = append(channels, id)
						}
					}
				}
			}
		}
	}

	lastTS := make(map[string]string)

	for {
		for _, channelID := range channels {
			params := map[string]string{"channel": channelID, "limit": "10"}
			if ts, ok := lastTS[channelID]; ok {
				params["oldest"] = ts
			}
			result, err := slackGet(token, "conversations.history", params)
			if err != nil {
				continue
			}
			msgs, _ := result["messages"].([]interface{})
			for i := len(msgs) - 1; i >= 0; i-- {
				msgMap, ok := msgs[i].(map[string]interface{})
				if !ok {
					continue
				}
				ts, _ := msgMap["ts"].(string)
				if ts == "" {
					continue
				}
				if lastTS[channelID] == ts {
					continue
				}
				lastTS[channelID] = ts
				text, _ := msgMap["text"].(string)
				userID, _ := msgMap["user"].(string)
				emitMessage(map[string]interface{}{
					"channel_id":   channelID,
					"sender_id":    userID,
					"sender_name":  userID,
					"content":      text,
					"is_group":     true,
					"is_mentioned": false,
					"metadata":     map[string]string{"channel_type": "slack", "ts": ts},
				})
			}
		}
		time.Sleep(3 * time.Second)
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
			BotToken string `json:"bot_token"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}

	body, _ := json.Marshal(map[string]string{
		"channel": input.ChannelID,
		"text":    input.Content,
	})
	req, err := http.NewRequest("POST", slackAPI+"/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+input.Config.BotToken)
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
