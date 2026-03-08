// Copyright (c) OpenLobster contributors. See LICENSE for details.

// Package scheduler provides a libuv-inspired event loop for task execution.
//
// The Scheduler drives task execution with sub-second accuracy using a
// min-heap of (nextAt, task) entries and a single time.Timer that always
// points at the nearest deadline.  The loop wakes ONLY when:
//
//  1. The soonest entry is due   → fireDue: pop all due entries, dispatch goroutines
//  2. An external change arrives → Notify: reload DB, reset timer
//  3. A cyclic task completes    → rescheduleCh: re-insert at next cron time
//  4. The memory interval fires  → consolidateMemory goroutine
//  5. Context is cancelled       → clean shutdown
package scheduler

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// MemoryConsolidationPrompt is issued to the agent on each periodic memory
// consolidation cycle.
const MemoryConsolidationPrompt = "Run scheduled memory consolidation: " +
	"review recent conversations for all users and extract key information " +
	"to long-term memory. Store important facts, preferences, and context."

const idleWait = 30 * time.Minute

type schedulerEntry struct {
	at   time.Time
	task models.Task
}

type taskHeap []schedulerEntry

func (h taskHeap) Len() int            { return len(h) }
func (h taskHeap) Less(i, j int) bool  { return h[i].at.Before(h[j].at) }
func (h taskHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *taskHeap) Push(x interface{}) { *h = append(*h, x.(schedulerEntry)) }
func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// Scheduler is a libuv-inspired, single-threaded event loop.
type Scheduler struct {
	dispatcher  ports.TaskDispatcherPort
	taskRepo    ports.TaskRepositoryPort
	memInterval time.Duration
	memEnabled  bool

	heap         taskHeap
	notifyCh     chan struct{}
	rescheduleCh chan schedulerEntry
	inFlight     sync.Map
}

// NewScheduler constructs a Scheduler ready to be started with Run.
func NewScheduler(
	memInterval time.Duration,
	memEnabled bool,
	dispatcher ports.TaskDispatcherPort,
	taskRepo ports.TaskRepositoryPort,
) *Scheduler {
	if memInterval <= 0 {
		memInterval = 4 * time.Hour
	}
	return &Scheduler{
		dispatcher:   dispatcher,
		taskRepo:     taskRepo,
		memInterval:  memInterval,
		memEnabled:   memEnabled,
		heap:         make(taskHeap, 0, 16),
		notifyCh:     make(chan struct{}, 1),
		rescheduleCh: make(chan schedulerEntry, 64),
	}
}

// Notify wakes the event loop so it reloads pending tasks from the database.
func (s *Scheduler) Notify() {
	select {
	case s.notifyCh <- struct{}{}:
	default:
	}
}

// Run starts the event loop. It blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.reload(ctx)

	timer := time.NewTimer(s.nextSleep())
	defer timer.Stop()

	var memTickerC <-chan time.Time
	if s.memEnabled && s.memInterval > 0 {
		mt := time.NewTicker(s.memInterval)
		defer mt.Stop()
		memTickerC = mt.C
	}

	log.Printf("scheduler: event loop started (pending=%d memConsolidation=%v)",
		len(s.heap), s.memEnabled)

	for {
		select {
		case <-ctx.Done():
			log.Println("scheduler: event loop stopped")
			return

		case <-timer.C:
			s.fireDue(ctx)
			resetTimer(timer, s.nextSleep())

		case entry := <-s.rescheduleCh:
			heap.Push(&s.heap, entry)
			resetTimer(timer, s.nextSleep())

		case <-s.notifyCh:
			s.reload(ctx)
			resetTimer(timer, s.nextSleep())

		case <-memTickerC:
			go s.consolidateMemory(ctx)
		}
	}
}

func (s *Scheduler) fireDue(ctx context.Context) {
	now := time.Now()
	for len(s.heap) > 0 && !s.heap[0].at.After(now) {
		entry := heap.Pop(&s.heap).(schedulerEntry)
		s.inFlight.Store(entry.task.ID, struct{}{})
		go s.run(ctx, entry)
	}
}

func (s *Scheduler) run(ctx context.Context, entry schedulerEntry) {
	defer s.inFlight.Delete(entry.task.ID)

	task := entry.task
	log.Printf("scheduler: executing task %s [%s] schedule=%q",
		task.ID, task.TaskType, task.Schedule)

	err := s.dispatcher.Dispatch(ctx, task.Prompt)
	if err != nil {
		log.Printf("scheduler: task %s execution error: %v", task.ID, err)
		if !isOneShotSchedule(task.Schedule) {
			s.requeueCyclic(task)
		}
		return
	}

	if isOneShotSchedule(task.Schedule) {
		if err := s.taskRepo.Delete(ctx, task.ID); err != nil {
			log.Printf("scheduler: failed to delete one-shot task %s: %v", task.ID, err)
		}
		log.Printf("scheduler: one-shot task %s completed and removed", task.ID)
		return
	}

	s.requeueCyclic(task)
}

func (s *Scheduler) requeueCyclic(task models.Task) {
	next := schedulerNextCronRun(task.Schedule, time.Now())
	log.Printf("scheduler: cyclic task %s rescheduled at %s", task.ID, next.Format(time.RFC3339))
	s.rescheduleCh <- schedulerEntry{at: next, task: task}
}

func (s *Scheduler) reload(ctx context.Context) {
	if s.taskRepo == nil {
		return
	}
	tasks, err := s.taskRepo.GetPending(ctx)
	if err != nil {
		log.Printf("scheduler: reload error: %v", err)
		return
	}

	// Debug: log how many pending tasks were returned and their metadata.
	log.Printf("scheduler: reload fetched %d pending task(s) from DB", len(tasks))
	for _, t := range tasks {
		log.Printf("scheduler: pending task id=%s schedule=%q addedAt=%s enabled=%v status=%s", t.ID, t.Schedule, t.AddedAt.Format(time.RFC3339), t.Enabled, t.Status)
	}

	inHeap := make(map[string]struct{}, len(s.heap))
	for _, e := range s.heap {
		inHeap[e.task.ID] = struct{}{}
	}

	added := 0
	for _, task := range tasks {
		if _, ok := inHeap[task.ID]; ok {
			continue
		}
		if _, ok := s.inFlight.Load(task.ID); ok {
			continue
		}
		heap.Push(&s.heap, schedulerEntry{at: computeNextAt(task), task: task})
		added++
	}
	if added > 0 {
		log.Printf("scheduler: loaded %d task(s) from DB (heap size=%d)", added, len(s.heap))
	}
}

func (s *Scheduler) nextSleep() time.Duration {
	if len(s.heap) == 0 {
		return idleWait
	}
	d := time.Until(s.heap[0].at)
	if d < 0 {
		return 0
	}
	return d
}

func (s *Scheduler) consolidateMemory(ctx context.Context) {
	log.Println("scheduler: running memory consolidation")
	if err := s.dispatcher.Dispatch(ctx, MemoryConsolidationPrompt); err != nil {
		log.Printf("scheduler: memory consolidation error: %v", err)
	}
}

func computeNextAt(task models.Task) time.Time {
	switch {
	case task.Schedule == "":
		return task.AddedAt
	case isDatetimeSchedule(task.Schedule):
		t, _ := time.Parse(time.RFC3339, task.Schedule)
		return t
	default:
		return schedulerNextCronRun(task.Schedule, time.Now())
	}
}

func isDatetimeSchedule(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func isOneShotSchedule(s string) bool {
	return s == "" || isDatetimeSchedule(s)
}

func schedulerNextCronRun(schedule string, after time.Time) time.Time {
	fields := splitCronFields(schedule)
	if len(fields) != 5 {
		return after.Add(time.Hour)
	}

	candidate := after.Truncate(time.Minute).Add(time.Minute)
	deadline := after.Add(366 * 24 * time.Hour)

	for candidate.Before(deadline) {
		if cronFieldMatches(fields[1], candidate.Hour()) &&
			cronFieldMatches(fields[0], candidate.Minute()) &&
			cronFieldMatches(fields[2], candidate.Day()) &&
			cronFieldMatches(fields[3], int(candidate.Month())) &&
			cronFieldMatches(fields[4], int(candidate.Weekday())) {
			return candidate
		}
		candidate = candidate.Add(time.Minute)
	}
	return after.Add(time.Hour)
}

func splitCronFields(s string) []string {
	var fields []string
	cur := ""
	for _, ch := range s {
		if ch == ' ' || ch == '\t' {
			if cur != "" {
				fields = append(fields, cur)
				cur = ""
			}
		} else {
			cur += string(ch)
		}
	}
	if cur != "" {
		fields = append(fields, cur)
	}
	return fields
}

func cronFieldMatches(f string, value int) bool {
	if f == "*" {
		return true
	}
	if len(f) > 2 && f[:2] == "*/" {
		var step int
		if _, err := fmt.Sscanf(f[2:], "%d", &step); err == nil && step > 0 {
			return value%step == 0
		}
		return false
	}
	var n int
	if _, err := fmt.Sscanf(f, "%d", &n); err == nil {
		return n == value
	}
	return false
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}
