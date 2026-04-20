package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockSecretsProvider struct {
	mock.Mock
}

func (m *mockSecretsProvider) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *mockSecretsProvider) Set(ctx context.Context, key, value string) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *mockSecretsProvider) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *mockSecretsProvider) List(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	return args.Get(0).([]string), args.Error(1)
}

func TestMCPClient_ConnectHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		method := req["method"].(string)
		id := req["id"]

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
		}

		switch method {
		case "initialize":
			response["result"] = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]interface{}{
					"name":    "mock-mcp-server",
					"version": "1.0.0",
				},
				"capabilities": map[string]interface{}{},
			}
		case "tools/list":
			response["result"] = map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "echo",
						"description": "Echo a message",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"message": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			}
		default:
			response["result"] = nil
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	secretsProvider := new(mockSecretsProvider)
	secretsProvider.On("Get", mock.Anything, mock.Anything).Return("", nil).Maybe()

	client := mcp.NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), mcp.ServerConfig{
		Name: "test-server",
		Type: "http",
		URL:  server.URL,
	})

	assert.NoError(t, err)

	tools, err := client.ListTools(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "echo", tools[0].Name)

	err = client.Close()
	assert.NoError(t, err)
}

func TestMCPClient_ConnectHTTP_WithAuth(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": "test", "version": "1.0"},
				"capabilities":    map[string]interface{}{},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	secretsProvider := new(mockSecretsProvider)
	secretsProvider.On("Get", mock.Anything, "mcp/remote/test-server/token").Return("test-token-123", nil)

	client := mcp.NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), mcp.ServerConfig{
		Name: "test-server",
		Type: "http",
		URL:  server.URL,
	})

	assert.NoError(t, err)
	assert.Equal(t, "Bearer test-token-123", receivedAuth)

	client.Close()
}

func TestMCPClient_ConnectHTTP_InvalidServerType(t *testing.T) {
	secretsProvider := new(mockSecretsProvider)
	client := mcp.NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), mcp.ServerConfig{
		Name: "test",
		Type: "unknown",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown server type")
}

func TestMCPClient_CallTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		method := req["method"].(string)
		id := req["id"]

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
		}

		switch method {
		case "initialize":
			response["result"] = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": "test", "version": "1.0"},
				"capabilities":    map[string]interface{}{},
			}
		case "tools/list":
			response["result"] = map[string]interface{}{
				"tools": []map[string]interface{}{
					{"name": "echo", "description": "Echo", "inputSchema": map[string]interface{}{"type": "object"}},
				},
			}
		case "tools/call":
			response["result"] = map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": `{"echo": "hello"}`},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	secretsProvider := new(mockSecretsProvider)
	secretsProvider.On("Get", mock.Anything, mock.Anything).Return("", nil).Maybe()

	client := mcp.NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), mcp.ServerConfig{
		Name: "test-server",
		Type: "http",
		URL:  server.URL,
	})
	assert.NoError(t, err)

	result, err := client.CallTool(context.Background(), "test-server:echo", map[string]interface{}{
		"message": "hello",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "hello")

	client.Close()
}

func TestMCPClient_GetServerTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		method := req["method"].(string)
		id := req["id"]

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
		}

		switch method {
		case "initialize":
			response["result"] = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": "test", "version": "1.0"},
				"capabilities":    map[string]interface{}{},
			}
		case "tools/list":
			response["result"] = map[string]interface{}{
				"tools": []map[string]interface{}{
					{"name": "tool1", "description": "Tool 1", "inputSchema": map[string]interface{}{"type": "object"}},
					{"name": "tool2", "description": "Tool 2", "inputSchema": map[string]interface{}{"type": "object"}},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	secretsProvider := new(mockSecretsProvider)
	secretsProvider.On("Get", mock.Anything, mock.Anything).Return("", nil).Maybe()

	client := mcp.NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), mcp.ServerConfig{
		Name: "test-server",
		Type: "http",
		URL:  server.URL,
	})
	assert.NoError(t, err)

	tools := client.GetServerTools("test-server")
	assert.Len(t, tools, 2)

	client.Close()
}

func TestMCPClient_Close(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": "test", "version": "1.0"},
				"capabilities":    map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	secretsProvider := new(mockSecretsProvider)
	secretsProvider.On("Get", mock.Anything, mock.Anything).Return("", nil).Maybe()

	client := mcp.NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), mcp.ServerConfig{
		Name: "test-server",
		Type: "http",
		URL:  server.URL,
	})
	assert.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)

	tools, err := client.ListTools(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tools, 0)
}

func TestMCPClient_ConnectTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": "test", "version": "1.0"},
				"capabilities":    map[string]interface{}{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	secretsProvider := new(mockSecretsProvider)
	secretsProvider.On("Get", mock.Anything, mock.Anything).Return("", nil).Maybe()

	client := mcp.NewMCPClientSDK(secretsProvider)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.Connect(ctx, mcp.ServerConfig{
		Name: "test-server",
		Type: "http",
		URL:  server.URL,
	})

	assert.Error(t, err)
}

func TestMCPClient_ConnectMultipleServers(t *testing.T) {
	server1 := createMockMCPServer("server1")
	server2 := createMockMCPServer("server2")
	defer server1.Close()
	defer server2.Close()

	secretsProvider := new(mockSecretsProvider)
	secretsProvider.On("Get", mock.Anything, mock.Anything).Return("", nil).Maybe()

	client := mcp.NewMCPClientSDK(secretsProvider)

	err := client.Connect(context.Background(), mcp.ServerConfig{
		Name: "server1",
		Type: "http",
		URL:  server1.URL,
	})
	assert.NoError(t, err)

	err = client.Connect(context.Background(), mcp.ServerConfig{
		Name: "server2",
		Type: "http",
		URL:  server2.URL,
	})
	assert.NoError(t, err)

	tools, err := client.ListTools(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tools, 2)

	client.Close()
}

func createMockMCPServer(serverName string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)

		method := req["method"].(string)
		id := req["id"]

		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
		}

		switch method {
		case "initialize":
			response["result"] = map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo":      map[string]interface{}{"name": serverName, "version": "1.0"},
				"capabilities":    map[string]interface{}{},
			}
		case "tools/list":
			response["result"] = map[string]interface{}{
				"tools": []map[string]interface{}{
					{"name": "tool_" + serverName, "description": "Tool from " + serverName, "inputSchema": map[string]interface{}{"type": "object"}},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}
