// Package webhooks provides HTTP handlers for inbound messaging webhooks.
package webhooks

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/models"
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

const webhookNotSupportedPrefix = "webhook_not_supported"
const handleWebhookFn = "handle_webhook"

type webhookAdapterPayload struct {
	Request webhookRequestEnvelope `json:"request"`
}

type webhookRequestEnvelope struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   map[string][]string `json:"query,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body,omitempty"`
}

// Handler registers a dynamic /webhook/{channel_id} route on the mux.
type Handler struct {
	adapters   AdapterRegistry
	dispatcher MessageDispatcher
}

// NewHandler creates a webhooks handler.
func NewHandler(adapters AdapterRegistry, dispatcher MessageDispatcher) *Handler {
	return &Handler{adapters: adapters, dispatcher: dispatcher}
}

// Register adds /webhook/{channel_id} to the mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/webhook/", h.serveWebhook)
	// Keep strict migration semantics: legacy /webhooks/* must not fall through
	// to SPA/static handlers and should return 404 explicitly.
	mux.HandleFunc("/webhooks/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	log.Println("webhooks: /webhook/{channel_id} registered (legacy /webhooks/* disabled)")
}

// serveWebhook handles all inbound webhooks using /webhook/{channel_id}.
// WhatsApp verification challenge is handled here for ABI-minimal compatibility.
func (h *Handler) serveWebhook(w http.ResponseWriter, r *http.Request) {
	channelType, ok := channelTypeFromWebhookPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if channelType == "whatsapp" && r.Method == http.MethodGet {
		if c := r.URL.Query().Get("hub.challenge"); c != "" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(c))
			return
		}
		http.Error(w, "missing hub.challenge", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	adapter := h.adapters.Get(channelType)
	if adapter == nil {
		http.NotFound(w, r)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("webhooks/%s: read body: %v", channelType, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	envelopeRaw, err := json.Marshal(webhookAdapterPayload{Request: webhookRequestEnvelope{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   r.URL.Query(),
		Headers: r.Header,
		Body:    string(body),
	}})
	if err != nil {
		log.Printf("webhooks/%s: marshal envelope: %v", channelType, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	msg, err := adapter.HandleWebhook(r.Context(), envelopeRaw)
	if err != nil {
		if isWebhookNotSupportedError(err) {
			http.NotFound(w, r)
			return
		}
		log.Printf("webhooks/%s: parse: %v", channelType, err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if isEmptyMessage(msg) {
		w.WriteHeader(http.StatusOK)
		return
	}

	if h.dispatcher == nil {
		log.Printf("webhooks/%s: dispatcher nil, dropping parsed message", channelType)
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
		ChannelType: channelType,
		SenderID:    senderID,
		SenderName:  msg.SenderName,
		IsGroup:     msg.IsGroup,
		IsMentioned: msg.IsMentioned,
		GroupName:   msg.GroupName,
		Attachments: msg.Attachments,
		Audio:       msg.Audio,
	}); hErr != nil {
		log.Printf("webhooks/%s: dispatch: %v", channelType, hErr)
	}
	w.WriteHeader(http.StatusOK)
}

func isEmptyMessage(msg *models.Message) bool {
	return msg == nil || (msg.Content == "" && len(msg.Attachments) == 0 && msg.Audio == nil)
}

func channelTypeFromWebhookPath(path string) (string, bool) {
	p := strings.TrimSpace(path)
	if !strings.HasPrefix(p, "/webhook/") {
		return "", false
	}
	rest := strings.TrimPrefix(p, "/webhook/")
	rest = strings.Trim(rest, "/")
	if rest == "" || strings.Contains(rest, "/") {
		return "", false
	}
	return strings.ToLower(rest), true
}

func isWebhookNotSupportedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, webhookNotSupportedPrefix) {
		return true
	}
	return strings.Contains(msg, `function "`+handleWebhookFn+`" not exported`)
}
