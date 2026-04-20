package events

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewEvent(t *testing.T) {
	payload := map[string]string{"key": "value"}
	event := NewEvent("test_event", payload)

	assert.Equal(t, "test_event", event.GetType())
	assert.Equal(t, payload, event.GetPayload())
	assert.WithinDuration(t, time.Now(), event.GetTimestamp(), time.Second)
}

func TestEventConstants(t *testing.T) {
	assert.Equal(t, "message_received", EventMessageReceived)
	assert.Equal(t, "message_sent", EventMessageSent)
	assert.Equal(t, "message_processed", EventMessageProcessed)
	assert.Equal(t, "session_started", EventSessionStarted)
	assert.Equal(t, "session_ended", EventSessionEnded)
	assert.Equal(t, "user_paired", EventUserPaired)
	assert.Equal(t, "user_unpaired", EventUserUnpaired)
	assert.Equal(t, "task_added", EventTaskAdded)
	assert.Equal(t, "task_completed", EventTaskCompleted)
	assert.Equal(t, "cron_job_executed", EventCronJobExecuted)
	assert.Equal(t, "mcp_server_connected", EventMCPServerConnected)
	assert.Equal(t, "mcp_server_disconnected", EventMCPServerDisconnected)
	assert.Equal(t, "memory_updated", EventMemoryUpdated)
	assert.Equal(t, "compaction_triggered", EventCompactionTriggered)
	assert.Equal(t, "compaction_completed", EventCompactionCompleted)
}

func TestBaseEvent(t *testing.T) {
	event := &BaseEvent{
		eventType: "test",
		timestamp: time.Now(),
		payload:   "payload",
	}

	assert.Equal(t, "test", event.GetType())
	assert.Equal(t, "payload", event.GetPayload())
}
