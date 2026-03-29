// Copyright (c) OpenLobster contributors. See LICENSE for details.

package handlers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newPoolHandler builds a minimal MessageHandler wired only with the given
// channelChecker and AI provider. It starts `workers` pool goroutines.
// Using the struct literal directly (same package) avoids the full constructor.
func newPoolHandler(workers int, checker userChannelChecker, ai ports.AIProviderPort) *MessageHandler {
	h := &MessageHandler{
		runner:         agenticRunner{aiProvider: ai},
		channelChecker: checker,
		queue:          newJobQueue(),
	}
	for i := 0; i < workers; i++ {
		go h.runWorker()
	}
	return h
}

// recordingChecker implements userChannelChecker.
// It reports every pairKey that reaches handle() via the calls slice.
// If gate is not nil, each call blocks until the gate is closed.
// ready receives the pairKey as soon as the call starts (before blocking).
type recordingChecker struct {
	mu    sync.Mutex
	calls []string
	gate  chan struct{} // nil = don't block; close() to unblock all
	ready chan string   // non-nil = send pairKey when call begins
}

func (r *recordingChecker) ExistsByPlatformUserID(_ context.Context, pairKey string) (bool, error) {
	if r.ready != nil {
		r.ready <- pairKey
	}
	if r.gate != nil {
		<-r.gate
	}
	r.mu.Lock()
	r.calls = append(r.calls, pairKey)
	r.mu.Unlock()
	return true, nil // paired → continues to LLM
}
func (r *recordingChecker) GetUserIDByPlatformUserID(_ context.Context, pairKey string) (string, error) {
	return pairKey, nil
}
func (r *recordingChecker) GetDisplayNameByPlatformUserID(_ context.Context, pairKey string) (string, error) {
	return pairKey, nil
}
func (r *recordingChecker) GetDisplayNameByUserID(_ context.Context, userID string) (string, error) {
	return userID, nil
}
func (r *recordingChecker) UpdateLastSeen(_ context.Context, _, _ string) error { return nil }

// countingAI implements ports.AIProviderPort and counts Chat calls.
type countingAI struct{ calls int32 }

func (c *countingAI) Chat(_ context.Context, _ ports.ChatRequest) (ports.ChatResponse, error) {
	atomic.AddInt32(&c.calls, 1)
	return ports.ChatResponse{Content: "NO_REPLY"}, nil
}
func (c *countingAI) ChatWithAudio(_ context.Context, _ ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return ports.ChatResponse{}, nil
}
func (c *countingAI) ChatToAudio(_ context.Context, _ ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, nil
}
func (c *countingAI) SupportsAudioInput() bool  { return false }
func (c *countingAI) SupportsAudioOutput() bool { return false }
func (c *countingAI) GetMaxTokens() int         { return 500 }
func (c *countingAI) GetContextWindow() int     { return 8000 }

// ─── jobQueue unit tests ──────────────────────────────────────────────────────

// TestJobQueue_Unbounded verifies that the queue grows without limit: no message
// is ever dropped regardless of how many are enqueued before any worker starts.
func TestJobQueue_Unbounded(t *testing.T) {
	const n = 10_000
	q := newJobQueue()
	for i := range n {
		q.enqueue(convJob{
			inp:  HandleMessageInput{SenderID: fmt.Sprintf("u%d", i), Content: fmt.Sprintf("msg%d", i)},
			done: make(chan error, 1),
		})
	}
	if q.Len() != n {
		t.Fatalf("queue length = %d, want %d (messages were dropped)", q.Len(), n)
	}
}

func TestJobQueue_EnqueueDequeue(t *testing.T) {
	q := newJobQueue()
	done := make(chan error, 1)
	q.enqueue(convJob{inp: HandleMessageInput{ChannelID: "ch1", Content: "hello"}, done: done})

	got := q.dequeue()
	assert.Equal(t, "hello", got.inp.Content)
}

func TestJobQueue_DrainUser_CollapsesSameUser(t *testing.T) {
	q := newJobQueue()
	done1 := make(chan error, 1)
	done2 := make(chan error, 1)
	done3 := make(chan error, 1)

	q.enqueue(convJob{inp: HandleMessageInput{SenderID: "u1", Content: "msg1"}, done: done1})
	q.enqueue(convJob{inp: HandleMessageInput{SenderID: "u1", Content: "msg2"}, done: done2})
	q.enqueue(convJob{inp: HandleMessageInput{SenderID: "u1", Content: "msg3"}, done: done3})

	// Simulate the worker: dequeue the first job, then drain the rest.
	first := q.dequeue()
	assert.Equal(t, "msg1", first.inp.Content)

	latest := q.drainUser("u1")
	require.NotNil(t, latest)
	assert.Equal(t, "msg3", latest.inp.Content, "latest should be the last enqueued")

	// msg2 should be signalled done (discarded intermediate).
	select {
	case err := <-done2:
		assert.NoError(t, err)
	default:
		t.Fatal("done2 should already be signalled")
	}
	// msg3's done is still pending (worker will signal it after processing).
	select {
	case <-done3:
		t.Fatal("done3 should not be signalled yet")
	default:
	}
}

func TestJobQueue_DrainUser_PreservesOtherUsers(t *testing.T) {
	q := newJobQueue()
	doneA := make(chan error, 1)
	doneB := make(chan error, 1)
	doneA2 := make(chan error, 1)

	q.enqueue(convJob{inp: HandleMessageInput{SenderID: "alice", Content: "a1"}, done: doneA})
	q.enqueue(convJob{inp: HandleMessageInput{SenderID: "bob", Content: "b1"}, done: doneB})
	q.enqueue(convJob{inp: HandleMessageInput{SenderID: "alice", Content: "a2"}, done: doneA2})

	_ = q.dequeue() // worker picks up alice's first job
	latest := q.drainUser("alice")

	require.NotNil(t, latest)
	assert.Equal(t, "a2", latest.inp.Content)

	// Bob's message must still be in the queue.
	bobJob := q.dequeue()
	assert.Equal(t, "bob", bobJob.inp.SenderID)
	assert.Equal(t, "b1", bobJob.inp.Content)
}

func TestJobQueue_DrainUser_SkipsLoopback(t *testing.T) {
	q := newJobQueue()
	doneL := make(chan error, 1)
	doneU := make(chan error, 1)
	doneL2 := make(chan error, 1)

	loopID := "loopback-uuid-1"
	q.enqueue(convJob{inp: HandleMessageInput{SenderID: loopID, ChannelType: "loopback", Content: "task1"}, done: doneL})
	q.enqueue(convJob{inp: HandleMessageInput{SenderID: "user1", Content: "hi"}, done: doneU})
	q.enqueue(convJob{inp: HandleMessageInput{SenderID: loopID, ChannelType: "loopback", Content: "task2"}, done: doneL2})

	_ = q.dequeue() // worker picks loopback task1

	// drainUser for the loopback pairKey must not collapse task2.
	latest := q.drainUser(loopID)
	assert.Nil(t, latest, "loopback entries must not be drained")

	// Both remaining jobs still in the queue.
	j1 := q.dequeue()
	j2 := q.dequeue()
	keys := []string{j1.inp.SenderID, j2.inp.SenderID}
	assert.Contains(t, keys, "user1")
	assert.Contains(t, keys, loopID)
}

func TestJobQueue_DrainUser_NothingToDrain(t *testing.T) {
	q := newJobQueue()
	done := make(chan error, 1)
	q.enqueue(convJob{inp: HandleMessageInput{SenderID: "u1"}, done: done})
	_ = q.dequeue()

	latest := q.drainUser("u1")
	assert.Nil(t, latest)
}

// ─── pool / Handle integration tests ─────────────────────────────────────────

func TestHandle_EmptyChannelID_Dropped(t *testing.T) {
	h := newPoolHandler(1, nil, nil)
	err := h.Handle(context.Background(), HandleMessageInput{})
	assert.NoError(t, err)
}

// TestHandle_BurstDrained verifies that when a user sends several messages
// in rapid succession, only the latest reaches the LLM. Intermediate jobs
// are drained and their callers unblock with nil immediately.
//
// The queue is pre-filled before the worker starts so the drain is
// deterministic (no race between enqueue and dequeue).
func TestHandle_BurstDrained(t *testing.T) {
	ai := &countingAI{}
	checker := &recordingChecker{}

	// Build handler without starting workers yet.
	h := &MessageHandler{
		runner:         agenticRunner{aiProvider: ai},
		channelChecker: checker,
		queue:          newJobQueue(),
	}

	ctx := context.Background()
	dones := make([]chan error, 3)
	for i := range dones {
		dones[i] = make(chan error, 1)
		h.queue.enqueue(convJob{
			ctx: ctx,
			inp: HandleMessageInput{
				ChannelID:   "ch-user1",
				SenderID:    "user1",
				ChannelType: "telegram",
				Content:     fmt.Sprintf("msg%d", i+1),
			},
			done: dones[i],
		})
	}

	// Start a single worker after the queue is fully loaded.
	go h.runWorker()

	// All three callers must unblock.
	for i, ch := range dones {
		select {
		case err := <-ch:
			assert.NoError(t, err, "done[%d]", i)
		case <-time.After(2 * time.Second):
			t.Fatalf("done[%d] did not complete", i)
		}
	}

	// Only 1 LLM call: the worker dequeued job1, drained job2+job3 (keeping
	// only the latest), and called handle() once for the last message.
	assert.Equal(t, int32(1), atomic.LoadInt32(&ai.calls))
}

// TestHandle_DifferentUsersConcurrent verifies that messages from different
// users are processed concurrently (they do not block each other).
func TestHandle_DifferentUsersConcurrent(t *testing.T) {
	gate := make(chan struct{})
	checker := &recordingChecker{gate: gate, ready: make(chan string, 2)}
	h := newPoolHandler(2, checker, &countingAI{})

	ctx := context.Background()
	done1 := make(chan error, 1)
	done2 := make(chan error, 1)

	go func() {
		done1 <- h.Handle(ctx, HandleMessageInput{ChannelID: "ch-neirth", SenderID: "neirth", ChannelType: "telegram", Content: "hola"})
	}()
	go func() {
		done2 <- h.Handle(ctx, HandleMessageInput{ChannelID: "ch-josue", SenderID: "josue", ChannelType: "whatsapp", Content: "hey"})
	}()

	// Both workers should reach the gate simultaneously (concurrent).
	started := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case key := <-checker.ready:
			started[key] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of 2 workers started; want both concurrent", len(started))
		}
	}
	assert.True(t, started["neirth"])
	assert.True(t, started["josue"])

	close(gate)
	for _, ch := range []chan error{done1, done2} {
		select {
		case err := <-ch:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("message did not complete within timeout")
		}
	}
}

// TestHandle_SameUserSerialized verifies that two concurrent Handle calls for
// the same user do not run the agentic loop in parallel.
func TestHandle_SameUserSerialized(t *testing.T) {
	gate := make(chan struct{})
	checker := &recordingChecker{gate: gate, ready: make(chan string, 2)}
	h := newPoolHandler(2, checker, &countingAI{})

	ctx := context.Background()
	done1 := make(chan error, 1)
	done2 := make(chan error, 1)

	go func() {
		done1 <- h.Handle(ctx, HandleMessageInput{ChannelID: "ch1", SenderID: "neirth", ChannelType: "telegram", Content: "msg1"})
	}()
	go func() {
		done2 <- h.Handle(ctx, HandleMessageInput{ChannelID: "ch1", SenderID: "neirth", ChannelType: "telegram", Content: "msg2"})
	}()

	// Only one worker should reach the gate: either the first is processing
	// and the second is queued, or the second was drained.
	select {
	case <-checker.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("no worker started")
	}

	// The second call must NOT appear simultaneously.
	select {
	case key := <-checker.ready:
		t.Fatalf("second concurrent handle() call started for %s — not serialised", key)
	case <-time.After(100 * time.Millisecond):
		// Good: only one active at a time.
	}

	close(gate)
	for _, ch := range []chan error{done1, done2} {
		select {
		case err := <-ch:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("message did not complete within timeout")
		}
	}
}

// TestHandle_ScenarioNeirthJosueDashboard verifies the scenario where:
//   - Neirth sends a Telegram message
//   - Josué sends a WhatsApp message
//   - An admin sends a message via the dashboard loading Bob's conversation
//
// Each has a distinct pairKey so none blocks the others, and each reaches
// the LLM exactly once.
//
// Note: dashboard messages skip channelChecker (isInternal=true) so
// concurrency is observed via the AI provider call count rather than the
// checker. Neirth and Josué concurrency is verified via the gate.
func TestHandle_ScenarioNeirthJosueDashboard(t *testing.T) {
	gate := make(chan struct{})
	checker := &recordingChecker{gate: gate, ready: make(chan string, 2)}
	ai := &countingAI{}
	h := newPoolHandler(3, checker, ai)

	ctx := context.Background()
	// Must be a valid UUID so handle() does not return early on parse error.
	bobConvID := "00000000-0000-0000-0000-000000000042"

	results := make([]chan error, 3)
	for i := range results {
		results[i] = make(chan error, 1)
	}

	go func() {
		results[0] <- h.Handle(ctx, HandleMessageInput{
			ChannelID: "tg-neirth", SenderID: "neirth", ChannelType: "telegram", Content: "hola",
		})
	}()
	go func() {
		results[1] <- h.Handle(ctx, HandleMessageInput{
			ChannelID: "wa-josue", SenderID: "josue", ChannelType: "whatsapp", Content: "hey",
		})
	}()
	go func() {
		results[2] <- h.Handle(ctx, HandleMessageInput{
			ChannelID:      "dashboard",
			SenderID:       "dashboard",
			ChannelType:    "dashboard",
			ConversationID: &bobConvID,
			Content:        "what's up bob",
		})
	}()

	// Neirth and Josué must both reach the checker concurrently (gate blocks them).
	started := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case key := <-checker.ready:
			started[key] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d/2 external users started; expected concurrent", len(started))
		}
	}
	assert.True(t, started["neirth"], "neirth must be processed")
	assert.True(t, started["josue"], "josué must be processed")

	// Dashboard bypasses channelChecker (isInternal) so it completes on its own.
	// Unblock the gate so Neirth and Josué can finish.
	close(gate)

	for i, ch := range results {
		select {
		case err := <-ch:
			assert.NoError(t, err, "result %d", i)
		case <-time.After(2 * time.Second):
			t.Fatalf("result %d did not complete", i)
		}
	}

	// Each of the three messages triggered exactly one LLM call.
	assert.Equal(t, int32(3), atomic.LoadInt32(&ai.calls))
}
