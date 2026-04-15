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

	ttsRaw, err := client.CallJSON("tts", map[string]any{"text": "OpenLobster smoke test audio", "config": cfg})
	if err != nil {
		addSmokeFailure(report, "audio.tts", err.Error(), file)
		return
	}
	var ttsResp struct {
		Audio  string `json:"audio"`
		Format string `json:"format"`
	}
	if err := json.Unmarshal(ttsRaw, &ttsResp); err != nil {
		addSmokeFailure(report, "audio.tts", "invalid JSON", file)
		return
	}
	if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ttsResp.Audio)); err != nil {
		addSmokeFailure(report, "audio.tts", "invalid base64 audio", file)
		return
	}

	format := ttsResp.Format
	if format == "" {
		format = "mp3"
	}
	sttRaw, err := client.CallJSON("stt", map[string]any{"audio": ttsResp.Audio, "format": format, "config": cfg})
	if err != nil {
		addSmokeFailure(report, "audio.stt", err.Error(), file)
		return
	}
	var sttResp struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(sttRaw, &sttResp); err != nil || strings.TrimSpace(sttResp.Text) == "" {
		addSmokeFailure(report, "audio.stt", "empty transcription", file)
	}
}
