package plugin

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/neirth/openlobster/internal/domain/ports"
)

type fakeAIPlugin struct {
	id     string
	callFn func(function string, input []byte) ([]byte, error)
}

func (f *fakeAIPlugin) ID() string              { return f.id }
func (f *fakeAIPlugin) Name() string            { return f.id }
func (f *fakeAIPlugin) Version() string         { return "test" }
func (f *fakeAIPlugin) Description() string     { return "test" }
func (f *fakeAIPlugin) Type() string            { return "ai" }
func (f *fakeAIPlugin) Schema() ([]byte, error) { return []byte(`{"type":"object"}`), nil }
func (f *fakeAIPlugin) Close() error            { return nil }
func (f *fakeAIPlugin) Call(function string, input []byte) ([]byte, error) {
	if f.callFn == nil {
		return nil, nil
	}
	return f.callFn(function, input)
}

func TestAIWrapperSupportsAudioCapabilities(t *testing.T) {
	p := &fakeAIPlugin{id: "openlobster-ai-fake"}
	p.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case metadataFn:
			return []byte(`{"properties":{"supports_audio_input":true,"supports_audio_output":true}}`), nil
		default:
			return nil, fmt.Errorf("unexpected function %s", function)
		}
	}

	w := NewAIWrapper(p, map[string]interface{}{})
	if !w.SupportsAudioInput() {
		t.Fatalf("expected SupportsAudioInput to be true")
	}
	if !w.SupportsAudioOutput() {
		t.Fatalf("expected SupportsAudioOutput to be true")
	}
}

func TestAIWrapperSupportsAudioCapabilitiesMissingFunction(t *testing.T) {
	p := &fakeAIPlugin{id: "openlobster-ai-fake"}
	p.callFn = func(function string, input []byte) ([]byte, error) {
		return nil, fmt.Errorf("plugin %s: function %q not exported", p.id, function)
	}

	w := NewAIWrapper(p, map[string]interface{}{})
	if w.SupportsAudioInput() {
		t.Fatalf("expected SupportsAudioInput to be false")
	}
	if w.SupportsAudioOutput() {
		t.Fatalf("expected SupportsAudioOutput to be false")
	}
}

func TestAIWrapperChatToAudioSuccess(t *testing.T) {
	p := &fakeAIPlugin{id: "openlobster-ai-fake"}
	audioBytes := []byte{0x01, 0x02, 0x03}
	p.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case chatToAudioFn:
			return []byte(`{"content":"ok","audio_data":"` + base64.StdEncoding.EncodeToString(audioBytes) + `","stop_reason":"stop"}`), nil
		default:
			return nil, fmt.Errorf("unexpected function %s", function)
		}
	}

	w := NewAIWrapper(p, map[string]interface{}{})
	resp, err := w.ChatToAudio(context.Background(), ports.ChatRequest{Messages: []ports.ChatMessage{{Role: "user", Content: "hola"}}})
	if err != nil {
		t.Fatalf("ChatToAudio returned error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected content ok, got %q", resp.Content)
	}
	if len(resp.AudioData) != len(audioBytes) {
		t.Fatalf("unexpected audio bytes len: got %d want %d", len(resp.AudioData), len(audioBytes))
	}
}

func TestAIWrapperChatToAudioMissingFunction(t *testing.T) {
	p := &fakeAIPlugin{id: "openlobster-ai-fake"}
	p.callFn = func(function string, input []byte) ([]byte, error) {
		if function == chatToAudioFn {
			return nil, fmt.Errorf("plugin %s: function %q not exported", p.id, chatToAudioFn)
		}
		return nil, fmt.Errorf("unexpected function %s", function)
	}

	w := NewAIWrapper(p, map[string]interface{}{})
	_, err := w.ChatToAudio(context.Background(), ports.ChatRequest{Messages: []ports.ChatMessage{{Role: "user", Content: "hola"}}})
	if err == nil {
		t.Fatalf("expected error for missing chat_to_audio")
	}
	if !strings.Contains(err.Error(), "audio output not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}
