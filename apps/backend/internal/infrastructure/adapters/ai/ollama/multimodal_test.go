// Copyright (c) OpenLobster contributors. See LICENSE for details.

package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAdapter() *Adapter {
	return &Adapter{model: "llava", maxTokens: 512}
}

func TestCollectImageBlocks_Empty(t *testing.T) {
	images := collectImageBlocks(nil)
	assert.Nil(t, images)

	images = collectImageBlocks([]ports.ContentBlock{})
	assert.Nil(t, images)
}

func TestCollectImageBlocks_RawData(t *testing.T) {
	blocks := []ports.ContentBlock{
		{Type: ports.ContentBlockImage, Data: []byte{0xFF, 0xD8, 0xFF}, MIMEType: "image/jpeg"},
	}
	images := collectImageBlocks(blocks)

	require.Len(t, images, 1)
	assert.Equal(t, []byte{0xFF, 0xD8, 0xFF}, []byte(images[0]))
}

func TestCollectImageBlocks_URLFetch(t *testing.T) {
	imgBytes := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(imgBytes)
	}))
	defer srv.Close()

	blocks := []ports.ContentBlock{
		{Type: ports.ContentBlockImage, URL: srv.URL + "/img.png", MIMEType: "image/png"},
	}
	images := collectImageBlocks(blocks)

	// URL fallback removed: collectImageBlocks should not fetch external URLs.
	assert.Empty(t, images)
}

func TestCollectImageBlocks_URLFetchError(t *testing.T) {
	blocks := []ports.ContentBlock{
		{Type: ports.ContentBlockImage, URL: "http://127.0.0.1:1/nonexistent"},
	}
	// Should not panic; failed fetch is logged and skipped.
	images := collectImageBlocks(blocks)
	assert.Empty(t, images)
}

func TestCollectImageBlocks_SkipsNonImage(t *testing.T) {
	blocks := []ports.ContentBlock{
		{Type: ports.ContentBlockText, Text: "hello"},
		{Type: ports.ContentBlockAudio, Data: []byte{0x01}},
		{Type: ports.ContentBlockImage, Data: []byte{0xFF}},
	}
	images := collectImageBlocks(blocks)

	require.Len(t, images, 1)
}

func TestConvertMessages_AttachesImages(t *testing.T) {
	a := testAdapter()
	msgs := []ports.ChatMessage{
		{
			Role:    "user",
			Content: "what is this?",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockText, Text: "what is this?"},
				{Type: ports.ContentBlockImage, Data: []byte{0xDE, 0xAD}},
			},
		},
	}
	result := a.convertMessages(msgs)

	require.Len(t, result, 1)
	assert.Equal(t, "user", result[0].Role)
	assert.Equal(t, "what is this?", result[0].Content)
	require.Len(t, result[0].Images, 1)
	assert.Equal(t, []byte{0xDE, 0xAD}, []byte(result[0].Images[0]))
}

func TestConvertMessages_NoBlocksNoImages(t *testing.T) {
	a := testAdapter()
	msgs := []ports.ChatMessage{
		{Role: "user", Content: "hello"},
	}
	result := a.convertMessages(msgs)

	require.Len(t, result, 1)
	assert.Empty(t, result[0].Images)
}

func TestConvertMessages_AssistantNotAffected(t *testing.T) {
	a := testAdapter()
	msgs := []ports.ChatMessage{
		{
			Role:    "assistant",
			Content: "I see an image",
			Blocks: []ports.ContentBlock{
				{Type: ports.ContentBlockImage, Data: []byte{0xFF}},
			},
		},
	}
	result := a.convertMessages(msgs)

	// Images are only extracted for user-role messages.
	require.Len(t, result, 1)
	assert.Empty(t, result[0].Images)
}

// TestClient_JSONErrorBody verifies that when Ollama returns a non-200 status with a
// JSON body containing an "error" field, the error message is clean and human-readable.
func TestClient_JSONErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"model 'llama99' not found, try pulling it first"}`))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := NewClient(u, http.DefaultClient)

	chatErr := c.Chat(context.Background(), &ChatRequest{Model: "llama99"}, func(ChatResponse) error { return nil })

	require.Error(t, chatErr)
	assert.Contains(t, chatErr.Error(), "model 'llama99' not found")
	assert.Contains(t, chatErr.Error(), "HTTP 500")
	// Must NOT contain raw JSON braces.
	assert.NotContains(t, chatErr.Error(), `{"error"`)
}

// TestClient_NonJSONErrorBody verifies that when a proxy in front of Ollama returns
// a non-200 status with a non-JSON body (e.g. HTML), the error is truncated and does
// not flood the logs with kilobytes of content.
func TestClient_NonJSONErrorBody(t *testing.T) {
	bigHTML := strings.Repeat("<html>proxy error body</html>", 100) // ~2 900 chars

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(bigHTML))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := NewClient(u, http.DefaultClient)

	chatErr := c.Chat(context.Background(), &ChatRequest{Model: "any"}, func(ChatResponse) error { return nil })

	require.Error(t, chatErr)
	assert.Contains(t, chatErr.Error(), "503")
	// Error message must be short — no more than 300 characters total.
	assert.LessOrEqual(t, len(chatErr.Error()), 300)
}
