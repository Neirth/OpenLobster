package subscriptions

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/services"
)

type SubscriptionManager struct {
	mu       sync.RWMutex
	clients  map[*Client]bool
	eventBus services.EventBus
	upgrader websocket.Upgrader
}

type EventBus interface {
	Subscribe(eventType string, handler func(ctx context.Context, event events.Event) error) error
}

type Client struct {
	conn    *websocket.Conn
	send    chan []byte
	manager *SubscriptionManager
	eventCh chan events.Event
}

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type SubscriptionMessage struct {
	Type      string                 `json:"type"` // "start", "stop", "connection_init"
	ID        string                 `json:"id"`
	Query     string                 `json:"query,omitempty"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

func NewSubscriptionManager(eb services.EventBus) *SubscriptionManager {
	return &SubscriptionManager{
		clients:  make(map[*Client]bool),
		eventBus: eb,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (m *SubscriptionManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	client := &Client{
		conn:    conn,
		send:    make(chan []byte, 256),
		manager: m,
		eventCh: make(chan events.Event, 256),
	}

	m.mu.Lock()
	m.clients[client] = true
	m.mu.Unlock()

	go client.writePump()
	go client.readPump()
	go client.forwardEvents()
}

func (m *SubscriptionManager) Broadcast(event events.Event) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for client := range m.clients {
		select {
		case client.eventCh <- event:
		default:
			log.Printf("client event channel full, dropping event")
		}
	}
}

func (m *SubscriptionManager) removeClient(client *Client) {
	m.mu.Lock()
	if _, ok := m.clients[client]; ok {
		delete(m.clients, client)
		close(client.send)
		close(client.eventCh)
	}
	m.mu.Unlock()
}

func (c *Client) readPump() {
	defer func() {
		c.manager.removeClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}

		var subMsg SubscriptionMessage
		if err := json.Unmarshal(message, &subMsg); err != nil {
			log.Printf("failed to unmarshal subscription message: %v", err)
			continue
		}

		c.handleMessage(subMsg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) forwardEvents() {
	for event := range c.eventCh {
		payload, err := json.Marshal(event.GetPayload())
		if err != nil {
			log.Printf("subscriptions: failed to marshal event payload: %v", err)
			continue
		}

		// Formato compatible con useSubscriptions: type "next", payload.type, payload.data
		msg := map[string]interface{}{
			"type": "next",
			"id":   event.GetType(),
			"payload": map[string]interface{}{
				"type":      event.GetType(),
				"timestamp": event.GetTimestamp().Format(time.RFC3339),
				"data":      json.RawMessage(payload),
			},
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("subscriptions: failed to marshal next message: %v", err)
			continue
		}

		select {
		case c.send <- data:
		default:
			log.Printf("subscriptions: client send channel full, dropping event %s", event.GetType())
		}
	}
}

func (c *Client) handleMessage(msg SubscriptionMessage) {
	switch msg.Type {
	case "connection_init":
		c.sendConnectionAck()
	case "start":
		c.handleStart(msg)
	case "stop":
		c.handleStop(msg)
	}
}

func (c *Client) sendConnectionAck() {
	msg := map[string]interface{}{
		"type": "connection_ack",
	}
	data, _ := json.Marshal(msg)
	c.send <- data
}

func (c *Client) handleStart(msg SubscriptionMessage) {
	eventType := msg.Variables["eventType"]
	if eventType == nil {
		eventType = msg.Query
	}

	c.sendStartAck(msg.ID, eventType)
}

func (c *Client) handleStop(msg SubscriptionMessage) {
	log.Printf("client stopped subscription: %s", msg.ID)
}

func (c *Client) sendStartAck(id string, eventType interface{}) {
	msg := map[string]interface{}{
		"type": "start_ack",
		"id":   id,
		"payload": map[string]interface{}{
			"subscription": eventType,
		},
	}
	data, _ := json.Marshal(msg)
	c.send <- data
}

func (m *SubscriptionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}
