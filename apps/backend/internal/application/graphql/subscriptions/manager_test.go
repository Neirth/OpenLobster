package subscriptions

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSubscriptionManager(t *testing.T) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.clients)
	assert.Equal(t, 0, manager.Count())
}

func TestSubscriptionManager_HandleWebSocket(t *testing.T) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(manager.HandleWebSocket))
	defer server.Close()

	// The handler should not panic
	assert.NotNil(t, server)
}

func TestSubscriptionManager_HandleWebSocket_NonUpgradeRequest(t *testing.T) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)
	server := httptest.NewServer(http.HandlerFunc(manager.HandleWebSocket))
	defer server.Close()

	// Regular HTTP GET without Upgrade header - Upgrade will fail, handler returns early
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Upgrader returns 400 Bad Request when not a WebSocket request
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError)
	assert.Equal(t, 0, manager.Count())
}

func TestSubscriptionManager_Broadcast_NoClients(t *testing.T) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)

	// Should not panic when broadcasting with no clients
	event := events.NewEvent("test_event", map[string]string{"key": "value"})
	manager.Broadcast(event)
}

func TestSubscriptionManager_Broadcast_WithMockClient(t *testing.T) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)

	// The client writePump runs in a goroutine, so we can't easily test
	// without a real WebSocket connection. This test just verifies the
	// method doesn't panic.
	event := events.NewEvent("test_event", map[string]string{"key": "value"})
	manager.Broadcast(event)

	// Should have 0 clients connected
	assert.Equal(t, 0, manager.Count())
}

func TestSubscriptionManager_RemoveClient(t *testing.T) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)

	// Create a mock client - but we can't easily test this without a real connection
	// The removeClient method should handle nil gracefully
	// This test just verifies the method exists
	assert.NotNil(t, manager)
}

func TestClient_HandleMessage_ViaWebSocket(t *testing.T) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)

	server := httptest.NewServer(http.HandlerFunc(manager.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	u, err := url.Parse(wsURL + "/")
	require.NoError(t, err)

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	// Give time for client to register
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, manager.Count())

	// Send connection_init
	err = conn.WriteJSON(map[string]string{"type": "connection_init"})
	require.NoError(t, err)

	// Read connection_ack
	_, p, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(p), "connection_ack")

	// Send start
	err = conn.WriteJSON(map[string]interface{}{
		"type":      "start",
		"id":        "sub-1",
		"variables": map[string]string{"eventType": "heartbeat"},
	})
	require.NoError(t, err)

	_, p, err = conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(p), "start_ack")

	// Broadcast triggers forwardEvents
	event := events.NewEvent("heartbeat", map[string]string{"status": "ok"})
	manager.Broadcast(event)

	_, p, err = conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(p), "heartbeat")

	// Send stop
	err = conn.WriteJSON(map[string]interface{}{"type": "stop", "id": "sub-1"})
	require.NoError(t, err)

	conn.Close()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, manager.Count())
}

func TestSubscriptionMessage_JSON(t *testing.T) {
	msg := SubscriptionMessage{
		Type:      "start",
		ID:        "test-1",
		Query:     "subscription { event }",
		Variables: map[string]interface{}{"key": "value"},
	}

	assert.Equal(t, "start", msg.Type)
	assert.Equal(t, "test-1", msg.ID)
	assert.Equal(t, "subscription { event }", msg.Query)
	assert.Equal(t, "value", msg.Variables["key"])
}

func TestMessage_JSON(t *testing.T) {
	msg := Message{
		Type:    "test",
		Payload: []byte(`{"key":"value"}`),
	}

	assert.Equal(t, "test", msg.Type)
	assert.Equal(t, `{"key":"value"}`, string(msg.Payload))
}

// Benchmark tests

func BenchmarkSubscriptionManager_New(b *testing.B) {
	eventBus := services.NewEventBus()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSubscriptionManager(eventBus)
	}
}

func BenchmarkSubscriptionManager_Broadcast(b *testing.B) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)

	event := events.NewEvent("test_event", map[string]string{
		"data": strings.Repeat("x", 1000),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Broadcast(event)
	}
}

func BenchmarkSubscriptionManager_Count(b *testing.B) {
	eventBus := services.NewEventBus()
	manager := NewSubscriptionManager(eventBus)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Count()
	}
}
