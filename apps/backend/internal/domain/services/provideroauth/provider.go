// Package provideroauth implements OAuth authentication flows for AI providers.
//
// It supports OpenAI Codex (ChatGPT Plus/Pro), Anthropic (Claude Pro/Max),
// GitHub Copilot, Google Gemini CLI, and Google Antigravity — ported from
// the pi-ai library used by OpenClaw.
package provideroauth

import (
	"context"
	"fmt"
	"sync"
)

// Credentials holds OAuth tokens for an AI provider.
type Credentials struct {
	Access    string            `json:"access"`
	Refresh   string            `json:"refresh"`
	Expires   int64             `json:"expires"` // unix milliseconds
	AccountID string            `json:"account_id,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

// IsExpired reports whether the access token has expired.
func (c *Credentials) IsExpired() bool {
	return timeNowMs() >= c.Expires
}

// LoginCallbacks provides interaction hooks for OAuth flows that need
// user input (opening a browser, pasting a redirect URL, etc.).
type LoginCallbacks struct {
	// OnAuth is called with the URL the user should open and optional instructions.
	OnAuth func(url string, instructions string)
	// OnPrompt asks the user for text input (e.g. paste the redirect URL).
	OnPrompt func(message string) (string, error)
	// OnProgress reports status messages during the flow.
	OnProgress func(message string)
}

// OAuthProvider defines the contract for an AI provider OAuth flow.
type OAuthProvider interface {
	// ID returns the unique provider identifier (e.g. "openai-codex").
	ID() string
	// Name returns a human-readable name (e.g. "ChatGPT Plus/Pro").
	Name() string
	// Login initiates the OAuth flow and returns credentials.
	Login(ctx context.Context, callbacks LoginCallbacks) (*Credentials, error)
	// RefreshToken refreshes expired credentials.
	RefreshToken(ctx context.Context, creds *Credentials) (*Credentials, error)
	// GetAPIKey extracts the usable API key/bearer token from credentials.
	GetAPIKey(creds *Credentials) string
}

// registry holds all registered OAuth providers.
var (
	registryMu sync.RWMutex
	registry   = make(map[string]OAuthProvider)
)

// Register adds a provider to the global registry.
func Register(p OAuthProvider) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[p.ID()] = p
}

// Get returns a registered provider by ID, or nil if not found.
func Get(id string) OAuthProvider {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[id]
}

// List returns all registered providers.
func List() []OAuthProvider {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]OAuthProvider, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	return out
}

// ProfileInfo describes a named OAuth profile for a provider.
type ProfileInfo struct {
	Name          string
	Authenticated bool
	AccountID     string
	Email         string
}

// ErrProviderNotFound is returned when a provider ID is not in the registry.
var ErrProviderNotFound = fmt.Errorf("oauth provider not found")
