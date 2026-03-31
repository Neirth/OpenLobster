package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
func getName() int64 { return writeStringResult("openlobster-audio-elevenlabs") }

//go:wasmexport openlobster_get_version
func getVersion() int64 { return writeStringResult("0.1.0") }

//go:wasmexport openlobster_get_description
func getDescription() int64 {
	return writeStringResult("ElevenLabs TTS/STT audio plugin for OpenLobster")
}

//go:wasmexport openlobster_get_type
func getType() int64 { return writeStringResult("audio") }

//go:wasmexport openlobster_get_schema
func getSchema() int64 {
	return writeStringResult(`{
  "type": "object",
  "properties": {
    "api_key":       {"type":"string","title":"API Key"},
    "voice_id":      {"type":"string","title":"Voice ID","default":"21m00Tcm4TlvDq8ikWAM"},
    "model_id":      {"type":"string","title":"TTS Model","default":"eleven_multilingual_v2"},
    "stt_model_id":  {"type":"string","title":"STT Model","default":"scribe_v1"},
    "output_format": {"type":"string","title":"Audio Output Format","default":"mp3_44100_128"}
  },
  "required": ["api_key"]
}`)
}

// ---------------------------------------------------------------------------
// TTS
// ---------------------------------------------------------------------------

const elevenLabsAPI = "https://api.elevenlabs.io/v1"

type ttsInput struct {
	Text    string `json:"text"`
	VoiceID string `json:"voice_id,omitempty"`
	Config  struct {
		APIKey       string `json:"api_key"`
		VoiceID      string `json:"voice_id,omitempty"`
		ModelID      string `json:"model_id,omitempty"`
		OutputFormat string `json:"output_format,omitempty"`
	} `json:"config"`
}

//go:wasmexport openlobster_tts
func tts() int32 {
	var input ttsInput
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}

	apiKey := input.Config.APIKey
	if apiKey == "" {
		resultBuf = []byte(`{"error":"api_key required"}`)
		return 1
	}

	voiceID := input.Config.VoiceID
	if voiceID == "" {
		voiceID = input.VoiceID
	}
	if voiceID == "" {
		voiceID = "21m00Tcm4TlvDq8ikWAM"
	}

	modelID := input.Config.ModelID
	if modelID == "" {
		modelID = "eleven_multilingual_v2"
	}

	outputFormat := input.Config.OutputFormat
	if outputFormat == "" {
		outputFormat = "mp3_44100_128"
	}

	body, err := json.Marshal(map[string]interface{}{
		"text":          input.Text,
		"model_id":      modelID,
		"output_format": outputFormat,
	})
	if err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}

	url := fmt.Sprintf("%s/text-to-speech/%s", elevenLabsAPI, voiceID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeResult(map[string]string{"error": "http request failed: " + err.Error()})
		return 1
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeResult(map[string]string{"error": "read response: " + err.Error()})
		return 1
	}

	if resp.StatusCode != http.StatusOK {
		msg := string(audioBytes)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		writeResult(map[string]string{"error": fmt.Sprintf("ElevenLabs TTS %d: %s", resp.StatusCode, msg)})
		return 1
	}

	return writeResult(map[string]interface{}{
		"audio":  base64.StdEncoding.EncodeToString(audioBytes),
		"format": "mp3",
	})
}

// ---------------------------------------------------------------------------
// STT
// ---------------------------------------------------------------------------

type sttInput struct {
	Audio    string `json:"audio"` // base64-encoded
	Format   string `json:"format"`
	Language string `json:"language,omitempty"`
	Config   struct {
		APIKey     string `json:"api_key"`
		STTModelID string `json:"stt_model_id,omitempty"`
	} `json:"config"`
}

//go:wasmexport openlobster_stt
func stt() int32 {
	var input sttInput
	if err := json.Unmarshal(inputBuf, &input); err != nil {
		resultBuf = []byte(`{"error":"invalid input"}`)
		return 1
	}

	apiKey := input.Config.APIKey
	if apiKey == "" {
		resultBuf = []byte(`{"error":"api_key required"}`)
		return 1
	}

	modelID := input.Config.STTModelID
	if modelID == "" {
		modelID = "scribe_v1"
	}

	audioBytes, err := base64.StdEncoding.DecodeString(input.Audio)
	if err != nil {
		writeResult(map[string]string{"error": "base64 decode: " + err.Error()})
		return 1
	}

	// Build multipart form
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model_id", modelID)
	if input.Language != "" {
		_ = w.WriteField("language_code", input.Language)
	}

	filename := "audio.wav"
	if input.Format != "" && input.Format != "wav" {
		filename = "audio." + input.Format
	}
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		writeResult(map[string]string{"error": "create form file: " + err.Error()})
		return 1
	}
	if _, err := fw.Write(audioBytes); err != nil {
		writeResult(map[string]string{"error": "write audio: " + err.Error()})
		return 1
	}
	w.Close()

	req, err := http.NewRequest("POST", elevenLabsAPI+"/speech-to-text", &buf)
	if err != nil {
		writeResult(map[string]string{"error": err.Error()})
		return 1
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("xi-api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeResult(map[string]string{"error": "http request failed: " + err.Error()})
		return 1
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeResult(map[string]string{"error": "read response: " + err.Error()})
		return 1
	}

	if resp.StatusCode != http.StatusOK {
		msg := string(respBytes)
		if len(msg) > 200 {
			msg = msg[:200]
		}
		writeResult(map[string]string{"error": fmt.Sprintf("ElevenLabs STT %d: %s", resp.StatusCode, msg)})
		return 1
	}

	var sttResp struct {
		Text         string `json:"text"`
		LanguageCode string `json:"language_code"`
	}
	if err := json.Unmarshal(respBytes, &sttResp); err != nil {
		writeResult(map[string]string{"error": "parse STT response: " + err.Error()})
		return 1
	}

	return writeResult(map[string]string{
		"text":     sttResp.Text,
		"language": sttResp.LanguageCode,
	})
}

//go:wasmexport openlobster_configure
func configure() int32 {
	return writeResult(map[string]bool{"ok": true})
}

func main() {}
