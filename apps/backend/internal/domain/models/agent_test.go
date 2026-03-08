package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCapabilities_IsEnabled(t *testing.T) {
	c := Capabilities{HasBrowser: true, HasTerminal: false, HasMemory: true}
	assert.True(t, c.IsEnabled("browser"))
	assert.False(t, c.IsEnabled("terminal"))
	assert.True(t, c.IsEnabled("memory"))
	assert.False(t, c.IsEnabled("subagents"))
	assert.False(t, c.IsEnabled("unknown"))
}

func TestCapabilities_SetEnabled(t *testing.T) {
	c := Capabilities{}
	c.SetEnabled("browser", true)
	assert.True(t, c.HasBrowser)
	c.SetEnabled("terminal", true)
	assert.True(t, c.HasTerminal)
	c.SetEnabled("memory", false)
	assert.False(t, c.HasMemory)
}

func TestComputeTaskType(t *testing.T) {
	assert.Equal(t, TaskTypeOneShot, ComputeTaskType(""))
	assert.Equal(t, TaskTypeOneShot, ComputeTaskType("2030-01-01T00:00:00Z"))
	assert.Equal(t, TaskTypeCyclic, ComputeTaskType("* * * * *"))
	assert.Equal(t, TaskTypeCyclic, ComputeTaskType("0 9 * * 1"))
}

func TestNewTask(t *testing.T) {
	task := NewTask("hello", "* * * * *")
	assert.NotNil(t, task)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "hello", task.Prompt)
	assert.Equal(t, "* * * * *", task.Schedule)
	assert.Equal(t, TaskTypeCyclic, task.TaskType)
	assert.Equal(t, "pending", task.Status)
	assert.True(t, task.Enabled)
	assert.False(t, task.AddedAt.IsZero())
}

func TestNewTask_OneShot(t *testing.T) {
	task := NewTask("one shot", "")
	assert.Equal(t, TaskTypeOneShot, task.TaskType)
}

func TestTask_MarkDone(t *testing.T) {
	task := NewTask("x", "")
	assert.Equal(t, "pending", task.Status)
	assert.Nil(t, task.FinishedAt)
	task.MarkDone()
	assert.Equal(t, "done", task.Status)
	assert.NotNil(t, task.FinishedAt)
	assert.True(t, task.FinishedAt.Before(time.Now().Add(time.Second)) || task.FinishedAt.After(time.Now().Add(-time.Second)))
}

func TestNewAIProviderConfig(t *testing.T) {
	cfg := NewAIProviderConfig(ProviderOpenAI, "key123", "gpt-4", 4096)
	assert.NotNil(t, cfg)
	assert.Equal(t, ProviderOpenAI, cfg.Type)
	assert.Equal(t, "key123", cfg.APIKey)
	assert.Equal(t, "gpt-4", cfg.Model)
	assert.Equal(t, 4096, cfg.MaxTokens)
}
