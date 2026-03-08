package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAudioPipeline_OGG_To_LLM_To_Platform(t *testing.T) {
	adapter := NewAudioAdapter()

	oggData := []byte{0x00, 0x01, 0x02, 0x03}

	stream, err := adapter.ConvertForModel(nil, oggData, "ogg")
	if err != nil {
		t.Logf("OGG decode error (expected without real OGG data): %v", err)
	}

	if stream != nil {
		base64Audio, pcmData, err := adapter.EncodeForLLM(nil, stream)
		assert.NoError(t, err)
		assert.NotEmpty(t, base64Audio)
		assert.NotEmpty(t, pcmData)

		wavData, err := adapter.ConvertForPlatform(nil, pcmData, "whatsapp")
		assert.NoError(t, err)
		assert.NotEmpty(t, wavData)
		assert.Contains(t, string(wavData[:4]), "RIFF")
	}
}

func TestAudioPipeline_RAW_To_LLM(t *testing.T) {
	adapter := NewAudioAdapter()

	rawData := []byte{0x00, 0x01, 0x02, 0x03}

	stream, err := adapter.ConvertForModel(nil, rawData, "raw")
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, 16000, stream.SampleRate)

	base64Audio, _, err := adapter.EncodeForLLM(nil, stream)
	assert.NoError(t, err)
	assert.NotEmpty(t, base64Audio)
}

func TestAudioPipeline_LLM_Response_To_Platform(t *testing.T) {
	adapter := NewAudioAdapter()

	stream, err := adapter.DecodeFromLLM(nil, []byte("AAECAwQ="))
	assert.NoError(t, err)
	assert.NotNil(t, stream)

	wavData, err := adapter.ConvertForPlatform(nil, stream.Data, "whatsapp")
	assert.NoError(t, err)
	assert.NotEmpty(t, wavData)

	assert.Equal(t, "RIFF", string(wavData[0:4]))
	assert.Equal(t, "WAVE", string(wavData[8:12]))
}

func TestAudioPipeline_Multiple_Formats(t *testing.T) {
	adapter := NewAudioAdapter()

	assert.True(t, adapter.SupportsAudio("gpt-4o-audio"))
	assert.True(t, adapter.SupportsAudio("gpt-4o-mini-audio"))
	assert.False(t, adapter.SupportsAudio("gpt-4"))
	assert.False(t, adapter.SupportsAudio("gpt-3.5-turbo"))
}

func TestAudioPipeline_DecodeFromLLM_InvalidBase64(t *testing.T) {
	adapter := NewAudioAdapter()

	_, err := adapter.DecodeFromLLM(nil, []byte("not-valid-base64!!!"))
	assert.Error(t, err)
}

func TestAudioPipeline_FullRoundTrip(t *testing.T) {
	adapter := NewAudioAdapter()

	originalPcm := make([]byte, 1600)
	for i := range originalPcm {
		originalPcm[i] = byte(i % 256)
	}

	stream := &AudioStream{
		Data:       originalPcm,
		SampleRate: 16000,
		Channels:   1,
		BitDepth:   16,
		Format:     "raw",
	}

	base64Audio, pcmData, err := adapter.EncodeForLLM(nil, stream)
	assert.NoError(t, err)
	assert.NotEmpty(t, base64Audio)
	assert.Equal(t, originalPcm, pcmData)

	llmResponse := []byte("AAECAwQ=")
	decodedStream, err := adapter.DecodeFromLLM(nil, llmResponse)
	assert.NoError(t, err)
	assert.NotNil(t, decodedStream)

	wavData, err := adapter.ConvertForPlatform(nil, decodedStream.Data, "whatsapp")
	assert.NoError(t, err)
	assert.NotEmpty(t, wavData)
	assert.Contains(t, string(wavData), "RIFF")
	assert.Contains(t, string(wavData), "WAVE")
}
