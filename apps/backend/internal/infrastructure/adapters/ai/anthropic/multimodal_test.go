// Copyright (c) OpenLobster contributors. See LICENSE for details.

package anthropic

import (
	"testing"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertUserBlocks_TextOnly(t *testing.T) {
	msg := ports.ChatMessage{Role: "user", Content: "hello"}
	blocks := convertUserBlocks(msg)

	require.Len(t, blocks, 1)
	require.NotNil(t, blocks[0].OfText)
	assert.Equal(t, "hello", blocks[0].OfText.Text)
}

func TestConvertUserBlocks_FallsBackToContentWhenBlocksEmpty(t *testing.T) {
	msg := ports.ChatMessage{Role: "user", Content: "fallback text", Blocks: []ports.ContentBlock{}}
	blocks := convertUserBlocks(msg)

	require.Len(t, blocks, 1)
	require.NotNil(t, blocks[0].OfText)
	assert.Equal(t, "fallback text", blocks[0].OfText.Text)
}

func TestConvertUserBlocks_ImageURL(t *testing.T) {
	msg := ports.ChatMessage{
		Role:    "user",
		Content: "look at this",
		Blocks: []ports.ContentBlock{
			{Type: ports.ContentBlockText, Text: "look at this"},
			{Type: ports.ContentBlockImage, URL: "https://example.com/img.jpg", MIMEType: "image/jpeg"},
		},
	}
	blocks := convertUserBlocks(msg)

	require.Len(t, blocks, 2)
	assert.NotNil(t, blocks[0].OfText)
	require.NotNil(t, blocks[1].OfImage)
	require.NotNil(t, blocks[1].OfImage.Source.OfURL)
	assert.Equal(t, "https://example.com/img.jpg", blocks[1].OfImage.Source.OfURL.URL)
}

func TestConvertUserBlocks_ImageBase64(t *testing.T) {
	msg := ports.ChatMessage{
		Role:    "user",
		Content: "",
		Blocks: []ports.ContentBlock{
			{Type: ports.ContentBlockImage, Data: []byte{0xFF, 0xD8, 0xFF}, MIMEType: "image/jpeg"},
		},
	}
	blocks := convertUserBlocks(msg)

	require.Len(t, blocks, 1)
	require.NotNil(t, blocks[0].OfImage)
	require.NotNil(t, blocks[0].OfImage.Source.OfBase64)
	assert.Equal(t, "/9j/", blocks[0].OfImage.Source.OfBase64.Data[:4])
}

func TestConvertUserBlocks_AudioSkipped(t *testing.T) {
	msg := ports.ChatMessage{
		Role: "user",
		Blocks: []ports.ContentBlock{
			{Type: ports.ContentBlockText, Text: "transcribe"},
			{Type: ports.ContentBlockAudio, Data: []byte{0x01}, MIMEType: "audio/wav"},
		},
	}
	blocks := convertUserBlocks(msg)

	// Audio is skipped; only text block should remain.
	require.Len(t, blocks, 1)
	assert.NotNil(t, blocks[0].OfText)
	assert.Equal(t, "transcribe", blocks[0].OfText.Text)
}

func TestConvertUserBlocks_AllAudioFallsBackToContent(t *testing.T) {
	msg := ports.ChatMessage{
		Role:    "user",
		Content: "voice message",
		Blocks: []ports.ContentBlock{
			{Type: ports.ContentBlockAudio, Data: []byte{0x01}, MIMEType: "audio/wav"},
		},
	}
	blocks := convertUserBlocks(msg)

	// Audio-only message: all blocks skipped, fallback to Content text.
	require.Len(t, blocks, 1)
	assert.NotNil(t, blocks[0].OfText)
	assert.Equal(t, "voice message", blocks[0].OfText.Text)
}

func TestConvertMessages_SystemExtracted(t *testing.T) {
	msgs := []ports.ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi"},
	}
	systemBlocks, params := convertMessages(msgs)

	require.Len(t, systemBlocks, 1)
	assert.Equal(t, "You are helpful.", systemBlocks[0].Text)
	require.Len(t, params, 1)
}

func TestConvertMessages_MultimodalUser(t *testing.T) {
	msgs := []ports.ChatMessage{
		{
			Role:    "user",
			Content: "describe",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockText, Text: "describe"},
				{Type: ports.ContentBlockImage, URL: "https://example.com/x.png", MIMEType: "image/png"},
			},
		},
	}
	_, params := convertMessages(msgs)

	require.Len(t, params, 1)
}
