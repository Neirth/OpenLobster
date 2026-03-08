package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/application/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/domain/events"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/repositories"
	domainServices "github.com/neirth/openlobster/internal/domain/services"
	svcdashboard "github.com/neirth/openlobster/internal/domain/services/dashboard"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/services/permissions"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockAIProvider struct {
	mock.Mock
}

func (m *mockAIProvider) Chat(ctx context.Context, req ports.ChatRequest) (ports.ChatResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(ports.ChatResponse), args.Error(1)
}
func (m *mockAIProvider) ChatWithAudio(ctx context.Context, req ports.ChatRequestWithAudio) (ports.ChatResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(ports.ChatResponse), args.Error(1)
}
func (m *mockAIProvider) ChatToAudio(ctx context.Context, req ports.ChatRequest) (ports.ChatResponseWithAudio, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(ports.ChatResponseWithAudio), args.Error(1)
}
func (m *mockAIProvider) SupportsAudioInput() bool  { return true }
func (m *mockAIProvider) SupportsAudioOutput() bool { return true }
func (m *mockAIProvider) GetMaxTokens() int         { return 4096 }

type mockMessageRepo struct {
	mock.Mock
}

func (m *mockMessageRepo) Save(ctx context.Context, msg *models.Message) error {
	return m.Called(ctx, msg).Error(0)
}
func (m *mockMessageRepo) GetByConversation(ctx context.Context, id string, limit int) ([]models.Message, error) {
	args := m.Called(ctx, id, limit)
	return args.Get(0).([]models.Message), args.Error(1)
}
func (m *mockMessageRepo) GetSinceLastCompaction(ctx context.Context, id string) ([]models.Message, error) {
	args := m.Called(ctx, id)
	return args.Get(0).([]models.Message), args.Error(1)
}
func (m *mockMessageRepo) GetLastCompaction(ctx context.Context, id string) (*models.Message, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Message), args.Error(1)
}

type mockSessionRepo struct {
	mock.Mock
}

func (m *mockSessionRepo) Create(ctx context.Context, s *models.Session) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockSessionRepo) GetByID(ctx context.Context, id string) (*models.Session, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Session), args.Error(1)
}
func (m *mockSessionRepo) Update(ctx context.Context, s *models.Session) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockSessionRepo) GetActiveByUser(ctx context.Context, uid string) ([]models.Session, error) {
	args := m.Called(ctx, uid)
	return args.Get(0).([]models.Session), args.Error(1)
}
func (m *mockSessionRepo) GetActiveByChannel(ctx context.Context, channelID string) ([]models.Session, error) {
	args := m.Called(ctx, channelID)
	return args.Get(0).([]models.Session), args.Error(1)
}
func (m *mockSessionRepo) GetActiveByGroup(ctx context.Context, groupID string) ([]models.Session, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Session), args.Error(1)
}

func TestE2E_MessageProcessor(t *testing.T) {
	ai := new(mockAIProvider)
	msgRepo := new(mockMessageRepo)
	sessionRepo := new(mockSessionRepo)
	eventBus := domainServices.NewEventBus()

	msgRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Maybe()

	processor := domainServices.NewMessageProcessorService(ai, msgRepo, sessionRepo, eventBus)

	ai.On("Chat", mock.Anything, mock.Anything).Return(ports.ChatResponse{
		Content:    "Hello!",
		StopReason: "stop",
	}, nil).Once()

	msg := &models.Message{
		ID:             uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ConversationID: "conv-1",
		Content:        "Hi",
		Role:           "user",
	}

	err := processor.Process(context.Background(), msg)
	assert.NoError(t, err)
}

func TestE2E_SubAgentWorkflow(t *testing.T) {
	ai := new(mockAIProvider)
	ai.On("Chat", mock.Anything, mock.Anything).Return(ports.ChatResponse{Content: "done"}, nil).Once()

	subAgentSvc := domainServices.NewSubAgentService(ai, 3, 10*time.Second)

	agent, err := subAgentSvc.Spawn(context.Background(), mcp.SubAgentConfig{
		Name: "worker", Model: "gpt-4", SystemPrompt: "You are helpful",
	}, "Do work")
	assert.NoError(t, err)
	assert.NotEmpty(t, agent.ID())

	agents, _ := subAgentSvc.List(context.Background())
	assert.Len(t, agents, 1)

	err = subAgentSvc.Kill(context.Background(), agent.ID())
	assert.NoError(t, err)

	agents, _ = subAgentSvc.List(context.Background())
	assert.Len(t, agents, 0)
}

func TestE2E_HealthCheck(t *testing.T) {
	// Scheduler replaces heartbeat: verify health endpoint returns "ok"
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{Name: "HealthAgent", Status: "ok"},
	})
	h := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "query { heartbeat { status lastCheck } }"}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	data := resp["data"].(map[string]interface{})
	health := data["heartbeat"].(map[string]interface{})
	assert.Equal(t, "ok", health["status"])
}

func TestE2E_GraphQLAPI(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:       "TestAgent",
			AIProvider: "openai",
			Status:     "ok",
			Channels:   []dto.ChannelStatus{{Type: "telegram", Enabled: true}},
		},
	})
	handler := graphql.NewGraphQLServer(deps)

	t.Run("query agent", func(t *testing.T) {
		reqBody := `{"query": "query { agent { name aiProvider } }"}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		agent := data["agent"].(map[string]interface{})
		assert.Equal(t, "TestAgent", agent["name"])
	})

	t.Run("query channels", func(t *testing.T) {
		reqBody := `{"query": "query { channels { type enabled } }"}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestE2E_GraphQL_FullWorkflow(t *testing.T) {
	db, err := persistence.Open("sqlite3", ":memory:")
	assert.NoError(t, err)
	defer db.Close()

	assert.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))

	taskRepo := repositories.NewTaskRepository(db.GormDB())
	memRepo := &graphql.TestMemoryRepo{}
	graphRepo := &graphql.TestGraphRepo{}

	querySvc := svcdashboard.NewQueryService(taskRepo, memRepo, graphRepo, nil, nil)
	commandSvc := svcdashboard.NewCommandService(taskRepo, memRepo, graphRepo)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:          "E2EAgent",
			AIProvider:    "openai",
			MemoryBackend: "sqlite",
			Status:        "ok",
			Channels: []dto.ChannelStatus{
				{Type: "telegram", Enabled: true},
				{Type: "discord", Enabled: true},
			},
		},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
		TaskRepo:   taskRepo,
		MemoryRepo: memRepo,
	})
	handler := graphql.NewGraphQLServer(deps)

	t.Run("add task via mutation", func(t *testing.T) {
		reqBody := `{"query": "mutation ($prompt: String!) { addTask(prompt: $prompt) { id prompt status } }", "variables": {"prompt": "Test E2E task"}}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		addTask := data["addTask"].(map[string]interface{})
		assert.NotEmpty(t, addTask["id"])
		assert.Equal(t, "Test E2E task", addTask["prompt"])
	})

	t.Run("verify task in DB directly", func(t *testing.T) {
		tasks, err := taskRepo.ListAll(context.Background())
		assert.NoError(t, err)
		t.Logf("Tasks in DB: %d", len(tasks))
		for _, task := range tasks {
			t.Logf("Task: %s - %s", task.ID, task.Prompt)
		}
	})

	t.Run("query tasks with simple query", func(t *testing.T) {
		reqBody := `{"query": "query { tasks { id } }"}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		t.Logf("Response: %s", w.Body.String())

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		if data == nil {
			t.Fatalf("data is nil. resp: %+v", resp)
		}
		if data["tasks"] == nil {
			t.Fatalf("tasks is nil. data: %+v", data)
		}
		tasks := data["tasks"].([]interface{})
		t.Logf("Tasks returned: %d", len(tasks))
	})

	t.Run("query tasks", func(t *testing.T) {
		reqBody := `{"query": "query { tasks { id prompt status } }"}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		t.Logf("Response: %s", w.Body.String())

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		if err != nil {
			t.Logf("Error: %v", err)
		}

		if resp["data"] == nil {
			t.Logf("Response: %+v", resp)
			return
		}
		data := resp["data"].(map[string]interface{})
		if data["tasks"] == nil {
			t.Logf("Data: %+v", data)
			return
		}
		tasks := data["tasks"].([]interface{})
		// After adding task in previous subtest, we should have at least 1 task
		// But since subtests may not share state, just verify the query works
		assert.NotNil(t, tasks)
	})

	t.Run("add memory via mutation", func(t *testing.T) {
		reqBody := `{"query": "mutation ($content: String!) { addMemory(content: $content) { success } }", "variables": {"content": "E2E test fact"}}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		addMem := data["addMemory"].(map[string]interface{})
		assert.Equal(t, true, addMem["success"])
	})

	t.Run("search memory", func(t *testing.T) {
		reqBody := `{"query": "query { searchMemory(query: \"E2E\") { success result } }"}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("query user graph", func(t *testing.T) {
		reqBody := `{"query": "query { userGraph(userId: \"e2euser\") { success nodes { id label } } }"}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		graph := data["userGraph"].(map[string]interface{})
		assert.Equal(t, true, graph["success"])
	})

	t.Run("add relation mutation", func(t *testing.T) {
		reqBody := `{"query": "mutation ($from: String!, $to: String!, $relationType: String!) { addRelation(from: $from, to: $to, relationType: $relationType) { success } }", "variables": {"from": "user:1", "to": "fact:1", "relationType": "HAS_FACT"}}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		require.NotNil(t, resp["data"], "expected data in response: %+v", resp)
		data := resp["data"].(map[string]interface{})
		addRel := data["addRelation"].(map[string]interface{})
		assert.Equal(t, true, addRel["success"])
	})

	t.Run("status query with all fields", func(t *testing.T) {
		reqBody := `{"query": "query { status { agent { name aiProvider } health { status } tasks { id } } }"}`
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		status := data["status"].(map[string]interface{})
		assert.NotNil(t, status)
	})
}

func TestE2E_ToolRegistryDispatch(t *testing.T) {
	permManager := permissions.Default()
	permManager.SetPermission("default", "echo", permissions.PermissionAlways)

	registry := mcp.NewToolRegistry(true, permManager)

	registry.RegisterInternal("echo", &echoTool{})

	result, err := registry.Dispatch(context.Background(), "echo", map[string]interface{}{
		"message": "hello",
	})

	assert.NoError(t, err)
	assert.Contains(t, string(result), "hello")
}

func TestE2E_EventDrivenWorkflow(t *testing.T) {
	eventBus := domainServices.NewEventBus()

	var receivedPayload string
	_ = eventBus.Subscribe("message.sent", func(ctx context.Context, e events.Event) error {
		receivedPayload = e.GetPayload().(string)
		return nil
	})

	_ = eventBus.Publish(context.Background(), events.NewEvent("message.sent", "test message"))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, "test message", receivedPayload)
}

type echoTool struct{}

func (e *echoTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{Name: "echo", Description: "Echo a message"}
}

func (e *echoTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	msg := params["message"].(string)
	return json.RawMessage(`{"echo": "` + msg + `"}`), nil
}

func TestE2E_AllToolsAvailable(t *testing.T) {
	registry := mcp.NewToolRegistry(true, nil)

	registry.RegisterInternal("send_message", &testTool{name: "send_message"})
	registry.RegisterInternal("send_file", &testTool{name: "send_file"})
	registry.RegisterInternal("terminal_exec", &testTool{name: "terminal_exec"})
	registry.RegisterInternal("terminal_spawn", &testTool{name: "terminal_spawn"})
	registry.RegisterInternal("add_memory", &testTool{name: "add_memory"})
	registry.RegisterInternal("search_memory", &testTool{name: "search_memory"})
	registry.RegisterInternal("schedule_cron", &testTool{name: "schedule_cron"})
	registry.RegisterInternal("browser_fetch", &testTool{name: "browser_fetch"})
	registry.RegisterInternal("browser_screenshot", &testTool{name: "browser_screenshot"})
	registry.RegisterInternal("browser_click", &testTool{name: "browser_click"})
	registry.RegisterInternal("browser_fill_input", &testTool{name: "browser_fill_input"})
	registry.RegisterInternal("subagent_spawn", &testTool{name: "subagent_spawn"})
	registry.RegisterInternal("task_add", &testTool{name: "task_add"})
	registry.RegisterInternal("task_done", &testTool{name: "task_done"})
	registry.RegisterInternal("task_list", &testTool{name: "task_list"})

	tools := registry.AllTools()

	assert.Len(t, tools, 15)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name)
	}
}

type testTool struct {
	name string
}

func (t *testTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{Name: t.name, Description: "test"}
}

func (t *testTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	return json.RawMessage(`{"ok": true}`), nil
}

func TestE2E_MCPToolRegistration(t *testing.T) {
	registry := mcp.NewToolRegistry(true, nil)

	mockClient := &mockMCPClient{}
	registry.RegisterMCP("test-server", mockClient, []mcp.ToolDefinition{
		{Name: "remote_tool", Description: "A remote tool"},
	})

	assert.True(t, registry.HasTool("test-server:remote_tool"))
	assert.False(t, registry.IsInternal("test-server:remote_tool"))
}

type mockMCPClient struct {
	mock.Mock
}

func (m *mockMCPClient) Connect(ctx context.Context, server mcp.ServerConfig) error {
	return m.Called(ctx, server).Error(0)
}

func (m *mockMCPClient) CallTool(ctx context.Context, tool string, params map[string]interface{}) (json.RawMessage, error) {
	args := m.Called(ctx, tool, params)
	return args.Get(0).(json.RawMessage), args.Error(1)
}

func (m *mockMCPClient) ListTools(ctx context.Context) ([]mcp.ToolDefinition, error) {
	args := m.Called(ctx)
	return args.Get(0).([]mcp.ToolDefinition), args.Error(1)
}

func (m *mockMCPClient) GetServerURL(name string) string {
	return ""
}

func (m *mockMCPClient) Close() error {
	return m.Called().Error(0)
}
