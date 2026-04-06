//go:build !tinygo

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	pdk "github.com/neirth/openlobster/plugins/openlobster-sdk-base/src/sdk/runtime"
	elevenlabs "github.com/plexusone/elevenlabs-go"
	_ "github.com/stealthrocket/net/http"
)

// ---------------------------------------------------------------------------
// Metadata
// ---------------------------------------------------------------------------

func getName() int32 { pdk.OutputString("openlobster-audio-elevenlabs"); return 0 }

func getVersion() int32 { pdk.OutputString("0.1.0"); return 0 }

func getDescription() int32 {
	pdk.OutputString("ElevenLabs TTS/STT audio plugin for OpenLobster")
	return 0
}

func getType() int32 { pdk.OutputString("audio"); return 0 }

func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key","description":"ElevenLabs API key"},"voice_id":{"type":"string","title":"Voice ID","default":"21m00Tcm4TlvDq8ikWAM","description":"Default voice identifier for TTS"},"model_id":{"type":"string","title":"TTS Model","default":"eleven_multilingual_v2","description":"Text-to-speech model identifier"},"stt_model_id":{"type":"string","title":"STT Model","default":"scribe_v1","description":"Speech-to-text model identifier"}},"required":["api_key"]}`)
	return 0
}

func getMetadata() int32 {
	metadata := map[string]interface{}{
		"id":          "openlobster-audio-elevenlabs",
		"name":        "openlobster-audio-elevenlabs",
		"version":     "0.1.0",
		"description": "ElevenLabs TTS/STT audio plugin for OpenLobster",
		"type":        "audio",
		"schema":      json.RawMessage(`{"type":"object","properties":{"api_key":{"type":"string","title":"API Key","description":"ElevenLabs API key"},"voice_id":{"type":"string","title":"Voice ID","default":"21m00Tcm4TlvDq8ikWAM","description":"Default voice identifier for TTS"},"model_id":{"type":"string","title":"TTS Model","default":"eleven_multilingual_v2","description":"Text-to-speech model identifier"},"stt_model_id":{"type":"string","title":"STT Model","default":"scribe_v1","description":"Speech-to-text model identifier"}},"required":["api_key"]}`),
		"properties":  json.RawMessage(`{}`),
	}
	if err := pdk.OutputJSON(metadata); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// tts
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

func tts() int32 {
	var input ttsInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	apiKey := input.Config.APIKey
	if apiKey == "" {
		pdk.SetError(fmt.Errorf("api_key required"))
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
		pdk.SetError(fmt.Errorf("client init: %w", err))
		return 1
	}

	resp, err := client.TextToSpeech().Generate(context.Background(), &elevenlabs.TTSRequest{
		VoiceID:      voiceID,
		Text:         input.Text,
		ModelID:      modelID,
		OutputFormat: outputFormat,
	})
	if err != nil {
		pdk.SetError(fmt.Errorf("TTS: %w", err))
		return 1
	}

	audioBytes, err := io.ReadAll(resp.Audio)
	if err != nil {
		pdk.SetError(fmt.Errorf("read audio: %w", err))
		return 1
	}

	if err := pdk.OutputJSON(map[string]interface{}{
		"audio":  base64.StdEncoding.EncodeToString(audioBytes),
		"format": "mp3",
	}); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// stt
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

func stt() int32 {
	var input sttInput
	if err := pdk.InputJSON(&input); err != nil {
		pdk.SetError(err)
		return 1
	}

	apiKey := input.Config.APIKey
	if apiKey == "" {
		pdk.SetError(fmt.Errorf("api_key required"))
		return 1
	}

	modelID := input.Config.STTModelID
	if modelID == "" {
		modelID = "scribe_v1"
	}

	client, err := elevenlabs.NewClient(elevenlabs.WithAPIKey(apiKey))
	if err != nil {
		pdk.SetError(fmt.Errorf("client init: %w", err))
		return 1
	}

	result, err := client.SpeechToText().Transcribe(context.Background(), &elevenlabs.TranscriptionRequest{
		FileContent:  input.Audio,
		ModelID:      modelID,
		LanguageCode: input.Language,
	})
	if err != nil {
		pdk.SetError(fmt.Errorf("STT: %w", err))
		return 1
	}

	if err := pdk.OutputJSON(map[string]string{
		"text":     result.Text,
		"language": result.LanguageCode,
	}); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func main() {
	pdk.MustRun(pdk.Plugin{
		ID: "openlobster-audio-elevenlabs",
		Exports: map[string]pdk.Function{
			"get_metadata": getMetadata,
			"configure":    configureHot,
			"tts":          tts,
			"stt":          stt,
		},
	})
}
