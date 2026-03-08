package audio

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pion/opus"
	"github.com/pion/opus/pkg/oggreader"
)

const maxOPUSFrameSize = 5760

type AudioStream struct {
	Data       []byte
	SampleRate int
	Channels   int
	BitDepth   int
	Format     string
}

type AudioAdapter struct{}

func NewAudioAdapter() *AudioAdapter {
	return &AudioAdapter{}
}

func (a *AudioAdapter) ConvertForModel(ctx interface{}, audioData []byte, format string) (*AudioStream, error) {
	switch format {
	case "wav":
		return a.decodeWav(audioData)
	case "ogg", "opus":
		return a.decodeOggOpus(audioData)
	case "raw", "pcm":
		return &AudioStream{
			Data:       audioData,
			SampleRate: 16000,
			Channels:   1,
			BitDepth:   16,
			Format:     "raw",
		}, nil
	case "mp3", "aac", "m4a":
		return nil, fmt.Errorf("format %s not supported, convert to WAV first", format)
	default:
		return nil, fmt.Errorf("unsupported input format: %s", format)
	}
}

func (a *AudioAdapter) ConvertForPlatform(ctx interface{}, audioData []byte, platform string) ([]byte, error) {
	return a.encodeWav(audioData, 16000, 1)
}

func (a *AudioAdapter) EncodeForLLM(ctx interface{}, stream *AudioStream) (string, []byte, error) {
	pcmData, err := a.prepareForLLM(stream)
	if err != nil {
		return "", nil, err
	}

	base64Audio := base64.StdEncoding.EncodeToString(pcmData)
	return base64Audio, pcmData, nil
}

func (a *AudioAdapter) DecodeFromLLM(ctx interface{}, response []byte) (*AudioStream, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(response))
	if err != nil {
		return nil, err
	}

	return &AudioStream{
		Data:       decoded,
		SampleRate: 24000,
		Channels:   1,
		BitDepth:   16,
		Format:     "raw",
	}, nil
}

func (a *AudioAdapter) SupportsAudio(modelID string) bool {
	audioModels := []string{
		"gpt-4o-audio",
		"gpt-4o-mini-audio",
		"whisper",
		"gemini-2.0-flash-exp",
		"gemini-2.0-flash",
	}
	for _, m := range audioModels {
		if contains(modelID, m) {
			return true
		}
	}
	return false
}

func (a *AudioAdapter) decodeWav(data []byte) (*AudioStream, error) {
	reader := bytes.NewReader(data)

	var riff [4]byte
	if _, err := reader.Read(riff[:]); err != nil {
		return nil, fmt.Errorf("not a valid WAV file")
	}
	if string(riff[:]) != "RIFF" {
		return nil, fmt.Errorf("not a valid RIFF file")
	}

	var wave [4]byte
	if _, err := reader.Read(wave[:]); err != nil {
		return nil, fmt.Errorf("not a valid WAV file")
	}
	if string(wave[:]) != "WAVE" {
		return nil, fmt.Errorf("not a valid WAVE file")
	}

	var sampleRate int
	var numChannels int
	var bitsPerSample int

	for {
		var chunkID [4]byte
		n, err := reader.Read(chunkID[:])
		if err != nil {
			break
		}
		if n == 0 {
			break
		}

		var chunkSize [4]byte
		if _, err := reader.Read(chunkSize[:]); err != nil {
			break
		}
		size := int(binary.LittleEndian.Uint32(chunkSize[:]))

		if string(chunkID[:]) == "fmt " {
			subchunk := make([]byte, size)
			reader.Read(subchunk)

			audioFormat := binary.LittleEndian.Uint16(subchunk[0:2])
			if audioFormat != 1 && audioFormat != 3 {
				return nil, fmt.Errorf("only PCM WAV supported")
			}

			numChannels = int(binary.LittleEndian.Uint16(subchunk[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(subchunk[4:8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(subchunk[14:16]))
		} else if string(chunkID[:]) == "data" {
			audioData := make([]byte, size)
			reader.Read(audioData)

			if bitsPerSample == 16 {
				return &AudioStream{
					Data:       audioData,
					SampleRate: sampleRate,
					Channels:   numChannels,
					BitDepth:   bitsPerSample,
					Format:     "wav",
				}, nil
			} else if bitsPerSample == 8 {
				samples := make([]int16, len(audioData))
				for i, b := range audioData {
					samples[i] = int16(b) - 128
				}
				return &AudioStream{
					Data:       int16ToBytes(samples),
					SampleRate: sampleRate,
					Channels:   numChannels,
					BitDepth:   16,
					Format:     "wav",
				}, nil
			}
			return nil, fmt.Errorf("unsupported bit depth: %d", bitsPerSample)
		} else {
			reader.Seek(int64(size), 1)
		}
	}

	return nil, fmt.Errorf("could not find audio data in WAV file")
}

func (a *AudioAdapter) encodeWav(data []byte, sampleRate int, channels int) ([]byte, error) {
	buf := &bytes.Buffer{}

	buf.Write([]byte("RIFF"))
	var fileSize [4]byte
	binary.LittleEndian.PutUint32(fileSize[:], uint32(36+len(data)))
	buf.Write(fileSize[:])
	buf.Write([]byte("WAVE"))

	buf.Write([]byte("fmt "))
	var fmtSize [4]byte
	binary.LittleEndian.PutUint32(fmtSize[:], 16)
	buf.Write(fmtSize[:])
	var audioFormat [2]byte
	binary.LittleEndian.PutUint16(audioFormat[:], 1)
	buf.Write(audioFormat[:])
	var numCh [2]byte
	binary.LittleEndian.PutUint16(numCh[:], uint16(channels))
	buf.Write(numCh[:])
	var sampRate [4]byte
	binary.LittleEndian.PutUint32(sampRate[:], uint32(sampleRate))
	buf.Write(sampRate[:])
	var byteRate [4]byte
	binary.LittleEndian.PutUint32(byteRate[:], uint32(sampleRate*channels*2))
	buf.Write(byteRate[:])
	var blockAlign [2]byte
	binary.LittleEndian.PutUint16(blockAlign[:], uint16(channels*2))
	buf.Write(blockAlign[:])
	var bitsPerSample [2]byte
	binary.LittleEndian.PutUint16(bitsPerSample[:], 16)
	buf.Write(bitsPerSample[:])

	buf.Write([]byte("data"))
	var dataSize [4]byte
	binary.LittleEndian.PutUint32(dataSize[:], uint32(len(data)))
	buf.Write(dataSize[:])
	buf.Write(data)

	return buf.Bytes(), nil
}

func (a *AudioAdapter) decodeOggOpus(data []byte) (*AudioStream, error) {
	reader, header, err := oggreader.NewWith(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create OGG reader: %w", err)
	}

	decoder := opus.NewDecoder()
	sampleRate := int(header.SampleRate)
	channels := int(header.Channels)

	out := make([]byte, maxOPUSFrameSize*channels*2)
	var allPcm []int16

	for {
		packets, _, err := reader.ParseNextPage()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error parsing OGG page: %w", err)
		}

		for _, packet := range packets {
			if len(packet) < 2 {
				continue
			}

			n, _, err := decoder.Decode(packet, out)
			if err != nil {
				continue
			}

			if n > 0 {
				pcmSamples := bytesToInt16(out[:n*2])
				allPcm = append(allPcm, pcmSamples...)
			}
		}
	}

	if len(allPcm) == 0 {
		return nil, fmt.Errorf("no audio data decoded from OGG/Opus")
	}

	return &AudioStream{
		Data:       int16ToBytes(allPcm),
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   16,
		Format:     "opus",
	}, nil
}

func (a *AudioAdapter) prepareForLLM(stream *AudioStream) ([]byte, error) {
	samples := bytesToInt16(stream.Data)

	if stream.Channels > 1 {
		samples = a.toMono(samples, stream.Channels)
	}

	if stream.SampleRate != 16000 {
		samples = a.resample(samples, stream.SampleRate, 16000)
	}

	return int16ToBytes(samples), nil
}

func (a *AudioAdapter) toMono(samples []int16, channels int) []int16 {
	mono := make([]int16, len(samples)/channels)
	for i := 0; i < len(mono); i++ {
		var sum int32
		for c := 0; c < channels; c++ {
			sum += int32(samples[i*channels+c])
		}
		mono[i] = int16(sum / int32(channels))
	}
	return mono
}

func (a *AudioAdapter) resample(samples []int16, fromRate, toRate int) []int16 {
	if fromRate == toRate {
		return samples
	}

	ratio := float64(toRate) / float64(fromRate)
	outputLen := int(float64(len(samples)) * ratio)
	output := make([]int16, outputLen)

	for i := 0; i < outputLen; i++ {
		srcIdx := float64(i) / ratio
		idx := int(srcIdx)
		frac := srcIdx - float64(idx)

		if idx+1 >= len(samples) {
			output[i] = samples[idx]
			continue
		}

		output[i] = int16(float64(samples[idx])*(1-frac) + float64(samples[idx+1])*frac)
	}

	return output
}

func int16ToBytes(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		buf[i*2] = byte(s)
		buf[i*2+1] = byte(s >> 8)
	}
	return buf
}

func bytesToInt16(data []byte) []int16 {
	samples := make([]int16, len(data)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(data[i*2]) | int16(data[i*2+1])<<8
	}
	return samples
}

func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
