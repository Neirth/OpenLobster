package ports

import "context"

// AudioProviderPort is the domain interface for text-to-speech and
// speech-to-text services.  Implementations (e.g. ElevenLabs) are loaded as
// native plugins of type "audio".
type AudioProviderPort interface {
	TextToSpeech(ctx context.Context, req TTSRequest) (TTSResponse, error)
	SpeechToText(ctx context.Context, req STTRequest) (STTResponse, error)
}

// TTSRequest is the input for a text-to-speech conversion.
type TTSRequest struct {
	Text    string                 `json:"text"`
	VoiceID string                 `json:"voice_id,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// TTSResponse carries the synthesised audio bytes.
type TTSResponse struct {
	Audio  []byte `json:"audio"`  // raw audio bytes
	Format string `json:"format"` // e.g. "mp3"
}

// STTRequest is the input for a speech-to-text transcription.
type STTRequest struct {
	Audio    []byte                 `json:"audio"`
	Format   string                 `json:"format"`
	Language string                 `json:"language,omitempty"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// STTResponse carries the transcribed text.
type STTResponse struct {
	Text     string `json:"text"`
	Language string `json:"language,omitempty"`
}
