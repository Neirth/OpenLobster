package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"unsafe"

	elevenlabs "github.com/plexusone/elevenlabs-go"
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

	client, err := elevenlabs.NewClient(elevenlabs.WithAPIKey(apiKey))
	if err != nil {
		writeResult(map[string]string{"error": "client init: " + err.Error()})
		return 1
	}

	resp, err := client.TextToSpeech().Generate(context.Background(), &elevenlabs.TTSRequest{
		VoiceID:      voiceID,
		Text:         input.Text,
		ModelID:      modelID,
		OutputFormat: outputFormat,
	})
	if err != nil {
		writeResult(map[string]string{"error": "TTS: " + err.Error()})
		return 1
	}

	audioBytes, err := io.ReadAll(resp.Audio)
	if err != nil {
		writeResult(map[string]string{"error": "read audio: " + err.Error()})
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

	client, err := elevenlabs.NewClient(elevenlabs.WithAPIKey(apiKey))
	if err != nil {
		writeResult(map[string]string{"error": "client init: " + err.Error()})
		return 1
	}

	result, err := client.SpeechToText().Transcribe(context.Background(), &elevenlabs.TranscriptionRequest{
		FileContent:  input.Audio, // already base64-encoded — SDK accepts this directly
		ModelID:      modelID,
		LanguageCode: input.Language,
	})
	if err != nil {
		writeResult(map[string]string{"error": "STT: " + err.Error()})
		return 1
	}

	return writeResult(map[string]string{
		"text":     result.Text,
		"language": result.LanguageCode,
	})
}

//go:wasmexport openlobster_configure
func configure() int32 {
	return writeResult(map[string]bool{"ok": true})
}

func main() {}
