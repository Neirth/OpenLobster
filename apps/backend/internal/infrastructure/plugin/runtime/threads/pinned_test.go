// Copyright (c) OpenLobster contributors. See LICENSE for details.

package threads

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStartPinnedPublishesLifecycleEvents(t *testing.T) {
	bus := NewEventBus()
	events := make(chan Event, 4)
	unsub := bus.Subscribe(func(event Event) {
		events <- event
	})
	defer unsub()

	errCh, err := StartPinned(StartConfig{
		Context:     context.Background(),
		PluginID:    "openlobster-messages-telegram",
		ChannelType: "telegram",
		Attempt:     3,
		Bus:         bus,
		Work: func(context.Context) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("StartPinned returned error: %v", err)
	}

	select {
	case runErr := <-errCh:
		if runErr != nil {
			t.Fatalf("unexpected runner error: %v", runErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for pinned worker completion")
	}

	var sawStart bool
	var sawExit bool
	deadline := time.After(2 * time.Second)
	for !sawStart || !sawExit {
		select {
		case event := <-events:
			switch event.Type {
			case EventLoopStarting:
				sawStart = true
			case EventLoopExited:
				sawExit = true
			}
		case <-deadline:
			t.Fatalf("missing lifecycle events (start=%v exit=%v)", sawStart, sawExit)
		}
	}
}

func TestStartPinnedPublishesExitError(t *testing.T) {
	bus := NewEventBus()
	events := make(chan Event, 2)
	unsub := bus.Subscribe(func(event Event) {
		events <- event
	})
	defer unsub()

	wantErr := errors.New("boom")
	errCh, err := StartPinned(StartConfig{
		PluginID: "openlobster-messages-discord",
		Bus:      bus,
		Work: func(context.Context) error {
			return wantErr
		},
	})
	if err != nil {
		t.Fatalf("StartPinned returned error: %v", err)
	}

	select {
	case gotErr := <-errCh:
		if gotErr == nil || gotErr.Error() != wantErr.Error() {
			t.Fatalf("unexpected worker error: %v", gotErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for worker error")
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-events:
			if event.Type == EventLoopExited {
				if event.Error != wantErr.Error() {
					t.Fatalf("unexpected exit event error: %q", event.Error)
				}
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for exit event")
		}
	}
}

func TestPublishPluginMessageCopiesPayload(t *testing.T) {
	bus := NewEventBus()
	events := make(chan Event, 1)
	unsub := bus.Subscribe(func(event Event) {
		events <- event
	})
	defer unsub()

	payload := []byte("hello")
	publishPluginMessageWithBus(bus, "openlobster-messages-telegram", "telegram", payload)
	payload[0] = 'X'

	select {
	case event := <-events:
		if event.Type != EventMessage {
			t.Fatalf("unexpected event type: %s", event.Type)
		}
		if string(event.Payload) != "hello" {
			t.Fatalf("payload not copied, got %q", string(event.Payload))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message event")
	}
}

func TestSubscribePluginFiltersEvents(t *testing.T) {
	bus := NewEventBus()
	events := make(chan Event, 2)
	unsub := bus.SubscribePlugin("openlobster-messages-telegram", "telegram", func(event Event) {
		events <- event
	})
	defer unsub()

	bus.Publish(Event{Type: EventMessage, PluginID: "openlobster-messages-discord", ChannelType: "discord"})
	bus.Publish(Event{Type: EventMessage, PluginID: "openlobster-messages-telegram", ChannelType: "telegram"})

	select {
	case event := <-events:
		if event.PluginID != "openlobster-messages-telegram" {
			t.Fatalf("unexpected plugin id: %s", event.PluginID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for filtered event")
	}

	select {
	case event := <-events:
		t.Fatalf("unexpected extra event: %+v", event)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestWorkerRegistryStartOrReplaceStopsPrevious(t *testing.T) {
	registry := NewWorkerRegistry()
	firstStopped := make(chan struct{}, 1)
	firstStarted := make(chan struct{}, 1)

	firstWorker, err := registry.StartOrReplace(StartConfig{
		PluginID:    "openlobster-messages-telegram",
		ChannelType: "telegram",
		Work: func(ctx context.Context) error {
			firstStarted <- struct{}{}
			<-ctx.Done()
			firstStopped <- struct{}{}
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("failed to start first worker: %v", err)
	}
	defer firstWorker.Stop()

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first worker did not start")
	}

	secondWorker, err := registry.StartOrReplace(StartConfig{
		PluginID:    "openlobster-messages-telegram",
		ChannelType: "telegram",
		Work: func(context.Context) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("failed to replace worker: %v", err)
	}
	defer secondWorker.Stop()

	select {
	case <-firstStopped:
	case <-time.After(2 * time.Second):
		t.Fatal("previous worker was not stopped on replace")
	}

	select {
	case <-secondWorker.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("replacement worker did not complete")
	}
}
