// Copyright (c) OpenLobster contributors. See LICENSE for details.

package scheduler

import (
	"container/heap"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/mock"
)

type mockConsolidation struct {
	mock.Mock
}

func (m *mockConsolidation) Consolidate(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func TestTaskHeap_Order(t *testing.T) {
	now := time.Now()
	h := &taskHeap{}
	heap.Init(h)

	for _, e := range []schedulerEntry{
		{at: now.Add(3 * time.Minute), task: models.Task{ID: "c"}},
		{at: now.Add(1 * time.Minute), task: models.Task{ID: "a"}},
		{at: now.Add(2 * time.Minute), task: models.Task{ID: "b"}},
	} {
		heap.Push(h, e)
	}

	if h.Len() != 3 {
		t.Fatalf("heap len: got %d, want 3", h.Len())
	}

	last := time.Time{}
	for h.Len() > 0 {
		e := heap.Pop(h).(schedulerEntry)
		if e.at.Before(last) {
			t.Errorf("wrong order: %v before %v", e.at, last)
		}
		last = e.at
	}
}

func TestSchedulerNextCronRun_StepExpression(t *testing.T) {
	ref := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	got := schedulerNextCronRun("*/15 * * * *", ref)
	want := time.Date(2024, 1, 1, 12, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSchedulerNextCronRun_InvalidFields(t *testing.T) {
	ref := time.Now()
	got := schedulerNextCronRun("bad", ref)
	delta := got.Sub(ref)
	if delta < 59*time.Minute || delta > 61*time.Minute {
		t.Errorf("invalid cron fallback should be ~1h, got delta=%v", delta)
	}
}

func TestCronFieldMatches(t *testing.T) {
	cases := []struct {
		field string
		value int
		want  bool
	}{
		{"*", 0, true},
		{"*", 59, true},
		{"30", 30, true},
		{"30", 0, false},
		{"*/15", 0, true},
		{"*/15", 15, true},
		{"*/15", 30, true},
		{"*/15", 7, false},
		{"*/0", 0, false},
		{"bad", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got := cronFieldMatches(tc.field, tc.value)
		if got != tc.want {
			t.Errorf("cronFieldMatches(%q, %d) = %v, want %v",
				tc.field, tc.value, got, tc.want)
		}
	}
}

func TestNextSleep_EmptyHeap(t *testing.T) {
	s := &Scheduler{heap: make(taskHeap, 0)}
	if got := s.nextSleep(); got != idleWait {
		t.Errorf("empty heap: got %v, want %v", got, idleWait)
	}
}

func TestNextSleep_FutureEntry(t *testing.T) {
	s := &Scheduler{heap: make(taskHeap, 0, 4)}
	heap.Init(&s.heap)
	future := time.Now().Add(5 * time.Minute)
	heap.Push(&s.heap, schedulerEntry{at: future, task: models.Task{ID: "x"}})
	got := s.nextSleep()
	if got < 4*time.Minute+55*time.Second || got > 5*time.Minute+5*time.Second {
		t.Errorf("nextSleep should be ~5m, got %v", got)
	}
}

func TestNextSleep_PastEntry(t *testing.T) {
	s := &Scheduler{heap: make(taskHeap, 0, 4)}
	heap.Init(&s.heap)
	heap.Push(&s.heap, schedulerEntry{
		at:   time.Now().Add(-1 * time.Minute),
		task: models.Task{ID: "y"},
	})
	if got := s.nextSleep(); got != 0 {
		t.Errorf("past entry: got %v, want 0", got)
	}
}

func TestScheduler_Notify_NonBlocking(t *testing.T) {
	s := NewScheduler(time.Hour, false, nil, nil, nil)
	s.Notify()
	done := make(chan struct{})
	go func() {
		s.Notify()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Notify() blocked on a full channel")
	}
}

func TestScheduler_ReloadSkipsAlreadyInHeap(t *testing.T) {
	task := models.Task{ID: "task-1", Prompt: "hello", AddedAt: time.Now()}
	s := newTestScheduler(&mockTaskRepo{tasks: []models.Task{task}})
	heap.Push(&s.heap, schedulerEntry{at: task.AddedAt, task: task})

	initial := s.heap.Len()
	s.reload(context.Background())

	if s.heap.Len() != initial {
		t.Errorf("heap grew from %d to %d; duplicate should have been skipped",
			initial, s.heap.Len())
	}
}

func TestScheduler_ReloadSkipsInFlight(t *testing.T) {
	task := models.Task{ID: "task-2", Prompt: "world", AddedAt: time.Now()}
	s := newTestScheduler(&mockTaskRepo{tasks: []models.Task{task}})
	s.inFlight.Store(task.ID, struct{}{})

	s.reload(context.Background())

	if s.heap.Len() != 0 {
		t.Errorf("in-flight task should not be added to heap, got size=%d", s.heap.Len())
	}
}

func TestSplitCronFields(t *testing.T) {
	got := splitCronFields("0 9 * * 1")
	if len(got) != 5 {
		t.Fatalf("want 5 fields, got %d: %v", len(got), got)
	}
	for i, w := range []string{"0", "9", "*", "*", "1"} {
		if got[i] != w {
			t.Errorf("field[%d]: got %q, want %q", i, got[i], w)
		}
	}
}

func newTestScheduler(repo ports.TaskRepositoryPort) *Scheduler {
	s := &Scheduler{
		heap:         make(taskHeap, 0, 8),
		notifyCh:     make(chan struct{}, 1),
		rescheduleCh: make(chan schedulerEntry, 8),
		taskRepo:     repo,
	}
	heap.Init(&s.heap)
	return s
}

type mockTaskRepo struct {
	tasks    []models.Task
	deleteFn func(context.Context, string) error
}

func (m *mockTaskRepo) GetPending(_ context.Context) ([]models.Task, error) {
	return m.tasks, nil
}
func (m *mockTaskRepo) Add(_ context.Context, _ *models.Task) error { return nil }
func (m *mockTaskRepo) MarkDone(_ context.Context, _ string) error  { return nil }
func (m *mockTaskRepo) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}
func (m *mockTaskRepo) Update(_ context.Context, _ *models.Task) error       { return nil }
func (m *mockTaskRepo) ListAll(_ context.Context) ([]models.Task, error)     { return m.tasks, nil }
func (m *mockTaskRepo) SetEnabled(_ context.Context, _ string, _ bool) error { return nil }
func (m *mockTaskRepo) SetStatus(_ context.Context, _ string, _ string) error {
	return nil
}

type errorTaskRepo struct{ err error }

func (e *errorTaskRepo) GetPending(_ context.Context) ([]models.Task, error)  { return nil, e.err }
func (e *errorTaskRepo) Add(_ context.Context, _ *models.Task) error          { return nil }
func (e *errorTaskRepo) MarkDone(_ context.Context, _ string) error           { return nil }
func (e *errorTaskRepo) Delete(_ context.Context, _ string) error             { return nil }
func (e *errorTaskRepo) Update(_ context.Context, _ *models.Task) error       { return nil }
func (e *errorTaskRepo) ListAll(_ context.Context) ([]models.Task, error)     { return nil, nil }
func (e *errorTaskRepo) SetEnabled(_ context.Context, _ string, _ bool) error { return nil }
func (e *errorTaskRepo) SetStatus(_ context.Context, _ string, _ string) error {
	return nil
}

func TestNewScheduler_ZeroInterval(t *testing.T) {
	s := NewScheduler(0, false, nil, nil, nil)
	if s.memInterval != 4*time.Hour {
		t.Errorf("zero memInterval should default to 4h, got %v", s.memInterval)
	}
}

func TestScheduler_Reload_NilRepo(t *testing.T) {
	s := newTestScheduler(nil)
	s.reload(context.Background())
	if s.heap.Len() != 0 {
		t.Errorf("nil repo should not add tasks, got heap size %d", s.heap.Len())
	}
}

func TestScheduler_Reload_GetPendingError(t *testing.T) {
	s := newTestScheduler(&errorTaskRepo{err: fmt.Errorf("db error")})
	s.reload(context.Background())
	if s.heap.Len() != 0 {
		t.Errorf("GetPending error should not add tasks, got heap size %d", s.heap.Len())
	}
}

func TestScheduler_Run_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewScheduler(time.Hour, false, nil, nil, nil)
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}

func TestScheduler_FireDue_DispatchesTask(t *testing.T) {
	task := models.Task{ID: "t1", Prompt: "p1", Schedule: "* * * * *", AddedAt: time.Now().Add(-time.Minute)}
	dispatched := make(chan string, 1)
	disp := &mockDispatcher{onDispatch: func(_ context.Context, p string) error {
		dispatched <- p
		return nil
	}}
	repo := &mockTaskRepo{tasks: []models.Task{task}}
	s := NewScheduler(time.Hour, false, disp, repo, nil)
	heap.Push(&s.heap, schedulerEntry{at: time.Now().Add(-time.Second), task: task})
	heap.Init(&s.heap)

	ctx := context.Background()
	s.fireDue(ctx)

	select {
	case p := <-dispatched:
		if p != "p1" {
			t.Errorf("dispatched prompt %q, want p1", p)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("dispatch was not called")
	}
}

type mockDispatcher struct {
	onDispatch func(context.Context, string) error
}

func (m *mockDispatcher) Dispatch(ctx context.Context, prompt string) error {
	if m.onDispatch != nil {
		return m.onDispatch(ctx, prompt)
	}
	return nil
}

func TestScheduler_Run_DispatchError_RequeuesCyclic(t *testing.T) {
	task := models.Task{ID: "t1", Prompt: "p1", Schedule: "* * * * *", AddedAt: time.Now()}
	disp := &mockDispatcher{onDispatch: func(_ context.Context, _ string) error {
		return fmt.Errorf("dispatch failed")
	}}
	repo := &mockTaskRepo{tasks: []models.Task{task}}
	s := NewScheduler(time.Hour, false, disp, repo, nil)
	heap.Push(&s.heap, schedulerEntry{at: time.Now().Add(-time.Second), task: task})
	heap.Init(&s.heap)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	s.fireDue(ctx)

	select {
	case e := <-s.rescheduleCh:
		if e.task.ID != "t1" {
			t.Errorf("requeued wrong task: %s", e.task.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cyclic task should have been requeued")
	}
}

func TestScheduler_Run_OneShot_DeletesTask(t *testing.T) {
	task := models.Task{ID: "t1", Prompt: "p1", Schedule: "", AddedAt: time.Now()}
	deleted := make(chan string, 1)
	disp := &mockDispatcher{onDispatch: func(_ context.Context, _ string) error { return nil }}
	repo := &mockTaskRepo{tasks: []models.Task{task}}
	repo.deleteFn = func(_ context.Context, id string) error {
		deleted <- id
		return nil
	}
	s := NewScheduler(time.Hour, false, disp, repo, nil)
	heap.Push(&s.heap, schedulerEntry{at: time.Now().Add(-time.Second), task: task})
	heap.Init(&s.heap)

	s.fireDue(context.Background())

	select {
	case id := <-deleted:
		if id != "t1" {
			t.Errorf("deleted wrong task: %s", id)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("one-shot task should have been deleted")
	}
}

func TestScheduler_Run_WithMemoryConsolidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	disp := &mockDispatcher{}
	mc := &mockConsolidation{}
	mc.On("Consolidate", mock.Anything).Return(nil)
	s := NewScheduler(10*time.Millisecond, true, disp, nil, mc)
	go s.Run(ctx)
	time.Sleep(30 * time.Millisecond)
}
