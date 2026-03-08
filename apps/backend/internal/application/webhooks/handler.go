// Package webhooks provides HTTP handlers for inbound webhooks from
// messaging platforms (WhatsApp, Twilio) that receive messages via POST.
package webhooks

import (
	"context"
	"io"
	"log"
	"net/http"

	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// AdapterRegistry returns the MessagingPort for a channel type.
type AdapterRegistry interface {
	Get(channelType string) ports.MessagingPort
}

// MessageDispatcher processes incoming messages (e.g. domainhandlers.MessageHandler.Handle).
type MessageDispatcher interface {
	Handle(ctx context.Context, input domainhandlers.HandleMessageInput) error
}

// Handler registers WhatsApp and Twilio webhook routes on the given mux.
type Handler struct {
	adapters   AdapterRegistry
	dispatcher MessageDispatcher
}

// NewHandler creates a webhooks handler.
func NewHandler(adapters AdapterRegistry, dispatcher MessageDispatcher) *Handler {
	return &Handler{adapters: adapters, dispatcher: dispatcher}
}

// Register adds /webhooks/whatsapp and /webhooks/twilio to the mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/webhooks/whatsapp", h.serveWhatsApp)
	mux.HandleFunc("/webhooks/twilio", h.serveTwilio)
	log.Println("webhooks: /webhooks/whatsapp, /webhooks/twilio registered")
}

// serveWhatsApp handles WhatsApp Cloud API webhooks.
// GET: verification challenge (hub.mode=subscribe, hub.challenge).
// POST: incoming message payload (JSON).
func (h *Handler) serveWhatsApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Method == http.MethodGet {
		if c := r.URL.Query().Get("hub.challenge"); c != "" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(c))
			return
		}
		http.Error(w, "missing hub.challenge", http.StatusBadRequest)
		return
	}

	adapter := h.adapters.Get("whatsapp")
	if adapter == nil {
		http.Error(w, "whatsapp not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("webhooks/whatsapp: read body: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	msg, err := adapter.HandleWebhook(r.Context(), body)
	if err != nil {
		log.Printf("webhooks/whatsapp: parse: %v", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if msg == nil || msg.Content == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	senderID := msg.SenderID
	if senderID == "" {
		senderID = msg.ChannelID
	}
	if hErr := h.dispatcher.Handle(r.Context(), domainhandlers.HandleMessageInput{
		ChannelID:   msg.ChannelID,
		Content:     msg.Content,
		ChannelType: "whatsapp",
		SenderID:    senderID,
		SenderName:  msg.SenderName,
		IsGroup:     msg.IsGroup,
		IsMentioned: msg.IsMentioned,
		GroupName:   msg.GroupName,
	}); hErr != nil {
		log.Printf("webhooks/whatsapp: dispatch: %v", hErr)
	}
	w.WriteHeader(http.StatusOK)
}

// serveTwilio handles Twilio SMS/MMS webhooks.
// POST: application/x-www-form-urlencoded (From, Body, etc.).
func (h *Handler) serveTwilio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	adapter := h.adapters.Get("twilio")
	if adapter == nil {
		http.Error(w, "twilio not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("webhooks/twilio: read body: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	msg, err := adapter.HandleWebhook(r.Context(), body)
	if err != nil {
		log.Printf("webhooks/twilio: parse: %v", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if msg == nil || msg.Content == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	senderID := msg.SenderID
	if senderID == "" {
		senderID = msg.ChannelID
	}
	if hErr := h.dispatcher.Handle(r.Context(), domainhandlers.HandleMessageInput{
		ChannelID:   msg.ChannelID,
		Content:     msg.Content,
		ChannelType: "twilio",
		SenderID:    senderID,
		SenderName:  msg.SenderName,
	}); hErr != nil {
		log.Printf("webhooks/twilio: dispatch: %v", hErr)
	}
	w.WriteHeader(http.StatusOK)
}
