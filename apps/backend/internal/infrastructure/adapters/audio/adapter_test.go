package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAudioAdapter(t *testing.T) {
	adapter := NewAudioAdapter()
	assert.NotNil(t, adapter)
}

func TestAudioAdapter_ConvertForModel_Raw(t *testing.T) {
	adapter := NewAudioAdapter()

	stream, err := adapter.ConvertForModel(nil, []byte{0x00, 0x01}, "raw")

	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, 16000, stream.SampleRate)
	assert.Equal(t, 1, stream.Channels)
	assert.Equal(t, 16, stream.BitDepth)
}

func TestAudioAdapter_ConvertForModel_Unsupported(t *testing.T) {
	adapter := NewAudioAdapter()

	_, err := adapter.ConvertForModel(nil, []byte{}, "mp3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mp3")

	_, err = adapter.ConvertForModel(nil, []byte{}, "aac")
	assert.Error(t, err)

	_, err = adapter.ConvertForModel(nil, []byte{}, "unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestAudioAdapter_EncodeForLLM(t *testing.T) {
	adapter := NewAudioAdapter()

	stream := &AudioStream{
		Data:       []byte{0x00, 0x01, 0x02, 0x03},
		SampleRate: 16000,
		Channels:   1,
		BitDepth:   16,
		Format:     "raw",
	}

	base64Data, pcmData, err := adapter.EncodeForLLM(nil, stream)

	assert.NoError(t, err)
	assert.NotEmpty(t, base64Data)
	assert.NotEmpty(t, pcmData)
}

func TestAudioAdapter_DecodeFromLLM(t *testing.T) {
	adapter := NewAudioAdapter()

	stream, err := adapter.DecodeFromLLM(nil, []byte("AAECAw=="))

	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.NotEmpty(t, stream.Data)
}

func TestAudioAdapter_SupportsAudio(t *testing.T) {
	adapter := NewAudioAdapter()

	assert.True(t, adapter.SupportsAudio("gpt-4o-audio"))
	assert.True(t, adapter.SupportsAudio("whisper-1"))
	assert.True(t, adapter.SupportsAudio("gpt-4o-mini-audio"))
	assert.True(t, adapter.SupportsAudio("gemini-2.0-flash"))
	assert.False(t, adapter.SupportsAudio(""))
	assert.False(t, adapter.SupportsAudio("gpt-4"))
}

func TestAudioAdapter_ConvertForModel_PCM(t *testing.T) {
	adapter := NewAudioAdapter()
	stream, err := adapter.ConvertForModel(nil, []byte{1, 2, 3, 4}, "pcm")
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, "raw", stream.Format)
}

func TestAudioAdapter_ConvertForModel_InvalidWAV(t *testing.T) {
	adapter := NewAudioAdapter()

	_, err := adapter.ConvertForModel(nil, []byte("XXXX"), "wav")
	assert.Error(t, err)

	_, err = adapter.ConvertForModel(nil, []byte("RIFF"), "wav")
	assert.Error(t, err)
}

func TestAudioAdapter_ConvertForPlatform(t *testing.T) {
	adapter := NewAudioAdapter()

	wavData, err := adapter.ConvertForPlatform(nil, []byte{0x00, 0x01, 0x02, 0x03}, "whatsapp")

	assert.NoError(t, err)
	assert.NotEmpty(t, wavData)
	assert.Contains(t, string(wavData), "RIFF")
	assert.Contains(t, string(wavData), "WAVE")
}

func TestInt16ToBytes(t *testing.T) {
	samples := []int16{0, 1, 255, -1}
	result := int16ToBytes(samples)

	assert.Len(t, result, 8)
}

func TestBytesToInt16(t *testing.T) {
	data := []byte{0, 0, 1, 0, 255, 0, 255, 255}
	result := bytesToInt16(data)

	assert.Len(t, result, 4)
	assert.Equal(t, int16(0), result[0])
	assert.Equal(t, int16(1), result[1])
	assert.Equal(t, int16(255), result[2])
	assert.Equal(t, int16(-1), result[3])
}

func TestAudioAdapter_toMono(t *testing.T) {
	adapter := NewAudioAdapter()

	samples := []int16{100, 200, 300, 400}
	mono := adapter.toMono(samples, 2)

	assert.Len(t, mono, 2)
	assert.Equal(t, int16(150), mono[0])
	assert.Equal(t, int16(350), mono[1])
}

func TestAudioAdapter_resample(t *testing.T) {
	adapter := NewAudioAdapter()

	samples := []int16{0, 100, 200, 300, 400, 500, 600, 700, 800, 900}
	resampled := adapter.resample(samples, 48000, 16000)

	assert.LessOrEqual(t, len(resampled), len(samples))
}

func TestAudioAdapter_resample_SameRate(t *testing.T) {
	adapter := NewAudioAdapter()
	samples := []int16{1, 2, 3}
	out := adapter.resample(samples, 16000, 16000)
	assert.Equal(t, samples, out)
}

func TestAudioAdapter_EncodeForLLM_MultiChannel(t *testing.T) {
	adapter := NewAudioAdapter()
	// stereo: 4 samples = 2 per channel
	stream := &AudioStream{
		Data:       []byte{0, 0, 1, 0, 2, 0, 3, 0},
		SampleRate: 16000,
		Channels:   2,
		BitDepth:   16,
	}
	_, _, err := adapter.EncodeForLLM(nil, stream)
	assert.NoError(t, err)
}

func TestAudioAdapter_EncodeForLLM_Resample(t *testing.T) {
	adapter := NewAudioAdapter()
	stream := &AudioStream{
		Data:       []byte{0, 0, 1, 0, 2, 0, 3, 0},
		SampleRate: 48000,
		Channels:   1,
		BitDepth:   16,
	}
	_, _, err := adapter.EncodeForLLM(nil, stream)
	assert.NoError(t, err)
}

func TestAudioAdapter_EncodeWav(t *testing.T) {
	adapter := NewAudioAdapter()
	wav, err := adapter.encodeWav([]byte{0, 0, 1, 0}, 16000, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, wav)
	assert.Contains(t, string(wav[:12]), "RIFF")
	assert.Contains(t, string(wav[8:12]), "WAVE")
}

func TestAudioAdapter_DecodeFromLLM_InvalidBase64(t *testing.T) {
	adapter := NewAudioAdapter()
	_, err := adapter.DecodeFromLLM(nil, []byte("!!!invalid!!!"))
	assert.Error(t, err)
}

func TestAudioAdapter_SupportsAudio_WhisperExp(t *testing.T) {
	adapter := NewAudioAdapter()
	assert.True(t, adapter.SupportsAudio("gemini-2.0-flash-exp"))
}
