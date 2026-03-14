// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package mcp provides the OAuth 2.1 Authorization Code + PKCE flow required
// by Streamable HTTP MCP servers that protect their endpoints.
//
// Flow overview
//
//  1. Client calls InitiateOAuth(serverName, mcpURL) → returns authorizationURL.
//  2. The user opens that URL in a browser and authorizes.
//  3. The authorization server redirects to http://127.0.0.1:<port>/oauth/callback.
//  4. HandleCallback exchanges the code for a token and stores it via SecretsProvider.
//  5. The MCP client retries connectHTTP; the token is now present in secrets.
package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/infrastructure/secrets"
)

// Timeouts for OAuth flow so failures surface quickly (no long freezes).
const (
	oauthDiscoveryTimeout    = 3 * time.Second
	oauthRegistrationTimeout = 3 * time.Second
	oauthSecretsTimeout      = 5 * time.Second
	oauthTokenExchangeTimeout = 5 * time.Second
	oauthInitiateTotalTimeout = 12 * time.Second
)

// DefaultCallbackBaseURL is the fallback redirect_uri when none is configured.
const DefaultCallbackBaseURL = "http://127.0.0.1:8080/oauth/callback"

// ─── PKCE helpers ────────────────────────────────────────────────────────────

// generateCodeVerifier creates a high-entropy PKCE code_verifier (RFC 7636).
// Uses 32 random bytes → 43 URL-safe base64 chars (within the 43-128 range).
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate code verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallenge derives S256 code_challenge from a code_verifier.
func codeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// generateState creates a random CSRF state token.
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ─── Auth Server Metadata (RFC 8414) ─────────────────────────────────────────

// authServerMetadata holds the subset of RFC 8414 fields we need.
type authServerMetadata struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RegistrationEndpoint  string `json:"registration_endpoint"`
}

// discoverAuthServer fetches OAuth 2.0 Authorization Server Metadata for the
// given MCP server URL following MCP spec 2025-03-26 §2.3.2 and RFC 8414 §3.
//
// Discovery order:
//  1. GET /.well-known/oauth-protected-resource{mcpPath} to locate the canonical
//     authorization server URL (authorization_servers[0]).
//  2. Fetch RFC 8414 metadata from that authorization server:
//     a. Path-qualified: https://host/.well-known/oauth-authorization-server{path}
//     b. Root: https://authServerBase/.well-known/oauth-authorization-server
//  3. GET /.well-known/oauth-authorization-server directly on the resource host.
//  4. Fall back to default paths derived from the resource server base URL.
func discoverAuthServer(ctx context.Context, mcpURL string) (*authServerMetadata, error) {
	parsed, err := url.Parse(mcpURL)
	if err != nil {
		return nil, fmt.Errorf("oauth: invalid URL %q: %w", mcpURL, err)
	}
	// Resource server base URL: scheme + host (discard path per spec §3.1)
	resourceBase := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	mcpPath := parsed.Path // e.g. "/mcp/" — used for path-qualified well-known URLs

	httpClient := &http.Client{Timeout: oauthDiscoveryTimeout}

	fetchMeta := func(metaURL string) (*authServerMetadata, bool) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
		req.Header.Set("MCP-Protocol-Version", "2025-03-26")
		resp, err := httpClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			if err != nil {
				log.Printf("oauth: discovery GET %s: %v", metaURL, err)
			}
			return nil, false
		}
		defer resp.Body.Close()
		var meta authServerMetadata
		if err := json.NewDecoder(resp.Body).Decode(&meta); err == nil && meta.AuthorizationEndpoint != "" {
			return &meta, true
		}
		return nil, false
	}

	// fetchAuthServerMeta tries RFC 8414 §3 metadata discovery for an auth server URL.
	// Tries the path-qualified form first, then the root form.
	fetchAuthServerMeta := func(authServerURL string) (*authServerMetadata, bool) {
		asURL, err := url.Parse(authServerURL)
		if err != nil {
			return nil, false
		}
		asBase := fmt.Sprintf("%s://%s", asURL.Scheme, asURL.Host)
		asPath := asURL.Path

		// Try path-qualified per RFC 8414 §3: host/.well-known/oauth-authorization-server{path}
		if asPath != "" && asPath != "/" {
			if meta, ok := fetchMeta(asBase + "/.well-known/oauth-authorization-server" + asPath); ok {
				return meta, true
			}
		}
		// Try root form
		if meta, ok := fetchMeta(asBase + "/.well-known/oauth-authorization-server"); ok {
			return meta, true
		}
		// Fall back to default paths rooted at the full auth server URL
		return &authServerMetadata{
			AuthorizationEndpoint: strings.TrimRight(authServerURL, "/") + "/authorize",
			TokenEndpoint:         strings.TrimRight(authServerURL, "/") + "/token",
			RegistrationEndpoint:  strings.TrimRight(authServerURL, "/") + "/register",
		}, true
	}

	// Step 1: protected-resource document with path-qualified URL (RFC 9728 §3.1)
	type protectedResourceDoc struct {
		AuthorizationServers []string `json:"authorization_servers"`
	}
	protectedReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, resourceBase+"/.well-known/oauth-protected-resource"+mcpPath, nil)
	protectedReq.Header.Set("MCP-Protocol-Version", "2025-03-26")
	if protectedResp, err := httpClient.Do(protectedReq); err == nil && protectedResp.StatusCode == http.StatusOK {
		var doc protectedResourceDoc
		decErr := json.NewDecoder(protectedResp.Body).Decode(&doc)
		protectedResp.Body.Close()
		if decErr == nil && len(doc.AuthorizationServers) > 0 {
			// Step 2: fetch metadata from the declared authorization server
			if meta, ok := fetchAuthServerMeta(doc.AuthorizationServers[0]); ok {
				return meta, nil
			}
		}
	}

	// Step 3: oauth-authorization-server directly on the resource host
	if meta, ok := fetchMeta(resourceBase + "/.well-known/oauth-authorization-server"); ok {
		return meta, nil
	}

	// Step 4: fall back to default paths (spec §3.1.5)
	log.Printf("oauth: discovery: no well-known document for %s, using default authorize/token paths", resourceBase)
	return &authServerMetadata{
		AuthorizationEndpoint: resourceBase + "/authorize",
		TokenEndpoint:         resourceBase + "/token",
		RegistrationEndpoint:  resourceBase + "/register",
	}, nil
}

// ─── Dynamic Client Registration (RFC 7591) ──────────────────────────────────

// registerClient performs Dynamic Client Registration and returns the
// assigned client_id. If registration_endpoint is empty, returns "" without
// error so callers can fall back to a hard-coded client_id.
func registerClient(ctx context.Context, registrationEndpoint, callbackBaseURL string) (clientID string, err error) {
	if registrationEndpoint == "" {
		return "", nil
	}
	body := `{
		"client_name": "OpenLobster",
		"redirect_uris": ["` + callbackBaseURL + `"],
		"grant_types": ["authorization_code"],
		"response_types": ["code"],
		"token_endpoint_auth_method": "none"
	}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationEndpoint, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("oauth: create registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: oauthRegistrationTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth: dynamic client registration at %s: %w", registrationEndpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("oauth: dynamic registration failed (%d) at %s: %s", resp.StatusCode, registrationEndpoint, raw)
	}
	var result struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("oauth: invalid registration response: %w", err)
	}
	return result.ClientID, nil
}

// ─── Pending OAuth state ─────────────────────────────────────────────────────

// oauthPendingEntry tracks everything needed to complete a pending OAuth flow.
type oauthPendingEntry struct {
	ServerName      string
	MCPURL          string
	CodeVerifier    string
	TokenEndpoint   string
	ClientID        string
	CallbackBaseURL string
	CreatedAt       time.Time
}

// OAuthStatus represents the current OAuth authorization status for a server.
type OAuthStatus string

const (
	OAuthStatusPending    OAuthStatus = "pending"
	OAuthStatusAuthorized OAuthStatus = "authorized"
	OAuthStatusError      OAuthStatus = "error"
	OAuthStatusNone       OAuthStatus = "none"
)

// ─── OAuthManager ────────────────────────────────────────────────────────────

// OAuthManager orchestrates the OAuth 2.1 Authorization Code + PKCE flow for
// Streamable HTTP MCP servers.
type OAuthManager struct {
	secrets         secrets.SecretsProvider
	callbackBaseURL string // redirect_uri; must match the daemon's /oauth/callback
	pending         map[string]*oauthPendingEntry
	statuses        map[string]OAuthStatus
	errs            map[string]string
	serverURLs      map[string]string
	mu              sync.Mutex
}

// NewOAuthManager creates an OAuthManager backed by the given SecretsProvider.
// callbackBaseURL is the redirect_uri for OAuth callbacks (e.g. http://127.0.0.1:8080/oauth/callback).
// If empty, DefaultCallbackBaseURL is used.
func NewOAuthManager(sp secrets.SecretsProvider, callbackBaseURL string) *OAuthManager {
	if callbackBaseURL == "" {
		callbackBaseURL = DefaultCallbackBaseURL
	}
	return &OAuthManager{
		secrets:         sp,
		callbackBaseURL: callbackBaseURL,
		pending:         make(map[string]*oauthPendingEntry),
		statuses:        make(map[string]OAuthStatus),
		errs:            make(map[string]string),
		serverURLs:      make(map[string]string),
	}
}

// RegisterPendingServer stores a server that requires OAuth before it can connect.
// It is called when a connection attempt is rejected with a 401 status.
func (m *OAuthManager) RegisterPendingServer(name, mcpURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serverURLs[name] = mcpURL
	if _, ok := m.statuses[name]; !ok {
		m.statuses[name] = OAuthStatusNone
	}
}

// GetPendingServers returns a snapshot of servers awaiting initial OAuth (name→url).
func (m *OAuthManager) GetPendingServers() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy := make(map[string]string, len(m.serverURLs))
	for k, v := range m.serverURLs {
		copy[k] = v
	}
	return copy
}

// RemovePendingServer removes a server from the pending-auth registry (e.g.,
// after a successful connection following token acquisition).
func (m *OAuthManager) RemovePendingServer(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.serverURLs, name)
}

// SetClientID persists a custom client_id for the given server (e.g. from "advanced options"
// when adding an MCP server). InitiateOAuth will use it instead of dynamic registration.
func (m *OAuthManager) SetClientID(ctx context.Context, serverName, clientID string) error {
	if clientID == "" {
		return nil
	}
	key := fmt.Sprintf("mcp/remote/%s/client_id", serverName)
	return m.secrets.Set(ctx, key, clientID)
}

// InitiateOAuth starts the Authorization Code + PKCE flow for a remote MCP.
// Returns the authorization URL that the user must open in a browser.
func (m *OAuthManager) InitiateOAuth(ctx context.Context, serverName, mcpURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, oauthInitiateTotalTimeout)
	defer cancel()

	log.Printf("oauth: starting flow for server %q (%s)", serverName, mcpURL)

	meta, err := discoverAuthServer(ctx, mcpURL)
	if err != nil {
		return "", fmt.Errorf("oauth: discover authorization server for %s: %w", mcpURL, err)
	}

	clientIDKey := fmt.Sprintf("mcp/remote/%s/client_id", serverName)
	var clientID string
	// Use custom client_id from secrets if set (e.g. via "advanced options" when adding the server)
	if existing, getErr := m.secrets.Get(ctx, clientIDKey); getErr == nil && existing != "" {
		clientID = existing
	} else {
		// Try dynamic client registration; use "openlobster" as fallback client_id
		clientID, err = registerClient(ctx, meta.RegistrationEndpoint, m.callbackBaseURL)
		if err != nil || clientID == "" {
			if err != nil {
				log.Printf("oauth: dynamic registration failed for %s: %v (using client_id openlobster)", serverName, err)
			}
			clientID = "openlobster"
		}
	}

	// Persist client_id for future sessions; if secrets backend (e.g. OpenBao) is unreachable, fail fast with a clear error
	if err := m.secrets.Set(ctx, clientIDKey, clientID); err != nil {
		log.Printf("oauth: ERROR saving client_id in secrets (key %s): %v", clientIDKey, err)
		return "", fmt.Errorf("oauth: could not persist client_id in secrets backend (ensure OpenBao/Vault is reachable from the pod): %w", err)
	}

	verifier, err := generateCodeVerifier()
	if err != nil {
		return "", fmt.Errorf("oauth: generate code_verifier: %w", err)
	}
	state, err := generateState()
	if err != nil {
		return "", fmt.Errorf("oauth: generate state: %w", err)
	}

	m.mu.Lock()
	m.pending[state] = &oauthPendingEntry{
		ServerName:      serverName,
		MCPURL:          mcpURL,
		CodeVerifier:    verifier,
		TokenEndpoint:   meta.TokenEndpoint,
		ClientID:        clientID,
		CallbackBaseURL: m.callbackBaseURL,
		CreatedAt:       time.Now(),
	}
	m.serverURLs[serverName] = mcpURL
	m.statuses[serverName] = OAuthStatusPending
	delete(m.errs, serverName)
	m.mu.Unlock()

	// Build the authorization URL
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", m.callbackBaseURL)
	params.Set("state", state)
	params.Set("code_challenge", codeChallenge(verifier))
	params.Set("code_challenge_method", "S256")

	authURL := meta.AuthorizationEndpoint + "?" + params.Encode()
	return authURL, nil
}

// HandleCallback processes the OAuth authorization callback, exchanges the
// code for a token, and stores it in the SecretsProvider.
// Returns the server name that was authorized, or an error.
func (m *OAuthManager) HandleCallback(ctx context.Context, code, state, errParam string) (string, error) {
	m.mu.Lock()
	entry, ok := m.pending[state]
	if !ok {
		m.mu.Unlock()
		return "", fmt.Errorf("unknown or expired oauth state")
	}
	delete(m.pending, state)
	m.mu.Unlock()

	if errParam != "" {
		m.mu.Lock()
		m.statuses[entry.ServerName] = OAuthStatusError
		m.errs[entry.ServerName] = errParam
		m.mu.Unlock()
		return "", fmt.Errorf("authorization denied: %s", errParam)
	}

	// Exchange authorization code for access token
	token, err := exchangeCode(entry, code)
	if err != nil {
		m.mu.Lock()
		m.statuses[entry.ServerName] = OAuthStatusError
		m.errs[entry.ServerName] = err.Error()
		m.mu.Unlock()
		return "", err
	}

	// Persist token in SecretsProvider (short timeout so callback fails fast if OpenBao is unreachable)
	tokenKey := fmt.Sprintf("mcp/remote/%s/token", entry.ServerName)
	storeCtx, storeCancel := context.WithTimeout(ctx, oauthSecretsTimeout)
	defer storeCancel()
	if err := m.secrets.Set(storeCtx, tokenKey, token); err != nil {
		log.Printf("oauth: ERROR saving token in secrets (key %s): %v", tokenKey, err)
		m.mu.Lock()
		m.statuses[entry.ServerName] = OAuthStatusError
		m.errs[entry.ServerName] = err.Error()
		m.mu.Unlock()
		return "", fmt.Errorf("oauth: could not persist token in secrets backend (check OpenBao/Vault): %w", err)
	}

	m.mu.Lock()
	m.statuses[entry.ServerName] = OAuthStatusAuthorized
	delete(m.errs, entry.ServerName)
	m.mu.Unlock()

	return entry.ServerName, nil
}

// Status returns the current OAuth status for a server name.
func (m *OAuthManager) Status(serverName string) (OAuthStatus, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.statuses[serverName]
	if !ok {
		return OAuthStatusNone, ""
	}
	return s, m.errs[serverName]
}

// exchangeCode performs the token exchange POST against the token endpoint.
func exchangeCode(entry *oauthPendingEntry, code string) (string, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	body.Set("redirect_uri", entry.CallbackBaseURL)
	body.Set("client_id", entry.ClientID)
	body.Set("code_verifier", entry.CodeVerifier)

	client := &http.Client{Timeout: oauthTokenExchangeTimeout}
	resp, err := client.PostForm(entry.TokenEndpoint, body)
	if err != nil {
		return "", fmt.Errorf("oauth: code exchange at %s: %w", entry.TokenEndpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("oauth: token endpoint %s returned %d: %s", entry.TokenEndpoint, resp.StatusCode, raw)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("oauth: invalid token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("oauth: token endpoint returned an empty access_token")
	}
	return result.AccessToken, nil
}
