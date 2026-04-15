// Copyright (c) OpenLobster contributors. See LICENSE for details.

package plugin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"

	"github.com/neirth/openlobster/internal/domain/models"
)

type fakeMessagingPlugin struct {
	id         string
	properties []byte
	callFn     func(function string, input []byte) ([]byte, error)
}

func (f *fakeMessagingPlugin) ID() string          { return f.id }
func (f *fakeMessagingPlugin) Name() string        { return f.id }
func (f *fakeMessagingPlugin) Version() string     { return "test" }
func (f *fakeMessagingPlugin) Description() string { return "test" }
func (f *fakeMessagingPlugin) Type() string        { return "messaging" }
func (f *fakeMessagingPlugin) Schema() ([]byte, error) {
	return []byte(`{"type":"object"}`), nil
}
func (f *fakeMessagingPlugin) Call(function string, input []byte) ([]byte, error) {
	if f.callFn == nil {
		return nil, nil
	}
	return f.callFn(function, input)
}
func (f *fakeMessagingPlugin) Properties() []byte { return f.properties }
func (f *fakeMessagingPlugin) Close() error       { return nil }

type fakeMessagingLoopFactory struct {
	*fakeMessagingPlugin
	loopRunner ports.PluginPort
}

func (f *fakeMessagingLoopFactory) CreateLoopRunner() (ports.PluginPort, error) {
	if f.loopRunner == nil {
		return nil, fmt.Errorf("missing loop runner")
	}
	return f.loopRunner, nil
}

func TestMessagingWrapperSendMessage_UsesResolvedChannelID(t *testing.T) {
	plugin := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}
	sentChannelID := ""

	plugin.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case resolveChannelIDFn:
			return []byte(`-100777000111`), nil
		case "send":
			var payload sendPluginInput
			if err := json.Unmarshal(input, &payload); err != nil {
				return nil, err
			}
			if payload.Message == nil {
				return nil, fmt.Errorf("missing message payload")
			}
			sentChannelID = payload.Message.ChannelID
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected function call: %s", function)
		}
	}

	wrapper := NewMessagingWrapper(plugin, "telegram", map[string]interface{}{"token": "test"})
	msg := models.NewMessage("telegram", "hola")

	err := wrapper.SendMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if sentChannelID != "-100777000111" {
		t.Fatalf("expected resolved channel id to be sent, got %q", sentChannelID)
	}
	if msg.ChannelID != "telegram" {
		t.Fatalf("expected original message channel id to remain unchanged, got %q", msg.ChannelID)
	}
}

func TestMessagingWrapperSendMessage_ResolveChannelIDIsRequired(t *testing.T) {
	plugin := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}
	sendCalled := false

	plugin.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case resolveChannelIDFn:
			return []byte(`   `), nil
		case "send":
			sendCalled = true
			return nil, nil
		default:
			return nil, nil
		}
	}

	wrapper := NewMessagingWrapper(plugin, "telegram", map[string]interface{}{"token": "test"})
	msg := models.NewMessage("telegram", "hola")

	err := wrapper.SendMessage(context.Background(), msg)
	if err == nil {
		t.Fatalf("expected SendMessage to fail when resolve_channel_id returns empty channel_id")
	}
	if !strings.Contains(err.Error(), "empty channel_id") {
		t.Fatalf("expected empty channel_id error, got: %v", err)
	}
	if sendCalled {
		t.Fatalf("send should not be called when resolve_channel_id fails")
	}
}

func TestMessagingWrapperSendTyping_UsesResolvedChannelID(t *testing.T) {
	plugin := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}
	typedChannelID := ""

	plugin.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case resolveChannelIDFn:
			return []byte(`-100888777666`), nil
		case typingFn:
			var payload sendPluginInput
			if err := json.Unmarshal(input, &payload); err != nil {
				return nil, err
			}
			if payload.Message == nil {
				return nil, fmt.Errorf("missing message payload")
			}
			typedChannelID = payload.Message.ChannelID
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected function call: %s", function)
		}
	}

	wrapper := NewMessagingWrapper(plugin, "telegram", map[string]interface{}{"token": "test"})
	ctx := context.WithValue(context.Background(), ports.ContextKeyChannelType, "telegram")

	err := wrapper.SendTyping(ctx, "telegram", 5000)
	if err != nil {
		t.Fatalf("SendTyping returned error: %v", err)
	}
	if typedChannelID != "-100888777666" {
		t.Fatalf("expected resolved channel id for typing, got %q", typedChannelID)
	}
}

func TestMessagingWrapperSendTyping_AllowsMissingTypingExport(t *testing.T) {
	plugin := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}

	plugin.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case resolveChannelIDFn:
			return []byte(`-100123`), nil
		case typingFn:
			return nil, fmt.Errorf("plugin openlobster-messages-telegram: function %q not exported", typingFn)
		default:
			return nil, nil
		}
	}

	wrapper := NewMessagingWrapper(plugin, "telegram", map[string]interface{}{"token": "test"})
	ctx := context.WithValue(context.Background(), ports.ContextKeyChannelType, "telegram")

	err := wrapper.SendTyping(ctx, "telegram", 5000)
	if err != nil {
		t.Fatalf("expected missing typing export to be ignored, got error: %v", err)
	}
}

func TestMessagingWrapperStart_UsesDedicatedLoopRunner(t *testing.T) {
	var primaryStartCalls atomic.Int32
	primary := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}
	primary.callFn = func(function string, input []byte) ([]byte, error) {
		if function == "start" {
			primaryStartCalls.Add(1)
		}
		return nil, nil
	}

	var loopStartCalls atomic.Int32
	loop := &fakeMessagingPlugin{id: "openlobster-messages-telegram-loop"}
	loop.callFn = func(function string, input []byte) ([]byte, error) {
		if function == "start" {
			loopStartCalls.Add(1)
			return nil, nil
		}
		return nil, nil
	}

	factory := &fakeMessagingLoopFactory{
		fakeMessagingPlugin: primary,
		loopRunner:          loop,
	}

	wrapper := NewMessagingWrapper(factory, "telegram", map[string]interface{}{"token": "test"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := wrapper.Start(ctx, nil); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if loopStartCalls.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if loopStartCalls.Load() == 0 {
		t.Fatalf("expected dedicated loop runner start to be called")
	}
	if got := primaryStartCalls.Load(); got != 0 {
		t.Fatalf("expected primary plugin start not to be called, got %d", got)
	}
}

func TestMessagingWrapperStart_DoesNotBlockSendOnPrimary(t *testing.T) {
	primary := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}
	primary.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case resolveChannelIDFn:
			return []byte("-100123"), nil
		case "send":
			return nil, nil
		case "start":
			t.Fatalf("primary plugin start must not be called")
		}
		return nil, nil
	}

	loop := &fakeMessagingPlugin{id: "openlobster-messages-telegram-loop"}
	loop.callFn = func(function string, input []byte) ([]byte, error) {
		if function == "start" {
			time.Sleep(120 * time.Millisecond)
			return nil, nil
		}
		return nil, nil
	}

	factory := &fakeMessagingLoopFactory{
		fakeMessagingPlugin: primary,
		loopRunner:          loop,
	}

	wrapper := NewMessagingWrapper(factory, "telegram", map[string]interface{}{"token": "test"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := wrapper.Start(ctx, nil); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		msg := models.NewMessage("telegram", "hola")
		errCh <- wrapper.SendMessage(context.Background(), msg)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("SendMessage returned error: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("SendMessage timed out while start loop was running")
	}
}

func TestMessagingWrapperConvertAudioForPlatform_PassThroughWhenMissing(t *testing.T) {
	plugin := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}
	plugin.callFn = func(function string, input []byte) ([]byte, error) {
		if function == convertAudioForPlatformFn {
			return nil, fmt.Errorf("plugin %s: function %q not exported", plugin.id, convertAudioForPlatformFn)
		}
		return nil, nil
	}

	wrapper := NewMessagingWrapper(plugin, "telegram", map[string]interface{}{"token": "test"})
	original := []byte{0x10, 0x11, 0x12}
	converted, format, err := wrapper.ConvertAudioForPlatform(context.Background(), original, "mp3")
	if err != nil {
		t.Fatalf("ConvertAudioForPlatform returned error: %v", err)
	}
	if len(converted) != len(original) {
		t.Fatalf("unexpected audio len: got %d want %d", len(converted), len(original))
	}
	if format != "mp3" {
		t.Fatalf("unexpected format: got %q", format)
	}
}

func TestMessagingWrapperConvertAudioForPlatform_UsesPluginConversion(t *testing.T) {
	plugin := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}
	target := []byte{0x21, 0x22}
	plugin.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case convertAudioForPlatformFn:
			return []byte(`{"audio":"` + base64.StdEncoding.EncodeToString(target) + `","format":"ogg"}`), nil
		default:
			return nil, nil
		}
	}

	wrapper := NewMessagingWrapper(plugin, "telegram", map[string]interface{}{"token": "test"})
	converted, format, err := wrapper.ConvertAudioForPlatform(context.Background(), []byte{0x01}, "mp3")
	if err != nil {
		t.Fatalf("ConvertAudioForPlatform returned error: %v", err)
	}
	if len(converted) != len(target) {
		t.Fatalf("unexpected converted len: got %d want %d", len(converted), len(target))
	}
	if format != "ogg" {
		t.Fatalf("unexpected format: got %q want %q", format, "ogg")
	}
}

func TestMessagingWrapperStart_SkipsLoopForWebhookInboundMode(t *testing.T) {
	var startCalls atomic.Int64
	plugin := &fakeMessagingPlugin{
		id:         "openlobster-messages-twilio",
		properties: []byte(`{"inbound_mode":"webhook"}`),
	}
	plugin.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case "start":
			startCalls.Add(1)
			return nil, nil
		default:
			return nil, nil
		}
	}

	wrapper := NewMessagingWrapper(plugin, "twilio", map[string]interface{}{})
	if err := wrapper.Start(context.Background(), nil); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if got := startCalls.Load(); got != 0 {
		t.Fatalf("expected no start calls for webhook inbound mode, got %d", got)
	}
}

func TestMessagingWrapperStart_RetriesOnFastSuccessfulExit(t *testing.T) {
	var startCalls atomic.Int64
	plugin := &fakeMessagingPlugin{
		id:         "openlobster-messages-telegram",
		properties: []byte(`{"inbound_mode":"polling"}`),
	}
	plugin.callFn = func(function string, input []byte) ([]byte, error) {
		switch function {
		case "start":
			startCalls.Add(1)
			return nil, nil
		default:
			return nil, nil
		}
	}

	wrapper := NewMessagingWrapper(plugin, "telegram", map[string]interface{}{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := wrapper.Start(ctx, nil); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if startCalls.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := startCalls.Load(); got == 0 {
		t.Fatalf("expected start to be called at least once")
	}

	deadline = time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if startCalls.Load() > 1 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if got := startCalls.Load(); got <= 1 {
		t.Fatalf("expected retries after fast successful exit, got %d start calls", got)
	}
}
