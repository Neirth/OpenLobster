// Copyright (c) OpenLobster contributors. See LICENSE for details.

package mappers

import (
	"testing"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Pointer helpers ─────────────────────────────────────────────────────────

func TestStrPtr(t *testing.T) {
	s := "hello"
	p := StrPtr(s)
	require.NotNil(t, p)
	assert.Equal(t, s, *p)
}

func TestBoolPtr(t *testing.T) {
	p := BoolPtr(true)
	require.NotNil(t, p)
	assert.True(t, *p)

	p2 := BoolPtr(false)
	require.NotNil(t, p2)
	assert.False(t, *p2)
}

func TestIntPtr(t *testing.T) {
	p := IntPtr(42)
	require.NotNil(t, p)
	assert.Equal(t, 42, *p)
}

// ─── SnapshotToAgent ─────────────────────────────────────────────────────────

func TestSnapshotToAgent_Nil(t *testing.T) {
	result := SnapshotToAgent(nil)
	assert.Nil(t, result)
}

func TestSnapshotToAgent_MinimalFields(t *testing.T) {
	snap := &dto.AgentSnapshot{
		ID:      "agent-1",
		Name:    "TestBot",
		Version: "1.0.0",
		Status:  "running",
		Uptime:  120,
	}
	result := SnapshotToAgent(snap)
	require.NotNil(t, result)
	assert.Equal(t, "agent-1", result.ID)
	assert.Equal(t, "TestBot", result.Name)
	assert.Equal(t, "1.0.0", result.Version)
	assert.Equal(t, "running", result.Status)
	assert.Equal(t, 120, result.Uptime)
	assert.Nil(t, result.Provider)
	assert.Nil(t, result.AiProvider)
	assert.Nil(t, result.MemoryBackend)
}

func TestSnapshotToAgent_WithOptionalFields(t *testing.T) {
	snap := &dto.AgentSnapshot{
		ID:            "agent-2",
		Name:          "Bot",
		Provider:      "telegram",
		AIProvider:    "anthropic",
		MemoryBackend: "neo4j",
	}
	result := SnapshotToAgent(snap)
	require.NotNil(t, result)
	require.NotNil(t, result.Provider)
	assert.Equal(t, "telegram", *result.Provider)
	require.NotNil(t, result.AiProvider)
	assert.Equal(t, "anthropic", *result.AiProvider)
	require.NotNil(t, result.MemoryBackend)
	assert.Equal(t, "neo4j", *result.MemoryBackend)
}

func TestSnapshotToAgent_WithChannels(t *testing.T) {
	snap := &dto.AgentSnapshot{
		ID:   "agent-3",
		Name: "Bot",
		Channels: []dto.ChannelStatus{
			{ID: "ch1", Name: "telegram", Type: "telegram", Status: "online", Enabled: true},
		},
	}
	result := SnapshotToAgent(snap)
	require.NotNil(t, result)
	require.Len(t, result.Channels, 1)
	assert.Equal(t, "ch1", result.Channels[0].ID)
}

// ─── ChannelsToGenerated ─────────────────────────────────────────────────────

func TestChannelsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, ChannelsToGenerated(nil))
	assert.Nil(t, ChannelsToGenerated([]dto.ChannelStatus{}))
}

func TestChannelsToGenerated_Multiple(t *testing.T) {
	list := []dto.ChannelStatus{
		{ID: "c1", Name: "telegram", Type: "telegram", Status: "online", Enabled: true,
			Capabilities: dto.ChannelCapabilities{HasVoiceMessage: true, HasTextStream: true}},
		{ID: "c2", Name: "discord", Type: "discord", Status: "offline", Enabled: false,
			Capabilities: dto.ChannelCapabilities{HasCallStream: true, HasMediaSupport: true}},
	}
	result := ChannelsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "c1", result[0].ID)
	assert.True(t, result[0].Enabled)
	assert.True(t, result[0].Capabilities.HasVoiceMessage)
	assert.Equal(t, "c2", result[1].ID)
	assert.False(t, result[1].Enabled)
	assert.True(t, result[1].Capabilities.HasCallStream)
}

// ─── ChannelToGenerated ──────────────────────────────────────────────────────

func TestChannelToGenerated(t *testing.T) {
	c := dto.ChannelStatus{
		ID:      "ch1",
		Name:    "Telegram",
		Type:    "telegram",
		Status:  "online",
		Enabled: true,
		Capabilities: dto.ChannelCapabilities{
			HasVoiceMessage: true,
			HasCallStream:   false,
			HasTextStream:   true,
			HasMediaSupport: true,
		},
	}
	result := ChannelToGenerated(c)
	require.NotNil(t, result)
	assert.Equal(t, "ch1", result.ID)
	assert.Equal(t, "Telegram", result.Name)
	assert.Equal(t, "telegram", result.Type)
	assert.Equal(t, "online", result.Status)
	assert.True(t, result.Enabled)
	assert.True(t, result.Capabilities.HasVoiceMessage)
	assert.False(t, result.Capabilities.HasCallStream)
	assert.True(t, result.Capabilities.HasTextStream)
	assert.True(t, result.Capabilities.HasMediaSupport)
}

// ─── HeartbeatToGenerated ────────────────────────────────────────────────────

func TestHeartbeatToGenerated_Nil(t *testing.T) {
	assert.Nil(t, HeartbeatToGenerated(nil))
}

func TestHeartbeatToGenerated(t *testing.T) {
	h := &dto.HeartbeatSnapshot{Status: "healthy", LastCheck: 1700000000}
	result := HeartbeatToGenerated(h)
	require.NotNil(t, result)
	assert.Equal(t, "healthy", result.Status)
	assert.Equal(t, int(1700000000), result.LastCheck)
}

// ─── ToolsToGenerated ────────────────────────────────────────────────────────

func TestToolsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, ToolsToGenerated(nil))
	assert.Nil(t, ToolsToGenerated([]dto.ToolSnapshot{}))
}

func TestToolsToGenerated_WithOptionalFields(t *testing.T) {
	list := []dto.ToolSnapshot{
		{Name: "search_web", Description: "Search the web", Source: "mcp"},
		{Name: "no_desc", Description: "", Source: ""},
	}
	result := ToolsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "search_web", result[0].Name)
	require.NotNil(t, result[0].Description)
	assert.Equal(t, "Search the web", *result[0].Description)
	require.NotNil(t, result[0].Source)
	assert.Equal(t, "mcp", *result[0].Source)
	assert.Equal(t, "no_desc", result[1].Name)
	assert.Nil(t, result[1].Description)
	assert.Nil(t, result[1].Source)
}

// ─── SubAgentsToGenerated ────────────────────────────────────────────────────

func TestSubAgentsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, SubAgentsToGenerated(nil))
	assert.Nil(t, SubAgentsToGenerated([]dto.SubAgentSnapshot{}))
}

func TestSubAgentsToGenerated(t *testing.T) {
	list := []dto.SubAgentSnapshot{
		{ID: "sa1", Name: "worker", Status: "running", Task: "summarize"},
		{ID: "sa2", Name: "idle", Status: "idle", Task: ""},
	}
	result := SubAgentsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "sa1", result[0].ID)
	require.NotNil(t, result[0].Task)
	assert.Equal(t, "summarize", *result[0].Task)
	assert.Equal(t, "sa2", result[1].ID)
	assert.Nil(t, result[1].Task)
}

// ─── StatusToGenerated ───────────────────────────────────────────────────────

func TestStatusToGenerated_Nil(t *testing.T) {
	assert.Nil(t, StatusToGenerated(nil))
}

func TestStatusToGenerated_Full(t *testing.T) {
	s := &dto.StatusSnapshot{
		Agent:  &dto.AgentSnapshot{ID: "a1", Name: "Bot"},
		Health: &dto.HeartbeatSnapshot{Status: "ok", LastCheck: 100},
		Channels: []dto.ChannelStatus{
			{ID: "c1", Name: "tg", Type: "telegram", Enabled: true},
		},
		Tools: []dto.ToolSnapshot{
			{Name: "search"},
		},
		SubAgents: []dto.SubAgentSnapshot{
			{ID: "sa1", Name: "worker", Status: "running"},
		},
		Tasks: []dto.TaskSnapshot{
			{ID: "t1", Prompt: "do something", Status: "pending", Enabled: true},
		},
		Mcps: []dto.MCPSnapshot{
			{Name: "server1"},
		},
	}
	result := StatusToGenerated(s)
	require.NotNil(t, result)
	assert.NotNil(t, result.Agent)
	assert.NotNil(t, result.Health)
	assert.Len(t, result.Channels, 1)
	assert.Len(t, result.Tools, 1)
	assert.Len(t, result.SubAgents, 1)
	assert.Len(t, result.Tasks, 1)
	assert.Len(t, result.Mcps, 1)
}

// ─── TaskSnapshotToGenerated ─────────────────────────────────────────────────

func TestTaskSnapshotToGenerated_MinimalFields(t *testing.T) {
	ts := dto.TaskSnapshot{
		ID:      "t1",
		Prompt:  "do something",
		Status:  "pending",
		Enabled: true,
	}
	result := TaskSnapshotToGenerated(ts)
	require.NotNil(t, result)
	assert.Equal(t, "t1", result.ID)
	assert.Equal(t, "do something", result.Prompt)
	assert.Equal(t, "pending", result.Status)
	assert.True(t, result.Enabled)
	assert.Nil(t, result.Schedule)
	assert.Nil(t, result.TaskType)
}

func TestTaskSnapshotToGenerated_WithOptionalFields(t *testing.T) {
	ts := dto.TaskSnapshot{
		ID:       "t2",
		Prompt:   "run daily",
		Status:   "running",
		Enabled:  true,
		Schedule: "0 9 * * *",
		TaskType: "cyclic",
	}
	result := TaskSnapshotToGenerated(ts)
	require.NotNil(t, result.Schedule)
	assert.Equal(t, "0 9 * * *", *result.Schedule)
	require.NotNil(t, result.TaskType)
	assert.Equal(t, "cyclic", *result.TaskType)
}

// ─── TaskSnapshotToGeneratedFull ─────────────────────────────────────────────

func TestTaskSnapshotToGeneratedFull_WithCyclicAndDates(t *testing.T) {
	ts := dto.TaskSnapshot{
		ID:        "t3",
		Prompt:    "run weekly",
		Status:    "pending",
		Enabled:   true,
		Schedule:  "0 9 * * 1",
		TaskType:  "cyclic",
		IsCyclic:  true,
		CreatedAt: "2024-01-01T00:00:00Z",
		LastRunAt: "2024-01-08T09:00:00Z",
		NextRunAt: "2024-01-15T09:00:00Z",
	}
	result := TaskSnapshotToGeneratedFull(ts)
	require.NotNil(t, result)
	require.NotNil(t, result.IsCyclic)
	assert.True(t, *result.IsCyclic)
	require.NotNil(t, result.CreatedAt)
	assert.Equal(t, "2024-01-01T00:00:00Z", *result.CreatedAt)
	require.NotNil(t, result.LastRunAt)
	assert.Equal(t, "2024-01-08T09:00:00Z", *result.LastRunAt)
	require.NotNil(t, result.NextRunAt)
	assert.Equal(t, "2024-01-15T09:00:00Z", *result.NextRunAt)
}

func TestTaskSnapshotToGeneratedFull_NotCyclic(t *testing.T) {
	ts := dto.TaskSnapshot{
		ID:     "t4",
		Prompt: "once",
		Status: "done",
	}
	result := TaskSnapshotToGeneratedFull(ts)
	assert.Nil(t, result.IsCyclic)
	assert.Nil(t, result.CreatedAt)
	assert.Nil(t, result.LastRunAt)
	assert.Nil(t, result.NextRunAt)
}

// ─── TasksToGenerated ────────────────────────────────────────────────────────

func TestTasksToGenerated_Empty(t *testing.T) {
	assert.Nil(t, TasksToGenerated(nil))
	assert.Nil(t, TasksToGenerated([]dto.TaskSnapshot{}))
}

func TestTasksToGenerated_Multiple(t *testing.T) {
	list := []dto.TaskSnapshot{
		{ID: "t1", Prompt: "a", Status: "pending", Enabled: true},
		{ID: "t2", Prompt: "b", Status: "done", Enabled: false},
	}
	result := TasksToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "t1", result[0].ID)
	assert.Equal(t, "t2", result[1].ID)
}

// ─── MetricsToGenerated ──────────────────────────────────────────────────────

func TestMetricsToGenerated_Nil(t *testing.T) {
	assert.Nil(t, MetricsToGenerated(nil))
}

func TestMetricsToGenerated(t *testing.T) {
	m := &dto.MetricsSnapshot{
		Uptime:           3600,
		MessagesReceived: 100,
		MessagesSent:     200,
		ActiveSessions:   5,
		MemoryNodes:      50,
		MemoryEdges:      30,
		McpTools:         10,
		TasksPending:     2,
		TasksRunning:     1,
		TasksDone:        8,
		ErrorsTotal:      3,
	}
	result := MetricsToGenerated(m)
	require.NotNil(t, result)
	assert.Equal(t, 3600, result.Uptime)
	assert.Equal(t, 100, result.MessagesReceived)
	assert.Equal(t, 200, result.MessagesSent)
	assert.Equal(t, 5, result.ActiveSessions)
	assert.Equal(t, 50, result.MemoryNodes)
	assert.Equal(t, 30, result.MemoryEdges)
	assert.Equal(t, 10, result.McpTools)
	assert.Equal(t, 2, result.TasksPending)
	assert.Equal(t, 1, result.TasksRunning)
	assert.Equal(t, 8, result.TasksDone)
	assert.Equal(t, 3, result.ErrorsTotal)
}

// ─── ConversationsToGenerated ────────────────────────────────────────────────

func TestConversationsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, ConversationsToGenerated(nil))
	assert.Nil(t, ConversationsToGenerated([]dto.ConversationSnapshot{}))
}

func TestConversationsToGenerated_Multiple(t *testing.T) {
	list := []dto.ConversationSnapshot{
		{ID: "cv1", ChannelID: "ch1", IsGroup: false, ParticipantName: "Alice", UnreadCount: 3},
		{ID: "cv2", ChannelID: "ch2", IsGroup: true, GroupName: "Team"},
	}
	result := ConversationsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "cv1", result[0].ID)
	assert.False(t, result[0].IsGroup)
	require.NotNil(t, result[0].ParticipantName)
	assert.Equal(t, "Alice", *result[0].ParticipantName)
	require.NotNil(t, result[0].UnreadCount)
	assert.Equal(t, 3, *result[0].UnreadCount)

	assert.Equal(t, "cv2", result[1].ID)
	assert.True(t, result[1].IsGroup)
	require.NotNil(t, result[1].GroupName)
	assert.Equal(t, "Team", *result[1].GroupName)
}

// ─── ConversationSnapshotToGenerated ─────────────────────────────────────────

func TestConversationSnapshotToGenerated_AllOptional(t *testing.T) {
	c := dto.ConversationSnapshot{
		ID:              "cv3",
		ChannelID:       "ch3",
		ChannelName:     "My Channel",
		GroupName:       "My Group",
		IsGroup:         true,
		ParticipantID:   "pid1",
		ParticipantName: "Bob",
		LastMessageAt:   "2024-01-01T00:00:00Z",
		UnreadCount:     7,
	}
	result := ConversationSnapshotToGenerated(c)
	require.NotNil(t, result.ChannelName)
	assert.Equal(t, "My Channel", *result.ChannelName)
	require.NotNil(t, result.GroupName)
	assert.Equal(t, "My Group", *result.GroupName)
	require.NotNil(t, result.ParticipantID)
	assert.Equal(t, "pid1", *result.ParticipantID)
	require.NotNil(t, result.ParticipantName)
	assert.Equal(t, "Bob", *result.ParticipantName)
	require.NotNil(t, result.LastMessageAt)
	assert.Equal(t, "2024-01-01T00:00:00Z", *result.LastMessageAt)
	require.NotNil(t, result.UnreadCount)
	assert.Equal(t, 7, *result.UnreadCount)
}

func TestConversationSnapshotToGenerated_EmptyOptionalFields(t *testing.T) {
	c := dto.ConversationSnapshot{
		ID:        "cv4",
		ChannelID: "ch4",
	}
	result := ConversationSnapshotToGenerated(c)
	assert.Nil(t, result.ChannelName)
	assert.Nil(t, result.GroupName)
	assert.Nil(t, result.ParticipantID)
	assert.Nil(t, result.ParticipantName)
	assert.Nil(t, result.LastMessageAt)
	assert.Nil(t, result.UnreadCount)
}

// ─── SendMessageResultToGenerated ────────────────────────────────────────────

func TestSendMessageResultToGenerated_Nil(t *testing.T) {
	assert.Nil(t, SendMessageResultToGenerated(nil))
}

func TestSendMessageResultToGenerated_Full(t *testing.T) {
	r := &dto.SendMessageResult{
		ID:             "msg-1",
		ConversationID: "conv-1",
		Role:           "assistant",
		Content:        "Hello!",
		CreatedAt:      "2024-01-01T00:00:00Z",
	}
	result := SendMessageResultToGenerated(r)
	require.NotNil(t, result)
	require.NotNil(t, result.Success)
	assert.True(t, *result.Success)
	require.NotNil(t, result.ID)
	assert.Equal(t, "msg-1", *result.ID)
	require.NotNil(t, result.ConversationID)
	assert.Equal(t, "conv-1", *result.ConversationID)
	require.NotNil(t, result.Role)
	assert.Equal(t, "assistant", *result.Role)
	require.NotNil(t, result.Content)
	assert.Equal(t, "Hello!", *result.Content)
	require.NotNil(t, result.CreatedAt)
	assert.Equal(t, "2024-01-01T00:00:00Z", *result.CreatedAt)
}

func TestSendMessageResultToGenerated_EmptyFields(t *testing.T) {
	r := &dto.SendMessageResult{}
	result := SendMessageResultToGenerated(r)
	require.NotNil(t, result)
	require.NotNil(t, result.Success)
	assert.True(t, *result.Success)
	assert.Nil(t, result.ID)
	assert.Nil(t, result.ConversationID)
	assert.Nil(t, result.Role)
	assert.Nil(t, result.Content)
	assert.Nil(t, result.CreatedAt)
}

// ─── MCPsToGenerated ─────────────────────────────────────────────────────────

func TestMCPsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, MCPsToGenerated(nil))
	assert.Nil(t, MCPsToGenerated([]dto.MCPSnapshot{}))
}

func TestMCPsToGenerated_WithOptionalFields(t *testing.T) {
	list := []dto.MCPSnapshot{
		{Name: "server1", Type: "sse", Status: "online", URL: "http://localhost:8080",
			Tools: []dto.ToolSnapshot{{Name: "tool1"}}},
		{Name: "server2", Type: "", Status: "", URL: ""},
	}
	result := MCPsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "server1", result[0].Name)
	require.NotNil(t, result[0].Type)
	assert.Equal(t, "sse", *result[0].Type)
	require.NotNil(t, result[0].Status)
	assert.Equal(t, "online", *result[0].Status)
	require.NotNil(t, result[0].URL)
	assert.Equal(t, "http://localhost:8080", *result[0].URL)
	require.Len(t, result[0].Tools, 1)

	assert.Equal(t, "server2", result[1].Name)
	assert.Nil(t, result[1].Type)
	assert.Nil(t, result[1].Status)
	assert.Nil(t, result[1].URL)
}

// ─── AppConfigSnapshotToGenerated ────────────────────────────────────────────

func TestAppConfigSnapshotToGenerated_Nil(t *testing.T) {
	result := AppConfigSnapshotToGenerated(nil)
	require.NotNil(t, result)
}

func TestAppConfigSnapshotToGenerated_EmptySnapshot(t *testing.T) {
	result := AppConfigSnapshotToGenerated(&dto.AppConfigSnapshot{})
	require.NotNil(t, result)
	assert.Nil(t, result.Agent)
	assert.Nil(t, result.Capabilities)
	assert.Nil(t, result.Database)
	assert.Nil(t, result.Memory)
}

func TestAppConfigSnapshotToGenerated_WithAgent(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Agent: &dto.AgentConfigSnapshot{
			Name:                      "Bot",
			SystemPrompt:              "Be helpful.",
			Provider:                  "openai",
			Model:                     "gpt-4o",
			APIKey:                    "sk-openai",
			BaseURL:                   "https://api.openai.com",
			OllamaHost:                "http://localhost:11434",
			OllamaApiKey:              "ollama-key",
			AnthropicApiKey:           "sk-ant",
			DockerModelRunnerEndpoint: "http://dmr:12434",
			DockerModelRunnerModel:    "ai/mistral",
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Agent)
	require.NotNil(t, result.Agent.Name)
	assert.Equal(t, "Bot", *result.Agent.Name)
	require.NotNil(t, result.Agent.Provider)
	assert.Equal(t, "openai", *result.Agent.Provider)
	require.NotNil(t, result.Agent.APIKey)
	assert.Equal(t, "sk-openai", *result.Agent.APIKey)
}

func TestAppConfigSnapshotToGenerated_WithCapabilities(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Capabilities: &dto.CapabilitiesSnapshot{
			Browser:    true,
			Terminal:   true,
			Subagents:  false,
			Memory:     true,
			MCP:        false,
			Filesystem: true,
			Sessions:   false,
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Capabilities)
	require.NotNil(t, result.Capabilities.Browser)
	assert.True(t, *result.Capabilities.Browser)
	require.NotNil(t, result.Capabilities.Terminal)
	assert.True(t, *result.Capabilities.Terminal)
	require.NotNil(t, result.Capabilities.Subagents)
	assert.False(t, *result.Capabilities.Subagents)
}

func TestAppConfigSnapshotToGenerated_WithDatabase(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Database: &dto.DatabaseConfigSnapshot{
			Driver:       "sqlite",
			DSN:          "./data.db",
			MaxOpenConns: 10,
			MaxIdleConns: 2,
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Database)
	require.NotNil(t, result.Database.Driver)
	assert.Equal(t, "sqlite", *result.Database.Driver)
	require.NotNil(t, result.Database.Dsn)
	assert.Equal(t, "./data.db", *result.Database.Dsn)
	require.NotNil(t, result.Database.MaxOpenConns)
	assert.Equal(t, 10, *result.Database.MaxOpenConns)
	require.NotNil(t, result.Database.MaxIdleConns)
	assert.Equal(t, 2, *result.Database.MaxIdleConns)
}

func TestAppConfigSnapshotToGenerated_DatabaseZeroConns(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Database: &dto.DatabaseConfigSnapshot{
			Driver:       "sqlite",
			DSN:          "./data.db",
			MaxOpenConns: 0,
			MaxIdleConns: 0,
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Database)
	// Zero values produce nil for optional int fields.
	assert.Nil(t, result.Database.MaxOpenConns)
	assert.Nil(t, result.Database.MaxIdleConns)
}

func TestAppConfigSnapshotToGenerated_WithMemoryNeo4j(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Memory: &dto.MemoryConfigSnapshot{
			Backend:  "neo4j",
			FilePath: "./mem.gml",
			Neo4j: &dto.Neo4jConfigSnapshot{
				URI:      "bolt://localhost:7687",
				User:     "neo4j",
				Password: "pass",
			},
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Memory)
	require.NotNil(t, result.Memory.Backend)
	assert.Equal(t, "neo4j", *result.Memory.Backend)
	require.NotNil(t, result.Memory.Neo4j)
	require.NotNil(t, result.Memory.Neo4j.URI)
	assert.Equal(t, "bolt://localhost:7687", *result.Memory.Neo4j.URI)
}

func TestAppConfigSnapshotToGenerated_WithMemoryNoNeo4j(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Memory: &dto.MemoryConfigSnapshot{
			Backend: "file",
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Memory)
	assert.Nil(t, result.Memory.Neo4j)
}

func TestAppConfigSnapshotToGenerated_WithSubagents(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Subagents: &dto.SubagentsConfigSnapshot{
			MaxConcurrent:  4,
			DefaultTimeout: "30s",
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Subagents)
	require.NotNil(t, result.Subagents.MaxConcurrent)
	assert.Equal(t, 4, *result.Subagents.MaxConcurrent)
	require.NotNil(t, result.Subagents.DefaultTimeout)
	assert.Equal(t, "30s", *result.Subagents.DefaultTimeout)
}

func TestAppConfigSnapshotToGenerated_WithGraphQL(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		GraphQL: &dto.GraphQLConfigSnapshot{
			Enabled: true,
			Port:    8080,
			Host:    "0.0.0.0",
			BaseURL: "https://app.example.com",
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Graphql)
	require.NotNil(t, result.Graphql.Enabled)
	assert.True(t, *result.Graphql.Enabled)
	require.NotNil(t, result.Graphql.Port)
	assert.Equal(t, 8080, *result.Graphql.Port)
	require.NotNil(t, result.Graphql.Host)
	assert.Equal(t, "0.0.0.0", *result.Graphql.Host)
	require.NotNil(t, result.Graphql.BaseURL)
	assert.Equal(t, "https://app.example.com", *result.Graphql.BaseURL)
}

func TestAppConfigSnapshotToGenerated_WithLogging(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Logging: &dto.LoggingConfigSnapshot{
			Level: "debug",
			Path:  "./app.log",
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Logging)
	require.NotNil(t, result.Logging.Level)
	assert.Equal(t, "debug", *result.Logging.Level)
	require.NotNil(t, result.Logging.Path)
	assert.Equal(t, "./app.log", *result.Logging.Path)
}

func TestAppConfigSnapshotToGenerated_WithScheduler(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Scheduler: &dto.SchedulerConfigSnapshot{
			Enabled:        true,
			MemoryEnabled:  true,
			MemoryInterval: "5m",
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Scheduler)
	require.NotNil(t, result.Scheduler.Enabled)
	assert.True(t, *result.Scheduler.Enabled)
	require.NotNil(t, result.Scheduler.MemoryEnabled)
	assert.True(t, *result.Scheduler.MemoryEnabled)
	require.NotNil(t, result.Scheduler.MemoryInterval)
	assert.Equal(t, "5m", *result.Scheduler.MemoryInterval)
}

func TestAppConfigSnapshotToGenerated_WithSecrets(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Secrets: &dto.SecretsConfigSnapshot{
			Backend: "openbao",
			File:    &dto.FileSecretsSnapshot{Path: "./secrets.json"},
			Openbao: &dto.OpenbaoSecretsSnapshot{
				URL:   "https://vault.example.com",
				Token: "hvs.token",
			},
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Secrets)
	require.NotNil(t, result.Secrets.Backend)
	assert.Equal(t, "openbao", *result.Secrets.Backend)
	require.NotNil(t, result.Secrets.File)
	require.NotNil(t, result.Secrets.Openbao)
	assert.Equal(t, "https://vault.example.com", *result.Secrets.Openbao.URL)
}

func TestAppConfigSnapshotToGenerated_WithSecretsNoFile(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Secrets: &dto.SecretsConfigSnapshot{
			Backend: "openbao",
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.Secrets)
	assert.Nil(t, result.Secrets.File)
	assert.Nil(t, result.Secrets.Openbao)
}

func TestAppConfigSnapshotToGenerated_WithActiveSessions(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		ActiveSessions: []dto.ActiveSessionSnapshot{
			{ID: "sess1", Address: "1.2.3.4", Status: "active", Channel: "telegram", User: "alice"},
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.Len(t, result.ActiveSessions, 1)
	assert.Equal(t, "sess1", result.ActiveSessions[0].ID)
	require.NotNil(t, result.ActiveSessions[0].Address)
	assert.Equal(t, "1.2.3.4", *result.ActiveSessions[0].Address)
}

func TestAppConfigSnapshotToGenerated_ActiveSessionEmptyFields(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		ActiveSessions: []dto.ActiveSessionSnapshot{
			{ID: "sess2"},
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.Len(t, result.ActiveSessions, 1)
	assert.Nil(t, result.ActiveSessions[0].Address)
	assert.Nil(t, result.ActiveSessions[0].Status)
	assert.Nil(t, result.ActiveSessions[0].Channel)
	assert.Nil(t, result.ActiveSessions[0].User)
}

func TestAppConfigSnapshotToGenerated_WithChannels(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Channels: []dto.ChannelConfigSnapshot{
			{ChannelID: "ch1", ChannelName: "Telegram", Enabled: true},
			{ChannelID: "ch2", Enabled: false},
		},
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.Len(t, result.Channels, 2)
	require.NotNil(t, result.Channels[0].ChannelName)
	assert.Equal(t, "Telegram", *result.Channels[0].ChannelName)
	assert.Nil(t, result.Channels[1].ChannelName)
}

func TestAppConfigSnapshotToGenerated_WithChannelSecrets(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		ChannelSecrets: &dto.ChannelSecretsSnapshot{
			TelegramEnabled:  true,
			TelegramToken:    "tg-token",
			DiscordEnabled:   true,
			DiscordToken:     "dc-token",
			WhatsAppEnabled:  true,
			WhatsAppPhoneId:  "+34600000000",
			WhatsAppApiToken: "wa-token",
			TwilioEnabled:    true,
			TwilioAccountSid: "AC123",
			TwilioAuthToken:  "tw-token",
			TwilioFromNumber: "+15550000000",
			SlackEnabled:     true,
			SlackBotToken:    "xoxb-bot",
			SlackAppToken:    "xapp-app",
		},
		WizardCompleted: true,
	}
	result := AppConfigSnapshotToGenerated(snap)
	require.NotNil(t, result.ChannelSecrets)
	require.NotNil(t, result.ChannelSecrets.TelegramEnabled)
	assert.True(t, *result.ChannelSecrets.TelegramEnabled)
	require.NotNil(t, result.ChannelSecrets.TelegramToken)
	assert.Equal(t, "tg-token", *result.ChannelSecrets.TelegramToken)
	require.NotNil(t, result.ChannelSecrets.DiscordEnabled)
	assert.True(t, *result.ChannelSecrets.DiscordEnabled)
	require.NotNil(t, result.ChannelSecrets.SlackEnabled)
	assert.True(t, *result.ChannelSecrets.SlackEnabled)
	require.NotNil(t, result.WizardCompleted)
	assert.True(t, *result.WizardCompleted)
}

// ─── AppConfigSnapshotToUpdateConfigResult ───────────────────────────────────

func TestAppConfigSnapshotToUpdateConfigResult_Nil(t *testing.T) {
	result := AppConfigSnapshotToUpdateConfigResult(nil)
	require.NotNil(t, result)
	assert.Nil(t, result.AgentName)
}

func TestAppConfigSnapshotToUpdateConfigResult_EmptySnapshot(t *testing.T) {
	result := AppConfigSnapshotToUpdateConfigResult(&dto.AppConfigSnapshot{})
	require.NotNil(t, result)
	assert.Nil(t, result.AgentName)
}

func TestAppConfigSnapshotToUpdateConfigResult_WithAgent(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Agent: &dto.AgentConfigSnapshot{
			Name:         "NewBot",
			SystemPrompt: "Be concise.",
			Provider:     "anthropic",
		},
	}
	result := AppConfigSnapshotToUpdateConfigResult(snap)
	require.NotNil(t, result.AgentName)
	assert.Equal(t, "NewBot", *result.AgentName)
	require.NotNil(t, result.SystemPrompt)
	assert.Equal(t, "Be concise.", *result.SystemPrompt)
	require.NotNil(t, result.Provider)
	assert.Equal(t, "anthropic", *result.Provider)
}

func TestAppConfigSnapshotToUpdateConfigResult_EmptyAgentFields(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Agent: &dto.AgentConfigSnapshot{},
	}
	result := AppConfigSnapshotToUpdateConfigResult(snap)
	assert.Nil(t, result.AgentName)
	assert.Nil(t, result.SystemPrompt)
	assert.Nil(t, result.Provider)
}

func TestAppConfigSnapshotToUpdateConfigResult_WithChannels(t *testing.T) {
	snap := &dto.AppConfigSnapshot{
		Channels: []dto.ChannelConfigSnapshot{
			{ChannelID: "ch1", ChannelName: "Telegram", Enabled: true},
		},
	}
	result := AppConfigSnapshotToUpdateConfigResult(snap)
	require.Len(t, result.Channels, 1)
	require.NotNil(t, result.Channels[0].ChannelName)
	assert.Equal(t, "Telegram", *result.Channels[0].ChannelName)
}

// ─── UpdateConfigInputToMap ──────────────────────────────────────────────────

func TestUpdateConfigInputToMap_Empty(t *testing.T) {
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{})
	assert.Empty(t, result)
}

func TestUpdateConfigInputToMap_AgentFields(t *testing.T) {
	name := "NewBot"
	sp := "Be helpful."
	prov := "anthropic"
	model := "claude-sonnet-4-6"
	apiKey := "sk-ant"
	baseURL := "https://api.anthropic.com"
	ollamaHost := "http://localhost:11434"
	ollamaKey := "ollama-key"
	anthropicKey := "sk-ant-key"
	dmrEndpoint := "http://dmr:12434"
	dmrModel := "ai/mistral"

	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		AgentName:                 &name,
		SystemPrompt:              &sp,
		Provider:                  &prov,
		Model:                     &model,
		APIKey:                    &apiKey,
		BaseURL:                   &baseURL,
		OllamaHost:                &ollamaHost,
		OllamaAPIKey:              &ollamaKey,
		AnthropicAPIKey:           &anthropicKey,
		DockerModelRunnerEndpoint: &dmrEndpoint,
		DockerModelRunnerModel:    &dmrModel,
	})

	assert.Equal(t, "NewBot", result["agentName"])
	assert.Equal(t, "Be helpful.", result["systemPrompt"])
	assert.Equal(t, "anthropic", result["provider"])
	assert.Equal(t, "claude-sonnet-4-6", result["model"])
	assert.Equal(t, "sk-ant", result["apiKey"])
	assert.Equal(t, "https://api.anthropic.com", result["baseURL"])
	assert.Equal(t, "http://localhost:11434", result["ollamaHost"])
	assert.Equal(t, "ollama-key", result["ollamaApiKey"])
	assert.Equal(t, "sk-ant-key", result["anthropicApiKey"])
	assert.Equal(t, "http://dmr:12434", result["dockerModelRunnerEndpoint"])
	assert.Equal(t, "ai/mistral", result["dockerModelRunnerModel"])
}

func TestUpdateConfigInputToMap_WithCapabilities(t *testing.T) {
	tr := true
	fa := false
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		Capabilities: &generated.CapabilitiesInput{
			Browser:    &tr,
			Terminal:   &tr,
			Subagents:  &fa,
			Memory:     &tr,
			Mcp:        &fa,
			Filesystem: &tr,
			Sessions:   &fa,
		},
	})

	caps, ok := result["capabilities"].(map[string]interface{})
	require.True(t, ok)
	assert.True(t, caps["browser"].(bool))
	assert.True(t, caps["terminal"].(bool))
	assert.False(t, caps["subagents"].(bool))
	assert.True(t, caps["memory"].(bool))
	assert.False(t, caps["mcp"].(bool))
	assert.True(t, caps["filesystem"].(bool))
	assert.False(t, caps["sessions"].(bool))
}

func TestUpdateConfigInputToMap_DatabaseFields(t *testing.T) {
	driver := "postgres"
	dsn := "postgres://localhost/db"
	maxOpen := 20
	maxIdle := 5
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		DatabaseDriver:       &driver,
		DatabaseDsn:          &dsn,
		DatabaseMaxOpenConns: &maxOpen,
		DatabaseMaxIdleConns: &maxIdle,
	})

	assert.Equal(t, "postgres", result["databaseDriver"])
	assert.Equal(t, "postgres://localhost/db", result["databaseDSN"])
	assert.Equal(t, 20, result["databaseMaxOpenConns"])
	assert.Equal(t, 5, result["databaseMaxIdleConns"])
}

func TestUpdateConfigInputToMap_MemoryFields(t *testing.T) {
	backend := "neo4j"
	filePath := "./data/mem.gml"
	neo4jURI := "bolt://localhost:7687"
	neo4jUser := "neo4j"
	neo4jPwd := "pass"
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		MemoryBackend:       &backend,
		MemoryFilePath:      &filePath,
		MemoryNeo4jURI:      &neo4jURI,
		MemoryNeo4jUser:     &neo4jUser,
		MemoryNeo4jPassword: &neo4jPwd,
	})

	assert.Equal(t, "neo4j", result["memoryBackend"])
	assert.Equal(t, "./data/mem.gml", result["memoryFilePath"])
	assert.Equal(t, "bolt://localhost:7687", result["memoryNeo4jURI"])
	assert.Equal(t, "neo4j", result["memoryNeo4jUser"])
	assert.Equal(t, "pass", result["memoryNeo4jPassword"])
}

func TestUpdateConfigInputToMap_SubagentsFields(t *testing.T) {
	maxC := 3
	timeout := "30s"
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		SubagentsMaxConcurrent:  &maxC,
		SubagentsDefaultTimeout: &timeout,
	})
	assert.Equal(t, 3, result["subagentsMaxConcurrent"])
	assert.Equal(t, "30s", result["subagentsDefaultTimeout"])
}

func TestUpdateConfigInputToMap_GraphQLFields(t *testing.T) {
	enabled := true
	port := 9090
	host := "0.0.0.0"
	baseURL := "https://myapp.example.com"
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		GraphqlEnabled: &enabled,
		GraphqlPort:    &port,
		GraphqlHost:    &host,
		GraphqlBaseURL: &baseURL,
	})
	assert.True(t, result["graphqlEnabled"].(bool))
	assert.Equal(t, 9090, result["graphqlPort"])
	assert.Equal(t, "0.0.0.0", result["graphqlHost"])
	assert.Equal(t, "https://myapp.example.com", result["graphqlBaseUrl"])
}

func TestUpdateConfigInputToMap_LoggingFields(t *testing.T) {
	level := "debug"
	path := "./logs/app.log"
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		LoggingLevel: &level,
		LoggingPath:  &path,
	})
	assert.Equal(t, "debug", result["loggingLevel"])
	assert.Equal(t, "./logs/app.log", result["loggingPath"])
}

func TestUpdateConfigInputToMap_SecretsFields(t *testing.T) {
	backend := "openbao"
	filePath := "./secrets.json"
	url := "https://vault.example.com"
	token := "hvs.token"
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		SecretsBackend:      &backend,
		SecretsFilePath:     &filePath,
		SecretsOpenbaoURL:   &url,
		SecretsOpenbaoToken: &token,
	})
	assert.Equal(t, "openbao", result["secretsBackend"])
	assert.Equal(t, "./secrets.json", result["secretsFilePath"])
	assert.Equal(t, "https://vault.example.com", result["secretsOpenbaoURL"])
	assert.Equal(t, "hvs.token", result["secretsOpenbaoToken"])
}

func TestUpdateConfigInputToMap_SchedulerFields(t *testing.T) {
	enabled := true
	memEnabled := true
	interval := "5m"
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		SchedulerEnabled:        &enabled,
		SchedulerMemoryEnabled:  &memEnabled,
		SchedulerMemoryInterval: &interval,
	})
	assert.True(t, result["schedulerEnabled"].(bool))
	assert.True(t, result["schedulerMemoryEnabled"].(bool))
	assert.Equal(t, "5m", result["schedulerMemoryInterval"])
}

func TestUpdateConfigInputToMap_ChannelFields(t *testing.T) {
	tgEnabled := true
	tgToken := "tg-bot-token"
	dcEnabled := true
	dcToken := "dc-bot-token"
	waEnabled := true
	waPhoneID := "+34600000000"
	waToken := "wa-token"
	twEnabled := true
	twSid := "AC123"
	twAuth := "tw-token"
	twFrom := "+15550000000"
	slEnabled := true
	slBot := "xoxb-bot"
	slApp := "xapp-app"
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		ChannelTelegramEnabled:  &tgEnabled,
		ChannelTelegramToken:    &tgToken,
		ChannelDiscordEnabled:   &dcEnabled,
		ChannelDiscordToken:     &dcToken,
		ChannelWhatsAppEnabled:  &waEnabled,
		ChannelWhatsAppPhoneID:  &waPhoneID,
		ChannelWhatsAppAPIToken: &waToken,
		ChannelTwilioEnabled:    &twEnabled,
		ChannelTwilioAccountSid: &twSid,
		ChannelTwilioAuthToken:  &twAuth,
		ChannelTwilioFromNumber: &twFrom,
		ChannelSlackEnabled:     &slEnabled,
		ChannelSlackBotToken:    &slBot,
		ChannelSlackAppToken:    &slApp,
	})
	assert.True(t, result["channelTelegramEnabled"].(bool))
	assert.Equal(t, "tg-bot-token", result["channelTelegramToken"])
	assert.True(t, result["channelDiscordEnabled"].(bool))
	assert.Equal(t, "dc-bot-token", result["channelDiscordToken"])
	assert.True(t, result["channelWhatsAppEnabled"].(bool))
	assert.Equal(t, "+34600000000", result["channelWhatsAppPhoneId"])
	assert.Equal(t, "wa-token", result["channelWhatsAppApiToken"])
	assert.True(t, result["channelTwilioEnabled"].(bool))
	assert.Equal(t, "AC123", result["channelTwilioAccountSid"])
	assert.Equal(t, "tw-token", result["channelTwilioAuthToken"])
	assert.Equal(t, "+15550000000", result["channelTwilioFromNumber"])
	assert.True(t, result["channelSlackEnabled"].(bool))
	assert.Equal(t, "xoxb-bot", result["channelSlackBotToken"])
	assert.Equal(t, "xapp-app", result["channelSlackAppToken"])
}

func TestUpdateConfigInputToMap_WizardCompleted(t *testing.T) {
	wc := true
	result := UpdateConfigInputToMap(generated.UpdateConfigInput{
		WizardCompleted: &wc,
	})
	assert.True(t, result["wizardCompleted"].(bool))
}

// ─── GraphNodesToGenerated ───────────────────────────────────────────────────

func TestGraphNodesToGenerated_Empty(t *testing.T) {
	assert.Nil(t, GraphNodesToGenerated(nil))
	assert.Nil(t, GraphNodesToGenerated([]dto.GraphNodeSnapshot{}))
}

func TestGraphNodesToGenerated_WithOptionalFields(t *testing.T) {
	nodes := []dto.GraphNodeSnapshot{
		{ID: "n1", Label: "user:1", Type: "user", Value: "Alice", Properties: map[string]string{"age": "30"}},
		{ID: "n2", Label: "", Type: "", Value: "", Properties: nil},
	}
	result := GraphNodesToGenerated(nodes)
	require.Len(t, result, 2)
	assert.Equal(t, "n1", result[0].ID)
	require.NotNil(t, result[0].Label)
	assert.Equal(t, "user:1", *result[0].Label)
	require.NotNil(t, result[0].Type)
	assert.Equal(t, "user", *result[0].Type)
	require.NotNil(t, result[0].Value)
	assert.Equal(t, "Alice", *result[0].Value)
	require.NotNil(t, result[0].Properties)
	assert.Equal(t, "30", result[0].Properties["age"])

	assert.Nil(t, result[1].Label)
	assert.Nil(t, result[1].Type)
	assert.Nil(t, result[1].Value)
	assert.Nil(t, result[1].Properties)
}

// ─── GraphEdgesToGenerated ───────────────────────────────────────────────────

func TestGraphEdgesToGenerated_Empty(t *testing.T) {
	assert.Nil(t, GraphEdgesToGenerated(nil))
	assert.Nil(t, GraphEdgesToGenerated([]dto.GraphEdgeSnapshot{}))
}

func TestGraphEdgesToGenerated(t *testing.T) {
	edges := []dto.GraphEdgeSnapshot{
		{Source: "n1", Target: "n2", Label: "LIKES"},
		{Source: "n3", Target: "n4", Label: ""},
	}
	result := GraphEdgesToGenerated(edges)
	require.Len(t, result, 2)
	assert.Equal(t, "n1", result[0].Source)
	assert.Equal(t, "n2", result[0].Target)
	require.NotNil(t, result[0].Label)
	assert.Equal(t, "LIKES", *result[0].Label)
	assert.Nil(t, result[1].Label)
}

// ─── MemoryNodesFromSnapshot ─────────────────────────────────────────────────

func TestMemoryNodesFromSnapshot_Empty(t *testing.T) {
	assert.Nil(t, MemoryNodesFromSnapshot(nil))
	assert.Nil(t, MemoryNodesFromSnapshot([]dto.GraphNodeSnapshot{}))
}

func TestMemoryNodesFromSnapshot(t *testing.T) {
	nodes := []dto.GraphNodeSnapshot{
		{ID: "n1", Label: "user:1", Type: "user", Value: "Alice", Properties: map[string]string{"k": "v"}},
		{ID: "n2"},
	}
	result := MemoryNodesFromSnapshot(nodes)
	require.Len(t, result, 2)
	assert.Equal(t, "n1", result[0].ID)
	require.NotNil(t, result[0].Label)
	assert.Equal(t, "user:1", *result[0].Label)
	require.NotNil(t, result[0].Properties)
	assert.Equal(t, "v", result[0].Properties["k"])
	assert.Nil(t, result[1].Properties)
}

// ─── MemoryEdgesFromSnapshot ─────────────────────────────────────────────────

func TestMemoryEdgesFromSnapshot_Empty(t *testing.T) {
	assert.Nil(t, MemoryEdgesFromSnapshot(nil))
	assert.Nil(t, MemoryEdgesFromSnapshot([]dto.GraphEdgeSnapshot{}))
}

func TestMemoryEdgesFromSnapshot(t *testing.T) {
	edges := []dto.GraphEdgeSnapshot{
		{Source: "n1", Target: "n2", Label: "LIKES"},
	}
	result := MemoryEdgesFromSnapshot(edges)
	require.Len(t, result, 1)
	assert.Equal(t, "n1-n2", result[0].ID)
	assert.Equal(t, "n1", result[0].SourceID)
	assert.Equal(t, "n2", result[0].TargetID)
	require.NotNil(t, result[0].Relation)
	assert.Equal(t, "LIKES", *result[0].Relation)
}

// ─── ToolPermissionToGenerated ────────────────────────────────────────────────

func TestToolPermissionToGenerated(t *testing.T) {
	r := dto.ToolPermissionRecord{UserID: "u1", ToolName: "search_web", Mode: "allow"}
	result := ToolPermissionToGenerated(r)
	require.NotNil(t, result)
	assert.Equal(t, "search_web", result.ToolName)
	assert.Equal(t, "allow", result.Mode)
}

func TestToolPermissionsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, ToolPermissionsToGenerated(nil))
	assert.Nil(t, ToolPermissionsToGenerated([]dto.ToolPermissionRecord{}))
}

func TestToolPermissionsToGenerated_Multiple(t *testing.T) {
	list := []dto.ToolPermissionRecord{
		{ToolName: "tool1", Mode: "allow"},
		{ToolName: "tool2", Mode: "deny"},
	}
	result := ToolPermissionsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "tool1", result[0].ToolName)
	assert.Equal(t, "deny", result[1].Mode)
}

// ─── PairingToPendingPairing ──────────────────────────────────────────────────

func TestPairingToPendingPairing_WithOptionalFields(t *testing.T) {
	p := dto.PairingSnapshot{
		Code:             "abc123",
		Status:           "pending",
		ChannelID:        "ch1",
		ChannelType:      "telegram",
		PlatformUserName: "alice",
	}
	result := PairingToPendingPairing(p)
	require.NotNil(t, result)
	assert.Equal(t, "abc123", result.Code)
	assert.Equal(t, "pending", result.Status)
	require.NotNil(t, result.ChannelID)
	assert.Equal(t, "ch1", *result.ChannelID)
	require.NotNil(t, result.ChannelType)
	assert.Equal(t, "telegram", *result.ChannelType)
	require.NotNil(t, result.PlatformUserName)
	assert.Equal(t, "alice", *result.PlatformUserName)
}

func TestPairingToPendingPairing_EmptyOptionalFields(t *testing.T) {
	p := dto.PairingSnapshot{Code: "xyz", Status: "pending"}
	result := PairingToPendingPairing(p)
	assert.Nil(t, result.ChannelID)
	assert.Nil(t, result.ChannelType)
	assert.Nil(t, result.PlatformUserName)
}

func TestPairingsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, PairingsToGenerated(nil))
	assert.Nil(t, PairingsToGenerated([]dto.PairingSnapshot{}))
}

func TestPairingsToGenerated_Multiple(t *testing.T) {
	list := []dto.PairingSnapshot{
		{Code: "c1", Status: "pending"},
		{Code: "c2", Status: "approved"},
	}
	result := PairingsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "c1", result[0].Code)
	assert.Equal(t, "c2", result[1].Code)
}

// ─── UserSnapshotToGenerated ──────────────────────────────────────────────────

func TestUserSnapshotToGenerated_WithDisplayName(t *testing.T) {
	u := dto.UserSnapshot{ID: "user-1", DisplayName: "Alice"}
	result := UserSnapshotToGenerated(u)
	require.NotNil(t, result)
	assert.Equal(t, "user-1", result.ID)
	require.NotNil(t, result.PrimaryID)
	assert.Equal(t, "Alice", *result.PrimaryID)
}

func TestUserSnapshotToGenerated_NoDisplayName(t *testing.T) {
	u := dto.UserSnapshot{ID: "user-2"}
	result := UserSnapshotToGenerated(u)
	assert.Nil(t, result.PrimaryID)
}

func TestUsersToGenerated_Empty(t *testing.T) {
	assert.Nil(t, UsersToGenerated(nil))
	assert.Nil(t, UsersToGenerated([]dto.UserSnapshot{}))
}

func TestUsersToGenerated_Multiple(t *testing.T) {
	list := []dto.UserSnapshot{
		{ID: "u1", DisplayName: "Alice"},
		{ID: "u2", DisplayName: "Bob"},
	}
	result := UsersToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "u1", result[0].ID)
}

// ─── UsersToMcpUsers ──────────────────────────────────────────────────────────

func TestUsersToMcpUsers_Empty(t *testing.T) {
	assert.Nil(t, UsersToMcpUsers(nil))
	assert.Nil(t, UsersToMcpUsers([]dto.UserSnapshot{}))
}

func TestUsersToMcpUsers_WithDisplayName(t *testing.T) {
	list := []dto.UserSnapshot{
		{ID: "u1", DisplayName: "Alice"},
		{ID: "u2", DisplayName: ""},
	}
	result := UsersToMcpUsers(list)
	require.Len(t, result, 2)
	assert.Equal(t, "u1", result[0].ChannelID)
	require.NotNil(t, result[0].DisplayName)
	assert.Equal(t, "Alice", *result[0].DisplayName)
	assert.False(t, result[0].IsAgent)

	// When DisplayName is empty, falls back to ID.
	assert.Equal(t, "u2", result[1].ChannelID)
	require.NotNil(t, result[1].DisplayName)
	assert.Equal(t, "u2", *result[1].DisplayName)
}

// ─── PairingSnapshotToPairingInfo ─────────────────────────────────────────────

func TestPairingSnapshotToPairingInfo_Nil(t *testing.T) {
	assert.Nil(t, PairingSnapshotToPairingInfo(nil))
}

func TestPairingSnapshotToPairingInfo(t *testing.T) {
	p := &dto.PairingSnapshot{Code: "abc", Status: "pending"}
	result := PairingSnapshotToPairingInfo(p)
	require.NotNil(t, result)
	assert.Equal(t, "abc", result.Code)
	assert.Equal(t, "pending", result.Status)
}

// ─── MCPServerRecordToGenerated ──────────────────────────────────────────────

func TestMCPServerRecordToGenerated_WithURL(t *testing.T) {
	r := dto.MCPServerRecord{Name: "server1", URL: "http://localhost:8080", Status: "online", ToolCount: 5}
	result := MCPServerRecordToGenerated(r)
	require.NotNil(t, result)
	assert.Equal(t, "server1", result.Name)
	assert.Equal(t, "online", result.Status)
	assert.Equal(t, 5, result.ToolCount)
	require.NotNil(t, result.URL)
	assert.Equal(t, "http://localhost:8080", *result.URL)
}

func TestMCPServerRecordToGenerated_EmptyStatus(t *testing.T) {
	r := dto.MCPServerRecord{Name: "server2", Status: ""}
	result := MCPServerRecordToGenerated(r)
	// Empty status defaults to "unknown".
	assert.Equal(t, "unknown", result.Status)
	assert.Nil(t, result.URL)
}

func TestMCPServersToGenerated_Empty(t *testing.T) {
	assert.Nil(t, MCPServersToGenerated(nil))
	assert.Nil(t, MCPServersToGenerated([]dto.MCPServerRecord{}))
}

func TestMCPServersToGenerated_Multiple(t *testing.T) {
	list := []dto.MCPServerRecord{
		{Name: "s1", Status: "online"},
		{Name: "s2", Status: ""},
	}
	result := MCPServersToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "s1", result[0].Name)
	assert.Equal(t, "unknown", result[1].Status)
}

// ─── MCPToolSnapshotToGenerated ──────────────────────────────────────────────

func TestMCPToolSnapshotToGenerated_WithOptionalFields(t *testing.T) {
	t1 := dto.ToolSnapshot{Name: "search", Description: "Search the web", ServerName: "server1"}
	result := MCPToolSnapshotToGenerated(t1)
	require.NotNil(t, result)
	assert.Equal(t, "search", result.Name)
	require.NotNil(t, result.Description)
	assert.Equal(t, "Search the web", *result.Description)
	require.NotNil(t, result.ServerName)
	assert.Equal(t, "server1", *result.ServerName)
}

func TestMCPToolSnapshotToGenerated_EmptyOptionalFields(t *testing.T) {
	t1 := dto.ToolSnapshot{Name: "minimal"}
	result := MCPToolSnapshotToGenerated(t1)
	assert.Nil(t, result.Description)
	assert.Nil(t, result.ServerName)
}

func TestMCPToolsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, MCPToolsToGenerated(nil))
	assert.Nil(t, MCPToolsToGenerated([]dto.ToolSnapshot{}))
}

func TestMCPToolsToGenerated_Multiple(t *testing.T) {
	list := []dto.ToolSnapshot{
		{Name: "t1", Description: "desc1"},
		{Name: "t2"},
	}
	result := MCPToolsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "t1", result[0].Name)
	assert.Equal(t, "t2", result[1].Name)
}

// ─── SkillSnapshotToGenerated ────────────────────────────────────────────────

func TestSkillSnapshotToGenerated_WithOptionalFields(t *testing.T) {
	s := dto.SkillSnapshot{Name: "search", Description: "Search skill", Enabled: true, Path: "/skills/search.md"}
	result := SkillSnapshotToGenerated(s)
	require.NotNil(t, result)
	assert.Equal(t, "search", result.Name)
	assert.True(t, result.Enabled)
	require.NotNil(t, result.Description)
	assert.Equal(t, "Search skill", *result.Description)
	require.NotNil(t, result.Path)
	assert.Equal(t, "/skills/search.md", *result.Path)
}

func TestSkillSnapshotToGenerated_EmptyOptionalFields(t *testing.T) {
	s := dto.SkillSnapshot{Name: "minimal", Enabled: false}
	result := SkillSnapshotToGenerated(s)
	assert.Nil(t, result.Description)
	assert.Nil(t, result.Path)
}

func TestSkillsToGenerated_Empty(t *testing.T) {
	assert.Nil(t, SkillsToGenerated(nil))
	assert.Nil(t, SkillsToGenerated([]dto.SkillSnapshot{}))
}

func TestSkillsToGenerated_Multiple(t *testing.T) {
	list := []dto.SkillSnapshot{
		{Name: "sk1", Enabled: true},
		{Name: "sk2", Enabled: false},
	}
	result := SkillsToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "sk1", result[0].Name)
	assert.True(t, result[0].Enabled)
	assert.False(t, result[1].Enabled)
}

// ─── SystemFileSnapshotToGenerated ───────────────────────────────────────────

func TestSystemFileSnapshotToGenerated_WithLastModified(t *testing.T) {
	s := dto.SystemFileSnapshot{
		Name:         "agents.md",
		Path:         "/data/agents.md",
		Content:      "# Agents",
		LastModified: "2024-01-01T00:00:00Z",
	}
	result := SystemFileSnapshotToGenerated(s)
	require.NotNil(t, result)
	assert.Equal(t, "agents.md", result.Name)
	assert.Equal(t, "/data/agents.md", result.Path)
	require.NotNil(t, result.Content)
	assert.Equal(t, "# Agents", *result.Content)
	require.NotNil(t, result.LastModified)
	assert.Equal(t, "2024-01-01T00:00:00Z", *result.LastModified)
}

func TestSystemFileSnapshotToGenerated_NoLastModified(t *testing.T) {
	s := dto.SystemFileSnapshot{Name: "soul.md", Path: "/data/soul.md", Content: ""}
	result := SystemFileSnapshotToGenerated(s)
	assert.Nil(t, result.LastModified)
}

func TestSystemFilesToGenerated_Empty(t *testing.T) {
	assert.Nil(t, SystemFilesToGenerated(nil))
	assert.Nil(t, SystemFilesToGenerated([]dto.SystemFileSnapshot{}))
}

func TestSystemFilesToGenerated_Multiple(t *testing.T) {
	list := []dto.SystemFileSnapshot{
		{Name: "f1", Path: "/f1"},
		{Name: "f2", Path: "/f2"},
	}
	result := SystemFilesToGenerated(list)
	require.Len(t, result, 2)
	assert.Equal(t, "f1", result[0].Name)
}

// ─── Regression: editable agent config field coverage ────────────────────────
//
// CANONICAL LIST — add new backend-editable agent config fields here.
// Each entry is automatically checked for both mapper directions:
//   1. UpdateConfigInputToMap    – GraphQL input key → viper map key
//   2. AppConfigSnapshotToGenerated – snapshot field → GraphQL generated type
//
// To add a new field: append a row to agentFieldMapCases / agentSnapshotCases.
// If either mapper direction is missing the tests will fail automatically.
// ─────────────────────────────────────────────────────────────────────────────

// agentFieldMapCases defines the canonical mapping from GraphQL UpdateConfigInput
// field to the viper map key produced by UpdateConfigInputToMap.
var agentFieldMapCases = []struct {
	name   string
	setter func(*generated.UpdateConfigInput, string)
	mapKey string
}{
	{
		name:   "agentName",
		setter: func(i *generated.UpdateConfigInput, v string) { i.AgentName = &v },
		mapKey: "agentName",
	},
	{
		name:   "provider",
		setter: func(i *generated.UpdateConfigInput, v string) { i.Provider = &v },
		mapKey: "provider",
	},
	{
		name:   "model",
		setter: func(i *generated.UpdateConfigInput, v string) { i.Model = &v },
		mapKey: "model",
	},
	{
		name:   "apiKey",
		setter: func(i *generated.UpdateConfigInput, v string) { i.APIKey = &v },
		mapKey: "apiKey",
	},
	{
		name:   "baseURL",
		setter: func(i *generated.UpdateConfigInput, v string) { i.BaseURL = &v },
		mapKey: "baseURL",
	},
	{
		name:   "ollamaHost",
		setter: func(i *generated.UpdateConfigInput, v string) { i.OllamaHost = &v },
		mapKey: "ollamaHost",
	},
	{
		name:   "ollamaApiKey",
		setter: func(i *generated.UpdateConfigInput, v string) { i.OllamaAPIKey = &v },
		mapKey: "ollamaApiKey",
	},
	{
		name:   "anthropicApiKey",
		setter: func(i *generated.UpdateConfigInput, v string) { i.AnthropicAPIKey = &v },
		mapKey: "anthropicApiKey",
	},
	{
		name:   "dockerModelRunnerEndpoint",
		setter: func(i *generated.UpdateConfigInput, v string) { i.DockerModelRunnerEndpoint = &v },
		mapKey: "dockerModelRunnerEndpoint",
	},
	{
		name:   "reasoningLevel",
		setter: func(i *generated.UpdateConfigInput, v string) { i.ReasoningLevel = &v },
		mapKey: "reasoningLevel",
	},
	{
		name:   "systemPrompt",
		setter: func(i *generated.UpdateConfigInput, v string) { i.SystemPrompt = &v },
		mapKey: "systemPrompt",
	},
	{
		name:   "dockerModelRunnerModel",
		setter: func(i *generated.UpdateConfigInput, v string) { i.DockerModelRunnerModel = &v },
		mapKey: "dockerModelRunnerModel",
	},
}

// TestUpdateConfigInputToMap_AllAgentFields verifies that every entry in the
// canonical list is correctly mapped to its viper key.
func TestUpdateConfigInputToMap_AllAgentFields(t *testing.T) {
	const sentinel = "test-value"
	for _, tc := range agentFieldMapCases {
		t.Run(tc.name, func(t *testing.T) {
			input := generated.UpdateConfigInput{}
			tc.setter(&input, sentinel)
			m := UpdateConfigInputToMap(input)
			val, ok := m[tc.mapKey]
			assert.True(t, ok, "key %q must be present in the output map", tc.mapKey)
			assert.Equal(t, sentinel, val, "value must be forwarded unchanged")
		})
	}
}

// TestUpdateConfigInputToMap_NilFieldsAbsent verifies that unset (nil) fields
// are never written to the map (no accidental zero-value overwrites).
func TestUpdateConfigInputToMap_NilFieldsAbsent(t *testing.T) {
	m := UpdateConfigInputToMap(generated.UpdateConfigInput{})
	for _, tc := range agentFieldMapCases {
		_, ok := m[tc.mapKey]
		assert.False(t, ok, "nil field %q must not appear in the map", tc.mapKey)
	}
}

// agentSnapshotCases defines the canonical mapping from AgentConfigSnapshot
// field to the *string field on generated.AgentConfig.
var agentSnapshotCases = []struct {
	name     string
	snapshot func() *dto.AgentConfigSnapshot
	getter   func(*generated.AgentConfig) *string
}{
	{
		name:     "name",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{Name: "bot"} },
		getter:   func(a *generated.AgentConfig) *string { return a.Name },
	},
	{
		name:     "provider",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{Provider: "anthropic"} },
		getter:   func(a *generated.AgentConfig) *string { return a.Provider },
	},
	{
		name:     "model",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{Model: "claude-sonnet"} },
		getter:   func(a *generated.AgentConfig) *string { return a.Model },
	},
	{
		name:     "apiKey",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{APIKey: "sk-key"} },
		getter:   func(a *generated.AgentConfig) *string { return a.APIKey },
	},
	{
		name:     "baseURL",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{BaseURL: "https://api.example.com"} },
		getter:   func(a *generated.AgentConfig) *string { return a.BaseURL },
	},
	{
		name:     "ollamaHost",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{OllamaHost: "http://localhost:11434"} },
		getter:   func(a *generated.AgentConfig) *string { return a.OllamaHost },
	},
	{
		name:     "ollamaApiKey",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{OllamaApiKey: "ollama-key"} },
		getter:   func(a *generated.AgentConfig) *string { return a.OllamaAPIKey },
	},
	{
		name:     "anthropicApiKey",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{AnthropicApiKey: "sk-ant"} },
		getter:   func(a *generated.AgentConfig) *string { return a.AnthropicAPIKey },
	},
	{
		name: "dockerModelRunnerEndpoint",
		snapshot: func() *dto.AgentConfigSnapshot {
			return &dto.AgentConfigSnapshot{DockerModelRunnerEndpoint: "http://dmr"}
		},
		getter: func(a *generated.AgentConfig) *string { return a.DockerModelRunnerEndpoint },
	},
	{
		name:     "reasoningLevel",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{ReasoningLevel: "high"} },
		getter:   func(a *generated.AgentConfig) *string { return a.ReasoningLevel },
	},
	{
		name:     "systemPrompt",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{SystemPrompt: "Be helpful."} },
		getter:   func(a *generated.AgentConfig) *string { return a.SystemPrompt },
	},
	{
		name:     "dockerModelRunnerModel",
		snapshot: func() *dto.AgentConfigSnapshot { return &dto.AgentConfigSnapshot{DockerModelRunnerModel: "ai/mistral"} },
		getter:   func(a *generated.AgentConfig) *string { return a.DockerModelRunnerModel },
	},
}

// TestAppConfigSnapshotToGenerated_AllAgentFields verifies that every entry in the
// canonical list is correctly mapped from snapshot to the generated GraphQL type.
func TestAppConfigSnapshotToGenerated_AllAgentFields(t *testing.T) {
	for _, tc := range agentSnapshotCases {
		t.Run(tc.name, func(t *testing.T) {
			snap := &dto.AppConfigSnapshot{Agent: tc.snapshot()}
			result := AppConfigSnapshotToGenerated(snap)
			require.NotNil(t, result.Agent, "Agent must be mapped")
			ptr := tc.getter(result.Agent)
			require.NotNil(t, ptr, "field %q must be non-nil in generated output", tc.name)
			// Retrieve expected value from the original snapshot via the same getter.
			expected := tc.getter(&generated.AgentConfig{
				Name:                      strOrNilExported(snap.Agent.Name),
				Provider:                  strOrNilExported(snap.Agent.Provider),
				Model:                     strOrNilExported(snap.Agent.Model),
				APIKey:                    strOrNilExported(snap.Agent.APIKey),
				BaseURL:                   strOrNilExported(snap.Agent.BaseURL),
				OllamaHost:                strOrNilExported(snap.Agent.OllamaHost),
				OllamaAPIKey:              strOrNilExported(snap.Agent.OllamaApiKey),
				AnthropicAPIKey:           strOrNilExported(snap.Agent.AnthropicApiKey),
				DockerModelRunnerEndpoint: strOrNilExported(snap.Agent.DockerModelRunnerEndpoint),
				ReasoningLevel:            strOrNilExported(snap.Agent.ReasoningLevel),
				SystemPrompt:              strOrNilExported(snap.Agent.SystemPrompt),
				DockerModelRunnerModel:    strOrNilExported(snap.Agent.DockerModelRunnerModel),
			})
			if expected != nil {
				assert.Equal(t, *expected, *ptr, "field %q value must be forwarded unchanged", tc.name)
			}
		})
	}
}

// strOrNilExported returns a pointer to s, or nil if s is empty (mirrors the
// unexported strOrNil helper used by AppConfigSnapshotToGenerated).
func strOrNilExported(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
