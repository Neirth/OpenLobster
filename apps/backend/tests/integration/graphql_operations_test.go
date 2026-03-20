// Integration tests for all GraphQL queries and mutations.
// Each test exercises the full HTTP → handler → resolver → deps chain
// using in-memory databases or lightweight mock implementations.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/application/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/repositories"
	svcdashboard "github.com/neirth/openlobster/internal/domain/services/dashboard"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func gqlPost(t *testing.T, handler http.Handler, query string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(query))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "unexpected HTTP status: %s", w.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "failed to parse response: %s", w.Body.String())
	if errs, ok := resp["errors"]; ok {
		t.Logf("GraphQL errors: %v", errs)
	}
	return resp
}

func dataOf(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	d, ok := resp["data"].(map[string]interface{})
	require.True(t, ok, "response has no data field: %+v", resp)
	return d
}

func setupDB(t *testing.T) *persistence.Database {
	t.Helper()
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ─── mock implementations ─────────────────────────────────────────────────────

type stubSkillsPort struct {
	skills []dto.SkillSnapshot
}

func (s *stubSkillsPort) ListSkills() ([]dto.SkillSnapshot, error) { return s.skills, nil }
func (s *stubSkillsPort) EnableSkill(name string) error            { return nil }
func (s *stubSkillsPort) DisableSkill(name string) error           { return nil }
func (s *stubSkillsPort) DeleteSkill(name string) error            { return nil }
func (s *stubSkillsPort) ImportSkill(data []byte) error            { return nil }

type stubSysFilesPort struct {
	files []dto.SystemFileSnapshot
}

func (s *stubSysFilesPort) ListFiles() ([]dto.SystemFileSnapshot, error) { return s.files, nil }
func (s *stubSysFilesPort) WriteFile(name, content string) error         { return nil }

type stubToolPermRepo struct {
	perms []dto.ToolPermissionRecord
}

func (s *stubToolPermRepo) Set(ctx context.Context, userID, toolName, mode string) error {
	for i, p := range s.perms {
		if p.UserID == userID && p.ToolName == toolName {
			s.perms[i].Mode = mode
			return nil
		}
	}
	s.perms = append(s.perms, dto.ToolPermissionRecord{UserID: userID, ToolName: toolName, Mode: mode})
	return nil
}
func (s *stubToolPermRepo) Delete(ctx context.Context, userID, toolName string) error {
	out := s.perms[:0]
	for _, p := range s.perms {
		if p.UserID != userID || p.ToolName != toolName {
			out = append(out, p)
		}
	}
	s.perms = out
	return nil
}
func (s *stubToolPermRepo) ListByUser(ctx context.Context, userID string) ([]dto.ToolPermissionRecord, error) {
	var out []dto.ToolPermissionRecord
	for _, p := range s.perms {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (s *stubToolPermRepo) ListAll(ctx context.Context) ([]dto.ToolPermissionRecord, error) {
	return s.perms, nil
}

type stubPairingPort struct {
	active []dto.PairingSnapshot
}

func (s *stubPairingPort) ListActive(ctx context.Context) ([]dto.PairingSnapshot, error) {
	return s.active, nil
}
func (s *stubPairingPort) Approve(ctx context.Context, code, userID, displayName string) (*dto.PairingSnapshot, error) {
	for _, p := range s.active {
		if p.Code == code {
			snap := p
			return &snap, nil
		}
	}
	return nil, nil
}
func (s *stubPairingPort) Deny(ctx context.Context, code, reason string) error { return nil }

type stubUserRepo struct {
	users []models.User
}

func (s *stubUserRepo) Create(ctx context.Context, user *models.User) error {
	s.users = append(s.users, *user)
	return nil
}
func (s *stubUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	for _, u := range s.users {
		if u.ID.String() == id {
			cp := u
			return &cp, nil
		}
	}
	return nil, nil
}
func (s *stubUserRepo) ListAll(ctx context.Context) ([]models.User, error) { return s.users, nil }

type stubUserChannelRepo struct{}

func (s *stubUserChannelRepo) ExistsByPlatformUserID(ctx context.Context, platformUserID string) (bool, error) {
	return false, nil
}
func (s *stubUserChannelRepo) Create(ctx context.Context, userID, channelType, platformUserID, username string) error {
	return nil
}
func (s *stubUserChannelRepo) GetDisplayNameByUserID(ctx context.Context, userID string) (string, error) {
	return "DisplayName", nil
}

type stubMCPServerRepo struct {
	servers []dto.MCPServerRecord
}

func (s *stubMCPServerRepo) Save(ctx context.Context, name, url string) error {
	s.servers = append(s.servers, dto.MCPServerRecord{Name: name, URL: url})
	return nil
}
func (s *stubMCPServerRepo) Delete(ctx context.Context, name string) error {
	out := s.servers[:0]
	for _, srv := range s.servers {
		if srv.Name != name {
			out = append(out, srv)
		}
	}
	s.servers = out
	return nil
}
func (s *stubMCPServerRepo) ListAll(ctx context.Context) ([]dto.MCPServerRecord, error) {
	return s.servers, nil
}

type stubMcpConnectPort struct {
	connected map[string]bool
}

func (s *stubMcpConnectPort) Connect(ctx context.Context, name, transport, url string) (bool, error) {
	if s.connected == nil {
		s.connected = make(map[string]bool)
	}
	s.connected[name] = true
	return false, nil
}
func (s *stubMcpConnectPort) Disconnect(ctx context.Context, name string) error {
	if s.connected != nil {
		delete(s.connected, name)
	}
	return nil
}
func (s *stubMcpConnectPort) GetConnectionStatus(name string) string {
	if s.connected[name] {
		return "online"
	}
	return "unknown"
}
func (s *stubMcpConnectPort) GetServerToolCount(name string) int { return 0 }

type stubMcpOAuthPort struct{}

func (s *stubMcpOAuthPort) InitiateOAuth(ctx context.Context, serverName, mcpURL string) (string, error) {
	return "https://example.com/oauth", nil
}
func (s *stubMcpOAuthPort) Status(serverName string) (string, string) { return "none", "" }
func (s *stubMcpOAuthPort) SetClientID(ctx context.Context, serverName, clientID string) error {
	return nil
}

type stubSubAgentSvc struct {
	agents []dto.SubAgentSnapshot
}

func (s *stubSubAgentSvc) List(ctx context.Context) ([]dto.SubAgentSnapshot, error) {
	return s.agents, nil
}
func (s *stubSubAgentSvc) Spawn(ctx context.Context, name, model, task string) (string, error) {
	id := uuid.New().String()
	s.agents = append(s.agents, dto.SubAgentSnapshot{ID: id, Name: name, Status: "running", Task: task})
	return id, nil
}
func (s *stubSubAgentSvc) Kill(ctx context.Context, id string) error {
	out := s.agents[:0]
	for _, a := range s.agents {
		if a.ID != id {
			out = append(out, a)
		}
	}
	s.agents = out
	return nil
}

type stubConvPort struct {
	convs []dto.ConversationSnapshot
}

func (s *stubConvPort) ListConversations() ([]dto.ConversationSnapshot, error) {
	return s.convs, nil
}
func (s *stubConvPort) DeleteUser(ctx context.Context, conversationID string) error { return nil }

type stubMsgRepo struct {
	msgs []models.Message
}

func (s *stubMsgRepo) Save(ctx context.Context, msg *models.Message) error {
	s.msgs = append(s.msgs, *msg)
	return nil
}
func (s *stubMsgRepo) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Message, error) {
	var out []models.Message
	for _, m := range s.msgs {
		if m.ConversationID == conversationID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (s *stubMsgRepo) GetByConversationPaged(ctx context.Context, conversationID string, before *string, limit int) ([]models.Message, error) {
	return s.GetByConversation(ctx, conversationID, limit)
}
func (s *stubMsgRepo) GetSinceLastCompaction(ctx context.Context, conversationID string) ([]models.Message, error) {
	return s.GetByConversation(ctx, conversationID, 100)
}
func (s *stubMsgRepo) CountMessages(ctx context.Context) (int64, int64, error) {
	return int64(len(s.msgs)), 0, nil
}

type stubConfigWriter struct {
	applied map[string]interface{}
}

func (s *stubConfigWriter) Apply(ctx context.Context, input map[string]interface{}) ([]string, error) {
	if s.applied == nil {
		s.applied = make(map[string]interface{})
	}
	for k, v := range input {
		s.applied[k] = v
	}
	return nil, nil
}

// ─── Query: agent ─────────────────────────────────────────────────────────────

func TestIntegration_Query_Agent(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:          "MyAgent",
			AIProvider:    "anthropic",
			MemoryBackend: "file",
			Status:        "ok",
			Version:       "1.0.0",
		},
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ agent { name aiProvider memoryBackend status version } }"}`)
	agent := dataOf(t, resp)["agent"].(map[string]interface{})
	assert.Equal(t, "MyAgent", agent["name"])
	assert.Equal(t, "anthropic", agent["aiProvider"])
	assert.Equal(t, "ok", agent["status"])
}

// ─── Query: channels ──────────────────────────────────────────────────────────

func TestIntegration_Query_Channels(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:   "Bot",
			Status: "ok",
			Channels: []dto.ChannelStatus{
				{Type: "telegram", Enabled: true},
				{Type: "slack", Enabled: false},
			},
		},
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ channels { type enabled } }"}`)
	channels := dataOf(t, resp)["channels"].([]interface{})
	assert.Len(t, channels, 2)
	first := channels[0].(map[string]interface{})
	assert.Equal(t, "telegram", first["type"])
	assert.Equal(t, true, first["enabled"])
}

// ─── Query: heartbeat ─────────────────────────────────────────────────────────

func TestIntegration_Query_Heartbeat(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ heartbeat { status lastCheck } }"}`)
	hb := dataOf(t, resp)["heartbeat"].(map[string]interface{})
	assert.Equal(t, "ok", hb["status"])
	assert.NotNil(t, hb["lastCheck"])
}

// ─── Query: tools (mcpTools) ─────────────────────────────────────────────────

func TestIntegration_Query_McpTools_Empty(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ mcpTools { name description serverName } }"}`)
	tools := dataOf(t, resp)["mcpTools"].([]interface{})
	assert.Empty(t, tools)
}

// ─── Query: subAgents ─────────────────────────────────────────────────────────

func TestIntegration_Query_SubAgents(t *testing.T) {
	subSvc := &stubSubAgentSvc{
		agents: []dto.SubAgentSnapshot{
			{ID: "sa-1", Name: "Worker", Status: "running", Task: "do stuff"},
		},
	}
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	deps := &resolvers.Deps{AgentRegistry: reg, SubAgentSvc: subSvc}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ subAgents { id name status task } }"}`)
	agents := dataOf(t, resp)["subAgents"].([]interface{})
	require.Len(t, agents, 1)
	assert.Equal(t, "Worker", agents[0].(map[string]interface{})["name"])
}

// ─── Query: status ────────────────────────────────────────────────────────────

func TestIntegration_Query_Status(t *testing.T) {
	db := setupDB(t)
	taskRepo := repositories.NewTaskRepository(db.GormDB())
	qs := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	cs := svcdashboard.NewCommandService(taskRepo, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		TaskRepo:   taskRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ status { agent { name } health { status } tasks { id } } }"}`)
	status := dataOf(t, resp)["status"].(map[string]interface{})
	assert.NotNil(t, status["agent"])
	assert.NotNil(t, status["health"])
}

// ─── Query: metrics ───────────────────────────────────────────────────────────

func TestIntegration_Query_Metrics(t *testing.T) {
	db := setupDB(t)
	taskRepo := repositories.NewTaskRepository(db.GormDB())
	qs := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:    &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc: qs,
		TaskRepo: taskRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ metrics { uptime tasksPending tasksDone mcpTools } }"}`)
	metrics := dataOf(t, resp)["metrics"].(map[string]interface{})
	assert.NotNil(t, metrics["uptime"])
	assert.Equal(t, float64(0), metrics["tasksPending"])
}

// ─── Query: config ────────────────────────────────────────────────────────────

func TestIntegration_Query_Config(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
	})
	snap := &dto.AppConfigSnapshot{
		Agent: &dto.AgentConfigSnapshot{
			Name:     "Bot",
			Provider: "openai",
			Model:    "gpt-4o",
			APIKey:   "sk-test",
		},
		Capabilities: &dto.CapabilitiesSnapshot{Browser: true, Terminal: false},
		Database:     &dto.DatabaseConfigSnapshot{Driver: "sqlite3", DSN: ":memory:"},
		Memory:       &dto.MemoryConfigSnapshot{Backend: "file"},
		Subagents:    &dto.SubagentsConfigSnapshot{MaxConcurrent: 3},
		GraphQL:      &dto.GraphQLConfigSnapshot{Enabled: true, Port: 8080},
		Logging:      &dto.LoggingConfigSnapshot{Level: "info"},
		Scheduler:    &dto.SchedulerConfigSnapshot{Enabled: true},
		Secrets:      &dto.SecretsConfigSnapshot{Backend: "file", File: &dto.FileSecretsSnapshot{Path: "/tmp/secrets"}},
		ChannelSecrets: &dto.ChannelSecretsSnapshot{
			TelegramEnabled: true, TelegramToken: "tok",
			DiscordEnabled: false,
			WhatsAppEnabled: false,
			TwilioEnabled:  false,
			SlackEnabled:   false,
		},
		WizardCompleted: true,
	}
	deps.ConfigSnapshot = snap
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ config { agent { name provider model } capabilities { browser terminal } database { driver } wizardCompleted } }"}`)
	cfg := dataOf(t, resp)["config"].(map[string]interface{})
	agent := cfg["agent"].(map[string]interface{})
	assert.Equal(t, "Bot", agent["name"])
	assert.Equal(t, "openai", agent["provider"])
	assert.Equal(t, "gpt-4o", agent["model"])
	caps := cfg["capabilities"].(map[string]interface{})
	assert.Equal(t, true, caps["browser"])
	assert.Equal(t, false, caps["terminal"])
	assert.Equal(t, true, cfg["wizardCompleted"])
}

// ─── Query: config – channel secrets ─────────────────────────────────────────

func TestIntegration_Query_Config_ChannelSecrets(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
	})
	deps.ConfigSnapshot = &dto.AppConfigSnapshot{
		Agent:       &dto.AgentConfigSnapshot{Name: "Bot"},
		Capabilities: &dto.CapabilitiesSnapshot{},
		Database:    &dto.DatabaseConfigSnapshot{},
		Memory:      &dto.MemoryConfigSnapshot{},
		Subagents:   &dto.SubagentsConfigSnapshot{},
		GraphQL:     &dto.GraphQLConfigSnapshot{},
		Logging:     &dto.LoggingConfigSnapshot{},
		Scheduler:   &dto.SchedulerConfigSnapshot{},
		Secrets:     &dto.SecretsConfigSnapshot{File: &dto.FileSecretsSnapshot{}},
		ChannelSecrets: &dto.ChannelSecretsSnapshot{
			TelegramEnabled:  true,
			TelegramToken:    "tg-token",
			DiscordEnabled:   true,
			DiscordToken:     "dc-token",
			WhatsAppEnabled:  true,
			WhatsAppPhoneId:  "phone-123",
			WhatsAppApiToken: "wa-token",
			TwilioEnabled:    true,
			TwilioAccountSid: "AC123",
			TwilioAuthToken:  "auth-t",
			TwilioFromNumber: "+1234567890",
			SlackEnabled:     true,
			SlackBotToken:    "xoxb-123",
			SlackAppToken:    "xapp-123",
		},
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ config { channelSecrets { telegramEnabled telegramToken discordEnabled discordToken whatsAppEnabled whatsAppPhoneId whatsAppApiToken twilioEnabled twilioAccountSid twilioAuthToken twilioFromNumber slackEnabled slackBotToken slackAppToken } } }"}`)
	cs := dataOf(t, resp)["config"].(map[string]interface{})["channelSecrets"].(map[string]interface{})
	assert.Equal(t, true, cs["telegramEnabled"])
	assert.Equal(t, "tg-token", cs["telegramToken"])
	assert.Equal(t, true, cs["discordEnabled"])
	assert.Equal(t, "dc-token", cs["discordToken"])
	assert.Equal(t, true, cs["whatsAppEnabled"])
	assert.Equal(t, "phone-123", cs["whatsAppPhoneId"])
	assert.Equal(t, "wa-token", cs["whatsAppApiToken"])
	assert.Equal(t, true, cs["twilioEnabled"])
	assert.Equal(t, "AC123", cs["twilioAccountSid"])
	assert.Equal(t, "auth-t", cs["twilioAuthToken"])
	assert.Equal(t, "+1234567890", cs["twilioFromNumber"])
	assert.Equal(t, true, cs["slackEnabled"])
	assert.Equal(t, "xoxb-123", cs["slackBotToken"])
	assert.Equal(t, "xapp-123", cs["slackAppToken"])
}

// ─── Query: conversations ─────────────────────────────────────────────────────

func TestIntegration_Query_Conversations(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	convPort := &stubConvPort{
		convs: []dto.ConversationSnapshot{
			{ID: "conv-1", ChannelID: "ch-1", ChannelType: "telegram"},
		},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, ConvPort: convPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ conversations { id channelId channelName } }"}`)
	convs := dataOf(t, resp)["conversations"].([]interface{})
	require.Len(t, convs, 1)
	assert.Equal(t, "conv-1", convs[0].(map[string]interface{})["id"])
}

// ─── Query: messages ─────────────────────────────────────────────────────────

func TestIntegration_Query_Messages(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	msgRepo := &stubMsgRepo{}
	_ = msgRepo.Save(context.Background(), &models.Message{
		ID:             uuid.New(),
		ConversationID: "conv-1",
		Role:           "user",
		Content:        "hello",
	})
	deps := &resolvers.Deps{AgentRegistry: reg, MsgRepo: msgRepo}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ messages(conversationId: \"conv-1\") { id role content } }"}`)
	msgs := dataOf(t, resp)["messages"].([]interface{})
	require.Len(t, msgs, 1)
	assert.Equal(t, "user", msgs[0].(map[string]interface{})["role"])
	assert.Equal(t, "hello", msgs[0].(map[string]interface{})["content"])
}

// ─── Query: tasks ─────────────────────────────────────────────────────────────

func TestIntegration_Query_Tasks_WithData(t *testing.T) {
	db := setupDB(t)
	taskRepo := repositories.NewTaskRepository(db.GormDB())
	require.NoError(t, taskRepo.Add(context.Background(), &models.Task{
		ID: "task-x", Prompt: "Do something", Status: "pending",
	}))
	qs := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	cs := svcdashboard.NewCommandService(taskRepo, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		TaskRepo:   taskRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ tasks { id prompt status } }"}`)
	tasks := dataOf(t, resp)["tasks"].([]interface{})
	require.Len(t, tasks, 1)
	assert.Equal(t, "Do something", tasks[0].(map[string]interface{})["prompt"])
}

// ─── Query: searchMemory ─────────────────────────────────────────────────────

func TestIntegration_Query_SearchMemory(t *testing.T) {
	memRepo := &graphql.TestMemoryRepo{}
	_ = memRepo.AddKnowledge(context.Background(), "user1", "Go is great", "fact", "KNOWS", "fact", nil)
	qs := svcdashboard.NewQueryService(nil, memRepo, nil, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		MemoryRepo: memRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ searchMemory(query: \"Go\") { success result } }"}`)
	sm := dataOf(t, resp)["searchMemory"].(map[string]interface{})
	assert.Equal(t, true, sm["success"])
}

// ─── Query: userGraph ─────────────────────────────────────────────────────────

func TestIntegration_Query_UserGraph(t *testing.T) {
	gr := &graphql.TestGraphRepo{
		GetUserGraphFunc: func(ctx context.Context, userID string) (ports.Graph, error) {
			return ports.Graph{
				Nodes: []ports.GraphNode{{ID: "n1", Label: "User", Type: "user", Value: userID}},
				Edges: []ports.GraphEdge{},
			}, nil
		},
	}
	qs := svcdashboard.NewQueryService(nil, nil, gr, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:    &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc: qs,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ userGraph(userId: \"u1\") { success nodes { id label } edges { source target } } }"}`)
	ug := dataOf(t, resp)["userGraph"].(map[string]interface{})
	assert.Equal(t, true, ug["success"])
	nodes := ug["nodes"].([]interface{})
	require.Len(t, nodes, 1)
	assert.Equal(t, "n1", nodes[0].(map[string]interface{})["id"])
}

// ─── Query: memory (memoryGraph) ─────────────────────────────────────────────

func TestIntegration_Query_Memory(t *testing.T) {
	gr := &graphql.TestGraphRepo{}
	qs := svcdashboard.NewQueryService(nil, nil, gr, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:    &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc: qs,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ memory { nodes { id } edges { sourceId } } }"}`)
	mem := dataOf(t, resp)["memory"].(map[string]interface{})
	assert.NotNil(t, mem["nodes"])
}

// ─── Query: toolPermissions ──────────────────────────────────────────────────

func TestIntegration_Query_ToolPermissions(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	permRepo := &stubToolPermRepo{
		perms: []dto.ToolPermissionRecord{
			{UserID: "u1", ToolName: "browser_fetch", Mode: "allow"},
		},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, ToolPermRepo: permRepo}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ toolPermissions(userId: \"u1\") { toolName mode } }"}`)
	perms := dataOf(t, resp)["toolPermissions"].([]interface{})
	require.Len(t, perms, 1)
	p := perms[0].(map[string]interface{})
	assert.Equal(t, "browser_fetch", p["toolName"])
	assert.Equal(t, "allow", p["mode"])
}

// ─── Query: mcpUsers ─────────────────────────────────────────────────────────

func TestIntegration_Query_McpUsers(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "MyBot", Status: "ok"})
	userRepo := &stubUserRepo{
		users: []models.User{{ID: uuid.New(), PrimaryID: "ext-user-1"}},
	}
	deps := &resolvers.Deps{
		AgentRegistry:   reg,
		UserRepo:        userRepo,
		UserChannelRepo: &stubUserChannelRepo{},
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ mcpUsers { channelId displayName isAgent } }"}`)
	users := dataOf(t, resp)["mcpUsers"].([]interface{})
	// At least the loopback user
	require.GreaterOrEqual(t, len(users), 1)
	loopback := users[0].(map[string]interface{})
	assert.Equal(t, "loopback", loopback["channelId"])
	assert.Equal(t, true, loopback["isAgent"])
}

// ─── Query: pendingPairings ──────────────────────────────────────────────────

func TestIntegration_Query_PendingPairings(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	pairingPort := &stubPairingPort{
		active: []dto.PairingSnapshot{
			{Code: "ABC123", ChannelType: "telegram", Status: "pending"},
		},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, PairingPort: pairingPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ pendingPairings { code channelType status } }"}`)
	pairs := dataOf(t, resp)["pendingPairings"].([]interface{})
	require.Len(t, pairs, 1)
	assert.Equal(t, "ABC123", pairs[0].(map[string]interface{})["code"])
}

// ─── Query: users ─────────────────────────────────────────────────────────────

func TestIntegration_Query_Users(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	userID := uuid.New()
	userRepo := &stubUserRepo{users: []models.User{{ID: userID, PrimaryID: "user@example.com"}}}
	deps := &resolvers.Deps{
		AgentRegistry:   reg,
		UserRepo:        userRepo,
		UserChannelRepo: &stubUserChannelRepo{},
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ users { id primaryID } }"}`)
	users := dataOf(t, resp)["users"].([]interface{})
	require.Len(t, users, 1)
	assert.Equal(t, userID.String(), users[0].(map[string]interface{})["id"])
}

// ─── Query: mcps ─────────────────────────────────────────────────────────────

func TestIntegration_Query_Mcps(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ mcps { name type status } }"}`)
	mcps := dataOf(t, resp)["mcps"].([]interface{})
	assert.Empty(t, mcps) // no MCPs wired; expect empty
}

// ─── Query: mcpServers ────────────────────────────────────────────────────────

func TestIntegration_Query_McpServers(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	mcpRepo := &stubMCPServerRepo{
		servers: []dto.MCPServerRecord{{Name: "my-server", URL: "http://localhost:3000"}},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, MCPServerRepo: mcpRepo}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ mcpServers { name url status toolCount } }"}`)
	servers := dataOf(t, resp)["mcpServers"].([]interface{})
	require.Len(t, servers, 1)
	assert.Equal(t, "my-server", servers[0].(map[string]interface{})["name"])
}

// ─── Query: mcpOAuthStatus ────────────────────────────────────────────────────

func TestIntegration_Query_McpOAuthStatus(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	deps := &resolvers.Deps{AgentRegistry: reg, McpOAuthPort: &stubMcpOAuthPort{}}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ mcpOAuthStatus(name: \"my-server\") { status } }"}`)
	oauth := dataOf(t, resp)["mcpOAuthStatus"].(map[string]interface{})
	assert.Equal(t, "none", oauth["status"])
}

// ─── Query: skills ────────────────────────────────────────────────────────────

func TestIntegration_Query_Skills(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	skillsPort := &stubSkillsPort{
		skills: []dto.SkillSnapshot{
			{Name: "my-skill", Description: "does stuff", Enabled: true},
		},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, SkillsPort: skillsPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ skills { name description enabled } }"}`)
	skills := dataOf(t, resp)["skills"].([]interface{})
	require.Len(t, skills, 1)
	assert.Equal(t, "my-skill", skills[0].(map[string]interface{})["name"])
	assert.Equal(t, true, skills[0].(map[string]interface{})["enabled"])
}

// ─── Query: systemFiles ───────────────────────────────────────────────────────

func TestIntegration_Query_SystemFiles(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	sysFiles := &stubSysFilesPort{
		files: []dto.SystemFileSnapshot{
			{Name: "system_prompt.txt", Content: "You are helpful"},
		},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, SysFilesPort: sysFiles}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"{ systemFiles { name content } }"}`)
	files := dataOf(t, resp)["systemFiles"].([]interface{})
	require.Len(t, files, 1)
	assert.Equal(t, "system_prompt.txt", files[0].(map[string]interface{})["name"])
}

// ─── Mutation: updateConfig – agent fields ────────────────────────────────────

func TestIntegration_Mutation_UpdateConfig_AgentFields(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	configWriter := &stubConfigWriter{}
	snap := &dto.AppConfigSnapshot{
		Agent:        &dto.AgentConfigSnapshot{Name: "Bot", Provider: "openai"},
		Capabilities: &dto.CapabilitiesSnapshot{},
		Database:     &dto.DatabaseConfigSnapshot{},
		Memory:       &dto.MemoryConfigSnapshot{},
		Subagents:    &dto.SubagentsConfigSnapshot{},
		GraphQL:      &dto.GraphQLConfigSnapshot{},
		Logging:      &dto.LoggingConfigSnapshot{},
		Scheduler:    &dto.SchedulerConfigSnapshot{},
		Secrets:      &dto.SecretsConfigSnapshot{File: &dto.FileSecretsSnapshot{}},
		ChannelSecrets: &dto.ChannelSecretsSnapshot{},
	}
	deps := &resolvers.Deps{
		AgentRegistry:  reg,
		ConfigSnapshot: snap,
		ConfigWriter:   configWriter,
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateConfig(input: { agentName: \"NewName\", systemPrompt: \"Be helpful\" }) { agentName systemPrompt } }"}`)
	assert.NoError(t, firstError(resp))
	assert.Contains(t, configWriter.applied, "agentName")
	assert.Equal(t, "NewName", configWriter.applied["agentName"])
	assert.Equal(t, "Be helpful", configWriter.applied["systemPrompt"])
}

// ─── Mutation: updateConfig – provider openai ────────────────────────────────

func TestIntegration_Mutation_UpdateConfig_ProviderOpenAI(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	configWriter := &stubConfigWriter{}
	deps := &resolvers.Deps{
		AgentRegistry:  reg,
		ConfigWriter:   configWriter,
		ConfigSnapshot: minimalSnapshot(),
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateConfig(input: { provider: \"openai\", apiKey: \"sk-openai\", model: \"gpt-4o\" }) { provider } }"}`)
	assert.NoError(t, firstError(resp))
	assert.Equal(t, "openai", configWriter.applied["provider"])
	assert.Equal(t, "sk-openai", configWriter.applied["apiKey"])
	assert.Equal(t, "gpt-4o", configWriter.applied["model"])
}

// ─── Mutation: updateConfig – provider anthropic ─────────────────────────────

func TestIntegration_Mutation_UpdateConfig_ProviderAnthropic(t *testing.T) {
	configWriter := &stubConfigWriter{}
	deps := &resolvers.Deps{
		AgentRegistry:  newReg("Bot"),
		ConfigWriter:   configWriter,
		ConfigSnapshot: minimalSnapshot(),
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateConfig(input: { provider: \"anthropic\", anthropicApiKey: \"sk-ant\", model: \"claude-sonnet-4-6\" }) { provider } }"}`)
	assert.NoError(t, firstError(resp))
	assert.Equal(t, "anthropic", configWriter.applied["provider"])
	assert.Equal(t, "sk-ant", configWriter.applied["anthropicApiKey"])
}

// ─── Mutation: updateConfig – provider ollama ────────────────────────────────

func TestIntegration_Mutation_UpdateConfig_ProviderOllama(t *testing.T) {
	configWriter := &stubConfigWriter{}
	deps := &resolvers.Deps{
		AgentRegistry:  newReg("Bot"),
		ConfigWriter:   configWriter,
		ConfigSnapshot: minimalSnapshot(),
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateConfig(input: { provider: \"ollama\", ollamaHost: \"http://localhost:11434\", model: \"llama3\" }) { provider } }"}`)
	assert.NoError(t, firstError(resp))
	assert.Equal(t, "ollama", configWriter.applied["provider"])
	assert.Equal(t, "http://localhost:11434", configWriter.applied["ollamaHost"])
}

// ─── Mutation: updateConfig – capabilities ───────────────────────────────────

func TestIntegration_Mutation_UpdateConfig_Capabilities(t *testing.T) {
	configWriter := &stubConfigWriter{}
	deps := &resolvers.Deps{
		AgentRegistry:  newReg("Bot"),
		ConfigWriter:   configWriter,
		ConfigSnapshot: minimalSnapshot(),
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateConfig(input: { capabilities: { browser: true, terminal: false, memory: true } }) { agentName } }"}`)
	assert.NoError(t, firstError(resp))
	assert.Contains(t, configWriter.applied, "capabilities")
}

// ─── Mutation: updateConfig – channel secrets ────────────────────────────────

func TestIntegration_Mutation_UpdateConfig_ChannelSecrets(t *testing.T) {
	configWriter := &stubConfigWriter{}
	deps := &resolvers.Deps{
		AgentRegistry:  newReg("Bot"),
		ConfigWriter:   configWriter,
		ConfigSnapshot: minimalSnapshot(),
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateConfig(input: { channelTelegramEnabled: true, channelTelegramToken: \"tg-tok\", channelDiscordEnabled: false, channelDiscordToken: \"dc-tok\", channelWhatsAppEnabled: true, channelWhatsAppPhoneId: \"p123\", channelWhatsAppApiToken: \"wa-tok\", channelTwilioEnabled: true, channelTwilioAccountSid: \"AC123\", channelTwilioAuthToken: \"auth\", channelTwilioFromNumber: \"+1\", channelSlackEnabled: false, channelSlackBotToken: \"xoxb\", channelSlackAppToken: \"xapp\" }) { agentName } }"}`)
	assert.NoError(t, firstError(resp))
	assert.Equal(t, true, configWriter.applied["channelTelegramEnabled"])
	assert.Equal(t, "tg-tok", configWriter.applied["channelTelegramToken"])
	assert.Equal(t, false, configWriter.applied["channelDiscordEnabled"])
	assert.Equal(t, "dc-tok", configWriter.applied["channelDiscordToken"])
	assert.Equal(t, true, configWriter.applied["channelWhatsAppEnabled"])
	assert.Equal(t, "p123", configWriter.applied["channelWhatsAppPhoneId"])
	assert.Equal(t, "wa-tok", configWriter.applied["channelWhatsAppApiToken"])
	assert.Equal(t, true, configWriter.applied["channelTwilioEnabled"])
	assert.Equal(t, "AC123", configWriter.applied["channelTwilioAccountSid"])
	assert.Equal(t, "auth", configWriter.applied["channelTwilioAuthToken"])
	assert.Equal(t, "+1", configWriter.applied["channelTwilioFromNumber"])
	assert.Equal(t, false, configWriter.applied["channelSlackEnabled"])
	assert.Equal(t, "xoxb", configWriter.applied["channelSlackBotToken"])
	assert.Equal(t, "xapp", configWriter.applied["channelSlackAppToken"])
}

// ─── Mutation: updateConfig – logging & database ─────────────────────────────

func TestIntegration_Mutation_UpdateConfig_LoggingAndDatabase(t *testing.T) {
	configWriter := &stubConfigWriter{}
	deps := &resolvers.Deps{
		AgentRegistry:  newReg("Bot"),
		ConfigWriter:   configWriter,
		ConfigSnapshot: minimalSnapshot(),
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateConfig(input: { loggingLevel: \"debug\", loggingPath: \"/var/log/app.log\", databaseDriver: \"sqlite3\", databaseDSN: \"file:test.db\" }) { agentName } }"}`)
	assert.NoError(t, firstError(resp))
	assert.Equal(t, "debug", configWriter.applied["loggingLevel"])
	assert.Equal(t, "/var/log/app.log", configWriter.applied["loggingPath"])
	assert.Equal(t, "sqlite3", configWriter.applied["databaseDriver"])
	assert.Equal(t, "file:test.db", configWriter.applied["databaseDSN"])
}

// ─── Mutation: updateConfig – wizard completed ────────────────────────────────

func TestIntegration_Mutation_UpdateConfig_WizardCompleted(t *testing.T) {
	configWriter := &stubConfigWriter{}
	deps := &resolvers.Deps{
		AgentRegistry:  newReg("Bot"),
		ConfigWriter:   configWriter,
		ConfigSnapshot: minimalSnapshot(),
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateConfig(input: { wizardCompleted: true }) { agentName } }"}`)
	assert.NoError(t, firstError(resp))
	assert.Equal(t, true, configWriter.applied["wizardCompleted"])
}

// ─── Mutation: addTask ────────────────────────────────────────────────────────

func TestIntegration_Mutation_AddTask(t *testing.T) {
	db := setupDB(t)
	taskRepo := repositories.NewTaskRepository(db.GormDB())
	qs := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	cs := svcdashboard.NewCommandService(taskRepo, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		TaskRepo:   taskRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { addTask(prompt: \"Write report\") { id prompt status } }"}`)
	addTask := dataOf(t, resp)["addTask"].(map[string]interface{})
	assert.NotEmpty(t, addTask["id"])
	assert.Equal(t, "Write report", addTask["prompt"])
}

// ─── Mutation: removeTask ─────────────────────────────────────────────────────

func TestIntegration_Mutation_RemoveTask(t *testing.T) {
	db := setupDB(t)
	taskRepo := repositories.NewTaskRepository(db.GormDB())
	require.NoError(t, taskRepo.Add(context.Background(), &models.Task{ID: "del-1", Prompt: "to remove", Status: "pending"}))
	qs := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	cs := svcdashboard.NewCommandService(taskRepo, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		TaskRepo:   taskRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { removeTask(taskId: \"del-1\") }"}`)
	assert.Equal(t, true, dataOf(t, resp)["removeTask"])
}

// ─── Mutation: updateTask ─────────────────────────────────────────────────────

func TestIntegration_Mutation_UpdateTask(t *testing.T) {
	db := setupDB(t)
	taskRepo := repositories.NewTaskRepository(db.GormDB())
	require.NoError(t, taskRepo.Add(context.Background(), &models.Task{ID: "upd-1", Prompt: "old", Status: "pending"}))
	qs := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	cs := svcdashboard.NewCommandService(taskRepo, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		TaskRepo:   taskRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateTask(id: \"upd-1\", prompt: \"new prompt\") { id prompt } }"}`)
	result := dataOf(t, resp)["updateTask"].(map[string]interface{})
	assert.Equal(t, "upd-1", result["id"])
}

// ─── Mutation: toggleTask ─────────────────────────────────────────────────────

func TestIntegration_Mutation_ToggleTask(t *testing.T) {
	db := setupDB(t)
	taskRepo := repositories.NewTaskRepository(db.GormDB())
	require.NoError(t, taskRepo.Add(context.Background(), &models.Task{ID: "tog-1", Prompt: "cyclic", Status: "pending", TaskType: "cyclic", Enabled: true}))
	qs := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	cs := svcdashboard.NewCommandService(taskRepo, nil, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		TaskRepo:   taskRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { toggleTask(id: \"tog-1\", enabled: false) { success } }"}`)
	result := dataOf(t, resp)["toggleTask"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
}

// ─── Mutation: addMemory ─────────────────────────────────────────────────────

func TestIntegration_Mutation_AddMemory(t *testing.T) {
	memRepo := &graphql.TestMemoryRepo{}
	qs := svcdashboard.NewQueryService(nil, memRepo, nil, nil, nil)
	cs := svcdashboard.NewCommandService(nil, memRepo, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		MemoryRepo: memRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { addMemory(content: \"important fact\") { success } }"}`)
	assert.Equal(t, true, dataOf(t, resp)["addMemory"].(map[string]interface{})["success"])
}

// ─── Mutation: addMemoryNode ─────────────────────────────────────────────────

func TestIntegration_Mutation_AddMemoryNode(t *testing.T) {
	memRepo := &graphql.TestMemoryRepo{}
	qs := svcdashboard.NewQueryService(nil, memRepo, nil, nil, nil)
	cs := svcdashboard.NewCommandService(nil, memRepo, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		MemoryRepo: memRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { addMemoryNode(label: \"Person\", type: \"entity\", value: \"Alice\") { id label value } }"}`)
	result := dataOf(t, resp)["addMemoryNode"].(map[string]interface{})
	assert.NotNil(t, result)
}

// ─── Mutation: updateMemoryNode ──────────────────────────────────────────────

func TestIntegration_Mutation_UpdateMemoryNode(t *testing.T) {
	memRepo := &graphql.TestMemoryRepo{}
	qs := svcdashboard.NewQueryService(nil, memRepo, nil, nil, nil)
	cs := svcdashboard.NewCommandService(nil, memRepo, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		MemoryRepo: memRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { updateMemoryNode(id: \"node-1\", label: \"Person\", type: \"entity\", value: \"Bob\") { id label value } }"}`)
	result := dataOf(t, resp)["updateMemoryNode"].(map[string]interface{})
	assert.NotNil(t, result)
}

// ─── Mutation: deleteMemoryNode ──────────────────────────────────────────────

func TestIntegration_Mutation_DeleteMemoryNode(t *testing.T) {
	memRepo := &graphql.TestMemoryRepo{}
	qs := svcdashboard.NewQueryService(nil, memRepo, nil, nil, nil)
	cs := svcdashboard.NewCommandService(nil, memRepo, nil)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
		MemoryRepo: memRepo,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { deleteMemoryNode(id: \"node-1\") }"}`)
	assert.Equal(t, true, dataOf(t, resp)["deleteMemoryNode"])
}

// ─── Mutation: addRelation ────────────────────────────────────────────────────

func TestIntegration_Mutation_AddRelation(t *testing.T) {
	gr := &graphql.TestGraphRepo{}
	qs := svcdashboard.NewQueryService(nil, nil, gr, nil, nil)
	cs := svcdashboard.NewCommandService(nil, nil, gr)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { addRelation(from: \"e1\", to: \"e2\", relationType: \"KNOWS\") { success } }"}`)
	assert.Equal(t, true, dataOf(t, resp)["addRelation"].(map[string]interface{})["success"])
}

// ─── Mutation: executeCypher ─────────────────────────────────────────────────

func TestIntegration_Mutation_ExecuteCypher(t *testing.T) {
	gr := &graphql.TestGraphRepo{}
	qs := svcdashboard.NewQueryService(nil, nil, gr, nil, nil)
	cs := svcdashboard.NewCommandService(nil, nil, gr)
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Bot", Status: "ok"},
		QuerySvc:   qs,
		CommandSvc: cs,
	})
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { executeCypher(cypher: \"MATCH (n) RETURN n\") { success } }"}`)
	assert.Equal(t, true, dataOf(t, resp)["executeCypher"].(map[string]interface{})["success"])
}

// ─── Mutation: setToolPermission ─────────────────────────────────────────────

func TestIntegration_Mutation_SetToolPermission(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	permRepo := &stubToolPermRepo{}
	deps := &resolvers.Deps{AgentRegistry: reg, ToolPermRepo: permRepo}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { setToolPermission(userId: \"u1\", toolName: \"browser_fetch\", mode: \"allow\") { success } }"}`)
	assert.Equal(t, true, dataOf(t, resp)["setToolPermission"].(map[string]interface{})["success"])
	assert.Len(t, permRepo.perms, 1)
	assert.Equal(t, "allow", permRepo.perms[0].Mode)
}

// ─── Mutation: deleteToolPermission ──────────────────────────────────────────

func TestIntegration_Mutation_DeleteToolPermission(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	permRepo := &stubToolPermRepo{
		perms: []dto.ToolPermissionRecord{{UserID: "u1", ToolName: "browser_fetch", Mode: "allow"}},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, ToolPermRepo: permRepo}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { deleteToolPermission(userId: \"u1\", toolName: \"browser_fetch\") { success } }"}`)
	assert.Equal(t, true, dataOf(t, resp)["deleteToolPermission"].(map[string]interface{})["success"])
	assert.Empty(t, permRepo.perms)
}

// ─── Mutation: setAllToolPermissions ─────────────────────────────────────────

func TestIntegration_Mutation_SetAllToolPermissions(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	permRepo := &stubToolPermRepo{
		perms: []dto.ToolPermissionRecord{
			{UserID: "u1", ToolName: "browser_fetch", Mode: "allow"},
			{UserID: "u1", ToolName: "terminal_exec", Mode: "allow"},
		},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, ToolPermRepo: permRepo}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { setAllToolPermissions(userId: \"u1\", mode: \"deny\") { success } }"}`)
	assert.Equal(t, true, dataOf(t, resp)["setAllToolPermissions"].(map[string]interface{})["success"])
	for _, p := range permRepo.perms {
		if p.UserID == "u1" {
			assert.Equal(t, "deny", p.Mode, "expected all perms for u1 to be 'deny'")
		}
	}
}

// ─── Mutation: approvePairing ─────────────────────────────────────────────────

func TestIntegration_Mutation_ApprovePairing(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	pairingPort := &stubPairingPort{
		active: []dto.PairingSnapshot{{Code: "XYZ", ChannelType: "telegram", Status: "pending"}},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, PairingPort: pairingPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { approvePairing(code: \"XYZ\", displayName: \"Alice\") { success pairing { code } } }"}`)
	result := dataOf(t, resp)["approvePairing"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
	assert.NotNil(t, result["pairing"])
}

// ─── Mutation: denyPairing ────────────────────────────────────────────────────

func TestIntegration_Mutation_DenyPairing(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	deps := &resolvers.Deps{AgentRegistry: reg, PairingPort: &stubPairingPort{}}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { denyPairing(code: \"XYZ\") { success } }"}`)
	result := dataOf(t, resp)["denyPairing"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
}

// ─── Mutation: connectMcp ─────────────────────────────────────────────────────

func TestIntegration_Mutation_ConnectMcp(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	connectPort := &stubMcpConnectPort{}
	mcpRepo := &stubMCPServerRepo{}
	deps := &resolvers.Deps{
		AgentRegistry:  reg,
		McpConnectPort: connectPort,
		MCPServerRepo:  mcpRepo,
	}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { connectMcp(name: \"srv1\", transport: \"http\", url: \"http://localhost:3000\") { success requiresAuth } }"}`)
	result := dataOf(t, resp)["connectMcp"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
	assert.Equal(t, false, result["requiresAuth"])
}

// ─── Mutation: disconnectMcp ─────────────────────────────────────────────────

func TestIntegration_Mutation_DisconnectMcp(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	connectPort := &stubMcpConnectPort{connected: map[string]bool{"srv1": true}}
	deps := &resolvers.Deps{AgentRegistry: reg, McpConnectPort: connectPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { disconnectMcp(name: \"srv1\") }"}`)
	assert.Equal(t, true, dataOf(t, resp)["disconnectMcp"])
	assert.False(t, connectPort.connected["srv1"])
}

// ─── Mutation: initiateOAuth ─────────────────────────────────────────────────

func TestIntegration_Mutation_InitiateOAuth(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	deps := &resolvers.Deps{AgentRegistry: reg, McpOAuthPort: &stubMcpOAuthPort{}}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { initiateOAuth(name: \"srv1\", url: \"http://localhost:3000\") { success authUrl } }"}`)
	result := dataOf(t, resp)["initiateOAuth"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
	assert.NotEmpty(t, result["authUrl"])
}

// ─── Mutation: deleteUser ─────────────────────────────────────────────────────

func TestIntegration_Mutation_DeleteUser(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	convPort := &stubConvPort{}
	deps := &resolvers.Deps{AgentRegistry: reg, ConvPort: convPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { deleteUser(conversationId: \"conv-1\") { success } }"}`)
	result := dataOf(t, resp)["deleteUser"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
}

// ─── Mutation: sendMessage ────────────────────────────────────────────────────

func TestIntegration_Mutation_SendMessage(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	msgRepo := &stubMsgRepo{}
	deps := &resolvers.Deps{AgentRegistry: reg, MsgRepo: msgRepo}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { sendMessage(conversationId: \"conv-1\", content: \"hello\") { success } }"}`)
	result := dataOf(t, resp)["sendMessage"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
}

// ─── Mutation: enableSkill ────────────────────────────────────────────────────

func TestIntegration_Mutation_EnableSkill(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	skillsPort := &stubSkillsPort{}
	deps := &resolvers.Deps{AgentRegistry: reg, SkillsPort: skillsPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { enableSkill(name: \"my-skill\") }"}`)
	assert.Equal(t, true, dataOf(t, resp)["enableSkill"])
}

// ─── Mutation: disableSkill ───────────────────────────────────────────────────

func TestIntegration_Mutation_DisableSkill(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	skillsPort := &stubSkillsPort{}
	deps := &resolvers.Deps{AgentRegistry: reg, SkillsPort: skillsPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { disableSkill(name: \"my-skill\") }"}`)
	assert.Equal(t, true, dataOf(t, resp)["disableSkill"])
}

// ─── Mutation: deleteSkill ────────────────────────────────────────────────────

func TestIntegration_Mutation_DeleteSkill(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	skillsPort := &stubSkillsPort{}
	deps := &resolvers.Deps{AgentRegistry: reg, SkillsPort: skillsPort}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { deleteSkill(name: \"my-skill\") }"}`)
	assert.Equal(t, true, dataOf(t, resp)["deleteSkill"])
}

// ─── Mutation: importSkill ────────────────────────────────────────────────────

func TestIntegration_Mutation_ImportSkill(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	skillsPort := &stubSkillsPort{}
	deps := &resolvers.Deps{AgentRegistry: reg, SkillsPort: skillsPort}
	h := graphql.NewGraphQLServer(deps)

	// base64 of "hello"
	resp := gqlPost(t, h, `{"query":"mutation { importSkill(data: \"aGVsbG8=\") { success } }"}`)
	result := dataOf(t, resp)["importSkill"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
}

// ─── Mutation: writeSystemFile ────────────────────────────────────────────────

func TestIntegration_Mutation_WriteSystemFile(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	sysFiles := &stubSysFilesPort{}
	deps := &resolvers.Deps{AgentRegistry: reg, SysFilesPort: sysFiles}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { writeSystemFile(name: \"prompt.txt\", content: \"Be helpful.\") { success } }"}`)
	result := dataOf(t, resp)["writeSystemFile"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
}

// ─── Mutation: spawnSubAgent ──────────────────────────────────────────────────

func TestIntegration_Mutation_SpawnSubAgent(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	subSvc := &stubSubAgentSvc{}
	deps := &resolvers.Deps{AgentRegistry: reg, SubAgentSvc: subSvc}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { spawnSubAgent(name: \"worker\", model: \"gpt-4o\", task: \"do stuff\") { success id } }"}`)
	result := dataOf(t, resp)["spawnSubAgent"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
	assert.NotEmpty(t, result["id"])
	assert.Len(t, subSvc.agents, 1)
}

// ─── Mutation: killSubAgent ───────────────────────────────────────────────────

func TestIntegration_Mutation_KillSubAgent(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: "Bot", Status: "ok"})
	subSvc := &stubSubAgentSvc{
		agents: []dto.SubAgentSnapshot{{ID: "sa-99", Name: "w", Status: "running", Task: "t"}},
	}
	deps := &resolvers.Deps{AgentRegistry: reg, SubAgentSvc: subSvc}
	h := graphql.NewGraphQLServer(deps)

	resp := gqlPost(t, h, `{"query":"mutation { killSubAgent(id: \"sa-99\") { success } }"}`)
	result := dataOf(t, resp)["killSubAgent"].(map[string]interface{})
	assert.Equal(t, true, result["success"])
	assert.Empty(t, subSvc.agents)
}

// ─── helpers used by multiple tests ──────────────────────────────────────────

func newReg(name string) *registry.AgentRegistry {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{Name: name, Status: "ok"})
	return reg
}

func minimalSnapshot() *dto.AppConfigSnapshot {
	return &dto.AppConfigSnapshot{
		Agent:          &dto.AgentConfigSnapshot{},
		Capabilities:   &dto.CapabilitiesSnapshot{},
		Database:       &dto.DatabaseConfigSnapshot{},
		Memory:         &dto.MemoryConfigSnapshot{},
		Subagents:      &dto.SubagentsConfigSnapshot{},
		GraphQL:        &dto.GraphQLConfigSnapshot{},
		Logging:        &dto.LoggingConfigSnapshot{},
		Scheduler:      &dto.SchedulerConfigSnapshot{},
		Secrets:        &dto.SecretsConfigSnapshot{File: &dto.FileSecretsSnapshot{}},
		ChannelSecrets: &dto.ChannelSecretsSnapshot{},
	}
}

func firstError(resp map[string]interface{}) error {
	if errs, ok := resp["errors"].([]interface{}); ok && len(errs) > 0 {
		if m, ok := errs[0].(map[string]interface{}); ok {
			if msg, ok := m["message"].(string); ok {
				return &gqlError{msg}
			}
		}
	}
	return nil
}

type gqlError struct{ msg string }

func (e *gqlError) Error() string { return e.msg }
