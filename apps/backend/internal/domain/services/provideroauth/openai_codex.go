package provideroauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	openaiClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	openaiAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	openaiTokenURL     = "https://auth.openai.com/oauth/token"
	openaiCallbackPort = 1455
	openaiCallbackPath = "/auth/callback"
	openaiRedirectURI  = "http://localhost:1455/auth/callback"
	openaiScope        = "openid profile email offline_access"
	openaiJWTClaimPath = "https://api.openai.com/auth"

	openaiCallbackTimeout = 60 * time.Second
)

// OpenAICodexProvider implements the OpenAI Codex OAuth flow (ChatGPT Plus/Pro).
type OpenAICodexProvider struct{}

func init() {
	Register(&OpenAICodexProvider{})
}

func (p *OpenAICodexProvider) ID() string   { return "openai-codex" }
func (p *OpenAICodexProvider) Name() string { return "ChatGPT Plus/Pro (Codex)" }

func (p *OpenAICodexProvider) Login(ctx context.Context, cb LoginCallbacks) (*Credentials, error) {
	verifier, err := generateCodeVerifier()
	if err != nil {
		return nil, err
	}
	challenge := codeChallenge(verifier)

	state, err := generateState()
	if err != nil {
		return nil, err
	}

	// Start local callback server
	srv, err := startCallbackServer(openaiCallbackPort, openaiCallbackPath, state)
	if err != nil {
		return nil, fmt.Errorf("openai-codex: start callback server: %w", err)
	}
	defer srv.close()

	// Build authorization URL
	params := url.Values{
		"response_type":               {"code"},
		"client_id":                   {openaiClientID},
		"redirect_uri":                {openaiRedirectURI},
		"scope":                       {openaiScope},
		"code_challenge":              {challenge},
		"code_challenge_method":       {"S256"},
		"state":                       {state},
		"id_token_add_organizations":  {"true"},
		"codex_cli_simplified_flow":   {"true"},
		"originator":                  {"openlobster"},
	}
	authURL := openaiAuthorizeURL + "?" + params.Encode()

	cb.OnAuth(authURL, "Complete sign-in in your browser.")
	if cb.OnProgress != nil {
		cb.OnProgress("Waiting for browser callback...")
	}

	// Wait for callback or manual input
	var code string
	result := srv.waitForCode(ctx, openaiCallbackTimeout)
	if result != nil {
		code = result.Code
	}

	// Fallback to manual prompt
	if code == "" && cb.OnPrompt != nil {
		input, promptErr := cb.OnPrompt("Paste the authorization code or full redirect URL:")
		if promptErr != nil {
			return nil, promptErr
		}
		parsedCode, parsedState := parseAuthorizationInput(input)
		if parsedState != "" && parsedState != state {
			return nil, fmt.Errorf("openai-codex: state mismatch")
		}
		code = parsedCode
	}

	if code == "" {
		return nil, fmt.Errorf("openai-codex: no authorization code received")
	}

	return openaiExchangeCode(code, verifier, openaiRedirectURI)
}

func (p *OpenAICodexProvider) RefreshToken(_ context.Context, creds *Credentials) (*Credentials, error) {
	return openaiRefreshToken(creds.Refresh)
}

func (p *OpenAICodexProvider) GetAPIKey(creds *Credentials) string {
	return creds.Access
}

func openaiExchangeCode(code, verifier, redirectURI string) (*Credentials, error) {
	body := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {openaiClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(openaiTokenURL, "application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("openai-codex: token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai-codex: token exchange failed (%d): %s", resp.StatusCode, raw)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("openai-codex: invalid token response: %w", err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("openai-codex: token response missing fields")
	}

	return &Credentials{
		Access:    tokenResp.AccessToken,
		Refresh:   tokenResp.RefreshToken,
		Expires:   timeNowMs() + tokenResp.ExpiresIn*1000,
		AccountID: extractJWTClaim(tokenResp.AccessToken, openaiJWTClaimPath, "chatgpt_account_id"),
	}, nil
}

func openaiRefreshToken(refreshToken string) (*Credentials, error) {
	body := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {openaiClientID},
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(openaiTokenURL, "application/x-www-form-urlencoded", strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("openai-codex: token refresh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai-codex: token refresh failed (%d): %s", resp.StatusCode, raw)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("openai-codex: invalid refresh response: %w", err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("openai-codex: refresh response missing fields")
	}

	return &Credentials{
		Access:    tokenResp.AccessToken,
		Refresh:   tokenResp.RefreshToken,
		Expires:   timeNowMs() + tokenResp.ExpiresIn*1000,
		AccountID: extractJWTClaim(tokenResp.AccessToken, openaiJWTClaimPath, "chatgpt_account_id"),
	}, nil
}

// extractJWTClaim extracts a nested claim from a JWT without signature verification.
// It looks for claims[claimPath][field].
func extractJWTClaim(token, claimPath, field string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims map[string]json.RawMessage
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}

	nested, ok := claims[claimPath]
	if !ok {
		return ""
	}

	var nestedMap map[string]interface{}
	if err := json.Unmarshal(nested, &nestedMap); err != nil {
		return ""
	}

	val, _ := nestedMap[field].(string)
	return val
}
