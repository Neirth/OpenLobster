package provideroauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Mock SecretsProvider (in-memory)
// ---------------------------------------------------------------------------

type mockSecretsProvider struct {
	mu   sync.RWMutex
	data map[string]string
}

func newMockSecretsProvider() *mockSecretsProvider {
	return &mockSecretsProvider{data: make(map[string]string)}
}

func (m *mockSecretsProvider) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[key], nil
}

func (m *mockSecretsProvider) Set(_ context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockSecretsProvider) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *mockSecretsProvider) List(_ context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for k := range m.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// ---------------------------------------------------------------------------
// Mock OAuthProvider
// ---------------------------------------------------------------------------

type mockOAuthProvider struct {
	id          string
	name        string
	loginCreds  *Credentials
	loginErr    error
	refreshFunc func(*Credentials) (*Credentials, error)
}

func (p *mockOAuthProvider) ID() string   { return p.id }
func (p *mockOAuthProvider) Name() string { return p.name }

func (p *mockOAuthProvider) Login(_ context.Context, _ LoginCallbacks) (*Credentials, error) {
	if p.loginErr != nil {
		return nil, p.loginErr
	}
	return p.loginCreds, nil
}

func (p *mockOAuthProvider) RefreshToken(_ context.Context, creds *Credentials) (*Credentials, error) {
	if p.refreshFunc != nil {
		return p.refreshFunc(creds)
	}
	return &Credentials{
		Access:  "refreshed-access",
		Refresh: creds.Refresh,
		Expires: timeNowMs() + 3600*1000,
	}, nil
}

func (p *mockOAuthProvider) GetAPIKey(creds *Credentials) string {
	return creds.Access
}

// ---------------------------------------------------------------------------
// Tests: PKCE
// ---------------------------------------------------------------------------

func TestGenerateCodeVerifier(t *testing.T) {
	v, err := generateCodeVerifier()
	assert.NoError(t, err)
	// 32 random bytes -> base64url without padding = 43 chars
	assert.Equal(t, 43, len(v))

	// Must be different each time
	v2, err := generateCodeVerifier()
	assert.NoError(t, err)
	assert.NotEqual(t, v, v2)
}

func TestCodeChallenge_SHA256(t *testing.T) {
	verifier := "test-verifier-value"
	challenge := codeChallenge(verifier)

	// Manually compute expected SHA256
	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	assert.Equal(t, expected, challenge)
}

func TestGenerateState(t *testing.T) {
	s, err := generateState()
	assert.NoError(t, err)
	// 16 bytes hex = 32 chars
	assert.Equal(t, 32, len(s))

	s2, err := generateState()
	assert.NoError(t, err)
	assert.NotEqual(t, s, s2)
}

// ---------------------------------------------------------------------------
// Tests: parseAuthorizationInput
// ---------------------------------------------------------------------------

func TestParseAuthorizationInput_URL(t *testing.T) {
	code, state := parseAuthorizationInput("http://localhost:1455/auth/callback?code=abc123&state=xyz789")
	assert.Equal(t, "abc123", code)
	assert.Equal(t, "xyz789", state)
}

func TestParseAuthorizationInput_QueryString(t *testing.T) {
	code, state := parseAuthorizationInput("code=qwerty&state=asdf")
	assert.Equal(t, "qwerty", code)
	assert.Equal(t, "asdf", state)
}

func TestParseAuthorizationInput_RawCode(t *testing.T) {
	code, state := parseAuthorizationInput("my-raw-code-value")
	assert.Equal(t, "my-raw-code-value", code)
	assert.Equal(t, "", state)
}

func TestParseAuthorizationInput_Empty(t *testing.T) {
	code, state := parseAuthorizationInput("")
	assert.Equal(t, "", code)
	assert.Equal(t, "", state)
}

func TestParseAuthorizationInput_URLWithoutState(t *testing.T) {
	code, state := parseAuthorizationInput("http://example.com/cb?code=onlycode")
	assert.Equal(t, "onlycode", code)
	assert.Equal(t, "", state)
}

// ---------------------------------------------------------------------------
// Tests: callbackServer
// ---------------------------------------------------------------------------

func TestCallbackServer_ReceivesCode(t *testing.T) {
	srv, err := startCallbackServer(19876, "/test/callback", "expected-state")
	assert.NoError(t, err)
	defer srv.close()

	// Send callback in a goroutine so waitForCode can receive it
	go func() {
		time.Sleep(50 * time.Millisecond)
		callbackURL := fmt.Sprintf("http://localhost:%d/test/callback?code=testcode&state=expected-state", srv.port)
		resp, err := http.Get(callbackURL) //nolint:gosec
		if err == nil {
			resp.Body.Close()
		}
	}()

	result := srv.waitForCode(context.Background(), 5*time.Second)
	assert.NotNil(t, result)
	if result != nil {
		assert.Equal(t, "testcode", result.Code)
		assert.Equal(t, "expected-state", result.State)
	}
}

func TestCallbackServer_StateMismatch(t *testing.T) {
	srv, err := startCallbackServer(19877, "/test/callback", "good-state")
	assert.NoError(t, err)
	defer srv.close()

	url := fmt.Sprintf("http://localhost:%d/test/callback?code=testcode&state=bad-state", srv.port)
	resp, err := http.Get(url)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestCallbackServer_MissingCode(t *testing.T) {
	srv, err := startCallbackServer(19878, "/test/callback", "")
	assert.NoError(t, err)
	defer srv.close()

	url := fmt.Sprintf("http://localhost:%d/test/callback?state=abc", srv.port)
	resp, err := http.Get(url)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Tests: Credentials.IsExpired
// ---------------------------------------------------------------------------

func TestCredentials_IsExpired(t *testing.T) {
	expired := &Credentials{
		Access:  "token",
		Expires: timeNowMs() - 1000,
	}
	assert.True(t, expired.IsExpired())

	valid := &Credentials{
		Access:  "token",
		Expires: timeNowMs() + 3600*1000,
	}
	assert.False(t, valid.IsExpired())

	edge := &Credentials{
		Access:  "token",
		Expires: timeNowMs(),
	}
	// At exact expiry time, should be considered expired (>=)
	assert.True(t, edge.IsExpired())
}

// ---------------------------------------------------------------------------
// Tests: Registry
// ---------------------------------------------------------------------------

func TestRegistry_RegisterGetList(t *testing.T) {
	// Save and restore registry state to avoid test pollution
	registryMu.Lock()
	origRegistry := registry
	registry = make(map[string]OAuthProvider)
	registryMu.Unlock()
	defer func() {
		registryMu.Lock()
		registry = origRegistry
		registryMu.Unlock()
	}()

	p1 := &mockOAuthProvider{id: "test-provider-1", name: "Test 1"}
	p2 := &mockOAuthProvider{id: "test-provider-2", name: "Test 2"}

	Register(p1)
	Register(p2)

	got := Get("test-provider-1")
	assert.NotNil(t, got)
	assert.Equal(t, "test-provider-1", got.ID())

	got = Get("test-provider-2")
	assert.NotNil(t, got)
	assert.Equal(t, "Test 2", got.Name())

	got = Get("nonexistent")
	assert.Nil(t, got)

	all := List()
	assert.Len(t, all, 2)
}

// ---------------------------------------------------------------------------
// Tests: Manager
// ---------------------------------------------------------------------------

// registerTestProvider registers a mock provider and returns a cleanup function.
func registerTestProvider(p OAuthProvider) func() {
	registryMu.Lock()
	origRegistry := make(map[string]OAuthProvider)
	for k, v := range registry {
		origRegistry[k] = v
	}
	registry[p.ID()] = p
	registryMu.Unlock()

	return func() {
		registryMu.Lock()
		registry = origRegistry
		registryMu.Unlock()
	}
}

func TestManager_LoginStoresCredentials(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-login",
		name: "Test Login",
		loginCreds: &Credentials{
			Access:  "access-token",
			Refresh: "refresh-token",
			Expires: timeNowMs() + 3600*1000,
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	creds, err := mgr.Login(ctx, "test-login", LoginCallbacks{})
	assert.NoError(t, err)
	assert.Equal(t, "access-token", creds.Access)

	// Verify stored in secrets (Login uses "default" profile)
	stored, err := sp.Get(ctx, "provider-oauth/test-login/default/credentials")
	assert.NoError(t, err)
	assert.Contains(t, stored, "access-token")
}

func TestManager_LoginProviderNotFound(t *testing.T) {
	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	_, err := mgr.Login(ctx, "nonexistent-provider", LoginCallbacks{})
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestManager_GetAPIKeyReturnsKey(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-apikey",
		name: "Test APIKey",
		loginCreds: &Credentials{
			Access:  "my-api-key",
			Refresh: "my-refresh",
			Expires: timeNowMs() + 3600*1000,
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	_, err := mgr.Login(ctx, "test-apikey", LoginCallbacks{})
	assert.NoError(t, err)

	key, err := mgr.GetAPIKey(ctx, "test-apikey")
	assert.NoError(t, err)
	assert.Equal(t, "my-api-key", key)
}

func TestManager_GetAPIKeyRefreshesExpired(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-refresh",
		name: "Test Refresh",
		loginCreds: &Credentials{
			Access:  "old-access",
			Refresh: "my-refresh",
			Expires: timeNowMs() - 1000, // already expired
		},
		refreshFunc: func(c *Credentials) (*Credentials, error) {
			return &Credentials{
				Access:  "new-access-after-refresh",
				Refresh: c.Refresh,
				Expires: timeNowMs() + 3600*1000,
			}, nil
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	_, err := mgr.Login(ctx, "test-refresh", LoginCallbacks{})
	assert.NoError(t, err)

	key, err := mgr.GetAPIKey(ctx, "test-refresh")
	assert.NoError(t, err)
	assert.Equal(t, "new-access-after-refresh", key)
}

func TestManager_GetAPIKeyNotAuthenticated(t *testing.T) {
	provider := &mockOAuthProvider{id: "test-noauth", name: "No Auth"}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	_, err := mgr.GetAPIKey(ctx, "test-noauth")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
}

func TestManager_IsAuthenticated(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-isauth",
		name: "Test IsAuth",
		loginCreds: &Credentials{
			Access:  "tok",
			Refresh: "ref",
			Expires: timeNowMs() + 3600*1000,
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	assert.False(t, mgr.IsAuthenticated(ctx, "test-isauth"))

	_, err := mgr.Login(ctx, "test-isauth", LoginCallbacks{})
	assert.NoError(t, err)

	assert.True(t, mgr.IsAuthenticated(ctx, "test-isauth"))
}

func TestManager_Logout(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-logout",
		name: "Test Logout",
		loginCreds: &Credentials{
			Access:  "tok",
			Refresh: "ref",
			Expires: timeNowMs() + 3600*1000,
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	_, err := mgr.Login(ctx, "test-logout", LoginCallbacks{})
	assert.NoError(t, err)
	assert.True(t, mgr.IsAuthenticated(ctx, "test-logout"))

	err = mgr.Logout(ctx, "test-logout")
	assert.NoError(t, err)
	assert.False(t, mgr.IsAuthenticated(ctx, "test-logout"))
}

func TestManager_GetCredentials_Empty(t *testing.T) {
	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	creds, err := mgr.GetCredentials(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, creds)
}

func TestManager_StartAutoRefreshAndStop(t *testing.T) {
	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr.StartAutoRefresh(ctx)
	// Starting again should be a no-op
	mgr.StartAutoRefresh(ctx)

	mgr.Stop()
	// Stopping again should be a no-op
	mgr.Stop()
}

// ---------------------------------------------------------------------------
// Tests: extractProviderID
// ---------------------------------------------------------------------------

func TestExtractProviderID(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"provider-oauth/openai-codex/default/credentials", "openai-codex"},
		{"provider-oauth/my-provider/work/credentials", "my-provider"},
		{"provider-oauth//default/credentials", ""},
		{"bad-prefix/id/profile/credentials", ""},
		{"provider-oauth/id/profile/wrong-suffix", ""},
		{"short", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractProviderID(tt.key))
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: callbackServer with httptest
// ---------------------------------------------------------------------------

func TestCallbackServer_ErrorParam(t *testing.T) {
	srv, err := startCallbackServer(19879, "/cb", "")
	assert.NoError(t, err)
	defer srv.close()

	url := fmt.Sprintf("http://localhost:%d/cb?error=access_denied", srv.port)
	resp, err := http.Get(url)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Tests: successHTML served on valid callback
// ---------------------------------------------------------------------------

func TestCallbackServer_SuccessHTML(t *testing.T) {
	srv, err := startCallbackServer(19880, "/cb", "s1")
	assert.NoError(t, err)
	defer srv.close()

	url := fmt.Sprintf("http://localhost:%d/cb?code=c1&state=s1", srv.port)
	resp, err := http.Get(url)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
}

// ---------------------------------------------------------------------------
// Tests: httptest-based callback verification
// ---------------------------------------------------------------------------

func TestCallbackHandler_HTTPTest(t *testing.T) {
	// Build a handler the same way callbackServer does and test via httptest.
	mux := http.NewServeMux()
	var mu sync.Mutex
	var captured *callbackResult

	mux.HandleFunc("/oauth/cb", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		captured = &callbackResult{Code: code, State: state}
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/oauth/cb?code=httptest-code&state=httptest-state")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	assert.NotNil(t, captured)
	assert.Equal(t, "httptest-code", captured.Code)
	assert.Equal(t, "httptest-state", captured.State)
}

// ---------------------------------------------------------------------------
// Tests: credentials key format
// ---------------------------------------------------------------------------

func TestCredentialsKey(t *testing.T) {
	assert.Equal(t, "provider-oauth/openai-codex/default/credentials", credentialsKey("openai-codex"))
	assert.Equal(t, "provider-oauth/my-provider/default/credentials", credentialsKey("my-provider"))
}

func TestProfileCredentialsKey(t *testing.T) {
	assert.Equal(t, "provider-oauth/openai-codex/work/credentials", profileCredentialsKey("openai-codex", "work"))
	assert.Equal(t, "provider-oauth/my-provider/personal/credentials", profileCredentialsKey("my-provider", "personal"))
}

// ---------------------------------------------------------------------------
// Tests: extractProviderAndProfile
// ---------------------------------------------------------------------------

func TestExtractProviderAndProfile(t *testing.T) {
	tests := []struct {
		key             string
		expectedID      string
		expectedProfile string
	}{
		{"provider-oauth/openai-codex/default/credentials", "openai-codex", "default"},
		{"provider-oauth/my-provider/work/credentials", "my-provider", "work"},
		{"provider-oauth/my-provider/personal/credentials", "my-provider", "personal"},
		{"provider-oauth//default/credentials", "", ""},
		{"provider-oauth/id//credentials", "", ""},
		{"bad-prefix/id/profile/credentials", "", ""},
		{"provider-oauth/id/profile/wrong-suffix", "", ""},
		{"short", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			id, profile := extractProviderAndProfile(tt.key)
			assert.Equal(t, tt.expectedID, id)
			assert.Equal(t, tt.expectedProfile, profile)
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: Multi-profile Manager
// ---------------------------------------------------------------------------

func TestManager_LoginWithProfile(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-profile-login",
		name: "Test Profile Login",
		loginCreds: &Credentials{
			Access:    "profile-access-token",
			Refresh:   "profile-refresh-token",
			Expires:   timeNowMs() + 3600*1000,
			AccountID: "acct-123",
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	creds, err := mgr.LoginWithProfile(ctx, "test-profile-login", "work", LoginCallbacks{})
	assert.NoError(t, err)
	assert.Equal(t, "profile-access-token", creds.Access)

	// Verify stored under profile key
	stored, err := sp.Get(ctx, "provider-oauth/test-profile-login/work/credentials")
	assert.NoError(t, err)
	assert.Contains(t, stored, "profile-access-token")

	// Default profile should be empty
	defaultStored, err := sp.Get(ctx, "provider-oauth/test-profile-login/default/credentials")
	assert.NoError(t, err)
	assert.Equal(t, "", defaultStored)
}

func TestManager_LoginWithProfileProviderNotFound(t *testing.T) {
	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	_, err := mgr.LoginWithProfile(ctx, "nonexistent", "work", LoginCallbacks{})
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestManager_ListProfiles(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-list-profiles",
		name: "Test List Profiles",
		loginCreds: &Credentials{
			Access:    "tok",
			Refresh:   "ref",
			Expires:   timeNowMs() + 3600*1000,
			AccountID: "acct-work",
			Extra:     map[string]string{"email": "work@example.com"},
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	// Login with two profiles
	_, err := mgr.LoginWithProfile(ctx, "test-list-profiles", "work", LoginCallbacks{})
	assert.NoError(t, err)

	provider.loginCreds = &Credentials{
		Access:    "tok2",
		Refresh:   "ref2",
		Expires:   timeNowMs() + 3600*1000,
		AccountID: "acct-personal",
		Extra:     map[string]string{"email": "personal@example.com"},
	}
	_, err = mgr.LoginWithProfile(ctx, "test-list-profiles", "personal", LoginCallbacks{})
	assert.NoError(t, err)

	profiles, err := mgr.ListProfiles(ctx, "test-list-profiles")
	assert.NoError(t, err)
	assert.Len(t, profiles, 2)

	// Build a map for easier assertions
	byName := map[string]ProfileInfo{}
	for _, p := range profiles {
		byName[p.Name] = p
	}

	work := byName["work"]
	assert.True(t, work.Authenticated)
	assert.Equal(t, "acct-work", work.AccountID)
	assert.Equal(t, "work@example.com", work.Email)

	personal := byName["personal"]
	assert.True(t, personal.Authenticated)
	assert.Equal(t, "acct-personal", personal.AccountID)
	assert.Equal(t, "personal@example.com", personal.Email)
}

func TestManager_SetAndGetActiveProfile(t *testing.T) {
	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	// Default should be "default"
	assert.Equal(t, "default", mgr.GetActiveProfile(ctx, "some-provider"))

	// Set a different profile
	err := mgr.SetActiveProfile(ctx, "some-provider", "work")
	assert.NoError(t, err)
	assert.Equal(t, "work", mgr.GetActiveProfile(ctx, "some-provider"))

	// Change again
	err = mgr.SetActiveProfile(ctx, "some-provider", "personal")
	assert.NoError(t, err)
	assert.Equal(t, "personal", mgr.GetActiveProfile(ctx, "some-provider"))
}

func TestManager_GetCredentialsUsesActiveProfile(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-active-creds",
		name: "Test Active Creds",
		loginCreds: &Credentials{
			Access:  "work-token",
			Refresh: "work-refresh",
			Expires: timeNowMs() + 3600*1000,
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	// Login with "work" profile
	_, err := mgr.LoginWithProfile(ctx, "test-active-creds", "work", LoginCallbacks{})
	assert.NoError(t, err)

	// Login with "personal" profile
	provider.loginCreds = &Credentials{
		Access:  "personal-token",
		Refresh: "personal-refresh",
		Expires: timeNowMs() + 3600*1000,
	}
	_, err = mgr.LoginWithProfile(ctx, "test-active-creds", "personal", LoginCallbacks{})
	assert.NoError(t, err)

	// Default active profile is "default", which has no creds
	creds, err := mgr.GetCredentials(ctx, "test-active-creds")
	assert.NoError(t, err)
	assert.Nil(t, creds)

	// Switch to "work" profile
	err = mgr.SetActiveProfile(ctx, "test-active-creds", "work")
	assert.NoError(t, err)
	creds, err = mgr.GetCredentials(ctx, "test-active-creds")
	assert.NoError(t, err)
	assert.NotNil(t, creds)
	assert.Equal(t, "work-token", creds.Access)

	// Switch to "personal" profile
	err = mgr.SetActiveProfile(ctx, "test-active-creds", "personal")
	assert.NoError(t, err)
	creds, err = mgr.GetCredentials(ctx, "test-active-creds")
	assert.NoError(t, err)
	assert.NotNil(t, creds)
	assert.Equal(t, "personal-token", creds.Access)
}

func TestManager_DeleteProfile(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-delete-profile",
		name: "Test Delete Profile",
		loginCreds: &Credentials{
			Access:  "tok",
			Refresh: "ref",
			Expires: timeNowMs() + 3600*1000,
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	// Login with two profiles
	_, err := mgr.LoginWithProfile(ctx, "test-delete-profile", "work", LoginCallbacks{})
	assert.NoError(t, err)
	_, err = mgr.LoginWithProfile(ctx, "test-delete-profile", "personal", LoginCallbacks{})
	assert.NoError(t, err)

	// Set "work" as active
	err = mgr.SetActiveProfile(ctx, "test-delete-profile", "work")
	assert.NoError(t, err)

	// Delete "work" - active profile should revert to default
	err = mgr.DeleteProfile(ctx, "test-delete-profile", "work")
	assert.NoError(t, err)
	assert.Equal(t, "default", mgr.GetActiveProfile(ctx, "test-delete-profile"))

	// "personal" should still exist
	profiles, err := mgr.ListProfiles(ctx, "test-delete-profile")
	assert.NoError(t, err)
	assert.Len(t, profiles, 1)
	assert.Equal(t, "personal", profiles[0].Name)
}

func TestManager_DeleteProfileNonActive(t *testing.T) {
	provider := &mockOAuthProvider{
		id:   "test-del-nonactive",
		name: "Test Del Non-Active",
		loginCreds: &Credentials{
			Access:  "tok",
			Refresh: "ref",
			Expires: timeNowMs() + 3600*1000,
		},
	}
	cleanup := registerTestProvider(provider)
	defer cleanup()

	sp := newMockSecretsProvider()
	mgr := NewManager(sp)
	ctx := context.Background()

	_, err := mgr.LoginWithProfile(ctx, "test-del-nonactive", "work", LoginCallbacks{})
	assert.NoError(t, err)
	_, err = mgr.LoginWithProfile(ctx, "test-del-nonactive", "personal", LoginCallbacks{})
	assert.NoError(t, err)

	// Set "work" as active, delete "personal"
	err = mgr.SetActiveProfile(ctx, "test-del-nonactive", "work")
	assert.NoError(t, err)

	err = mgr.DeleteProfile(ctx, "test-del-nonactive", "personal")
	assert.NoError(t, err)

	// Active should still be "work"
	assert.Equal(t, "work", mgr.GetActiveProfile(ctx, "test-del-nonactive"))
}
