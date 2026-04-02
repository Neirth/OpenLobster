// Copyright (c) OpenLobster contributors. See LICENSE for details.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"

	"github.com/neirth/openlobster/internal/domain/models"
)

type fakeMessagingPlugin struct {
	id     string
	callFn func(function string, input []byte) ([]byte, error)
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
func (f *fakeMessagingPlugin) Close() error { return nil }

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

	err := wrapper.SendTyping(ctx, "telegram")
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

	err := wrapper.SendTyping(ctx, "telegram")
	if err != nil {
		t.Fatalf("expected missing typing export to be ignored, got error: %v", err)
	}
}

func TestMessagingWrapperStart_UsesDedicatedLoopRunner(t *testing.T) {
	primaryStartCalls := 0
	primary := &fakeMessagingPlugin{id: "openlobster-messages-telegram"}
	primary.callFn = func(function string, input []byte) ([]byte, error) {
		if function == "start" {
			primaryStartCalls++
		}
		return nil, nil
	}

	loopStartCalls := 0
	loop := &fakeMessagingPlugin{id: "openlobster-messages-telegram-loop"}
	loop.callFn = func(function string, input []byte) ([]byte, error) {
		if function == "start" {
			loopStartCalls++
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
		if loopStartCalls > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if loopStartCalls == 0 {
		t.Fatalf("expected dedicated loop runner start to be called")
	}
	if primaryStartCalls != 0 {
		t.Fatalf("expected primary plugin start not to be called, got %d", primaryStartCalls)
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
