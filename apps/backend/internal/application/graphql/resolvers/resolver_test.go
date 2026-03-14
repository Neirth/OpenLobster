package resolvers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/generated"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	svcdashboard "github.com/neirth/openlobster/internal/domain/services/dashboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDeps(agent *dto.AgentSnapshot) *Deps {
	reg := registry.NewAgentRegistry()
	if agent != nil {
		reg.UpdateAgent(agent)
	}
	return &Deps{AgentRegistry: reg}
}

func TestNewResolver(t *testing.T) {
	deps := newTestDeps(&dto.AgentSnapshot{ID: "1", Name: "TestAgent", Status: "ok"})
	r := NewResolver(deps)
	require.NotNil(t, r)
	require.NotNil(t, r.Deps)
}

func TestResolver_SetEventSubscription(t *testing.T) {
	deps := newTestDeps(nil)
	r := NewResolver(deps)
	require.Nil(t, r.Sub)
	r.SetEventSubscription(nil)
	require.Nil(t, r.Sub)
}

func TestResolver_Query(t *testing.T) {
	deps := newTestDeps(&dto.AgentSnapshot{
		ID:         "1",
		Name:       "Bot",
		Provider:   "openai",
		AIProvider: "openai",
		Status:     "ok",
		Channels:   []dto.ChannelStatus{{Type: "telegram", Enabled: true}},
	})
	r := NewResolver(deps)

	queryResolver := r.Query()
	require.NotNil(t, queryResolver)

	agent, err := queryResolver.Agent(context.Background())
	require.NoError(t, err)
	require.NotNil(t, agent)
	assert.Equal(t, "Bot", agent.Name)
	require.NotNil(t, agent.Provider)
	assert.Equal(t, "openai", *agent.Provider)

	channels, err := queryResolver.Channels(context.Background())
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, "telegram", channels[0].Type)
	assert.True(t, channels[0].Enabled)

	heartbeat, err := queryResolver.Heartbeat(context.Background())
	require.NoError(t, err)
	require.NotNil(t, heartbeat)
	assert.Equal(t, "ok", heartbeat.Status)
}

func TestResolver_Mutation(t *testing.T) {
	deps := newTestDeps(&dto.AgentSnapshot{Name: "Test", Status: "ok"})
	r := NewResolver(deps)

	mutationResolver := r.Mutation()
	require.NotNil(t, mutationResolver)

	chID := "ch1"
	result, err := mutationResolver.SendMessage(context.Background(), nil, &chID, "hello")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Success)
	// With a nil msgRepo, SendMessage returns (nil, nil) and the resolver yields Success: true.
	assert.True(t, *result.Success)
}

func TestResolver_Subscription(t *testing.T) {
	deps := newTestDeps(&dto.AgentSnapshot{Name: "Test", Status: "ok"})
	r := NewResolver(deps)

	subResolver := r.Subscription()
	require.NotNil(t, subResolver)

	ch, err := subResolver.Events(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, ch)
}

func TestResolver_Subscription_Events_WithSub(t *testing.T) {
	deps := newTestDeps(nil)
	eventCh := make(chan events.Event, 2)
	mockSub := &mockEventSubscriptionPort{ch: eventCh}
	r := NewResolver(deps)
	r.SetEventSubscription(mockSub)

	ch, err := r.Subscription().Events(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, ch)

	eventCh <- events.NewEvent(events.EventTaskAdded, map[string]string{"id": "t1"})
	close(eventCh)

	payload := <-ch
	require.NotNil(t, payload)
	assert.Equal(t, events.EventTaskAdded, payload.Type)
}

func TestResolver_Subscription_OnMessageReceived(t *testing.T) {
	deps := newTestDeps(nil)
	eventCh := make(chan events.Event, 1)
	mockSub := &mockEventSubscriptionPort{ch: eventCh}
	r := NewResolver(deps)
	r.SetEventSubscription(mockSub)

	ch, err := r.Subscription().OnMessageReceived(context.Background())
	require.NoError(t, err)
	require.NotNil(t, ch)

	ev := events.NewEvent(events.EventMessageReceived, map[string]string{"id": "m1"})
	eventCh <- ev
	close(eventCh)

	payload := <-ch
	require.NotNil(t, payload)
	assert.Equal(t, events.EventMessageReceived, payload.Type)
}

func TestResolver_Subscription_OnEvents(t *testing.T) {
	eventTypes := []string{
		events.EventMessageSent, events.EventTaskAdded, events.EventMemoryUpdated,
		events.EventSessionStarted, events.EventPairingApproved,
	}
	for _, et := range eventTypes {
		deps := newTestDeps(nil)
		eventCh := make(chan events.Event, 1)
		mockSub := &mockEventSubscriptionPort{ch: eventCh}
		r := NewResolver(deps)
		r.SetEventSubscription(mockSub)

		var ch <-chan *generated.EventPayload
		var err error
		switch et {
		case events.EventMessageSent:
			ch, err = r.Subscription().OnMessageSent(context.Background())
		case events.EventTaskAdded:
			ch, err = r.Subscription().OnTaskAdded(context.Background())
		case events.EventMemoryUpdated:
			ch, err = r.Subscription().OnMemoryUpdated(context.Background())
		case events.EventSessionStarted:
			ch, err = r.Subscription().OnSessionStarted(context.Background())
		case events.EventPairingApproved:
			ch, err = r.Subscription().OnPairingApproved(context.Background())
		default:
			t.Skip("unknown event type")
		}
		require.NoError(t, err)
		require.NotNil(t, ch)
		eventCh <- events.NewEvent(et, nil)
		close(eventCh)
		payload := <-ch
		require.NotNil(t, payload, "event %s", et)
		assert.Equal(t, et, payload.Type)
	}
}

type mockEventSubscriptionPort struct {
	ch chan events.Event
}

func (m *mockEventSubscriptionPort) Subscribe(_ context.Context, _ string) (<-chan events.Event, error) {
	return m.ch, nil
}

func TestQueryResolver_Status(t *testing.T) {
	deps := newTestDeps(&dto.AgentSnapshot{
		Name:     "Agent",
		Status:   "ok",
		Channels: []dto.ChannelStatus{{Type: "discord", Enabled: true}},
	})
	r := NewResolver(deps)

	status, err := r.Query().Status(context.Background())
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.NotNil(t, status.Agent)
	assert.Equal(t, "Agent", status.Agent.Name)
	assert.NotNil(t, status.Health)
	assert.Len(t, status.Channels, 1)
}

func TestQueryResolver_Tools(t *testing.T) {
	deps := newTestDeps(&dto.AgentSnapshot{Name: "Test", Status: "ok"})
	deps.AgentRegistry.UpdateMCPTools([]dto.ToolSnapshot{
		{Name: "read_file", Description: "Read files", Source: "fs"},
	})
	r := NewResolver(deps)

	tools, err := r.Query().Tools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "read_file", tools[0].Name)
	assert.Equal(t, "Read files", *tools[0].Description)
	assert.Equal(t, "fs", *tools[0].Source)
}

func TestQueryResolver_Metrics(t *testing.T) {
	deps := newTestDeps(&dto.AgentSnapshot{Name: "Test", Status: "ok"})
	r := NewResolver(deps)

	metrics, err := r.Query().Metrics(context.Background())
	require.NoError(t, err)
	require.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.Uptime, 0)
}

func TestQueryResolver_Conversations(t *testing.T) {
	deps := newTestDeps(nil)
	deps.ConvPort = &mockConvPort{
		convs: []dto.ConversationSnapshot{
			{ID: "c1", ChannelID: "ch1", ParticipantName: "Alice"},
		},
	}
	r := NewResolver(deps)

	convs, err := r.Query().Conversations(context.Background())
	require.NoError(t, err)
	require.Len(t, convs, 1)
	assert.Equal(t, "c1", convs[0].ID)
	require.NotNil(t, convs[0].ParticipantName)
	assert.Equal(t, "Alice", *convs[0].ParticipantName)
}

func TestQueryResolver_Messages(t *testing.T) {
	deps := newTestDeps(nil)
	deps.MsgRepo = &mockMsgRepo{
		messages: []models.Message{
			{ID: uuid.New(), ConversationID: "c1", Role: "user", Content: "Hi", Timestamp: time.Now()},
		},
	}
	r := NewResolver(deps)

	msgs, err := r.Query().Messages(context.Background(), "c1", nil, nil)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "c1", msgs[0].ConversationID)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "Hi", msgs[0].Content)
}

func TestQueryResolver_SubAgents(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SubAgentSvc = &mockSubAgentSvc{
		list: []dto.SubAgentSnapshot{{ID: "sa1", Name: "worker", Status: "idle"}},
	}
	r := NewResolver(deps)

	agents, err := r.Query().SubAgents(context.Background())
	require.NoError(t, err)
	require.Len(t, agents, 1)
	assert.Equal(t, "sa1", agents[0].ID)
	assert.Equal(t, "worker", agents[0].Name)
}

func TestQueryResolver_Tasks(t *testing.T) {
	taskRepo := &mockTaskRepo{tasks: []models.Task{{ID: "t1", Prompt: "Do it", Status: "pending"}}}
	querySvc := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	commandSvc := svcdashboard.NewCommandService(taskRepo, nil, nil)
	deps := newTestDeps(nil)
	deps.QuerySvc = querySvc
	deps.CommandSvc = commandSvc
	deps.TaskRepo = taskRepo
	r := NewResolver(deps)

	tasks, err := r.Query().Tasks(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, "t1", tasks[0].ID)
	assert.Equal(t, "Do it", tasks[0].Prompt)
}

func TestMutationResolver_AddMemory(t *testing.T) {
	memRepo := &mockMemoryPort{}
	commandSvc := svcdashboard.NewCommandService(nil, memRepo, nil)
	deps := newTestDeps(nil)
	deps.CommandSvc = commandSvc
	r := NewResolver(deps)

	result, err := r.Mutation().AddMemory(context.Background(), "fact")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestMutationResolver_SpawnSubAgent(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SubAgentSvc = &mockSubAgentSvc{id: "openlobster"}
	r := NewResolver(deps)

	task := "task"
	result, err := r.Mutation().SpawnSubAgent(context.Background(), "worker", "gpt4", &task)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	require.NotNil(t, result.ID)
	assert.Equal(t, "openlobster", *result.ID)
}

func TestMutationResolver_KillSubAgent(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SubAgentSvc = &mockSubAgentSvc{}
	r := NewResolver(deps)

	result, err := r.Mutation().KillSubAgent(context.Background(), "openlobster")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestQueryResolver_SearchMemory(t *testing.T) {
	memRepo := &mockMemoryPort{}
	querySvc := svcdashboard.NewQueryService(nil, memRepo, nil, nil, nil)
	deps := newTestDeps(nil)
	deps.QuerySvc = querySvc
	r := NewResolver(deps)

	result, err := r.Query().SearchMemory(context.Background(), "query")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestQueryResolver_UserGraph(t *testing.T) {
	graphRepo := &mockGraphQueryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{{ID: "n1", Label: "Node", Type: "entity", Value: "val"}},
			Edges: []ports.GraphEdge{{Source: "n1", Target: "n2", Label: "rel"}},
		},
	}
	querySvc := svcdashboard.NewQueryService(nil, nil, graphRepo, nil, nil)
	deps := newTestDeps(nil)
	deps.QuerySvc = querySvc
	r := NewResolver(deps)

	result, err := r.Query().UserGraph(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestQueryResolver_Memory(t *testing.T) {
	deps := newTestDeps(nil)
	r := NewResolver(deps)

	graph, err := r.Query().Memory(context.Background())
	require.NoError(t, err)
	require.NotNil(t, graph)
}

func TestQueryResolver_Config(t *testing.T) {
	deps := newTestDeps(nil)
	r := NewResolver(deps)

	cfg, err := r.Query().Config(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestQueryResolver_Skills(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SkillsPort = &mockSkillsPort{skills: []dto.SkillSnapshot{{Name: "s1", Enabled: true}}}
	r := NewResolver(deps)

	skills, err := r.Query().Skills(context.Background())
	require.NoError(t, err)
	require.Len(t, skills, 1)
	assert.Equal(t, "s1", skills[0].Name)
}

func TestQueryResolver_SystemFiles(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SysFilesPort = &mockSysFilesPort{files: []dto.SystemFileSnapshot{{Path: "/x"}}}
	r := NewResolver(deps)

	files, err := r.Query().SystemFiles(context.Background())
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "/x", files[0].Path)
}

func TestQueryResolver_ToolPermissions(t *testing.T) {
	deps := newTestDeps(nil)
	deps.ToolPermRepo = &mockToolPermRepo{perms: []dto.ToolPermissionRecord{{ToolName: "t1", Mode: "allow"}}}
	r := NewResolver(deps)

	perms, err := r.Query().ToolPermissions(context.Background(), "u1")
	require.NoError(t, err)
	require.Len(t, perms, 1)
	assert.Equal(t, "t1", perms[0].ToolName)
}

func TestQueryResolver_PendingPairings_Users(t *testing.T) {
	deps := newTestDeps(nil)
	deps.PairingPort = &mockPairingPort{pairings: []dto.PairingSnapshot{{Code: "abc"}}}
	u := models.NewUser("p1")
	u.Channels = []models.UserChannel{{DisplayName: "Alice"}}
	deps.UserRepo = &mockUserRepo{users: []models.User{*u}}
	r := NewResolver(deps)

	pairings, err := r.Query().PendingPairings(context.Background())
	require.NoError(t, err)
	require.Len(t, pairings, 1)
	assert.Equal(t, "abc", pairings[0].Code)

	users, err := r.Query().Users(context.Background())
	require.NoError(t, err)
	require.NotNil(t, users)
}

func TestMutationResolver_UpdateConfig(t *testing.T) {
	deps := newTestDeps(nil)
	r := NewResolver(deps)

	result, err := r.Mutation().UpdateConfig(context.Background(), generated.UpdateConfigInput{})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMutationResolver_DeleteUser(t *testing.T) {
	deps := newTestDeps(nil)
	deps.ConvPort = &mockConvPort{}
	r := NewResolver(deps)

	result, err := r.Mutation().DeleteUser(context.Background(), "conv1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestMutationResolver_EnableDisableDeleteSkill(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SkillsPort = &mockSkillsPort{}
	r := NewResolver(deps)

	ok, err := r.Mutation().EnableSkill(context.Background(), "s1")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = r.Mutation().DisableSkill(context.Background(), "s1")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = r.Mutation().DeleteSkill(context.Background(), "s1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMutationResolver_ImportSkill_WriteSystemFile(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SkillsPort = &mockSkillsPort{}
	deps.SysFilesPort = &mockSysFilesPort{}
	r := NewResolver(deps)

	res, err := r.Mutation().ImportSkill(context.Background(), "c2tpbGxkYXRh") // base64 of "skilldata"
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Success)

	res2, err := r.Mutation().WriteSystemFile(context.Background(), "f", "content")
	require.NoError(t, err)
	assert.True(t, res2.Success)
}

func TestMutationResolver_SetToolPermission_Delete_SetAll(t *testing.T) {
	deps := newTestDeps(nil)
	deps.ToolPermRepo = &mockToolPermRepo{}
	r := NewResolver(deps)

	res, err := r.Mutation().SetToolPermission(context.Background(), "u1", "t1", "allow")
	require.NoError(t, err)
	assert.True(t, res.Success)

	res, err = r.Mutation().DeleteToolPermission(context.Background(), "u1", "t1")
	require.NoError(t, err)
	assert.True(t, res.Success)

	res, err = r.Mutation().SetAllToolPermissions(context.Background(), "u1", "deny")
	require.NoError(t, err)
	assert.True(t, res.Success)
}

func TestMutationResolver_SetAllToolPermissions_WithRecords(t *testing.T) {
	deps := newTestDeps(nil)
	deps.ToolPermRepo = &mockToolPermRepo{
		perms: []dto.ToolPermissionRecord{
			{UserID: "u1", ToolName: "t1", Mode: "allow"},
			{UserID: "u1", ToolName: "t2", Mode: "allow"},
			{UserID: "u2", ToolName: "t3", Mode: "allow"},
		},
	}
	r := NewResolver(deps)

	res, err := r.Mutation().SetAllToolPermissions(context.Background(), "u1", "deny")
	require.NoError(t, err)
	assert.True(t, res.Success)
}

func TestMutationResolver_ApprovePairing_DenyPairing(t *testing.T) {
	deps := newTestDeps(nil)
	deps.PairingPort = &mockPairingPort{approve: &dto.PairingSnapshot{Code: "x"}, denyErr: nil}
	r := NewResolver(deps)

	res, err := r.Mutation().ApprovePairing(context.Background(), "code", nil, nil)
	require.NoError(t, err)
	assert.True(t, res.Success)

	res2, err := r.Mutation().DenyPairing(context.Background(), "code", nil)
	require.NoError(t, err)
	assert.True(t, res2.Success)
}

func TestMutationResolver_AddTask_CompleteTask(t *testing.T) {
	cmdSvc := svcdashboard.NewCommandService(&mockTaskRepo{}, nil, nil)
	deps := newTestDeps(nil)
	deps.CommandSvc = cmdSvc
	r := NewResolver(deps)

	task, err := r.Mutation().AddTask(context.Background(), "prompt", nil)
	require.NoError(t, err)
	require.NotNil(t, task)

	ok, err := r.Mutation().CompleteTask(context.Background(), task.ID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestQueryResolver_Mcps_McpServers(t *testing.T) {
	deps := newTestDeps(nil)
	reg := registry.NewAgentRegistry()
	reg.UpdateMCPs([]dto.MCPSnapshot{{Name: "mcp1", Status: "connected"}})
	deps.AgentRegistry = reg
	deps.MCPServerRepo = &mockMCPServerRepo{servers: []dto.MCPServerRecord{{Name: "mcp1"}}}
	r := NewResolver(deps)

	mcps, err := r.Query().Mcps(context.Background())
	require.NoError(t, err)
	require.Len(t, mcps, 1)
	assert.Equal(t, "mcp1", mcps[0].Name)

	servers, err := r.Query().McpServers(context.Background())
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "mcp1", servers[0].Name)
}

func TestQueryResolver_McpTools_McpOAuthStatus_McpUsers(t *testing.T) {
	deps := newTestDeps(nil)
	deps.AgentRegistry = registry.NewAgentRegistry()
	deps.AgentRegistry.UpdateMCPTools([]dto.ToolSnapshot{{Name: "read_file"}})
	r := NewResolver(deps)

	tools, err := r.Query().McpTools(context.Background())
	require.NoError(t, err)
	require.NotNil(t, tools)

	status, err := r.Query().McpOAuthStatus(context.Background(), "server1")
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "unknown", status.Status)

	users, err := r.Query().McpUsers(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, users)
	assert.Equal(t, "loopback", users[0].ChannelID)
	assert.True(t, users[0].IsAgent)
}

func TestMutationResolver_InitiateOAuth(t *testing.T) {
	deps := newTestDeps(nil)
	deps.McpOAuthPort = &mockMcpOAuthPort{authURL: "https://auth.example.com/oauth"}
	r := NewResolver(deps)

	res, err := r.Mutation().InitiateOAuth(context.Background(), "server1", "https://example.com/mcp")
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Success)
	require.NotNil(t, res.AuthURL)
	assert.Equal(t, "https://auth.example.com/oauth", *res.AuthURL)
}

func TestMutationResolver_ConnectMcp_DisconnectMcp(t *testing.T) {
	deps := newTestDeps(nil)
	deps.McpConnectPort = &mockMcpConnectPort{} // nil connectErr → éxito
	r := NewResolver(deps)

	tport, u := "stdio", "cmd://echo"
	res, err := r.Mutation().ConnectMcp(context.Background(), "mcp1", tport, u, nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.Success)
	assert.True(t, *res.Success)

	ok, err := r.Mutation().DisconnectMcp(context.Background(), "mcp1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDeps_Agent_NilAgent(t *testing.T) {
	deps := newTestDeps(nil) // nil agent → GetAgent returns nil
	agent := deps.Agent(context.Background())
	require.NotNil(t, agent)
	assert.Equal(t, "agent-unknown", agent.ID)
	assert.Equal(t, "Unknown", agent.Name)
	assert.Equal(t, "not_initialized", agent.Status)
}

// TestDeps_Agent_PrefersConfigSnapshot verifies that Agent() returns name and provider
// from ConfigSnapshot when available (e.g. after wizard completion), so the GraphQL
// agent query reflects the latest config without relying on AgentRegistry being updated.
func TestDeps_Agent_PrefersConfigSnapshot(t *testing.T) {
	reg := registry.NewAgentRegistry()
	reg.UpdateAgent(&dto.AgentSnapshot{
		ID:         "openlobster",
		Name:       "OpenLobster",
		Provider:   "",
		AIProvider: "",
		Status:     "running",
	})
	deps := &Deps{AgentRegistry: reg}
	// Without ConfigSnapshot: returns values from the registry.
	agent := deps.Agent(context.Background())
	require.NotNil(t, agent)
	assert.Equal(t, "OpenLobster", agent.Name)
	assert.Equal(t, "", agent.Provider)

	// With ConfigSnapshot: name and provider come from config (post-wizard).
	deps.ConfigSnapshot = &dto.AppConfigSnapshot{
		Agent: &dto.AgentConfigSnapshot{
			Name:     "MiBot",
			Provider: "ollama",
		},
	}
	agent = deps.Agent(context.Background())
	require.NotNil(t, agent)
	assert.Equal(t, "MiBot", agent.Name)
	assert.Equal(t, "ollama", agent.Provider)
	assert.Equal(t, "ollama", agent.AIProvider)
}

func TestDeps_Channels_NilAgent(t *testing.T) {
	deps := newTestDeps(nil)
	channels := deps.Channels(context.Background())
	assert.Nil(t, channels)
}

func TestDeps_Conversations_NilPort(t *testing.T) {
	deps := newTestDeps(nil)
	deps.ConvPort = nil
	convs, err := deps.Conversations(context.Background())
	require.NoError(t, err)
	assert.Nil(t, convs)
}

func TestDeps_Messages_EmptyConversationID(t *testing.T) {
	deps := newTestDeps(nil)
	deps.MsgRepo = &mockMsgRepo{}
	msgs, err := deps.Messages(context.Background(), "", nil, nil)
	require.NoError(t, err)
	assert.Nil(t, msgs)
}

func TestDeps_SendMessage_WithMsgRepo(t *testing.T) {
	mockRepo := &mockMsgRepo{}
	deps := newTestDeps(nil)
	deps.MsgRepo = mockRepo
	deps.AIProvider = nil

	result, err := deps.SendMessage(context.Background(), "conv1", "hello")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "conv1", result.ConversationID)
	assert.Equal(t, "user", result.Role)
	assert.Equal(t, "hello", result.Content)
	assert.True(t, mockRepo.saved)
}

func TestDeps_SendMessage_WithAIProvider_ProcessWithLLM(t *testing.T) {
	saveCh := make(chan bool, 2)
	mockRepo := &mockMsgRepoWithHistory{
		history: []models.Message{{Role: "user", Content: "prior"}},
		onSave:  func() { saveCh <- true },
	}
	mockAI := &mockAIProviderPort{resp: ports.ChatResponse{Content: "Hi there!"}}
	deps := newTestDeps(nil)
	deps.MsgRepo = mockRepo
	deps.AIProvider = mockAI

	result, err := deps.SendMessage(context.Background(), "conv1", "hello")
	require.NoError(t, err)
	require.NotNil(t, result)

	<-saveCh
	<-saveCh
	assert.True(t, mockAI.chatCalled)
}

func TestDeps_SendMessage_AIProvider_EmptyContent(t *testing.T) {
	mockRepo := &mockMsgRepoWithHistory{history: nil, onSave: func() {}}
	mockAI := &mockAIProviderPort{resp: ports.ChatResponse{Content: ""}}
	deps := newTestDeps(nil)
	deps.MsgRepo = mockRepo
	deps.AIProvider = mockAI

	result, err := deps.SendMessage(context.Background(), "conv1", "hi")
	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
}

func TestDeps_SendMessage_AIProvider_ReturnsError(t *testing.T) {
	mockRepo := &mockMsgRepoWithHistory{history: nil, onSave: func() {}}
	mockAI := &mockAIProviderPort{resp: ports.ChatResponse{Content: "ok"}, err: errors.New("ai error")}
	deps := newTestDeps(nil)
	deps.MsgRepo = mockRepo
	deps.AIProvider = mockAI

	result, err := deps.SendMessage(context.Background(), "conv1", "hi")
	require.NoError(t, err)
	require.NotNil(t, result)
	time.Sleep(50 * time.Millisecond)
}

type mockAIProviderPort struct {
	resp       ports.ChatResponse
	err        error
	chatCalled bool
}

func (m *mockAIProviderPort) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	m.chatCalled = true
	return m.resp, m.err
}
func (m *mockAIProviderPort) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	return m.resp, m.err
}
func (m *mockAIProviderPort) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	return ports.ChatResponseWithAudio{}, nil
}
func (m *mockAIProviderPort) SupportsAudioInput() bool  { return false }
func (m *mockAIProviderPort) SupportsAudioOutput() bool { return false }
func (m *mockAIProviderPort) GetMaxTokens() int         { return 4096 }

type mockMsgRepoWithHistory struct {
	history []models.Message
	onSave  func()
}

func (m *mockMsgRepoWithHistory) Save(ctx context.Context, msg *models.Message) error {
	if m.onSave != nil {
		m.onSave()
	}
	return nil
}
func (m *mockMsgRepoWithHistory) GetByConversation(ctx context.Context, id string, limit int) ([]models.Message, error) {
	return m.history, nil
}
func (m *mockMsgRepoWithHistory) GetByConversationPaged(ctx context.Context, id string, before *string, limit int) ([]models.Message, error) {
	return m.history, nil
}
func (m *mockMsgRepoWithHistory) GetSinceLastCompaction(ctx context.Context, id string) ([]models.Message, error) {
	return m.history, nil
}
func (m *mockMsgRepoWithHistory) CountMessages(ctx context.Context) (int64, int64, error) {
	return 0, 0, nil
}

func TestDeps_DeleteUser(t *testing.T) {
	mockConv := &mockConvPort{}
	deps := newTestDeps(nil)
	deps.ConvPort = mockConv

	err := deps.DeleteUser(context.Background(), "conv1")
	require.NoError(t, err)
}

func TestDeps_Config(t *testing.T) {
	snap := &dto.AppConfigSnapshot{Agent: &dto.AgentConfigSnapshot{Name: "TestBot"}}
	deps := newTestDeps(nil)
	deps.ConfigSnapshot = snap

	cfg := deps.Config(context.Background())
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Agent)
	assert.Equal(t, "TestBot", cfg.Agent.Name)
}

func TestDeps_Metrics_WithQuerySvc(t *testing.T) {
	taskRepo := &mockTaskRepo{tasks: []models.Task{
		{ID: "t1", Status: "pending"},
		{ID: "t2", Status: "done"},
		{ID: "t3", Status: "running"},
	}}
	graphRepo := &mockGraphQueryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{{ID: "n1"}, {ID: "n2"}},
			Edges: []ports.GraphEdge{{Source: "n1", Target: "n2", Label: "rel"}},
		},
	}
	querySvc := svcdashboard.NewQueryService(taskRepo, nil, graphRepo, nil, nil)
	deps := newTestDeps(&dto.AgentSnapshot{Name: "A", Status: "ok"})
	deps.QuerySvc = querySvc
	deps.MemoryRepo = &mockMemoryPort{}

	metrics := deps.Metrics(context.Background())
	require.NotNil(t, metrics)
	assert.Equal(t, int64(1), metrics.TasksPending)
	assert.Equal(t, int64(1), metrics.TasksDone)
	assert.Equal(t, int64(1), metrics.TasksRunning)
	assert.Equal(t, int64(2), metrics.MemoryNodes)
	assert.Equal(t, int64(1), metrics.MemoryEdges)
}

func TestDeps_MCPServers(t *testing.T) {
	mockMCPServer := &mockMCPServerRepo{servers: []dto.MCPServerRecord{{Name: "s1", URL: "http://x"}}}
	deps := newTestDeps(nil)
	deps.MCPServerRepo = mockMCPServer
	deps.McpConnectPort = &mockMcpConnectPort{connectionStatus: map[string]string{"s1": "online"}}

	servers, err := deps.MCPServers(context.Background())
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "s1", servers[0].Name)
	assert.Equal(t, "online", servers[0].Status)
}

func TestDeps_MCPServers_NilRepo(t *testing.T) {
	deps := newTestDeps(nil)
	deps.MCPServerRepo = nil

	servers, err := deps.MCPServers(context.Background())
	require.NoError(t, err)
	assert.Nil(t, servers)
}

func TestDeps_ConnectMCP_DisconnectMCP(t *testing.T) {
	deps := newTestDeps(nil)
	deps.McpConnectPort = &mockMcpConnectPort{}
	_, err := deps.ConnectMCP(context.Background(), "mcp1", "stdio", "cmd://echo")
	assert.NoError(t, err)

	err = deps.DisconnectMCP(context.Background(), "mcp1")
	assert.NoError(t, err)
}

func TestDeps_Messages_Error(t *testing.T) {
	mockRepo := &mockMsgRepo{err: assert.AnError}
	deps := newTestDeps(nil)
	deps.MsgRepo = mockRepo

	msgs, err := deps.Messages(context.Background(), "conv1", nil, nil)
	assert.Error(t, err)
	assert.Nil(t, msgs)
}

func TestDeps_SendMessage_SaveError(t *testing.T) {
	mockRepo := &mockMsgRepo{err: assert.AnError}
	deps := newTestDeps(nil)
	deps.MsgRepo = mockRepo
	deps.AIProvider = nil

	result, err := deps.SendMessage(context.Background(), "conv1", "hello")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestMutationResolver_RemoveTask(t *testing.T) {
	commandSvc := svcdashboard.NewCommandService(&mockTaskRepo{}, nil, nil)
	deps := newTestDeps(nil)
	deps.CommandSvc = commandSvc
	r := NewResolver(deps)

	ok, err := r.Mutation().RemoveTask(context.Background(), "task1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMutationResolver_UpdateTask(t *testing.T) {
	commandSvc := svcdashboard.NewCommandService(&mockTaskRepo{}, nil, nil)
	deps := newTestDeps(nil)
	deps.CommandSvc = commandSvc
	r := NewResolver(deps)

	schedule := "* * * * *"
	result, err := r.Mutation().UpdateTask(context.Background(), "task1", "new prompt", &schedule)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMutationResolver_ToggleTask(t *testing.T) {
	commandSvc := svcdashboard.NewCommandService(&mockTaskRepo{}, nil, nil)
	deps := newTestDeps(nil)
	deps.CommandSvc = commandSvc
	r := NewResolver(deps)

	result, err := r.Mutation().ToggleTask(context.Background(), "task1", false)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestDeps_Skills_NilPort(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SkillsPort = nil
	skills, err := deps.Skills(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, skills)
}

func TestDeps_SystemFiles_NilPort(t *testing.T) {
	deps := newTestDeps(nil)
	deps.SysFilesPort = nil
	files, err := deps.SystemFiles(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, files)
}

func TestDeps_AddTask_CompleteTask_NilCommandSvc(t *testing.T) {
	deps := newTestDeps(nil)
	id, err := deps.AddTask(context.Background(), "prompt", "")
	assert.NoError(t, err)
	assert.Empty(t, id)

	err = deps.CompleteTask(context.Background(), "t1")
	assert.NoError(t, err)

	err = deps.RemoveTask(context.Background(), "t1")
	assert.NoError(t, err)
}

func TestDeps_AddTask_CompleteTask_WithCommandSvc(t *testing.T) {
	cmdSvc := svcdashboard.NewCommandService(&mockTaskRepo{tasks: []models.Task{}}, nil, nil)
	deps := newTestDeps(nil)
	deps.CommandSvc = cmdSvc

	id, err := deps.AddTask(context.Background(), "new task", "* * * * *")
	assert.NoError(t, err)
	assert.NotEmpty(t, id)

	err = deps.CompleteTask(context.Background(), id)
	assert.NoError(t, err)
}

func TestDeps_UserGraph_Error(t *testing.T) {
	graphRepo := &mockGraphQueryPort{err: assert.AnError}
	querySvc := svcdashboard.NewQueryService(nil, nil, graphRepo, nil, nil)
	deps := newTestDeps(nil)
	deps.QuerySvc = querySvc

	g, err := deps.UserGraph(context.Background(), "user1")
	assert.Error(t, err)
	assert.Nil(t, g)
}

func TestDeps_ExecuteCypher_Error(t *testing.T) {
	graphRepo := &mockGraphQueryPort{err: assert.AnError}
	querySvc := svcdashboard.NewQueryService(nil, nil, graphRepo, nil, nil)
	deps := newTestDeps(nil)
	deps.QuerySvc = querySvc

	g, err := deps.ExecuteCypher(context.Background(), "MATCH (n) RETURN n")
	assert.Error(t, err)
	assert.Nil(t, g)
}

func TestDeps_Skills_WithPort(t *testing.T) {
	mockSkills := &mockSkillsPort{skills: []dto.SkillSnapshot{{Name: "s1", Enabled: true}}}
	deps := newTestDeps(nil)
	deps.SkillsPort = mockSkills

	skills, err := deps.Skills(context.Background())
	assert.NoError(t, err)
	assert.Len(t, skills, 1)
	assert.Equal(t, "s1", skills[0].Name)
}

func TestDeps_EnableSkill_DisableSkill_DeleteSkill(t *testing.T) {
	mockSkills := &mockSkillsPort{}
	deps := newTestDeps(nil)
	deps.SkillsPort = mockSkills

	assert.NoError(t, deps.EnableSkill(context.Background(), "skill1"))
	assert.NoError(t, deps.DisableSkill(context.Background(), "skill1"))
	assert.NoError(t, deps.DeleteSkill(context.Background(), "skill1"))
}

func TestDeps_SystemFiles_WithPort(t *testing.T) {
	mockFiles := &mockSysFilesPort{files: []dto.SystemFileSnapshot{{Path: "/x"}}}
	deps := newTestDeps(nil)
	deps.SysFilesPort = mockFiles

	files, err := deps.SystemFiles(context.Background())
	assert.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "/x", files[0].Path)
}

func TestDeps_ToolPermissions(t *testing.T) {
	mockPerm := &mockToolPermRepo{perms: []dto.ToolPermissionRecord{{UserID: "u1", ToolName: "t1", Mode: "always"}}}
	deps := newTestDeps(nil)
	deps.ToolPermRepo = mockPerm

	perms, err := deps.ToolPermissions(context.Background(), "u1")
	assert.NoError(t, err)
	assert.Len(t, perms, 1)
	assert.Equal(t, "t1", perms[0].ToolName)
}

func TestDeps_ToolPermissions_NilRepo(t *testing.T) {
	deps := newTestDeps(nil)
	perms, err := deps.ToolPermissions(context.Background(), "u1")
	assert.NoError(t, err)
	assert.Nil(t, perms)
}

func TestDeps_SetToolPermission(t *testing.T) {
	mockPerm := &mockToolPermRepo{}
	deps := newTestDeps(nil)
	deps.ToolPermRepo = mockPerm

	err := deps.SetToolPermission(context.Background(), "u1", "tool1", "deny")
	assert.NoError(t, err)
}

func TestDeps_ImportSkill_WriteSystemFile(t *testing.T) {
	mockSkills := &mockSkillsPort{}
	mockFiles := &mockSysFilesPort{}
	deps := newTestDeps(nil)
	deps.SkillsPort = mockSkills
	deps.SysFilesPort = mockFiles

	assert.NoError(t, deps.ImportSkill(context.Background(), []byte("skill data")))
	assert.NoError(t, deps.WriteSystemFile(context.Background(), "f", "content"))
}

func TestDeps_UpdateConfig(t *testing.T) {
	deps := newTestDeps(nil)
	err := deps.UpdateConfig(context.Background(), map[string]interface{}{"agent": map[string]interface{}{"name": "X"}})
	assert.NoError(t, err)
}

func TestDeps_Users(t *testing.T) {
	u := models.NewUser("p1")
	mockUser := &mockUserRepo{users: []models.User{*u}}
	mockChannel := &mockUserChannelRepo{displayNames: map[string]string{u.ID.String(): "Alice"}}
	deps := newTestDeps(nil)
	deps.UserRepo = mockUser
	deps.UserChannelRepo = mockChannel

	users, err := deps.Users(context.Background())
	assert.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, "Alice", users[0].DisplayName)
}

type mockUserChannelRepo struct {
	displayNames map[string]string
}

func (m *mockUserChannelRepo) ExistsByPlatformUserID(ctx context.Context, platformUserID string) (bool, error) {
	return false, nil
}
func (m *mockUserChannelRepo) Create(ctx context.Context, userID, channelType, platformUserID, username string) error {
	return nil
}
func (m *mockUserChannelRepo) GetDisplayNameByUserID(ctx context.Context, userID string) (string, error) {
	if m != nil && m.displayNames != nil {
		return m.displayNames[userID], nil
	}
	return "", nil
}

type mockUserRepo struct {
	users []models.User
	err   error
}

func (m *mockUserRepo) Create(ctx context.Context, u *models.User) error { return m.err }
func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	for i := range m.users {
		if m.users[i].ID.String() == id {
			return &m.users[i], m.err
		}
	}
	return nil, m.err
}
func (m *mockUserRepo) ListAll(ctx context.Context) ([]models.User, error) { return m.users, m.err }

func TestQueryResolver_UserGraph_WithUserID(t *testing.T) {
	graphRepo := &mockGraphQueryPort{graph: ports.Graph{Nodes: []ports.GraphNode{{ID: "n1"}}}}
	querySvc := svcdashboard.NewQueryService(nil, nil, graphRepo, nil, nil)
	deps := newTestDeps(nil)
	deps.QuerySvc = querySvc
	r := NewResolver(deps)

	userID := "user123"
	result, err := r.Query().UserGraph(context.Background(), &userID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestDeps_Metrics_WithConvPortAndMsgRepo(t *testing.T) {
	mockConv := &mockConvPort{convs: []dto.ConversationSnapshot{{ID: "c1"}, {ID: "c2"}}}
	deps := newTestDeps(&dto.AgentSnapshot{Name: "A", Status: "ok"})
	deps.ConvPort = mockConv
	deps.MsgRepo = &mockMsgRepoWithCount{recv: 10, sent: 5}

	metrics := deps.Metrics(context.Background())
	require.NotNil(t, metrics)
	assert.Equal(t, int64(2), metrics.ActiveSessions)
	assert.Equal(t, int64(10), metrics.MessagesReceived)
	assert.Equal(t, int64(5), metrics.MessagesSent)
}

type mockMsgRepoWithCount struct {
	recv, sent int64
}

func (m *mockMsgRepoWithCount) Save(ctx context.Context, msg *models.Message) error { return nil }
func (m *mockMsgRepoWithCount) GetByConversation(ctx context.Context, id string, limit int) ([]models.Message, error) {
	return nil, nil
}
func (m *mockMsgRepoWithCount) GetByConversationPaged(ctx context.Context, id string, before *string, limit int) ([]models.Message, error) {
	return nil, nil
}
func (m *mockMsgRepoWithCount) GetSinceLastCompaction(ctx context.Context, id string) ([]models.Message, error) {
	return nil, nil
}
func (m *mockMsgRepoWithCount) CountMessages(ctx context.Context) (int64, int64, error) {
	return m.recv, m.sent, nil
}

func TestMutationResolver_AddRelation(t *testing.T) {
	graphCmd := &mockGraphCommandPort{}
	commandSvc := svcdashboard.NewCommandService(nil, nil, graphCmd)
	deps := newTestDeps(nil)
	deps.CommandSvc = commandSvc
	r := NewResolver(deps)

	result, err := r.Mutation().AddRelation(context.Background(), "a", "b", "KNOWS")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestMutationResolver_ExecuteCypher(t *testing.T) {
	graphRepo := &mockGraphQueryPort{graph: ports.Graph{Nodes: []ports.GraphNode{{ID: "n1"}}}}
	querySvc := svcdashboard.NewQueryService(nil, nil, graphRepo, nil, nil)
	deps := newTestDeps(nil)
	deps.QuerySvc = querySvc
	r := NewResolver(deps)

	result, err := r.Mutation().ExecuteCypher(context.Background(), "MATCH (n) RETURN n")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
}

func TestMutationResolver_DeleteMemoryNode(t *testing.T) {
	deps := newTestDeps(nil)
	r := NewResolver(deps)

	ok, err := r.Mutation().DeleteMemoryNode(context.Background(), "node1")
	require.NoError(t, err)
	// Con commandSvc nil, DeleteMemoryNode retorna nil (no-op), resolver devuelve true
	assert.True(t, ok)
}

func TestMutationResolver_AddMemoryNode_UpdateMemoryNode(t *testing.T) {
	mem := &mockMemoryPort{}
	cmdSvc := svcdashboard.NewCommandService(nil, mem, nil)
	deps := newTestDeps(nil)
	deps.CommandSvc = cmdSvc
	deps.MemoryRepo = mem
	r := NewResolver(deps)

	node, err := r.Mutation().AddMemoryNode(context.Background(), "Person", "entity", "Alice")
	require.NoError(t, err)
	require.NotNil(t, node)

	lbl, typ, val := "Updated", "entity", "Alice Smith"
	updated, err := r.Mutation().UpdateMemoryNode(context.Background(), node.ID, &lbl, &typ, &val, nil)
	require.NoError(t, err)
	require.NotNil(t, updated)
}

// ─── Mocks ────────────────────────────────────────────────────────────────────

type mockConvPort struct {
	convs []dto.ConversationSnapshot
	err   error
}

func (m *mockConvPort) ListConversations() ([]dto.ConversationSnapshot, error) {
	return m.convs, m.err
}
func (m *mockConvPort) DeleteUser(ctx context.Context, id string) error { return m.err }

type mockMsgRepo struct {
	messages []models.Message
	err      error
	saved    bool
}

func (m *mockMsgRepo) Save(ctx context.Context, msg *models.Message) error {
	if m.err != nil {
		return m.err
	}
	m.saved = true
	return nil
}
func (m *mockMsgRepo) GetByConversation(ctx context.Context, conversationID string, limit int) ([]models.Message, error) {
	return m.messages, m.err
}
func (m *mockMsgRepo) GetByConversationPaged(ctx context.Context, conversationID string, before *string, limit int) ([]models.Message, error) {
	return m.messages, m.err
}
func (m *mockMsgRepo) GetSinceLastCompaction(ctx context.Context, conversationID string) ([]models.Message, error) {
	return m.messages, m.err
}
func (m *mockMsgRepo) CountMessages(ctx context.Context) (int64, int64, error) {
	return 0, 0, m.err
}

type mockMemoryPort struct{}

func (m *mockMemoryPort) AddKnowledge(ctx context.Context, userID, content, label, relation string, embedding []float64) error {
	return nil
}
func (m *mockMemoryPort) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	return nil, nil
}
func (m *mockMemoryPort) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	return ports.Graph{}, nil
}
func (m *mockMemoryPort) AddRelation(ctx context.Context, from, to, relType string) error { return nil }
func (m *mockMemoryPort) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	return ports.GraphResult{}, nil
}
func (m *mockMemoryPort) InvalidateMemoryCache(ctx context.Context, userID string) error { return nil }
func (m *mockMemoryPort) SetUserProperty(ctx context.Context, userID, key, value string) error {
	return nil
}
func (m *mockMemoryPort) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	return nil
}
func (m *mockMemoryPort) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	return nil
}
func (m *mockMemoryPort) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	return nil
}

type mockSubAgentSvc struct {
	id   string
	list []dto.SubAgentSnapshot
	err  error
}

func (m *mockSubAgentSvc) List(ctx context.Context) ([]dto.SubAgentSnapshot, error) {
	return m.list, m.err
}
func (m *mockSubAgentSvc) Spawn(ctx context.Context, name, model, task string) (string, error) {
	return m.id, m.err
}
func (m *mockSubAgentSvc) Kill(ctx context.Context, id string) error { return m.err }

type mockGraphQueryPort struct {
	graph ports.Graph
	err   error
}

func (m *mockGraphQueryPort) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	if m.err != nil {
		return ports.Graph{}, m.err
	}
	return m.graph, nil
}

func (m *mockGraphQueryPort) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	if m.err != nil {
		return ports.GraphResult{}, m.err
	}
	return ports.GraphResult{Data: []map[string]interface{}{}}, nil
}

type mockGraphCommandPort struct{ err error }

func (m *mockGraphCommandPort) AddRelation(ctx context.Context, from, to, relType string) error {
	return m.err
}

type mockTaskRepo struct {
	tasks []models.Task
	err   error
}

func (m *mockTaskRepo) GetPending(ctx context.Context) ([]models.Task, error) {
	return m.tasks, m.err
}
func (m *mockTaskRepo) ListAll(ctx context.Context) ([]models.Task, error) {
	return m.tasks, m.err
}
func (m *mockTaskRepo) Add(ctx context.Context, t *models.Task) error { return m.err }
func (m *mockTaskRepo) MarkDone(ctx context.Context, id string) error { return m.err }
func (m *mockTaskRepo) Delete(ctx context.Context, id string) error   { return m.err }
func (m *mockTaskRepo) Update(ctx context.Context, t *models.Task) error {
	return m.err
}
func (m *mockTaskRepo) SetEnabled(ctx context.Context, id string, enabled bool) error {
	return m.err
}

type mockMCPServerRepo struct {
	servers []dto.MCPServerRecord
	err     error
}

func (m *mockMCPServerRepo) Save(ctx context.Context, name, url string) error { return m.err }
func (m *mockMCPServerRepo) Delete(ctx context.Context, name string) error    { return m.err }
func (m *mockMCPServerRepo) ListAll(ctx context.Context) ([]dto.MCPServerRecord, error) {
	return m.servers, m.err
}

// mockMcpConnectPort implementa dto.McpConnectPort para tests.
type mockMcpConnectPort struct {
	connectErr       error
	disconnectErr    error
	connectionStatus map[string]string // name -> "online"|"unknown"
	toolCount        map[string]int    // name -> count (opcional)
}

func (m *mockMcpConnectPort) Connect(ctx context.Context, name, transport, url string) (bool, error) {
	return false, m.connectErr
}
func (m *mockMcpConnectPort) Disconnect(ctx context.Context, name string) error {
	return m.disconnectErr
}
func (m *mockMcpConnectPort) GetConnectionStatus(name string) string {
	if m.connectionStatus != nil {
		if s, ok := m.connectionStatus[name]; ok {
			return s
		}
	}
	return "unknown"
}
func (m *mockMcpConnectPort) GetServerToolCount(name string) int {
	if m.toolCount != nil {
		if c, ok := m.toolCount[name]; ok {
			return c
		}
	}
	// If connectionStatus reports online, assume at least 1 tool for tests.
	if m.connectionStatus != nil && m.connectionStatus[name] == "online" {
		return 1
	}
	return 0
}

type mockMcpOAuthPort struct {
	authURL string
	err     error
}

func (m *mockMcpOAuthPort) InitiateOAuth(ctx context.Context, serverName, mcpURL string) (string, error) {
	return m.authURL, m.err
}
func (m *mockMcpOAuthPort) Status(serverName string) (string, string) {
	return "unknown", ""
}
func (m *mockMcpOAuthPort) SetClientID(ctx context.Context, serverName, clientID string) error {
	return nil
}

type mockSkillsPort struct {
	skills []dto.SkillSnapshot
	err    error
}

func (m *mockSkillsPort) ListSkills() ([]dto.SkillSnapshot, error) { return m.skills, m.err }
func (m *mockSkillsPort) EnableSkill(name string) error            { return m.err }
func (m *mockSkillsPort) DisableSkill(name string) error           { return m.err }
func (m *mockSkillsPort) ImportSkill(data []byte) error            { return m.err }
func (m *mockSkillsPort) DeleteSkill(name string) error            { return m.err }

type mockSysFilesPort struct {
	files []dto.SystemFileSnapshot
	err   error
}

func (m *mockSysFilesPort) ListFiles() ([]dto.SystemFileSnapshot, error) { return m.files, m.err }
func (m *mockSysFilesPort) WriteFile(name, content string) error         { return m.err }

type mockToolPermRepo struct {
	perms []dto.ToolPermissionRecord
	err   error
}

func (m *mockToolPermRepo) Set(ctx context.Context, userID, toolName, mode string) error {
	return m.err
}
func (m *mockToolPermRepo) Delete(ctx context.Context, userID, toolName string) error { return m.err }
func (m *mockToolPermRepo) ListByUser(ctx context.Context, userID string) ([]dto.ToolPermissionRecord, error) {
	return m.perms, m.err
}
func (m *mockToolPermRepo) ListAll(ctx context.Context) ([]dto.ToolPermissionRecord, error) {
	return m.perms, m.err
}

type mockPairingPort struct {
	pairings   []dto.PairingSnapshot
	approve    *dto.PairingSnapshot
	approveErr error
	denyErr    error
}

func (m *mockPairingPort) ListActive(ctx context.Context) ([]dto.PairingSnapshot, error) {
	return m.pairings, nil
}
func (m *mockPairingPort) Approve(ctx context.Context, code, userID, displayName string) (*dto.PairingSnapshot, error) {
	if m.approveErr != nil {
		return nil, m.approveErr
	}
	return m.approve, nil
}
func (m *mockPairingPort) Deny(ctx context.Context, code, reason string) error {
	return m.denyErr
}

	// Verify that concrete types implement the generated interfaces.
var (
	_ generated.QueryResolver        = (*queryResolver)(nil)
	_ generated.MutationResolver     = (*mutationResolver)(nil)
	_ generated.SubscriptionResolver = (*subscriptionResolver)(nil)
)
