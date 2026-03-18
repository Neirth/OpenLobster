package provideroauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	copilotClientID = "Iv1.b507a08c87ecfe98"

	copilotUserAgent    = "GitHubCopilotChat/0.35.0"
	copilotEditorVer    = "vscode/1.107.0"
	copilotPluginVer    = "copilot-chat/0.35.0"
	copilotIntegrationID = "vscode-chat"
)

var proxyEPRegexp = regexp.MustCompile(`proxy-ep=([^;]+)`)

// GitHubCopilotProvider implements the GitHub Copilot OAuth flow (device code).
type GitHubCopilotProvider struct{}

func init() {
	Register(&GitHubCopilotProvider{})
}

func (p *GitHubCopilotProvider) ID() string   { return "github-copilot" }
func (p *GitHubCopilotProvider) Name() string { return "GitHub Copilot" }

func (p *GitHubCopilotProvider) Login(ctx context.Context, cb LoginCallbacks) (*Credentials, error) {
	// Ask for enterprise domain (optional)
	domain := "github.com"
	if cb.OnPrompt != nil {
		input, err := cb.OnPrompt("GitHub Enterprise domain (blank for github.com):")
		if err != nil {
			return nil, err
		}
		if trimmed := strings.TrimSpace(input); trimmed != "" {
			domain = normalizeDomain(trimmed)
			if domain == "" {
				return nil, fmt.Errorf("github-copilot: invalid domain: %s", trimmed)
			}
		}
	}

	// Start device code flow
	deviceCode, err := copilotRequestDeviceCode(ctx, domain)
	if err != nil {
		return nil, err
	}

	cb.OnAuth(deviceCode.VerificationURI, fmt.Sprintf("Enter code: %s", deviceCode.UserCode))

	if cb.OnProgress != nil {
		cb.OnProgress("Waiting for GitHub authorization...")
	}

	// Poll for access token
	githubToken, err := copilotPollForToken(ctx, domain, deviceCode)
	if err != nil {
		return nil, err
	}

	// Exchange GitHub token for Copilot token
	creds, err := copilotRefreshToken(githubToken, domain)
	if err != nil {
		return nil, err
	}

	// Store GitHub token as refresh, Copilot token as access
	creds.Refresh = githubToken
	if domain != "github.com" {
		if creds.Extra == nil {
			creds.Extra = make(map[string]string)
		}
		creds.Extra["enterprise_domain"] = domain
	}

	return creds, nil
}

func (p *GitHubCopilotProvider) RefreshToken(_ context.Context, creds *Credentials) (*Credentials, error) {
	domain := "github.com"
	if creds.Extra != nil {
		if d, ok := creds.Extra["enterprise_domain"]; ok && d != "" {
			domain = d
		}
	}
	refreshed, err := copilotRefreshToken(creds.Refresh, domain)
	if err != nil {
		return nil, err
	}
	refreshed.Refresh = creds.Refresh
	refreshed.Extra = creds.Extra
	return refreshed, nil
}

func (p *GitHubCopilotProvider) GetAPIKey(creds *Credentials) string {
	return creds.Access
}

// GetBaseURL returns the API base URL extracted from a Copilot token.
func GetCopilotBaseURL(token string, enterpriseDomain string) string {
	if match := proxyEPRegexp.FindStringSubmatch(token); len(match) > 1 {
		apiHost := strings.Replace(match[1], "proxy.", "api.", 1)
		return "https://" + apiHost
	}
	if enterpriseDomain != "" {
		return "https://copilot-api." + enterpriseDomain
	}
	return "https://api.individual.githubcopilot.com"
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expires_in"`
}

func copilotRequestDeviceCode(ctx context.Context, domain string) (*deviceCodeResponse, error) {
	deviceCodeURL := fmt.Sprintf("https://%s/login/device/code", domain)

	body := url.Values{
		"client_id": {copilotClientID},
		"scope":     {"read:user"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("github-copilot: create device code request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", copilotUserAgent)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github-copilot: device code request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github-copilot: device code failed (%d): %s", resp.StatusCode, raw)
	}

	var result deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("github-copilot: invalid device code response: %w", err)
	}

	if result.DeviceCode == "" || result.UserCode == "" || result.VerificationURI == "" {
		return nil, fmt.Errorf("github-copilot: device code response missing fields")
	}

	return &result, nil
}

func copilotPollForToken(ctx context.Context, domain string, device *deviceCodeResponse) (string, error) {
	accessTokenURL := fmt.Sprintf("https://%s/login/oauth/access_token", domain)

	body := url.Values{
		"client_id":   {copilotClientID},
		"device_code": {device.DeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}

	deadline := time.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	interval := time.Duration(device.Interval) * time.Second
	if interval < time.Second {
		interval = time.Second
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, accessTokenURL, strings.NewReader(body.Encode()))
		if err != nil {
			return "", fmt.Errorf("github-copilot: create poll request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", copilotUserAgent)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("github-copilot: poll request: %w", err)
		}

		var raw map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&raw) //nolint:errcheck
		resp.Body.Close()

		if token, ok := raw["access_token"].(string); ok && token != "" {
			return token, nil
		}

		if errStr, ok := raw["error"].(string); ok {
			switch errStr {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5 * time.Second
				continue
			case "expired_token":
				return "", fmt.Errorf("github-copilot: device code expired")
			case "access_denied":
				return "", fmt.Errorf("github-copilot: login cancelled")
			default:
				desc, _ := raw["error_description"].(string)
				return "", fmt.Errorf("github-copilot: %s: %s", errStr, desc)
			}
		}
	}

	return "", fmt.Errorf("github-copilot: device code expired")
}

func copilotRefreshToken(githubToken, domain string) (*Credentials, error) {
	copilotTokenURL := fmt.Sprintf("https://api.%s/copilot_internal/v2/token", domain)

	req, err := http.NewRequest(http.MethodGet, copilotTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github-copilot: create token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("User-Agent", copilotUserAgent)
	req.Header.Set("Editor-Version", copilotEditorVer)
	req.Header.Set("Editor-Plugin-Version", copilotPluginVer)
	req.Header.Set("Copilot-Integration-Id", copilotIntegrationID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github-copilot: token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github-copilot: token exchange failed (%d): %s", resp.StatusCode, raw)
	}

	var result struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"` // unix seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("github-copilot: invalid token response: %w", err)
	}

	if result.Token == "" {
		return nil, fmt.Errorf("github-copilot: empty token in response")
	}

	return &Credentials{
		Access:  result.Token,
		Expires: result.ExpiresAt*1000 - 5*60*1000,
	}, nil
}

// normalizeDomain extracts the hostname from a URL or domain string.
func normalizeDomain(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
