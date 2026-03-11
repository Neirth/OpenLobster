package secrets

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	// Mock OpenBao KV v1 API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
