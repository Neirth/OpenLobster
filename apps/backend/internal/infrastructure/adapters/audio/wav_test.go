// Copyright (c) OpenLobster contributors. See LICENSE for details.

package audio

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helper: build a minimal PCM WAV file in memory
// ---------------------------------------------------------------------------

func buildWav(t *testing.T, sampleRate uint32, channels uint16, bitsPerSample uint16, pcmData []byte) []byte {
	t.Helper()
	buf := make([]byte, 0, 44+len(pcmData))

	// RIFF header
	buf = append(buf, []byte("RIFF")...)
	size := make([]byte, 4)
	binary.LittleEndian.PutUint32(size, uint32(36+len(pcmData)))
	buf = append(buf, size...)
	buf = append(buf, []byte("WAVE")...)

	// fmt chunk
	buf = append(buf, []byte("fmt ")...)
	binary.LittleEndian.PutUint32(size, 16)
	buf = append(buf, size...)
	audioFmt := make([]byte, 2)
	binary.LittleEndian.PutUint16(audioFmt, 1) // PCM
	buf = append(buf, audioFmt...)
	ch := make([]byte, 2)
	binary.LittleEndian.PutUint16(ch, channels)
	buf = append(buf, ch...)
	sr := make([]byte, 4)
	binary.LittleEndian.PutUint32(sr, sampleRate)
	buf = append(buf, sr...)
	byteRate := make([]byte, 4)
	binary.LittleEndian.PutUint32(byteRate, sampleRate*uint32(channels)*uint32(bitsPerSample/8))
	buf = append(buf, byteRate...)
	blockAlign := make([]byte, 2)
	binary.LittleEndian.PutUint16(blockAlign, channels*bitsPerSample/8)
	buf = append(buf, blockAlign...)
	bps := make([]byte, 2)
	binary.LittleEndian.PutUint16(bps, bitsPerSample)
	buf = append(buf, bps...)

	// data chunk
	buf = append(buf, []byte("data")...)
	binary.LittleEndian.PutUint32(size, uint32(len(pcmData)))
	buf = append(buf, size...)
	buf = append(buf, pcmData...)

	return buf
}

// ---------------------------------------------------------------------------
// decodeWav — success paths
// ---------------------------------------------------------------------------

func TestDecodeWav_16bit_Mono(t *testing.T) {
	pcm := []byte{0, 0, 100, 0, 200, 0} // 3 samples × 2 bytes
	wav := buildWav(t, 16000, 1, 16, pcm)

	t.Logf("wav header: %q", wav[:12])

	a := NewAudioAdapter()
	stream, err := a.decodeWav(wav)
	require.NoError(t, err)
	require.NotNil(t, stream)
	assert.Equal(t, 16000, stream.SampleRate)
	assert.Equal(t, 1, stream.Channels)
	assert.Equal(t, 16, stream.BitDepth)
	assert.Equal(t, "wav", stream.Format)
	assert.Equal(t, pcm, stream.Data)
}

func TestDecodeWav_16bit_Stereo(t *testing.T) {
	pcm := []byte{0, 0, 1, 0, 2, 0, 3, 0} // 2 stereo pairs
	wav := buildWav(t, 44100, 2, 16, pcm)

	a := NewAudioAdapter()
	stream, err := a.decodeWav(wav)
	require.NoError(t, err)
	assert.Equal(t, 44100, stream.SampleRate)
	assert.Equal(t, 2, stream.Channels)
}

func TestDecodeWav_8bit_Mono(t *testing.T) {
	pcm := []byte{128, 200, 50} // 3 8-bit samples
	wav := buildWav(t, 8000, 1, 8, pcm)

	a := NewAudioAdapter()
	stream, err := a.decodeWav(wav)
	require.NoError(t, err)
	assert.Equal(t, 16, stream.BitDepth) // upsampled to 16
}

func TestDecodeWav_UnsupportedBitDepth(t *testing.T) {
	pcm := []byte{0, 0, 0, 0} // garbage 32-bit samples
	wav := buildWav(t, 16000, 1, 32, pcm)

	a := NewAudioAdapter()
	_, err := a.decodeWav(wav)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported bit depth")
}

// ---------------------------------------------------------------------------
// decodeWav — error paths
// ---------------------------------------------------------------------------

func TestDecodeWav_NotRIFF(t *testing.T) {
	a := NewAudioAdapter()
	_, err := a.decodeWav([]byte("XXXX    WAVE"))
	assert.Error(t, err)
}

func TestDecodeWav_NotWAVE(t *testing.T) {
	a := NewAudioAdapter()
	data := []byte("RIFF    XXXX")
	_, err := a.decodeWav(data)
	assert.Error(t, err)
}

func TestDecodeWav_TruncatedAfterRIFF(t *testing.T) {
	a := NewAudioAdapter()
	_, err := a.decodeWav([]byte("RIFF"))
	assert.Error(t, err)
}

func TestDecodeWav_NonPCMAudioFormat(t *testing.T) {
	// Build a WAV with audioFormat=3 (IEEE float) which is rejected by the adapter.
	buf := make([]byte, 0, 44)
	buf = append(buf, []byte("RIFF")...)
	sz := make([]byte, 4)
	binary.LittleEndian.PutUint32(sz, 36)
	buf = append(buf, sz...)
	buf = append(buf, []byte("WAVE")...)
	buf = append(buf, []byte("fmt ")...)
	binary.LittleEndian.PutUint32(sz, 16)
	buf = append(buf, sz...)
	af := make([]byte, 2)
	binary.LittleEndian.PutUint16(af, 6) // A-law — neither 1 nor 3
	buf = append(buf, af...)
	buf = append(buf, make([]byte, 14)...) // rest of fmt chunk

	a := NewAudioAdapter()
	_, err := a.decodeWav(buf)
	// Will fail with "only PCM WAV supported" or "could not find audio data"
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// decodeWav — extra chunk before data
// ---------------------------------------------------------------------------

func TestDecodeWav_SkipsUnknownChunk(t *testing.T) {
	// Build WAV with an unknown "LIST" chunk before "data".
	pcm := []byte{0, 0, 1, 0}

	buf := make([]byte, 0, 60+len(pcm))
	listData := []byte{1, 2, 3, 4}

	totalSize := uint32(4 + 8 + 16 + 8 + 4 + 8 + len(listData) + 8 + len(pcm))
	buf = append(buf, []byte("RIFF")...)
	sz := make([]byte, 4)
	binary.LittleEndian.PutUint32(sz, totalSize-8)
	buf = append(buf, sz...)
	buf = append(buf, []byte("WAVE")...)

	// fmt chunk
	buf = append(buf, []byte("fmt ")...)
	binary.LittleEndian.PutUint32(sz, 16)
	buf = append(buf, sz...)
	af := make([]byte, 2)
	binary.LittleEndian.PutUint16(af, 1) // PCM
	buf = append(buf, af...)
	ch := make([]byte, 2)
	binary.LittleEndian.PutUint16(ch, 1) // mono
	buf = append(buf, ch...)
	sr := make([]byte, 4)
	binary.LittleEndian.PutUint32(sr, 16000)
	buf = append(buf, sr...)
	br := make([]byte, 4)
	binary.LittleEndian.PutUint32(br, 32000)
	buf = append(buf, br...)
	ba := make([]byte, 2)
	binary.LittleEndian.PutUint16(ba, 2)
	buf = append(buf, ba...)
	bps := make([]byte, 2)
	binary.LittleEndian.PutUint16(bps, 16)
	buf = append(buf, bps...)

	// Unknown LIST chunk
	buf = append(buf, []byte("LIST")...)
	binary.LittleEndian.PutUint32(sz, uint32(len(listData)))
	buf = append(buf, sz...)
	buf = append(buf, listData...)

	// data chunk
	buf = append(buf, []byte("data")...)
	binary.LittleEndian.PutUint32(sz, uint32(len(pcm)))
	buf = append(buf, sz...)
	buf = append(buf, pcm...)

	a := NewAudioAdapter()
	stream, err := a.decodeWav(buf)
	require.NoError(t, err)
	assert.Equal(t, pcm, stream.Data)
}

// ---------------------------------------------------------------------------
// contains helper
// ---------------------------------------------------------------------------

func TestContains(t *testing.T) {
	assert.True(t, contains("gpt-4o-audio-preview", "gpt-4o-audio"))
	assert.True(t, contains("whisper-1", "whisper"))
	assert.False(t, contains("gpt-4", "gpt-4o-audio"))
	assert.False(t, contains("short", "this-is-longer-than-short"))
	assert.True(t, contains("abc", "abc"))
	assert.False(t, contains("", "x"))
}

// ---------------------------------------------------------------------------
// encodeWav — round-trip with decodeWav
// ---------------------------------------------------------------------------

func TestEncodeWav_RoundTrip_WithDecodeWav(t *testing.T) {
	originalPcm := make([]byte, 400)
	for i := range originalPcm {
		originalPcm[i] = byte(i)
	}

	a := NewAudioAdapter()
	wav, err := a.encodeWav(originalPcm, 16000, 1)
	require.NoError(t, err)
	require.NotEmpty(t, wav)

	stream, err := a.decodeWav(wav)
	require.NoError(t, err)
	assert.Equal(t, 16000, stream.SampleRate)
	assert.Equal(t, 1, stream.Channels)
	assert.Equal(t, originalPcm, stream.Data)
}

// ---------------------------------------------------------------------------
// ConvertForModel — ogg error path
// ---------------------------------------------------------------------------

func TestConvertForModel_OGG_InvalidData(t *testing.T) {
	a := NewAudioAdapter()
	_, err := a.ConvertForModel(nil, []byte{0x00, 0x01, 0x02}, "ogg")
	assert.Error(t, err)
}

func TestConvertForModel_Opus_InvalidData(t *testing.T) {
	a := NewAudioAdapter()
	_, err := a.ConvertForModel(nil, []byte{0x00, 0x01}, "opus")
	assert.Error(t, err)
}

func TestConvertForModel_M4A_NotSupported(t *testing.T) {
	a := NewAudioAdapter()
	_, err := a.ConvertForModel(nil, []byte{}, "m4a")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// int16ToBytes / bytesToInt16 — round-trip
// ---------------------------------------------------------------------------

func TestInt16Bytes_RoundTrip(t *testing.T) {
	original := []int16{0, 1, -1, 32767, -32768, 100}
	bytes := int16ToBytes(original)
	back := bytesToInt16(bytes)
	assert.Equal(t, original, back)
}

func TestBytesToInt16_EmptyInput(t *testing.T) {
	result := bytesToInt16([]byte{})
	assert.Empty(t, result)
}

func TestInt16ToBytes_EmptyInput(t *testing.T) {
	result := int16ToBytes([]int16{})
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// toMono — various channel counts
// ---------------------------------------------------------------------------

func TestToMono_Quad(t *testing.T) {
	a := NewAudioAdapter()
	// 4-channel, 4 samples total = 1 output sample
	samples := []int16{100, 200, 300, 400}
	mono := a.toMono(samples, 4)
	require.Len(t, mono, 1)
	assert.Equal(t, int16(250), mono[0])
}

// ---------------------------------------------------------------------------
// resample — upsampling
// ---------------------------------------------------------------------------

func TestResample_Upsample(t *testing.T) {
	a := NewAudioAdapter()
	samples := []int16{0, 100, 200}
	out := a.resample(samples, 8000, 16000)
	assert.Greater(t, len(out), len(samples))
}

func TestResample_AtBoundary(t *testing.T) {
	a := NewAudioAdapter()
	// Only 1 sample — boundary condition where idx+1 >= len(samples)
	samples := []int16{500}
	out := a.resample(samples, 8000, 16000)
	assert.NotEmpty(t, out)
}
