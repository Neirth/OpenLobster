package webhooks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainhandlers "github.com/neirth/openlobster/internal/domain/handlers"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

type testDispatcher struct {
	calls int
	last  domainhandlers.HandleMessageInput
	err   error
}

func (d *testDispatcher) Handle(_ context.Context, input domainhandlers.HandleMessageInput) error {
	d.calls++
	d.last = input
	return d.err
}

type testAdapterRegistry struct {
	adapters map[string]ports.MessagingPort
}

func (r *testAdapterRegistry) Get(channelType string) ports.MessagingPort {
	if r.adapters == nil {
		return nil
	}
	return r.adapters[channelType]
}

type testMessagingAdapter struct {
	handleWebhookFn func(context.Context, []byte) (*models.Message, error)
}

func (a *testMessagingAdapter) SendMessage(context.Context, *models.Message) error {
	return nil
}

func (a *testMessagingAdapter) SendMedia(context.Context, *ports.Media) error {
	return nil
}

func (a *testMessagingAdapter) SendTyping(context.Context, string) error {
	return nil
}

func (a *testMessagingAdapter) HandleWebhook(ctx context.Context, payload []byte) (*models.Message, error) {
	if a.handleWebhookFn == nil {
		return nil, nil
	}
	return a.handleWebhookFn(ctx, payload)
}

func (a *testMessagingAdapter) GetUserInfo(context.Context, string) (*ports.UserInfo, error) {
	return nil, nil
}

func (a *testMessagingAdapter) React(context.Context, string, string) error {
	return nil
}

func (a *testMessagingAdapter) GetCapabilities() ports.ChannelCapabilities {
	return ports.ChannelCapabilities{}
}

func (a *testMessagingAdapter) ConvertAudioForPlatform(context.Context, []byte, string) ([]byte, string, error) {
	return nil, "", nil
}

func (a *testMessagingAdapter) Start(context.Context, func(context.Context, *models.Message)) error {
	return nil
}

func TestChannelTypeFromWebhookPath(t *testing.T) {
	tests := []struct {
		path string
		ok   bool
		want string
	}{
		{path: "/webhook/twilio", ok: true, want: "twilio"},
		{path: "/webhook/WHATSAPP", ok: true, want: "whatsapp"},
		{path: "/webhook/", ok: false, want: ""},
		{path: "/webhook/twilio/extra", ok: false, want: ""},
		{path: "/webhooks/twilio", ok: false, want: ""},
	}

	for _, tc := range tests {
		got, ok := channelTypeFromWebhookPath(tc.path)
		if ok != tc.ok {
			t.Fatalf("path=%q: ok=%v want=%v", tc.path, ok, tc.ok)
		}
		if got != tc.want {
			t.Fatalf("path=%q: got=%q want=%q", tc.path, got, tc.want)
		}
	}
}

func TestIsWebhookNotSupportedError(t *testing.T) {
	if !isWebhookNotSupportedError(errors.New("webhook_not_supported: x")) {
		t.Fatalf("expected webhook_not_supported prefix to match")
	}
	if !isWebhookNotSupportedError(errors.New(`plugin x: function "handle_webhook" not exported`)) {
		t.Fatalf("expected missing export to match")
	}
	if isWebhookNotSupportedError(errors.New("other error")) {
		t.Fatalf("unexpected match")
	}
}

func TestRegister_DisablesLegacyWebhookPrefix(t *testing.T) {
	mux := http.NewServeMux()
	h := NewHandler(&testAdapterRegistry{}, nil)
	h.Register(mux)

	// Simulate frontend/static fallback mounted on root path.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("spa"))
	})

	req := httptest.NewRequest(http.MethodGet, "/webhooks/whatsapp", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for legacy webhook path, got %d", rec.Code)
	}
}

func TestServeWebhook_WhatsAppChallengeGET(t *testing.T) {
	h := NewHandler(&testAdapterRegistry{}, &testDispatcher{})

	req := httptest.NewRequest(http.MethodGet, "/webhook/whatsapp?hub.challenge=abc123", nil)
	rec := httptest.NewRecorder()
	h.serveWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "abc123" {
		t.Fatalf("expected challenge body, got %q", rec.Body.String())
	}
}

func TestServeWebhook_AdapterMissingReturns404(t *testing.T) {
	h := NewHandler(&testAdapterRegistry{adapters: map[string]ports.MessagingPort{}}, &testDispatcher{})

	req := httptest.NewRequest(http.MethodPost, "/webhook/twilio", strings.NewReader("From=%2B34111&Body=hola"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.serveWebhook(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when adapter is missing, got %d", rec.Code)
	}
}

func TestServeWebhook_NotSupportedReturns404(t *testing.T) {
	adapter := &testMessagingAdapter{handleWebhookFn: func(context.Context, []byte) (*models.Message, error) {
		return nil, errors.New("webhook_not_supported: disabled")
	}}
	h := NewHandler(&testAdapterRegistry{adapters: map[string]ports.MessagingPort{"discord": adapter}}, &testDispatcher{})

	req := httptest.NewRequest(http.MethodPost, "/webhook/discord", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	h.serveWebhook(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unsupported webhook, got %d", rec.Code)
	}
}

func TestServeWebhook_ParsesAndDispatchesMessage(t *testing.T) {
	adapter := &testMessagingAdapter{handleWebhookFn: func(context.Context, []byte) (*models.Message, error) {
		return &models.Message{
			ChannelID:  "+34111222333",
			SenderID:   "+34111222333",
			SenderName: "alice",
			Content:    "hola",
		}, nil
	}}
	dispatcher := &testDispatcher{}
	h := NewHandler(&testAdapterRegistry{adapters: map[string]ports.MessagingPort{"twilio": adapter}}, dispatcher)

	req := httptest.NewRequest(http.MethodPost, "/webhook/twilio", strings.NewReader("From=%2B34111222333&Body=hola"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.serveWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if dispatcher.calls != 1 {
		t.Fatalf("expected dispatcher to be called once, got %d", dispatcher.calls)
	}
	if dispatcher.last.ChannelType != "twilio" {
		t.Fatalf("expected channel type twilio, got %q", dispatcher.last.ChannelType)
	}
	if dispatcher.last.Content != "hola" {
		t.Fatalf("expected dispatched content 'hola', got %q", dispatcher.last.Content)
	}
}

func TestServeWebhook_EmptyMessageReturns200WithoutDispatch(t *testing.T) {
	adapter := &testMessagingAdapter{handleWebhookFn: func(context.Context, []byte) (*models.Message, error) {
		return nil, nil
	}}
	dispatcher := &testDispatcher{}
	h := NewHandler(&testAdapterRegistry{adapters: map[string]ports.MessagingPort{"whatsapp": adapter}}, dispatcher)

	req := httptest.NewRequest(http.MethodPost, "/webhook/whatsapp", strings.NewReader(`{"entry":[]}`))
	rec := httptest.NewRecorder()
	h.serveWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if dispatcher.calls != 0 {
		t.Fatalf("expected dispatcher not to be called, got %d", dispatcher.calls)
	}
}

func TestServeWebhook_InvalidPayloadReturns400(t *testing.T) {
	adapter := &testMessagingAdapter{handleWebhookFn: func(context.Context, []byte) (*models.Message, error) {
		return nil, errors.New("invalid payload")
	}}
	h := NewHandler(&testAdapterRegistry{adapters: map[string]ports.MessagingPort{"twilio": adapter}}, &testDispatcher{})

	req := httptest.NewRequest(http.MethodPost, "/webhook/twilio", strings.NewReader("broken"))
	rec := httptest.NewRecorder()
	h.serveWebhook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
