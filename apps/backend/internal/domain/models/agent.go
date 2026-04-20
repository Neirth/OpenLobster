package models

import (
	"time"

	"github.com/google/uuid"
)

type Agent struct {
	ID           uuid.UUID            `json:"id"`
	Name         string               `json:"name"`
	SystemPrompt string               `json:"system_prompt"`
	Model        Model                `json:"model"`
	Memory       MemoryBackendType    `json:"memory"`
	MCPClients   []string             `json:"mcp_clients"`
	Channels     []AgentChannelConfig `json:"channels"`
	Capabilities Capabilities         `json:"capabilities"`
}

type Capabilities struct {
	HasBrowser    bool `json:"has_browser"`
	HasTerminal   bool `json:"has_terminal"`
	HasSubagents  bool `json:"has_subagents"`
	HasMemory     bool `json:"has_memory"`
	HasMCP        bool `json:"has_mcp"`
	HasAudio      bool `json:"has_audio"`
	HasFilesystem bool `json:"has_filesystem"`
	HasSessions   bool `json:"has_sessions"`
}

type Model struct {
	Provider ModelProvider `json:"provider"`
	ID       string        `json:"id"`
}

type AgentChannelConfig struct {
	Type      ChannelType `json:"type"`
	Enabled   bool        `json:"enabled"`
	ChannelID string      `json:"channel_id"`
}

func (c *Capabilities) IsEnabled(name string) bool {
	switch name {
	case "browser":
		return c.HasBrowser
	case "terminal":
		return c.HasTerminal
	case "subagents":
		return c.HasSubagents
	case "memory":
		return c.HasMemory
	case "mcp":
		return c.HasMCP
	case "audio":
		return c.HasAudio
	case "filesystem":
		return c.HasFilesystem
	case "sessions":
		return c.HasSessions
	}
	return false
}

func (c *Capabilities) SetEnabled(name string, enabled bool) {
	switch name {
	case "browser":
		c.HasBrowser = enabled
	case "terminal":
		c.HasTerminal = enabled
	case "subagents":
		c.HasSubagents = enabled
	case "memory":
		c.HasMemory = enabled
	case "mcp":
		c.HasMCP = enabled
	case "audio":
		c.HasAudio = enabled
	case "filesystem":
		c.HasFilesystem = enabled
	case "sessions":
		c.HasSessions = enabled
	}
}

type CronJob struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Schedule  string     `json:"schedule"`
	Prompt    string     `json:"prompt"`
	Enabled   bool       `json:"enabled"`
	ChannelID string     `json:"channel_id"`
	CreatedAt time.Time  `json:"created_at"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`
}

const TaskTypeOneShot = "one-shot"
const TaskTypeCyclic = "cyclic"

// ParseTaskOneShotTime parses supported one-shot datetime formats.
// It accepts RFC3339 and local datetime-local strings used by browser UIs.
func ParseTaskOneShotTime(schedule string) (time.Time, bool) {
	if schedule == "" {
		return time.Time{}, false
	}
	// Canonical format.
	// JS Date.toISOString() includes milliseconds (RFC3339Nano), so try Nano first.
	if t, err := time.Parse(time.RFC3339Nano, schedule); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, schedule); err == nil {
		return t, true
	}
	// Browser datetime-local (no timezone): interpret in local time.
	layouts := []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, schedule, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func ComputeTaskType(schedule string) string {
	if schedule == "" {
		return TaskTypeOneShot
	}
	if _, ok := ParseTaskOneShotTime(schedule); ok {
		return TaskTypeOneShot
	}
	return TaskTypeCyclic
}

type Task struct {
	ID         string     `json:"id"`
	Prompt     string     `json:"prompt"`
	Schedule   string     `json:"schedule,omitempty"`
	TaskType   string     `json:"task_type"`
	Status     string     `json:"status"`
	Enabled    bool       `json:"enabled"`
	AddedAt    time.Time  `json:"added_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

func NewTask(prompt, schedule string) *Task {
	return &Task{
		ID:       uuid.New().String(),
		Prompt:   prompt,
		Schedule: schedule,
		TaskType: ComputeTaskType(schedule),
		Status:   "pending",
		Enabled:  true,
		AddedAt:  time.Now(),
	}
}

func (t *Task) MarkDone() {
	now := time.Now()
	t.Status = "done"
	t.FinishedAt = &now
}
