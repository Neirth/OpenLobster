// Copyright (c) OpenLobster contributors. See LICENSE for details.

package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAudioProvider struct {
	sttText  string
	sttErr   error
	sttCalls int
	ttsCalls int
}

func (f *fakeAudioProvider) TextToSpeech(ctx context.Context, req ports.TTSRequest) (ports.TTSResponse, error) {
	f.ttsCalls++
	return ports.TTSResponse{Audio: []byte("audio"), Format: "mp3"}, nil
}

func (f *fakeAudioProvider) SpeechToText(ctx context.Context, req ports.STTRequest) (ports.STTResponse, error) {
	f.sttCalls++
	if f.sttErr != nil {
		return ports.STTResponse{}, f.sttErr
	}
	return ports.STTResponse{Text: f.sttText, Language: "es"}, nil
}

type fakeAIProviderCaps struct {
	supportsAudioInput  bool
	supportsAudioOutput bool
}

func (f *fakeAIProviderCaps) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, nil
}

func (f *fakeAIProviderCaps) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, nil
}

func (f *fakeAIProviderCaps) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, nil
}

func (f *fakeAIProviderCaps) SupportsAudioInput() bool  { return f.supportsAudioInput }
func (f *fakeAIProviderCaps) SupportsAudioOutput() bool { return f.supportsAudioOutput }
func (f *fakeAIProviderCaps) GetMaxTokens() int         { return 500 }
func (f *fakeAIProviderCaps) GetContextWindow() int     { return 128000 }

// newTestHandler returns a minimal MessageHandler sufficient for unit tests that
// do not require external dependencies.
func newTestHandler() *MessageHandler {
	return &MessageHandler{}
}

func TestBuildLatestUserMessage_TextOnly(t *testing.T) {
	h := newTestHandler()
	msg := h.buildLatestUserMessage(context.Background(), "hello", nil, nil)

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "hello", msg.Content)
	assert.Empty(t, msg.Blocks, "no blocks expected for plain text")
}

func TestBuildLatestUserMessage_ImageAttachment(t *testing.T) {
	h := newTestHandler()
	attachments := []models.Attachment{
		{Type: "image", Data: []byte("https://example.com/photo.jpg"), MIMEType: "image/jpeg"},
	}
	msg := h.buildLatestUserMessage(context.Background(), "check this", attachments, nil)

	require.Len(t, msg.Blocks, 2)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[0].Type)
	assert.Equal(t, "check this", msg.Blocks[0].Text)
	assert.Equal(t, ports.ContentBlockImage, msg.Blocks[1].Type)
	assert.Equal(t, []byte("https://example.com/photo.jpg"), msg.Blocks[1].Data)
	assert.Equal(t, "image/jpeg", msg.Blocks[1].MIMEType)
}

func TestBuildLatestUserMessage_AudioAttachment(t *testing.T) {
	h := newTestHandler()
	attachments := []models.Attachment{
		{Type: "audio", Data: []byte("https://example.com/voice.ogg"), MIMEType: "audio/ogg"},
	}
	msg := h.buildLatestUserMessage(context.Background(), "", attachments, nil)

	require.Len(t, msg.Blocks, 1)
	assert.Equal(t, ports.ContentBlockAudio, msg.Blocks[0].Type)
	assert.Equal(t, []byte("https://example.com/voice.ogg"), msg.Blocks[0].Data)
}

func TestBuildLatestUserMessage_RawAudio(t *testing.T) {
	h := newTestHandler()
	audio := &models.AudioContent{
		Data:   []byte{0x01, 0x02, 0x03},
		Format: "audio/wav",
	}
	msg := h.buildLatestUserMessage(context.Background(), "transcribe this", nil, audio)

	require.Len(t, msg.Blocks, 2)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[0].Type)
	assert.Equal(t, ports.ContentBlockAudio, msg.Blocks[1].Type)
	assert.Equal(t, []byte{0x01, 0x02, 0x03}, msg.Blocks[1].Data)
	assert.Equal(t, "audio/wav", msg.Blocks[1].MIMEType)
}

func TestBuildLatestUserMessage_UnsupportedAttachment(t *testing.T) {
	h := newTestHandler()
	attachments := []models.Attachment{
		{Type: "document", Filename: "report.pdf", MIMEType: "application/pdf"},
	}
	msg := h.buildLatestUserMessage(context.Background(), "see attached", attachments, nil)

	require.Len(t, msg.Blocks, 2)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[0].Type)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[1].Type)
	assert.Contains(t, msg.Blocks[1].Text, "application/pdf")
	assert.Contains(t, msg.Blocks[1].Text, "report.pdf")
}

func TestBuildLatestUserMessage_MultipleAttachments(t *testing.T) {
	h := newTestHandler()
	attachments := []models.Attachment{
		{Type: "image", Data: []byte("https://example.com/img1.png"), MIMEType: "image/png"},
		{Type: "image", Data: []byte("https://example.com/img2.png"), MIMEType: "image/png"},
		{Type: "document", Filename: "data.csv", MIMEType: "text/csv"},
	}
	msg := h.buildLatestUserMessage(context.Background(), "two images and a csv", attachments, nil)

	// text + img + img + unsupported notice = 4 blocks
	require.Len(t, msg.Blocks, 4)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[0].Type)
	assert.Equal(t, ports.ContentBlockImage, msg.Blocks[1].Type)
	assert.Equal(t, ports.ContentBlockImage, msg.Blocks[2].Type)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[3].Type)
	assert.Contains(t, msg.Blocks[3].Text, "text/csv")
}

func TestBuildLatestUserMessage_EmptyAudioIgnored(t *testing.T) {
	h := newTestHandler()
	// Audio with no data must not produce a block.
	audio := &models.AudioContent{Data: nil, Format: "audio/wav"}
	msg := h.buildLatestUserMessage(context.Background(), "hello", nil, audio)

	assert.Empty(t, msg.Blocks)
	assert.Equal(t, "hello", msg.Content)
}

func TestBuildLatestUserMessage_NoTextNoAttachment(t *testing.T) {
	h := newTestHandler()
	msg := h.buildLatestUserMessage(context.Background(), "", nil, nil)

	assert.Empty(t, msg.Blocks)
	assert.Equal(t, "", msg.Content)
}

func TestBuildLatestUserMessage_ImageAndAudioCombined(t *testing.T) {
	h := newTestHandler()
	attachments := []models.Attachment{
		{Type: "image", Data: []byte("https://example.com/photo.jpg"), MIMEType: "image/jpeg"},
	}
	audio := &models.AudioContent{Data: []byte{0xFF, 0xFE}, Format: "audio/wav"}

	msg := h.buildLatestUserMessage(context.Background(), "look and listen", attachments, audio)

	// text + image + audio = 3 blocks
	require.Len(t, msg.Blocks, 3)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[0].Type)
	assert.Equal(t, ports.ContentBlockImage, msg.Blocks[1].Type)
	assert.Equal(t, ports.ContentBlockAudio, msg.Blocks[2].Type)
}

func TestBuildLatestUserMessage_STTFallbackWhenAudioInputNotSupported(t *testing.T) {
	h := newTestHandler()
	fakeAudio := &fakeAudioProvider{sttText: "transcripcion"}
	h.audioProvider = fakeAudio
	h.runner.aiProvider = &fakeAIProviderCaps{supportsAudioInput: false, supportsAudioOutput: true}

	audio := &models.AudioContent{Data: []byte{0x01, 0x02}, Format: "audio/ogg"}
	msg := h.buildLatestUserMessage(context.Background(), "", nil, audio)

	require.Len(t, msg.Blocks, 1)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[0].Type)
	assert.Equal(t, "transcripcion", msg.Blocks[0].Text)
	assert.Equal(t, 1, fakeAudio.sttCalls)
}

func TestBuildLatestUserMessage_NoSTTFallbackWhenAudioInputSupported(t *testing.T) {
	h := newTestHandler()
	fakeAudio := &fakeAudioProvider{sttText: "ignored"}
	h.audioProvider = fakeAudio
	h.runner.aiProvider = &fakeAIProviderCaps{supportsAudioInput: true, supportsAudioOutput: false}

	audio := &models.AudioContent{Data: []byte{0x01, 0x02}, Format: "audio/ogg"}
	msg := h.buildLatestUserMessage(context.Background(), "", nil, audio)

	require.Len(t, msg.Blocks, 1)
	assert.Equal(t, ports.ContentBlockAudio, msg.Blocks[0].Type)
	assert.Equal(t, 0, fakeAudio.sttCalls)
}

func TestBuildLatestUserMessage_STTFallbackErrorKeepsAudioBlock(t *testing.T) {
	h := newTestHandler()
	fakeAudio := &fakeAudioProvider{sttErr: errors.New("stt failed")}
	h.audioProvider = fakeAudio
	h.runner.aiProvider = &fakeAIProviderCaps{supportsAudioInput: false, supportsAudioOutput: false}

	audio := &models.AudioContent{Data: []byte{0x01, 0x02}, Format: "audio/ogg"}
	msg := h.buildLatestUserMessage(context.Background(), "", nil, audio)

	require.Len(t, msg.Blocks, 1)
	assert.Equal(t, ports.ContentBlockAudio, msg.Blocks[0].Type)
	assert.Equal(t, 1, fakeAudio.sttCalls)
}
