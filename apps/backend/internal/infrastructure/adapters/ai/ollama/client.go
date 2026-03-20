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
	"os"
)

// chatClient is the internal interface used by Adapter to send chat requests.
// *Client satisfies it; errClient is a no-op implementation used when
// initialization fails so that a.client is never nil.
type chatClient interface {
	Chat(ctx context.Context, req *ChatRequest, fn func(ChatResponse) error) error
}

// Client is an Ollama API client that communicates directly over HTTP.
type Client struct {
	base       *url.URL
	httpClient *http.Client
}

// NewClient creates a new Client pointing at the given base URL.
func NewClient(base *url.URL, httpClient *http.Client) *Client {
	return &Client{base: base, httpClient: httpClient}
}

// errClient is a chatClient that always returns a fixed initialization error.
type errClient struct{ err error }

func (e *errClient) Chat(_ context.Context, _ *ChatRequest, _ func(ChatResponse) error) error {
	return e.err
}

// ClientFromEnvironment creates a Client pointing at the Ollama server.
// It reads OLLAMA_HOST from the environment, falling back to http://localhost:11434.
func ClientFromEnvironment() (*Client, error) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("ollama: invalid OLLAMA_HOST %q: %w", host, err)
	}
	return &Client{base: u, httpClient: http.DefaultClient}, nil
}

// Chat sends a chat completion request to POST /api/chat and calls fn once with
// the response. stream is forced to false; fn is called exactly once.
func (c *Client) Chat(ctx context.Context, req *ChatRequest, fn func(ChatResponse) error) error {
	streamFalse := false
	req.Stream = &streamFalse
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
		// Ollama always wraps errors as {"error":"<human-readable message>"}.
		// Parse that field so logs show something useful instead of raw JSON.
		var errBody struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(b, &errBody) == nil && errBody.Error != "" {
			return fmt.Errorf("ollama: %s (HTTP %d)", errBody.Error, resp.StatusCode)
		}
		// Fallback: body is not valid JSON (e.g. an HTML proxy page).
		// Truncate to avoid flooding logs with kilobytes of HTML.
		raw := string(b)
		if len(raw) > 200 {
			raw = raw[:200] + "…"
		}
		return fmt.Errorf("ollama: server error %d: %s", resp.StatusCode, raw)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return fmt.Errorf("ollama: decode response: %w", err)
	}

	return fn(chatResp)
}
