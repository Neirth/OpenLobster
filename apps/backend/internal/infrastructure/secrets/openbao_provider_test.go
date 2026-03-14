package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenBAOProvider(t *testing.T) {
	p, err := NewOpenBAOProvider("http://localhost:8200", "token", "secret")
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestNewOpenBAOProvider_DefaultMount(t *testing.T) {
	p, err := NewOpenBAOProvider("http://localhost", "", "")
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestOpenBAOProvider_GetSet_Integration(t *testing.T) {
	// Mock OpenBao KV v1 API. Paths containing "/data/" (KV v2 style) return 404
	// so the provider falls back to KV v1 paths.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		isV2Path := strings.Contains(path, "/data/")
		if isV2Path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("list") == "true" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{"keys": []string{"a", "b"}},
				})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{"value": "stored-token"},
			})
		case http.MethodPost, http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	p, err := NewOpenBAOProvider(server.URL, "test-token", "secret")
	require.NoError(t, err)
	ctx := context.Background()

	err = p.Set(ctx, "mcp/remote/Linear/token", "my-oauth-token")
	require.NoError(t, err)

	val, err := p.Get(ctx, "mcp/remote/Linear/token")
	require.NoError(t, err)
	assert.Equal(t, "stored-token", val)

	keys, err := p.List(ctx, "mcp/remote")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, keys)

	err = p.Delete(ctx, "mcp/remote/Linear/token")
	require.NoError(t, err)
}

func TestOpenBAOProvider_Get_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p, err := NewOpenBAOProvider(server.URL, "t", "secret")
	require.NoError(t, err)
	val, err := p.Get(context.Background(), "missing")
	require.NoError(t, err)
	assert.Empty(t, val)
}

// TestOpenBAOProvider_KVv2 simulates a KV v2 backend (path contains /data/,
// response has data.data.value). This is the typical OpenBao default.
func TestOpenBAOProvider_KVv2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("list") == "true" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": map[string]interface{}{"keys": []string{"notion", "slack"}},
				})
				return
			}
			// KV v2 read response: data.data holds the secret payload.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"data":     map[string]interface{}{"value": "notion-oauth-token"},
					"metadata": map[string]interface{}{"version": float64(1)},
				},
			})
		case http.MethodPost, http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	p, err := NewOpenBAOProvider(server.URL, "test-token", "secret")
	require.NoError(t, err)
	ctx := context.Background()

	val, err := p.Get(ctx, "mcp/remote/notion/token")
	require.NoError(t, err)
	assert.Equal(t, "notion-oauth-token", val, "Get must unwrap KV v2 data.data.value")

	keys, err := p.List(ctx, "mcp/remote")
	require.NoError(t, err)
	assert.Equal(t, []string{"notion", "slack"}, keys)
}
