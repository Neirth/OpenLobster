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
func getName() int64 { return writeStringResult("openlobster-messages-discord") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("Discord Bot messaging plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("messaging") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{"type":"object","properties":{"token":{"type":"string","title":"Bot Token"}},"required":["token"]}`)
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
// Discord Gateway types (minimal — receive MESSAGE_CREATE events via polling)
// ---------------------------------------------------------------------------

const discordAPI = "https://discord.com/api/v10"

type discordMessage struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Content   string `json:"content"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"author"`
}

// Discord doesn't support long-polling. We use the REST channel messages endpoint
// to simulate polling (simplified approach without WebSocket for WASM compatibility).
func pollChannel(token, channelID string, lastID *string) ([]discordMessage, error) {
	apiURL := fmt.Sprintf("%s/channels/%s/messages?limit=10", discordAPI, channelID)
	if *lastID != "" {
		apiURL += "&after=" + *lastID
	}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var msgs []discordMessage
	if err := json.Unmarshal(body, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// getGuildChannels lists text channels in a guild.
func getGuildChannels(token, guildID string) ([]string, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/guilds/%s/channels", discordAPI, guildID), nil)
	req.Header.Set("Authorization", "Bot "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var channels []struct {
		ID   string `json:"id"`
		Type int    `json:"type"`
	}
	if err := json.Unmarshal(body, &channels); err != nil {
		return nil, err
	}
	var ids []string
	for _, ch := range channels {
		if ch.Type == 0 { // GUILD_TEXT
			ids = append(ids, ch.ID)
		}
	}
	return ids, nil
}

// ---------------------------------------------------------------------------
// openlobster_start — REST polling loop
// ---------------------------------------------------------------------------

//go:wasmexport openlobster_start
func start() int32 {
	var input struct {
		Config struct {
			Token   string `json:"token"`
			GuildID string `json:"guild_id,omitempty"`
		} `json:"config"`
	}
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}
	token := input.Config.Token
	if token == "" {
		resultBuf = []byte(`{"error":"discord token required"}`)
		return 1
	}

	channelLastIDs := make(map[string]string)

	// Get channels to poll
	var channelIDs []string
	if input.Config.GuildID != "" {
		ids, err := getGuildChannels(token, input.Config.GuildID)
		if err == nil {
			channelIDs = ids
		}
	}

	for {
		for _, chID := range channelIDs {
			lastID := channelLastIDs[chID]
			msgs, err := pollChannel(token, chID, &lastID)
			if err != nil {
				continue
			}
			for _, msg := range msgs {
				channelLastIDs[chID] = msg.ID
				emitMessage(map[string]interface{}{
					"channel_id":   msg.ChannelID,
					"sender_id":    msg.Author.ID,
					"sender_name":  msg.Author.Username,
					"content":      msg.Content,
					"is_group":     true,
					"is_mentioned": false,
					"metadata":     map[string]string{"channel_type": "discord"},
				})
			}
		}
		time.Sleep(2 * time.Second)
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

	body, _ := json.Marshal(map[string]string{"content": input.Content})
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/channels/%s/messages", discordAPI, input.ChannelID), bytes.NewReader(body))
	if err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+input.Config.Token)
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
