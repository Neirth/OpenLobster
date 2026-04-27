// Copyright (c) OpenLobster contributors. See LICENSE for details.

package smoke

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	"github.com/neirth/openlobster/utils/validation_layer/src/config"
	"github.com/neirth/openlobster/utils/validation_layer/src/protocol"
	"github.com/neirth/openlobster/utils/validation_layer/src/types"
)

func runAudioSmoke(client protocol.PluginClient, report *types.PluginReport, opts types.ValidateOptions, file string) {
	cfg := cloneMap(opts.SmokeConfig)
	apiKey := config.ConfigString(cfg, "api_key")
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENLOBSTER_SMOKE_AUDIO_API_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("AUDIO_API_KEY"))
	}
	if apiKey == "" {
		addSmokeFailure(report, "audio", "missing api_key: provide via --config or OPENLOBSTER_SMOKE_AUDIO_API_KEY", file)
		return
	}

	cfg["api_key"] = apiKey
	if err := configurePlugin(client, cfg); err != nil {
		addSmokeFailure(report, "audio", err.Error(), file)
		return
	}

	// Smoke test step 1: TTS
	// We expect the plugin to adhere to the OpenLobster Audio Standard (PCM 16kHz Mono)
	ttsRaw, err := client.CallJSON("tts", map[string]any{
		"text":   "OpenLobster smoke test audio validation",
		"config": cfg,
	})
	if err != nil {
		addSmokeFailure(report, "audio.tts", err.Error(), file)
		return
	}
	var ttsResp struct {
		Audio  string `json:"audio"`
		Format string `json:"format"`
	}
	if err := json.Unmarshal(ttsRaw, &ttsResp); err != nil {
		addSmokeFailure(report, "audio.tts", "invalid JSON response", file)
		return
	}
	if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ttsResp.Audio)); err != nil {
		addSmokeFailure(report, "audio.tts", "invalid base64 audio data", file)
		return
	}

	// Enforcement: Plugin SHOULD return pcm if no specific format is requested
	if ttsResp.Format != "pcm" && ttsResp.Format != "mp3" && ttsResp.Format != "ogg" {
		addSmokeFailure(report, "audio.tts", "unsupported or missing format; expected 'pcm' (16kHz Mono) for OpenLobster compliance", file)
	}

	format := ttsResp.Format
	if format == "" {
		format = "pcm"
	}

	// Smoke test step 2: STT
	sttRaw, err := client.CallJSON("stt", map[string]any{
		"audio":  ttsResp.Audio,
		"format": format,
		"config": cfg,
	})
	if err != nil {
		addSmokeFailure(report, "audio.stt", err.Error(), file)
		return
	}
	var sttResp struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(sttRaw, &sttResp); err != nil || strings.TrimSpace(sttResp.Text) == "" {
		addSmokeFailure(report, "audio.stt", "empty or invalid transcription response", file)
	}
}
