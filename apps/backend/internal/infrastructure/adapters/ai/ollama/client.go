// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: see LICENSE

package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is an Ollama API client that communicates directly over HTTP.
type Client struct {
	base       *url.URL
	httpClient *http.Client
}

// NewClient creates a new Client pointing at the given base URL.
func NewClient(base *url.URL, httpClient *http.Client) *Client {
	return &Client{base: base, httpClient: httpClient}
}

// ClientFromEnvironment creates a Client pointing at the default Ollama server
// (http://localhost:11434). It mirrors the behaviour of the official SDK.
func ClientFromEnvironment() (*Client, error) {
	u, _ := url.Parse("http://localhost:11434")
	return &Client{base: u, httpClient: http.DefaultClient}, nil
}

// Chat sends a chat completion request to POST /api/chat and calls fn once with
// the response. stream is forced to false; fn is called exactly once.
func (c *Client) Chat(ctx context.Context, req *ChatRequest, fn func(ChatResponse) error) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("ollama: marshal request: %w", err)
	}

	endpoint := c.base.String() + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama: server %d: %s", resp.StatusCode, string(b))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return fmt.Errorf("ollama: decode response: %w", err)
	}

	return fn(chatResp)
}
