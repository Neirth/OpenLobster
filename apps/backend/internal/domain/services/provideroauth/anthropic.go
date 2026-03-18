package provideroauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	anthropicClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	anthropicAuthorizeURL = "https://claude.ai/oauth/authorize"
	anthropicTokenURL     = "https://platform.claude.com/v1/oauth/token"
	anthropicCallbackPort = 53692
	anthropicCallbackPath = "/callback"
	anthropicRedirectURI  = "http://localhost:53692/callback"
	anthropicScopes       = "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"

	anthropicCallbackTimeout = 60 * time.Second
)

// AnthropicProvider implements the Anthropic OAuth flow (Claude Pro/Max).
//
// NOTE: Anthropic has been restricting third-party OAuth usage since January 2026.
// This implementation follows the official flow but may not work in practice.
type AnthropicProvider struct{}

func init() {
	Register(&AnthropicProvider{})
}

func (p *AnthropicProvider) ID() string   { return "anthropic" }
func (p *AnthropicProvider) Name() string { return "Anthropic (Claude Pro/Max)" }

func (p *AnthropicProvider) Login(ctx context.Context, cb LoginCallbacks) (*Credentials, error) {
	verifier, err := generateCodeVerifier()
	if err != nil {
		return nil, err
	}
	challenge := codeChallenge(verifier)

	// Start local callback server (state = verifier for Anthropic's flow)
	srv, err := startCallbackServer(anthropicCallbackPort, anthropicCallbackPath, verifier)
	if err != nil {
		return nil, fmt.Errorf("anthropic: start callback server: %w", err)
	}
	defer srv.close()

	// Build authorization URL
	params := url.Values{
		"code":                  {"true"},
		"client_id":             {anthropicClientID},
		"response_type":         {"code"},
		"redirect_uri":          {anthropicRedirectURI},
		"scope":                 {anthropicScopes},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {verifier},
	}
	authURL := anthropicAuthorizeURL + "?" + params.Encode()

	cb.OnAuth(authURL, "Complete login in your browser.")
	if cb.OnProgress != nil {
		cb.OnProgress("Waiting for browser callback...")
	}

	// Wait for callback
	var code, state string
	result := srv.waitForCode(ctx, anthropicCallbackTimeout)
	if result != nil {
		code = result.Code
		state = result.State
	}

	// Fallback to manual prompt
	if code == "" && cb.OnPrompt != nil {
		input, promptErr := cb.OnPrompt("Paste the authorization code or full redirect URL:")
		if promptErr != nil {
			return nil, promptErr
		}
		parsedCode, parsedState := parseAuthorizationInput(input)
		if parsedState != "" && parsedState != verifier {
			return nil, fmt.Errorf("anthropic: state mismatch")
		}
		code = parsedCode
		if parsedState != "" {
			state = parsedState
		} else {
			state = verifier
		}
	}

	if code == "" {
		return nil, fmt.Errorf("anthropic: no authorization code received")
	}
	if state == "" {
		state = verifier
	}

	if cb.OnProgress != nil {
		cb.OnProgress("Exchanging authorization code for tokens...")
	}
	return anthropicExchangeCode(code, state, verifier, anthropicRedirectURI)
}

func (p *AnthropicProvider) RefreshToken(_ context.Context, creds *Credentials) (*Credentials, error) {
	return anthropicRefreshToken(creds.Refresh)
}

func (p *AnthropicProvider) GetAPIKey(creds *Credentials) string {
	return creds.Access
}

func anthropicExchangeCode(code, state, verifier, redirectURI string) (*Credentials, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"grant_type":    "authorization_code",
		"client_id":     anthropicClientID,
		"code":          code,
		"state":         state,
		"redirect_uri":  redirectURI,
		"code_verifier": verifier,
	})

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(anthropicTokenURL, "application/json", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("anthropic: token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic: token exchange failed (%d): %s", resp.StatusCode, raw)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("anthropic: invalid token response: %w", err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("anthropic: token response missing fields")
	}

	// Subtract 5 minutes buffer from expiry
	return &Credentials{
		Access:  tokenResp.AccessToken,
		Refresh: tokenResp.RefreshToken,
		Expires: timeNowMs() + tokenResp.ExpiresIn*1000 - 5*60*1000,
	}, nil
}

func anthropicRefreshToken(refreshToken string) (*Credentials, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     anthropicClientID,
		"refresh_token": refreshToken,
	})

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(anthropicTokenURL, "application/json", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("anthropic: token refresh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic: token refresh failed (%d): %s", resp.StatusCode, raw)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("anthropic: invalid refresh response: %w", err)
	}

	return &Credentials{
		Access:  tokenResp.AccessToken,
		Refresh: tokenResp.RefreshToken,
		Expires: timeNowMs() + tokenResp.ExpiresIn*1000 - 5*60*1000,
	}, nil
}
