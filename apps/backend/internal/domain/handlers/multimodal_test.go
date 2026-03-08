// Copyright (c) OpenLobster contributors. See LICENSE for details.

package handlers

import (
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHandler returns a minimal MessageHandler sufficient for unit tests that
// do not require external dependencies.
func newTestHandler() *MessageHandler {
	return &MessageHandler{}
}

func TestBuildLatestUserMessage_TextOnly(t *testing.T) {
	h := newTestHandler()
	msg := h.buildLatestUserMessage("hello", nil, nil)

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "hello", msg.Content)
	assert.Empty(t, msg.Blocks, "no blocks expected for plain text")
}

func TestBuildLatestUserMessage_ImageAttachment(t *testing.T) {
	h := newTestHandler()
	attachments := []models.Attachment{
		{Type: "image", Data: []byte("https://example.com/photo.jpg"), MIMEType: "image/jpeg"},
	}
	msg := h.buildLatestUserMessage("check this", attachments, nil)

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
	msg := h.buildLatestUserMessage("", attachments, nil)

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
	msg := h.buildLatestUserMessage("transcribe this", nil, audio)

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
	msg := h.buildLatestUserMessage("see attached", attachments, nil)

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
	msg := h.buildLatestUserMessage("two images and a csv", attachments, nil)

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
	msg := h.buildLatestUserMessage("hello", nil, audio)

	assert.Empty(t, msg.Blocks)
	assert.Equal(t, "hello", msg.Content)
}

func TestBuildLatestUserMessage_NoTextNoAttachment(t *testing.T) {
	h := newTestHandler()
	msg := h.buildLatestUserMessage("", nil, nil)

	assert.Empty(t, msg.Blocks)
	assert.Equal(t, "", msg.Content)
}

func TestBuildLatestUserMessage_ImageAndAudioCombined(t *testing.T) {
	h := newTestHandler()
	attachments := []models.Attachment{
		{Type: "image", Data: []byte("https://example.com/photo.jpg"), MIMEType: "image/jpeg"},
	}
	audio := &models.AudioContent{Data: []byte{0xFF, 0xFE}, Format: "audio/wav"}

	msg := h.buildLatestUserMessage("look and listen", attachments, audio)

	// text + image + audio = 3 blocks
	require.Len(t, msg.Blocks, 3)
	assert.Equal(t, ports.ContentBlockText, msg.Blocks[0].Type)
	assert.Equal(t, ports.ContentBlockImage, msg.Blocks[1].Type)
	assert.Equal(t, ports.ContentBlockAudio, msg.Blocks[2].Type)
}
