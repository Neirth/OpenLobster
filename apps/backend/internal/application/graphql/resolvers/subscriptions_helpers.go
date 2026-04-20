package resolvers

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/domain/events"
)

// allEventTypes lists every event type for subscribeAll.
var allEventTypes = []string{
	events.EventMessageReceived, events.EventMessageSent, events.EventMessageProcessed,
	events.EventSessionStarted, events.EventSessionEnded,
	events.EventUserPaired, events.EventUserUnpaired,
	events.EventPairingRequested, events.EventPairingApproved, events.EventPairingDenied,
	events.EventTaskAdded, events.EventTaskCompleted, events.EventCronJobExecuted,
	events.EventMCPServerConnected, events.EventMCPServerDisconnected,
	events.EventMemoryUpdated, events.EventCompactionTriggered, events.EventCompactionCompleted,
}

// subscribe subscribes to a single event type and converts it to EventPayload.
func (r *Resolver) subscribe(ctx context.Context, eventType string) (<-chan *generated.EventPayload, error) {
	if r.Sub == nil {
		ch := make(chan *generated.EventPayload)
		close(ch)
		return ch, nil
	}
	src, err := r.Sub.Subscribe(ctx, eventType)
	if err != nil {
		return nil, err
	}
	out := make(chan *generated.EventPayload, 64)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-src:
				if !ok {
					return
				}
				p := eventToPayload(ev)
				if p == nil {
					continue
				}
				select {
				case out <- p:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

// subscribeAll subscribes to all event types and merges them into a single channel.
func (r *Resolver) subscribeAll(ctx context.Context) (<-chan *generated.EventPayload, error) {
	if r.Sub == nil {
		ch := make(chan *generated.EventPayload)
		close(ch)
		return ch, nil
	}
	out := make(chan *generated.EventPayload, 64)
	done := ctx.Done()
	var wg sync.WaitGroup
	for _, et := range allEventTypes {
		src, err := r.Sub.Subscribe(ctx, et)
		if err != nil {
			continue
		}
		wg.Add(1)
		go func(eventCh <-chan events.Event) {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case ev, ok := <-eventCh:
					if !ok {
						return
					}
					p := eventToPayload(ev)
					if p == nil {
						continue
					}
					select {
					case out <- p:
					case <-done:
						return
					}
				}
			}
		}(src)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out, nil
}

func eventToPayload(ev events.Event) *generated.EventPayload {
	if ev == nil {
		return nil
	}
	data := payloadToMap(ev.GetPayload())
	return &generated.EventPayload{
		Type:      ev.GetType(),
		Timestamp: ev.GetTimestamp().Format(time.RFC3339),
		Data:      data,
	}
}

func payloadToMap(p interface{}) map[string]any {
	if p == nil {
		return nil
	}
	if m, ok := p.(map[string]any); ok {
		return m
	}
	// Convert structs or other types via JSON.
	b, err := json.Marshal(p)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}
