package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/neirth/openlobster/internal/domain/ports"
)

// AudioWrapper wraps an "audio"-type PluginPort and implements
// ports.AudioProviderPort.  The plugin must export openlobster_tts() and
// openlobster_stt().  Audio bytes are transferred as base64 strings in the
// JSON payload because binary-safe JSON serialisation
// requires encoding.
type AudioWrapper struct {
	plugin ports.PluginPort
	cfg    map[string]interface{}
}

// NewAudioWrapper returns an AudioWrapper backed by p.
func NewAudioWrapper(p ports.PluginPort, cfg map[string]interface{}) *AudioWrapper {
	return &AudioWrapper{plugin: p, cfg: cfg}
}

// ttsPluginInput is the JSON payload sent to the plugin's openlobster_tts export.
type ttsPluginInput struct {
	Text    string                 `json:"text"`
	VoiceID string                 `json:"voice_id,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// ttsPluginOutput is the JSON payload returned by the plugin.
type ttsPluginOutput struct {
	Audio  string `json:"audio"`  // base64-encoded bytes
	Format string `json:"format"` // e.g. "mp3"
	Error  string `json:"error,omitempty"`
}

// TextToSpeech converts text to speech using the backing audio plugin.
func (w *AudioWrapper) TextToSpeech(_ context.Context, req ports.TTSRequest) (ports.TTSResponse, error) {
	cfg := req.Config
	if cfg == nil {
		cfg = w.cfg
	}
	input := ttsPluginInput{
		Text:    req.Text,
		VoiceID: req.VoiceID,
		Config:  cfg,
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ports.TTSResponse{}, fmt.Errorf("audio plugin %s: marshal TTS input: %w", w.plugin.ID(), err)
	}
	out, err := w.plugin.Call("tts", raw)
	if err != nil {
		return ports.TTSResponse{}, fmt.Errorf("audio plugin %s: tts call: %w", w.plugin.ID(), err)
	}
	var resp ttsPluginOutput
	if err := json.Unmarshal(out, &resp); err != nil {
		return ports.TTSResponse{}, fmt.Errorf("audio plugin %s: unmarshal TTS output: %w", w.plugin.ID(), err)
	}
	if resp.Error != "" {
		return ports.TTSResponse{}, fmt.Errorf("audio plugin %s: tts: %s", w.plugin.ID(), resp.Error)
	}
	audioBytes, err := base64.StdEncoding.DecodeString(resp.Audio)
	if err != nil {
		return ports.TTSResponse{}, fmt.Errorf("audio plugin %s: decode TTS audio: %w", w.plugin.ID(), err)
	}
	return ports.TTSResponse{Audio: audioBytes, Format: resp.Format}, nil
}

// sttPluginInput is the JSON payload sent to the plugin's openlobster_stt export.
type sttPluginInput struct {
	Audio    string                 `json:"audio"` // base64-encoded
	Format   string                 `json:"format"`
	Language string                 `json:"language,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// sttPluginOutput is the JSON payload returned by the plugin.
type sttPluginOutput struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
	Error    string `json:"error,omitempty"`
}

// SpeechToText transcribes audio using the backing audio plugin.
func (w *AudioWrapper) SpeechToText(_ context.Context, req ports.STTRequest) (ports.STTResponse, error) {
	cfg := req.Config
	if cfg == nil {
		cfg = w.cfg
	}
	input := sttPluginInput{
		Audio:    base64.StdEncoding.EncodeToString(req.Audio),
		Format:   req.Format,
		Language: req.Language,
		Config:   cfg,
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return ports.STTResponse{}, fmt.Errorf("audio plugin %s: marshal STT input: %w", w.plugin.ID(), err)
	}
	out, err := w.plugin.Call("stt", raw)
	if err != nil {
		return ports.STTResponse{}, fmt.Errorf("audio plugin %s: stt call: %w", w.plugin.ID(), err)
	}
	var resp sttPluginOutput
	if err := json.Unmarshal(out, &resp); err != nil {
		return ports.STTResponse{}, fmt.Errorf("audio plugin %s: unmarshal STT output: %w", w.plugin.ID(), err)
	}
	if resp.Error != "" {
		return ports.STTResponse{}, fmt.Errorf("audio plugin %s: stt: %s", w.plugin.ID(), resp.Error)
	}
	return ports.STTResponse{Text: resp.Text, Language: resp.Language}, nil
}
