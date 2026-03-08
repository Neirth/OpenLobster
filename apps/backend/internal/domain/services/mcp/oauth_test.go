package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/neirth/openlobster/internal/infrastructure/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSecretsProvider(t *testing.T) secrets.SecretsProvider {
	tmpFile := t.TempDir() + "/secrets.json"
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	sp, err := secrets.NewFileSecretsProvider(tmpFile, key)
	require.NoError(t, err)
	return sp
}

func TestCodeChallenge(t *testing.T) {
	challenge := codeChallenge("test")
	require.NotEmpty(t, challenge)
	assert.Len(t, challenge, 43)
}

func TestNewOAuthManager(t *testing.T) {
	sp := newTestSecretsProvider(t)
	m := NewOAuthManager(sp, "")
	require.NotNil(t, m)
}

func TestOAuthManager_RegisterPendingServer(t *testing.T) {
	sp := newTestSecretsProvider(t)
	m := NewOAuthManager(sp, "")

	m.RegisterPendingServer("server1", "https://mcp.example.com")
	m.RegisterPendingServer("server2", "https://other.example.com")

	pending := m.GetPendingServers()
	assert.Len(t, pending, 2)
	assert.Equal(t, "https://mcp.example.com", pending["server1"])
}

func TestOAuthManager_RemovePendingServer(t *testing.T) {
	sp := newTestSecretsProvider(t)
	m := NewOAuthManager(sp, "")

	m.RegisterPendingServer("s1", "https://a.com")
	m.RemovePendingServer("s1")

	pending := m.GetPendingServers()
	assert.Empty(t, pending)
}

func TestOAuthManager_Status_None(t *testing.T) {
	sp := newTestSecretsProvider(t)
	m := NewOAuthManager(sp, "")

	status, errMsg := m.Status("unknown")
	assert.Equal(t, OAuthStatusNone, status)
	assert.Empty(t, errMsg)
}

func TestOAuthManager_HandleCallback_UnknownState(t *testing.T) {
	sp := newTestSecretsProvider(t)
	m := NewOAuthManager(sp, "")

	serverName, err := m.HandleCallback(context.Background(), "code", "invalid-state", "")
	assert.Error(t, err)
	assert.Empty(t, serverName)
	assert.Contains(t, err.Error(), "unknown or expired")
}

func TestOAuthManager_HandleCallback_AuthorizationDenied(t *testing.T) {
	sp := newTestSecretsProvider(t)
	m := NewOAuthManager(sp, "")

	authURL, err := m.InitiateOAuth(context.Background(), "test-server", "https://example.com/mcp")
	require.NoError(t, err)
	require.NotEmpty(t, authURL)

	u, err := url.Parse(authURL)
	require.NoError(t, err)
	state := u.Query().Get("state")
	require.NotEmpty(t, state)

	serverName, err := m.HandleCallback(context.Background(), "", state, "access_denied")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authorization denied")
	assert.Empty(t, serverName)

	status, errMsg := m.Status("test-server")
	assert.Equal(t, OAuthStatusError, status)
	assert.Contains(t, errMsg, "access_denied")
}

func TestDiscoverAuthServer_InvalidURL(t *testing.T) {
	_, err := discoverAuthServer("://invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestDiscoverAuthServer_ValidURL(t *testing.T) {
	// Uses fallback when metadata endpoints return non-200
	meta, err := discoverAuthServer("https://example.com/mcp")
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Contains(t, meta.AuthorizationEndpoint, "example.com")
	assert.Contains(t, meta.TokenEndpoint, "example.com")
}

func TestDiscoverAuthServer_WithMetadataServer(t *testing.T) {
	metaJSON := `{"authorization_endpoint":"https://auth.example.com/authorize","token_endpoint":"https://auth.example.com/token","registration_endpoint":"https://auth.example.com/register"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metaJSON))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	mcpURL := u.Scheme + "://" + u.Host + "/mcp"
	meta, err := discoverAuthServer(mcpURL)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, "https://auth.example.com/authorize", meta.AuthorizationEndpoint)
	assert.Equal(t, "https://auth.example.com/token", meta.TokenEndpoint)
}

func TestDiscoverAuthServer_ProtectedResourceDoc(t *testing.T) {
	authMeta := `{"authorization_endpoint":"https://auth.test/authorize","token_endpoint":"https://auth.test/token","registration_endpoint":""}`
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/.well-known/oauth-protected-resource/mcp" {
			doc := `{"authorization_servers":["http://` + r.Host + `"]}`
			_, _ = w.Write([]byte(doc))
		} else {
			_, _ = w.Write([]byte(authMeta))
		}
	}))
	defer srv.Close()
	baseURL = srv.URL

	u, _ := url.Parse(baseURL)
	mcpURL := u.Scheme + "://" + u.Host + "/mcp"
	meta, err := discoverAuthServer(mcpURL)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Contains(t, meta.AuthorizationEndpoint, "auth.test")
}

func TestInitiateOAuth_WithDynamicRegistration(t *testing.T) {
	authMeta := `{"authorization_endpoint":"https://auth.example/authorize","token_endpoint":"https://auth.example/token","registration_endpoint":"http://PLACEHOLDER/register"}`
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			meta := `{"authorization_endpoint":"http://` + r.Host + `/authorize","token_endpoint":"http://` + r.Host + `/token","registration_endpoint":"http://` + r.Host + `/register"}`
			_, _ = w.Write([]byte(meta))
		} else if r.URL.Path == "/register" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"client_id":"reg-client-123"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(authMeta))
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	sp := newTestSecretsProvider(t)
	m := NewOAuthManager(sp, "")
	u, _ := url.Parse(srvURL)
	mcpURL := u.Scheme + "://" + u.Host + "/mcp"

	authURL, err := m.InitiateOAuth(context.Background(), "srv1", mcpURL)
	require.NoError(t, err)
	require.NotEmpty(t, authURL)
	parsed, _ := url.Parse(authURL)
	assert.Equal(t, "reg-client-123", parsed.Query().Get("client_id"))
}

func TestHandleCallback_WithTokenExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			meta := `{"authorization_endpoint":"http://` + r.Host + `/authorize","token_endpoint":"http://` + r.Host + `/token","registration_endpoint":""}`
			_, _ = w.Write([]byte(meta))
		} else if r.URL.Path == "/token" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"tok_abc123","token_type":"Bearer"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	sp := newTestSecretsProvider(t)
	m := NewOAuthManager(sp, "")
	u, _ := url.Parse(srv.URL)
	mcpURL := u.Scheme + "://" + u.Host + "/mcp"

	authURL, err := m.InitiateOAuth(context.Background(), "srv2", mcpURL)
	require.NoError(t, err)
	require.NotEmpty(t, authURL)

	parsed, _ := url.Parse(authURL)
	state := parsed.Query().Get("state")
	require.NotEmpty(t, state)

	serverName, err := m.HandleCallback(context.Background(), "auth_code_123", state, "")
	require.NoError(t, err)
	assert.Equal(t, "srv2", serverName)

	token, _ := sp.Get(context.Background(), "mcp/remote/srv2/token")
	assert.Equal(t, "tok_abc123", token)

	status, _ := m.Status("srv2")
	assert.Equal(t, OAuthStatusAuthorized, status)
}
