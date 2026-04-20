// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package threads provides a pinned-thread runtime with plugin-scoped
// lifecycle/message events.
package threads

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// EventType identifies plugin worker event categories.
type EventType string

const (
	EventLoopStarting EventType = "plugin.loop.starting"
	EventLoopExited   EventType = "plugin.loop.exited"
	EventMessage      EventType = "plugin.message"
)

// Event describes one plugin runtime event.
type Event struct {
	Type        EventType
	PluginID    string
	ChannelType string
	Attempt     int
	Error       string
	Payload     []byte
	Timestamp   time.Time
}

const subscriberQueueSize = 64

type eventSubscriber struct {
	queue   chan Event
	handler func(Event)
}

// EventBus provides in-process pub/sub for plugin events.
type EventBus struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[uint64]*eventSubscriber
}

var defaultBus = NewEventBus()

// NewEventBus creates an empty event bus instance.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[uint64]*eventSubscriber),
	}
}

// DefaultEventBus returns the process-wide plugin event bus.
func DefaultEventBus() *EventBus {
	return defaultBus
}

// Subscribe registers a handler and returns an unsubscribe callback.
func (b *EventBus) Subscribe(handler func(Event)) func() {
	if b == nil || handler == nil {
		return func() {}
	}

	id := atomic.AddUint64(&b.nextID, 1)
	subscriber := &eventSubscriber{
		queue:   make(chan Event, subscriberQueueSize),
		handler: handler,
	}

	b.mu.Lock()
	b.subscribers[id] = subscriber
	b.mu.Unlock()

	go func(sub *eventSubscriber) {
		for event := range sub.queue {
			sub.handler(event)
		}
	}(subscriber)

	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			sub, ok := b.subscribers[id]
			if ok {
				delete(b.subscribers, id)
				close(sub.queue)
			}
			b.mu.Unlock()
		})
	}
}

// SubscribePlugin registers a plugin/channel filtered handler.
// Empty pluginID or channelType work as wildcard filters.
func (b *EventBus) SubscribePlugin(pluginID, channelType string, handler func(Event)) func() {
	if handler == nil {
		return func() {}
	}

	pluginFilter := normalizeKeyPart(pluginID)
	channelFilter := normalizeKeyPart(channelType)

	return b.Subscribe(func(event Event) {
		if pluginFilter != "" && normalizeKeyPart(event.PluginID) != pluginFilter {
			return
		}
		if channelFilter != "" && normalizeKeyPart(event.ChannelType) != channelFilter {
			return
		}
		handler(event)
	})
}

// Publish dispatches one event to all subscribers.
func (b *EventBus) Publish(event Event) {
	if b == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	b.mu.RLock()
	subs := make([]*eventSubscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subs = append(subs, sub)
	}
	b.mu.RUnlock()

	for _, sub := range subs {
		evt := cloneEvent(event)
		select {
		case sub.queue <- evt:
		default:
			// Favor fresh state over stale queue backlog.
			select {
			case <-sub.queue:
			default:
			}
			select {
			case sub.queue <- evt:
			default:
			}
		}
	}
}

// StartConfig describes one pinned worker launch.
type StartConfig struct {
	Context     context.Context
	PluginID    string
	ChannelType string
	Attempt     int
	Bus         *EventBus
	Work        func(context.Context) error
}

// WorkerHandle represents one running pinned worker.
type WorkerHandle struct {
	cancel   context.CancelFunc
	done     chan error
	stopOnce sync.Once
}

// Done returns a channel that receives one terminal worker error and closes.
func (h *WorkerHandle) Done() <-chan error {
	if h == nil || h.done == nil {
		empty := make(chan error)
		close(empty)
		return empty
	}
	return h.done
}

// Stop requests cancellation for the worker.
func (h *WorkerHandle) Stop() {
	if h == nil || h.cancel == nil {
		return
	}
	h.stopOnce.Do(func() {
		h.cancel()
	})
}

// Wait blocks until the worker exits or the wait context is canceled.
func (h *WorkerHandle) Wait(ctx context.Context) error {
	if h == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case err, ok := <-h.Done():
		if !ok {
			return nil
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WorkerRegistry coordinates workers by plugin/channel key.
type WorkerRegistry struct {
	mu      sync.Mutex
	workers map[string]*WorkerHandle
}

var defaultRegistry = NewWorkerRegistry()

// NewWorkerRegistry creates an empty worker registry.
func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{workers: make(map[string]*WorkerHandle)}
}

// DefaultWorkerRegistry returns the process-wide worker registry.
func DefaultWorkerRegistry() *WorkerRegistry {
	return defaultRegistry
}

// StartOrReplace launches a worker and cancels any previous worker with the
// same plugin/channel key.
func (r *WorkerRegistry) StartOrReplace(cfg StartConfig) (*WorkerHandle, error) {
	if r == nil {
		return StartWorker(cfg)
	}

	key := workerKey(cfg.PluginID, cfg.ChannelType)
	if key == "" {
		return StartWorker(cfg)
	}

	r.mu.Lock()
	if oldWorker, ok := r.workers[key]; ok {
		oldWorker.Stop()
		delete(r.workers, key)
	}

	worker, err := StartWorker(cfg)
	if err != nil {
		r.mu.Unlock()
		return nil, err
	}
	r.workers[key] = worker
	r.mu.Unlock()

	go func(registryKey string, expected *WorkerHandle) {
		<-expected.Done()
		r.mu.Lock()
		if current, ok := r.workers[registryKey]; ok && current == expected {
			delete(r.workers, registryKey)
		}
		r.mu.Unlock()
	}(key, worker)

	return worker, nil
}

// Stop cancels one worker by plugin/channel key.
func (r *WorkerRegistry) Stop(pluginID, channelType string) {
	if r == nil {
		return
	}

	key := workerKey(pluginID, channelType)
	if key == "" {
		return
	}

	r.mu.Lock()
	worker := r.workers[key]
	delete(r.workers, key)
	r.mu.Unlock()

	if worker != nil {
		worker.Stop()
	}
}

// StopAll cancels every worker currently tracked by the registry.
func (r *WorkerRegistry) StopAll() {
	if r == nil {
		return
	}

	r.mu.Lock()
	workers := make([]*WorkerHandle, 0, len(r.workers))
	for key, worker := range r.workers {
		workers = append(workers, worker)
		delete(r.workers, key)
	}
	r.mu.Unlock()

	for _, worker := range workers {
		if worker != nil {
			worker.Stop()
		}
	}
}

// StartPinned is a convenience helper that returns only the worker done channel.
func StartPinned(cfg StartConfig) (<-chan error, error) {
	worker, err := StartWorker(cfg)
	if err != nil {
		return nil, err
	}
	return worker.Done(), nil
}

// StartWorker launches the configured work inside a pinned OS thread.
func StartWorker(cfg StartConfig) (*WorkerHandle, error) {
	if cfg.Work == nil {
		return nil, fmt.Errorf("threads: start work is required")
	}

	ctx := cfg.Context
	if ctx == nil {
		ctx = context.Background()
	}

	bus := cfg.Bus
	if bus == nil {
		bus = DefaultEventBus()
	}

	runCtx, cancel := context.WithCancel(ctx)
	handle := &WorkerHandle{
		cancel: cancel,
		done:   make(chan error, 1),
	}

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		runErr := error(nil)

		defer func() {
			if recovered := recover(); recovered != nil {
				runErr = fmt.Errorf("threads: worker panic: %v", recovered)
			}

			exitEvent := Event{
				Type:        EventLoopExited,
				PluginID:    cfg.PluginID,
				ChannelType: cfg.ChannelType,
				Attempt:     cfg.Attempt,
			}
			if runErr != nil {
				exitEvent.Error = runErr.Error()
			}
			bus.Publish(exitEvent)

			handle.done <- runErr
			close(handle.done)
		}()

		bus.Publish(Event{
			Type:        EventLoopStarting,
			PluginID:    cfg.PluginID,
			ChannelType: cfg.ChannelType,
			Attempt:     cfg.Attempt,
		})

		runErr = cfg.Work(runCtx)
	}()

	return handle, nil
}

// PublishPluginMessage emits one inbound plugin message event.
func PublishPluginMessage(pluginID, channelType string, payload []byte) {
	publishPluginMessageWithBus(nil, pluginID, channelType, payload)
}

func publishPluginMessageWithBus(bus *EventBus, pluginID, channelType string, payload []byte) {
	if len(payload) == 0 {
		return
	}

	targetBus := bus
	if targetBus == nil {
		targetBus = DefaultEventBus()
	}

	targetBus.Publish(Event{
		Type:        EventMessage,
		PluginID:    strings.TrimSpace(pluginID),
		ChannelType: normalizeKeyPart(channelType),
		Payload:     append([]byte(nil), payload...),
	})
}

func cloneEvent(event Event) Event {
	cloned := event
	if len(event.Payload) > 0 {
		cloned.Payload = append([]byte(nil), event.Payload...)
	}
	return cloned
}

func workerKey(pluginID, channelType string) string {
	p := normalizeKeyPart(pluginID)
	c := normalizeKeyPart(channelType)
	if p == "" && c == "" {
		return ""
	}
	return p + "|" + c
}

func normalizeKeyPart(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}
