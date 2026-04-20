// Copyright (c) OpenLobster contributors. See LICENSE for details.

package plugin

import (
	"fmt"
	"strings"
	"testing"
)

type fakeMessagingABIPlugin struct {
	id        string
	inbound   string
	resolveID string
	exported  map[string]bool
}

func (f *fakeMessagingABIPlugin) ID() string          { return f.id }
func (f *fakeMessagingABIPlugin) Name() string        { return f.id }
func (f *fakeMessagingABIPlugin) Version() string     { return "test" }
func (f *fakeMessagingABIPlugin) Description() string { return "test" }
func (f *fakeMessagingABIPlugin) Type() string        { return "messaging" }
func (f *fakeMessagingABIPlugin) Schema() ([]byte, error) {
	return []byte(`{"type":"object"}`), nil
}
func (f *fakeMessagingABIPlugin) Call(function string, input []byte) ([]byte, error) {
	switch function {
	case inboundModeFn:
		return []byte(f.inbound), nil
	case resolveChannelIDFn:
		if strings.TrimSpace(f.resolveID) == "" {
			return []byte(""), nil
		}
		return []byte(f.resolveID), nil
	default:
		return nil, fmt.Errorf("unexpected function call: %s", function)
	}
}
func (f *fakeMessagingABIPlugin) Properties() []byte {
	return []byte(fmt.Sprintf(`{"inbound_mode":%q}`, f.inbound))
}
func (f *fakeMessagingABIPlugin) Close() error { return nil }
func (f *fakeMessagingABIPlugin) HasFunction(function string) bool {
	if f.exported == nil {
		return false
	}
	return f.exported[function]
}

type fakeMessagingABIPluginNoIntrospection struct {
	id        string
	inbound   string
	resolveID string
}

func (f *fakeMessagingABIPluginNoIntrospection) ID() string          { return f.id }
func (f *fakeMessagingABIPluginNoIntrospection) Name() string        { return f.id }
func (f *fakeMessagingABIPluginNoIntrospection) Version() string     { return "test" }
func (f *fakeMessagingABIPluginNoIntrospection) Description() string { return "test" }
func (f *fakeMessagingABIPluginNoIntrospection) Type() string        { return "messaging" }
func (f *fakeMessagingABIPluginNoIntrospection) Schema() ([]byte, error) {
	return []byte(`{"type":"object"}`), nil
}
func (f *fakeMessagingABIPluginNoIntrospection) Call(function string, input []byte) ([]byte, error) {
	switch function {
	case inboundModeFn:
		return []byte(f.inbound), nil
	case resolveChannelIDFn:
		if strings.TrimSpace(f.resolveID) == "" {
			return []byte(""), nil
		}
		return []byte(f.resolveID), nil
	default:
		return nil, fmt.Errorf("unexpected function call: %s", function)
	}
}
func (f *fakeMessagingABIPluginNoIntrospection) Properties() []byte {
	return []byte(fmt.Sprintf(`{"inbound_mode":%q}`, f.inbound))
}
func (f *fakeMessagingABIPluginNoIntrospection) Close() error { return nil }

func TestValidateMessagingInboundContract(t *testing.T) {
	tests := []struct {
		name             string
		inboundMode      string
		hasStart         bool
		hasHandleWebhook bool
		wantErrContains  string
	}{
		{
			name:             "polling requires start",
			inboundMode:      "polling",
			hasStart:         false,
			hasHandleWebhook: false,
			wantErrContains:  `requires exported function "start"`,
		},
		{
			name:             "gateway requires start",
			inboundMode:      "gateway",
			hasStart:         false,
			hasHandleWebhook: true,
			wantErrContains:  `requires exported function "start"`,
		},
		{
			name:             "webhook requires handle_webhook",
			inboundMode:      "webhook",
			hasStart:         false,
			hasHandleWebhook: false,
			wantErrContains:  `requires exported function "handle_webhook"`,
		},
		{
			name:             "webhook forbids start",
			inboundMode:      "webhook",
			hasStart:         true,
			hasHandleWebhook: true,
			wantErrContains:  `forbids exported function "start"`,
		},
		{
			name:             "disabled forbids start and handle_webhook",
			inboundMode:      "disabled",
			hasStart:         false,
			hasHandleWebhook: true,
			wantErrContains:  `forbids exported functions "start" and "handle_webhook"`,
		},
		{
			name:             "polling valid",
			inboundMode:      "polling",
			hasStart:         true,
			hasHandleWebhook: false,
		},
		{
			name:             "webhook valid",
			inboundMode:      "webhook",
			hasStart:         false,
			hasHandleWebhook: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateMessagingInboundContract(tc.inboundMode, tc.hasStart, tc.hasHandleWebhook)
			if tc.wantErrContains == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErrContains)
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErrContains, err)
			}
		})
	}
}

func TestValidateMessagingPluginABI_RequiresFunctionIntrospection(t *testing.T) {
	p := &fakeMessagingABIPluginNoIntrospection{
		id:        "openlobster-messages-test",
		inbound:   "polling",
		resolveID: "123",
	}

	err := validateMessagingPluginABI(p)
	if err == nil {
		t.Fatalf("expected function introspection error")
	}
	if !strings.Contains(err.Error(), "function introspection") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMessagingPluginABI_AcceptsWebhookWithoutStart(t *testing.T) {
	p := &fakeMessagingABIPlugin{
		id:        "openlobster-messages-webhook",
		inbound:   "webhook",
		resolveID: "whatsapp:+123",
		exported: map[string]bool{
			handleWebhookFn: true,
		},
	}

	if err := validateMessagingPluginABI(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMessagingPluginABI_RejectsWebhookWithStart(t *testing.T) {
	p := &fakeMessagingABIPlugin{
		id:        "openlobster-messages-webhook",
		inbound:   "webhook",
		resolveID: "whatsapp:+123",
		exported: map[string]bool{
			handleWebhookFn: true,
			"start":         true,
		},
	}

	err := validateMessagingPluginABI(p)
	if err == nil {
		t.Fatalf("expected webhook/start contract error")
	}
	if !strings.Contains(err.Error(), `forbids exported function "start"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
