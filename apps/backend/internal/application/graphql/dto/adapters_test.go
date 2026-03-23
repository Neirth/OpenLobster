// Copyright (c) OpenLobster contributors. See LICENSE for details.

package dto

import (
	"context"
	"errors"
	"testing"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ConversationPortAdapter ─────────────────────────────────────────────────

// fakeConversationRepo implements the anonymous interface required by ConversationPortAdapter.
type fakeConversationRepo struct {
	rows []repositories.ConversationRow
	err  error
}

func (r *fakeConversationRepo) ListConversations() ([]repositories.ConversationRow, error) {
	return r.rows, r.err
}
func (r *fakeConversationRepo) DeleteUser(ctx context.Context, conversationID string) error {
	return r.err
}
func (r *fakeConversationRepo) DeleteGroup(ctx context.Context, conversationID string) error {
	return r.err
}

func TestConversationPortAdapter_ListConversations_Error(t *testing.T) {
	repo := &fakeConversationRepo{err: errors.New("db error")}
	adapter := &ConversationPortAdapter{Repo: repo}
	_, err := adapter.ListConversations()
	require.Error(t, err)
}

func TestConversationPortAdapter_ListConversations_Empty(t *testing.T) {
	repo := &fakeConversationRepo{rows: nil}
	adapter := &ConversationPortAdapter{Repo: repo}
	result, err := adapter.ListConversations()
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestConversationPortAdapter_ListConversations_MapsFields(t *testing.T) {
	rows := []repositories.ConversationRow{
		{
			ID:              "conv-1",
			ChannelID:       "ch-1",
			ChannelType:     "telegram",
			ChannelName:     "Telegram",
			GroupName:       "",
			IsGroup:         false,
			ParticipantID:   "pid-1",
			ParticipantName: "Alice",
			LastMessageAt:   "2024-01-01T00:00:00Z",
			UnreadCount:     3,
		},
	}
	repo := &fakeConversationRepo{rows: rows}
	adapter := &ConversationPortAdapter{Repo: repo}
	result, err := adapter.ListConversations()
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "conv-1", result[0].ID)
	assert.Equal(t, "ch-1", result[0].ChannelID)
	assert.Equal(t, "telegram", result[0].ChannelType)
	assert.Equal(t, "Telegram", result[0].ChannelName)
	assert.Equal(t, "pid-1", result[0].ParticipantID)
	assert.Equal(t, "Alice", result[0].ParticipantName)
	assert.Equal(t, "2024-01-01T00:00:00Z", result[0].LastMessageAt)
	assert.Equal(t, 3, result[0].UnreadCount)
}

func TestConversationPortAdapter_DeleteUser(t *testing.T) {
	repo := &fakeConversationRepo{}
	adapter := &ConversationPortAdapter{Repo: repo}
	err := adapter.DeleteUser(context.Background(), "conv-1")
	assert.NoError(t, err)
}

func TestConversationPortAdapter_DeleteUser_Error(t *testing.T) {
	repo := &fakeConversationRepo{err: errors.New("delete error")}
	adapter := &ConversationPortAdapter{Repo: repo}
	err := adapter.DeleteUser(context.Background(), "conv-1")
	require.Error(t, err)
}

func TestConversationPortAdapter_DeleteGroup(t *testing.T) {
	repo := &fakeConversationRepo{}
	adapter := &ConversationPortAdapter{Repo: repo}
	err := adapter.DeleteGroup(context.Background(), "conv-1")
	assert.NoError(t, err)
}

func TestConversationPortAdapter_DeleteGroup_Error(t *testing.T) {
	repo := &fakeConversationRepo{err: errors.New("delete group error")}
	adapter := &ConversationPortAdapter{Repo: repo}
	err := adapter.DeleteGroup(context.Background(), "conv-1")
	require.Error(t, err)
}

// ─── ToolPermAdapter ──────────────────────────────────────────────────────────

// fakeToolPermRepo implements repositories.ToolPermissionRepositoryPort.
type fakeToolPermRepo struct {
	rows []repositories.ToolPermissionRecord
	err  error
}

func (r *fakeToolPermRepo) Set(ctx context.Context, userID, toolName, mode string) error {
	return r.err
}
func (r *fakeToolPermRepo) Delete(ctx context.Context, userID, toolName string) error {
	return r.err
}
func (r *fakeToolPermRepo) ListByUser(ctx context.Context, userID string) ([]repositories.ToolPermissionRecord, error) {
	return r.rows, r.err
}
func (r *fakeToolPermRepo) ListAll(ctx context.Context) ([]repositories.ToolPermissionRecord, error) {
	return r.rows, r.err
}

func TestToolPermAdapter_Set(t *testing.T) {
	repo := &fakeToolPermRepo{}
	adapter := &ToolPermAdapter{Repo: repo}
	err := adapter.Set(context.Background(), "u1", "search_web", "allow")
	assert.NoError(t, err)
}

func TestToolPermAdapter_Set_Error(t *testing.T) {
	repo := &fakeToolPermRepo{err: errors.New("set error")}
	adapter := &ToolPermAdapter{Repo: repo}
	err := adapter.Set(context.Background(), "u1", "tool", "allow")
	require.Error(t, err)
}

func TestToolPermAdapter_Delete(t *testing.T) {
	repo := &fakeToolPermRepo{}
	adapter := &ToolPermAdapter{Repo: repo}
	err := adapter.Delete(context.Background(), "u1", "search_web")
	assert.NoError(t, err)
}

func TestToolPermAdapter_ListByUser_Error(t *testing.T) {
	repo := &fakeToolPermRepo{err: errors.New("list error")}
	adapter := &ToolPermAdapter{Repo: repo}
	_, err := adapter.ListByUser(context.Background(), "u1")
	require.Error(t, err)
}

func TestToolPermAdapter_ListByUser_MapsFields(t *testing.T) {
	rows := []repositories.ToolPermissionRecord{
		{UserID: "u1", ToolName: "search_web", Mode: "allow"},
		{UserID: "u1", ToolName: "terminal_exec", Mode: "deny"},
	}
	repo := &fakeToolPermRepo{rows: rows}
	adapter := &ToolPermAdapter{Repo: repo}
	result, err := adapter.ListByUser(context.Background(), "u1")
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "u1", result[0].UserID)
	assert.Equal(t, "search_web", result[0].ToolName)
	assert.Equal(t, "allow", result[0].Mode)
	assert.Equal(t, "deny", result[1].Mode)
}

func TestToolPermAdapter_ListAll_Error(t *testing.T) {
	repo := &fakeToolPermRepo{err: errors.New("list all error")}
	adapter := &ToolPermAdapter{Repo: repo}
	_, err := adapter.ListAll(context.Background())
	require.Error(t, err)
}

func TestToolPermAdapter_ListAll_MapsFields(t *testing.T) {
	rows := []repositories.ToolPermissionRecord{
		{UserID: "u1", ToolName: "t1", Mode: "allow"},
		{UserID: "u2", ToolName: "t2", Mode: "deny"},
	}
	repo := &fakeToolPermRepo{rows: rows}
	adapter := &ToolPermAdapter{Repo: repo}
	result, err := adapter.ListAll(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "u1", result[0].UserID)
	assert.Equal(t, "u2", result[1].UserID)
}

// ─── MCPServerAdapter ────────────────────────────────────────────────────────

// fakeMCPServerRepo implements repositories.MCPServerRepositoryPort.
type fakeMCPServerRepo struct {
	rows []repositories.MCPServerRecord
	err  error
}

func (r *fakeMCPServerRepo) Save(ctx context.Context, name, url string) error  { return r.err }
func (r *fakeMCPServerRepo) Delete(ctx context.Context, name string) error      { return r.err }
func (r *fakeMCPServerRepo) ListAll(ctx context.Context) ([]repositories.MCPServerRecord, error) {
	return r.rows, r.err
}

func TestMCPServerAdapter_Save(t *testing.T) {
	repo := &fakeMCPServerRepo{}
	adapter := &MCPServerAdapter{Repo: repo}
	err := adapter.Save(context.Background(), "server1", "http://localhost:8080")
	assert.NoError(t, err)
}

func TestMCPServerAdapter_Save_Error(t *testing.T) {
	repo := &fakeMCPServerRepo{err: errors.New("save error")}
	adapter := &MCPServerAdapter{Repo: repo}
	err := adapter.Save(context.Background(), "server1", "http://localhost:8080")
	require.Error(t, err)
}

func TestMCPServerAdapter_Delete(t *testing.T) {
	repo := &fakeMCPServerRepo{}
	adapter := &MCPServerAdapter{Repo: repo}
	err := adapter.Delete(context.Background(), "server1")
	assert.NoError(t, err)
}

func TestMCPServerAdapter_ListAll_Error(t *testing.T) {
	repo := &fakeMCPServerRepo{err: errors.New("list error")}
	adapter := &MCPServerAdapter{Repo: repo}
	_, err := adapter.ListAll(context.Background())
	require.Error(t, err)
}

func TestMCPServerAdapter_ListAll_MapsFields(t *testing.T) {
	rows := []repositories.MCPServerRecord{
		{Name: "server1", URL: "http://localhost:8080"},
		{Name: "server2", URL: "http://localhost:9090"},
	}
	repo := &fakeMCPServerRepo{rows: rows}
	adapter := &MCPServerAdapter{Repo: repo}
	result, err := adapter.ListAll(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "server1", result[0].Name)
	assert.Equal(t, "http://localhost:8080", result[0].URL)
}

// ─── UserRepoAdapter ──────────────────────────────────────────────────────────

// fakeUserRepo implements ports.UserRepositoryPort.
type fakeUserRepo struct {
	user *models.User
	err  error
	list []models.User
}

func (r *fakeUserRepo) Create(ctx context.Context, user *models.User) error { return r.err }
func (r *fakeUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	return r.user, r.err
}
func (r *fakeUserRepo) GetByPrimaryID(ctx context.Context, primaryID string) (*models.User, error) {
	return r.user, r.err
}
func (r *fakeUserRepo) Update(ctx context.Context, user *models.User) error { return r.err }
func (r *fakeUserRepo) ListAll(ctx context.Context) ([]models.User, error) {
	return r.list, r.err
}

func TestUserRepoAdapter_Create(t *testing.T) {
	repo := &fakeUserRepo{}
	adapter := &UserRepoAdapter{Repo: repo}
	u := models.NewUser("primary-1")
	err := adapter.Create(context.Background(), u)
	assert.NoError(t, err)
}

func TestUserRepoAdapter_Create_Error(t *testing.T) {
	repo := &fakeUserRepo{err: errors.New("create error")}
	adapter := &UserRepoAdapter{Repo: repo}
	u := models.NewUser("primary-1")
	err := adapter.Create(context.Background(), u)
	require.Error(t, err)
}

func TestUserRepoAdapter_GetByID(t *testing.T) {
	u := models.NewUser("primary-1")
	repo := &fakeUserRepo{user: u}
	adapter := &UserRepoAdapter{Repo: repo}
	result, err := adapter.GetByID(context.Background(), u.ID.String())
	require.NoError(t, err)
	assert.Equal(t, u.ID, result.ID)
}

func TestUserRepoAdapter_GetByID_Error(t *testing.T) {
	repo := &fakeUserRepo{err: errors.New("not found")}
	adapter := &UserRepoAdapter{Repo: repo}
	_, err := adapter.GetByID(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestUserRepoAdapter_ListAll(t *testing.T) {
	users := []models.User{
		*models.NewUser("p1"),
		*models.NewUser("p2"),
	}
	repo := &fakeUserRepo{list: users}
	adapter := &UserRepoAdapter{Repo: repo}
	result, err := adapter.ListAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestUserRepoAdapter_ListAll_Error(t *testing.T) {
	repo := &fakeUserRepo{err: errors.New("list error")}
	adapter := &UserRepoAdapter{Repo: repo}
	_, err := adapter.ListAll(context.Background())
	require.Error(t, err)
}

// ─── EventBusAdapter ─────────────────────────────────────────────────────────

func TestEventBusAdapter_Publish_NilBus(t *testing.T) {
	adapter := &EventBusAdapter{Eb: nil}
	err := adapter.Publish(context.Background(), "test.event", map[string]string{"key": "value"})
	assert.NoError(t, err)
}

// ─── EventSubscriptionAdapter ────────────────────────────────────────────────

func TestEventSubscriptionAdapter_Subscribe_NilBus(t *testing.T) {
	adapter := &EventSubscriptionAdapter{Eb: nil}
	ctx := context.Background()
	ch, err := adapter.Subscribe(ctx, "test.event")
	require.NoError(t, err)
	require.NotNil(t, ch)
}

// ─── DTO struct field coverage ────────────────────────────────────────────────

func TestAgentSnapshot_Fields(t *testing.T) {
	snap := AgentSnapshot{
		ID:            "a1",
		Name:          "Bot",
		Version:       "1.0",
		Status:        "running",
		Uptime:        3600,
		Provider:      "telegram",
		AIProvider:    "anthropic",
		MemoryBackend: "neo4j",
		ToolsCount:    5,
		TasksCount:    2,
		Channels:      []ChannelStatus{{ID: "ch1", Name: "tg"}},
	}
	assert.Equal(t, "a1", snap.ID)
	assert.Equal(t, int64(3600), snap.Uptime)
	assert.Len(t, snap.Channels, 1)
}

func TestChannelStatus_Fields(t *testing.T) {
	cs := ChannelStatus{
		ID:      "ch1",
		Name:    "Telegram",
		Type:    "telegram",
		Status:  "online",
		Enabled: true,
		Capabilities: ChannelCapabilities{
			HasVoiceMessage: true,
			HasCallStream:   false,
			HasTextStream:   true,
			HasMediaSupport: true,
		},
	}
	assert.True(t, cs.Capabilities.HasVoiceMessage)
	assert.False(t, cs.Capabilities.HasCallStream)
}

func TestTaskSnapshot_Fields(t *testing.T) {
	ts := TaskSnapshot{
		ID:        "t1",
		Prompt:    "do it",
		Status:    "pending",
		Schedule:  "0 9 * * *",
		TaskType:  "cyclic",
		Enabled:   true,
		CreatedAt: "2024-01-01T00:00:00Z",
		LastRunAt: "2024-01-08T09:00:00Z",
		NextRunAt: "2024-01-15T09:00:00Z",
		IsCyclic:  true,
	}
	assert.True(t, ts.IsCyclic)
	assert.Equal(t, "0 9 * * *", ts.Schedule)
}

func TestConversationSnapshot_Fields(t *testing.T) {
	cs := ConversationSnapshot{
		ID:              "cv1",
		ChannelID:       "ch1",
		ChannelType:     "telegram",
		ChannelName:     "Telegram",
		GroupName:       "",
		IsGroup:         false,
		ParticipantID:   "p1",
		ParticipantName: "Alice",
		LastMessageAt:   "2024-01-01T00:00:00Z",
		UnreadCount:     5,
	}
	assert.Equal(t, 5, cs.UnreadCount)
}

func TestAttachmentSnapshot_Fields(t *testing.T) {
	a := AttachmentSnapshot{
		Type:     "image",
		URL:      "https://example.com/img.jpg",
		Filename: "img.jpg",
		MIMEType: "image/jpeg",
	}
	assert.Equal(t, "image", a.Type)
	assert.Equal(t, "image/jpeg", a.MIMEType)
}

func TestMessageSnapshot_Fields(t *testing.T) {
	m := MessageSnapshot{
		ID:             "msg-1",
		ConversationID: "conv-1",
		Role:           "user",
		Content:        "hello",
		CreatedAt:      "2024-01-01T00:00:00Z",
		Attachments:    []AttachmentSnapshot{{Type: "image"}},
	}
	assert.Equal(t, "user", m.Role)
	assert.Len(t, m.Attachments, 1)
}

func TestToolSnapshot_Fields(t *testing.T) {
	t1 := ToolSnapshot{
		Name:        "search",
		Description: "Search the web",
		Source:      "mcp",
		ServerName:  "server1",
	}
	assert.Equal(t, "server1", t1.ServerName)
}

func TestMCPSnapshot_Fields(t *testing.T) {
	m := MCPSnapshot{
		Name:   "server1",
		Type:   "sse",
		Status: "online",
		URL:    "http://localhost:8080",
		Tools:  []ToolSnapshot{{Name: "t1"}},
	}
	assert.Len(t, m.Tools, 1)
}

func TestSubAgentSnapshot_Fields(t *testing.T) {
	sa := SubAgentSnapshot{
		ID:     "sa1",
		Name:   "worker",
		Status: "running",
		Task:   "summarize",
	}
	assert.Equal(t, "summarize", sa.Task)
}

func TestMCPServerRecord_Fields(t *testing.T) {
	r := MCPServerRecord{
		Name:      "server1",
		URL:       "http://localhost:8080",
		Status:    "online",
		ToolCount: 5,
	}
	assert.Equal(t, 5, r.ToolCount)
}

func TestToolPermissionRecord_Fields(t *testing.T) {
	r := ToolPermissionRecord{
		UserID:   "u1",
		ToolName: "search",
		Mode:     "allow",
	}
	assert.Equal(t, "allow", r.Mode)
}

func TestSkillSnapshot_Fields(t *testing.T) {
	s := SkillSnapshot{
		Name:        "research",
		Description: "Research skill",
		Enabled:     true,
		Path:        "/skills/research.md",
	}
	assert.True(t, s.Enabled)
}

func TestGraphNodeSnapshot_Fields(t *testing.T) {
	n := GraphNodeSnapshot{
		ID:         "n1",
		Label:      "user:1",
		Type:       "user",
		Value:      "Alice",
		Properties: map[string]string{"age": "30"},
	}
	assert.Equal(t, "30", n.Properties["age"])
}

func TestGraphEdgeSnapshot_Fields(t *testing.T) {
	e := GraphEdgeSnapshot{Source: "n1", Target: "n2", Label: "LIKES"}
	assert.Equal(t, "LIKES", e.Label)
}

func TestGraphSnapshot_Fields(t *testing.T) {
	g := GraphSnapshot{
		Nodes: []GraphNodeSnapshot{{ID: "n1"}},
		Edges: []GraphEdgeSnapshot{{Source: "n1", Target: "n2"}},
	}
	assert.Len(t, g.Nodes, 1)
	assert.Len(t, g.Edges, 1)
}

func TestMetricsSnapshot_Fields(t *testing.T) {
	m := MetricsSnapshot{
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
	assert.Equal(t, int64(3600), m.Uptime)
	assert.Equal(t, int64(3), m.ErrorsTotal)
}

func TestStatusSnapshot_Fields(t *testing.T) {
	s := StatusSnapshot{
		Agent:  &AgentSnapshot{ID: "a1"},
		Health: &HeartbeatSnapshot{Status: "ok"},
	}
	assert.Equal(t, "a1", s.Agent.ID)
	assert.Equal(t, "ok", s.Health.Status)
}

func TestHeartbeatSnapshot_Fields(t *testing.T) {
	h := HeartbeatSnapshot{Status: "healthy", LastCheck: 1700000000}
	assert.Equal(t, int64(1700000000), h.LastCheck)
}

func TestSendMessageResult_Fields(t *testing.T) {
	r := SendMessageResult{
		ID:             "msg-1",
		ConversationID: "conv-1",
		Role:           "assistant",
		Content:        "Hello!",
		CreatedAt:      "2024-01-01T00:00:00Z",
	}
	assert.Equal(t, "assistant", r.Role)
}

func TestPairingSnapshot_Fields(t *testing.T) {
	p := PairingSnapshot{
		Code:             "abc123",
		Status:           "pending",
		ChannelID:        "ch1",
		ChannelType:      "telegram",
		PlatformUserName: "alice",
		CreatedAt:        "2024-01-01T00:00:00Z",
		ExpiresAt:        "2024-01-02T00:00:00Z",
	}
	assert.Equal(t, "2024-01-02T00:00:00Z", p.ExpiresAt)
}

func TestUserSnapshot_Fields(t *testing.T) {
	u := UserSnapshot{ID: "u1", DisplayName: "Alice"}
	assert.Equal(t, "Alice", u.DisplayName)
}

func TestSystemFileSnapshot_Fields(t *testing.T) {
	s := SystemFileSnapshot{
		Name:         "agents.md",
		Path:         "/data/agents.md",
		Content:      "# Agents",
		LastModified: "2024-01-01T00:00:00Z",
	}
	assert.Equal(t, "# Agents", s.Content)
}

func TestAppConfigSnapshot_Fields(t *testing.T) {
	snap := AppConfigSnapshot{
		WizardCompleted: true,
	}
	assert.True(t, snap.WizardCompleted)
}

func TestAgentConfigSnapshot_Fields(t *testing.T) {
	s := AgentConfigSnapshot{
		Name:                      "Bot",
		SystemPrompt:              "Be helpful.",
		Provider:                  "anthropic",
		Model:                     "claude-sonnet-4-6",
		APIKey:                    "sk-ant",
		BaseURL:                   "https://api.anthropic.com",
		OllamaHost:                "http://localhost:11434",
		OllamaApiKey:              "olk",
		AnthropicApiKey:           "sk-ant",
		DockerModelRunnerEndpoint: "http://dmr:12434",
		DockerModelRunnerModel:    "ai/mistral",
	}
	assert.Equal(t, "claude-sonnet-4-6", s.Model)
}

func TestCapabilitiesSnapshot_Fields(t *testing.T) {
	s := CapabilitiesSnapshot{
		Browser:    true,
		Terminal:   true,
		Subagents:  false,
		Memory:     true,
		MCP:        false,
		Filesystem: true,
		Sessions:   false,
	}
	assert.True(t, s.Browser)
	assert.False(t, s.MCP)
}

func TestDatabaseConfigSnapshot_Fields(t *testing.T) {
	s := DatabaseConfigSnapshot{
		Driver:       "postgres",
		DSN:          "postgres://localhost/db",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	}
	assert.Equal(t, "postgres", s.Driver)
	assert.Equal(t, 5, s.MaxIdleConns)
}

func TestMemoryConfigSnapshot_Fields(t *testing.T) {
	s := MemoryConfigSnapshot{
		Backend:  "neo4j",
		FilePath: "./mem.gml",
		Neo4j: &Neo4jConfigSnapshot{
			URI:      "bolt://localhost:7687",
			User:     "neo4j",
			Password: "pass",
		},
		Postgres: &PostgresConfigSnapshot{DSN: "postgres://localhost/mem"},
	}
	assert.Equal(t, "neo4j", s.Backend)
	assert.Equal(t, "bolt://localhost:7687", s.Neo4j.URI)
	assert.Equal(t, "postgres://localhost/mem", s.Postgres.DSN)
}

func TestSubagentsConfigSnapshot_Fields(t *testing.T) {
	s := SubagentsConfigSnapshot{MaxConcurrent: 4, DefaultTimeout: "30s"}
	assert.Equal(t, 4, s.MaxConcurrent)
}

func TestGraphQLConfigSnapshot_Fields(t *testing.T) {
	s := GraphQLConfigSnapshot{Enabled: true, Port: 8080, Host: "0.0.0.0", BaseURL: "https://app.example.com"}
	assert.Equal(t, 8080, s.Port)
}

func TestLoggingConfigSnapshot_Fields(t *testing.T) {
	s := LoggingConfigSnapshot{Level: "debug", Path: "./app.log"}
	assert.Equal(t, "debug", s.Level)
}

func TestSecretsConfigSnapshot_Fields(t *testing.T) {
	s := SecretsConfigSnapshot{
		Backend: "openbao",
		File:    &FileSecretsSnapshot{Path: "./secrets.json"},
		Openbao: &OpenbaoSecretsSnapshot{URL: "https://vault.example.com", Token: "hvs.token"},
	}
	assert.Equal(t, "openbao", s.Backend)
	assert.Equal(t, "hvs.token", s.Openbao.Token)
}

func TestSchedulerConfigSnapshot_Fields(t *testing.T) {
	s := SchedulerConfigSnapshot{Enabled: true, MemoryEnabled: true, MemoryInterval: "5m"}
	assert.Equal(t, "5m", s.MemoryInterval)
}

func TestActiveSessionSnapshot_Fields(t *testing.T) {
	s := ActiveSessionSnapshot{ID: "sess1", Address: "1.2.3.4", Status: "active", Channel: "telegram", User: "alice"}
	assert.Equal(t, "sess1", s.ID)
}

func TestChannelConfigSnapshot_Fields(t *testing.T) {
	s := ChannelConfigSnapshot{ChannelID: "ch1", ChannelName: "Telegram", Enabled: true}
	assert.True(t, s.Enabled)
}

func TestChannelSecretsSnapshot_Fields(t *testing.T) {
	s := ChannelSecretsSnapshot{
		TelegramEnabled:  true,
		TelegramToken:    "tg-token",
		DiscordEnabled:   false,
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
	}
	assert.True(t, s.TelegramEnabled)
	assert.Equal(t, "AC123", s.TwilioAccountSid)
}
