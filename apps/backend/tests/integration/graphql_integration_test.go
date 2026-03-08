package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/neirth/openlobster/internal/application/graphql"
	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/repositories"
	svcdashboard "github.com/neirth/openlobster/internal/domain/services/dashboard"
	"github.com/neirth/openlobster/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *persistence.Database {
	db, err := persistence.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	require.NoError(t, persistence.Migrate(db.GormDB(), "sqlite3"))
	return db
}

func TestGraphQL_QueryAgent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:          "TestAgent",
			AIProvider:    "openai",
			MemoryBackend: "file",
			Status:        "ok",
			Channels:      []dto.ChannelStatus{{Type: "telegram", Enabled: true}},
		},
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "query { agent { name aiProvider memoryBackend } }"}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	agent := data["agent"].(map[string]interface{})
	assert.Equal(t, "TestAgent", agent["name"])
	assert.Equal(t, "openai", agent["aiProvider"])
	assert.Equal(t, "file", agent["memoryBackend"])
}

func TestGraphQL_QueryTasks_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	taskRepo := repositories.NewTaskRepository(db.GormDB())
	querySvc := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	commandSvc := svcdashboard.NewCommandService(taskRepo, nil, nil)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Test", Status: "ok"},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
		TaskRepo:   taskRepo,
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "query { tasks { id prompt status } }"}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	tasks := data["tasks"].([]interface{})
	assert.Len(t, tasks, 0)
}

func TestGraphQL_MutationAddTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	taskRepo := repositories.NewTaskRepository(db.GormDB())
	querySvc := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	commandSvc := svcdashboard.NewCommandService(taskRepo, nil, nil)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Test", Status: "ok"},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
		TaskRepo:   taskRepo,
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "mutation ($prompt: String!) { addTask(prompt: $prompt) { id prompt status } }", "variables": {"prompt": "Test task"}}`
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
	assert.Equal(t, "Test task", addTask["prompt"])
}

func TestGraphQL_MutationAddAndCompleteTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	taskRepo := repositories.NewTaskRepository(db.GormDB())
	querySvc := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	commandSvc := svcdashboard.NewCommandService(taskRepo, nil, nil)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Test", Status: "ok"},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
		TaskRepo:   taskRepo,
	})
	handler := graphql.NewGraphQLServer(deps)

	addReq := `{"query": "mutation ($prompt: String!) { addTask(prompt: $prompt) { id prompt status } }", "variables": {"prompt": "Test task"}}`
	addReqHTTP := httptest.NewRequest("POST", "/graphql", strings.NewReader(addReq))
	addReqHTTP.Header.Set("Content-Type", "application/json")
	addResp := httptest.NewRecorder()
	handler.ServeHTTP(addResp, addReqHTTP)

	var addResult map[string]interface{}
	err := json.Unmarshal(addResp.Body.Bytes(), &addResult)
	require.NoError(t, err)

	addTaskData, ok := addResult["data"].(map[string]interface{})["addTask"].(map[string]interface{})
	require.True(t, ok, "addTask response should have data")
	taskID, ok := addTaskData["id"].(string)
	require.True(t, ok, "id should be a string")

	completeReqBody := fmt.Sprintf(`{"query": "mutation ($taskId: String!) { completeTask(taskId: $taskId) }", "variables": {"taskId": "%s"}}`, taskID)
	completeReqHTTP := httptest.NewRequest("POST", "/graphql", strings.NewReader(completeReqBody))
	completeReqHTTP.Header.Set("Content-Type", "application/json")
	completeResp := httptest.NewRecorder()
	handler.ServeHTTP(completeResp, completeReqHTTP)

	var completeResult map[string]interface{}
	err = json.Unmarshal(completeResp.Body.Bytes(), &completeResult)
	require.NoError(t, err)

	completeTaskResult := completeResult["data"].(map[string]interface{})["completeTask"]
	assert.Equal(t, true, completeTaskResult)

	listReq := `{"query": "query { tasks { id status } }"}`
	listReqHTTP := httptest.NewRequest("POST", "/graphql", strings.NewReader(listReq))
	listReqHTTP.Header.Set("Content-Type", "application/json")
	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, listReqHTTP)

	var listResult map[string]interface{}
	json.Unmarshal(listResp.Body.Bytes(), &listResult)

	tasks := listResult["data"].(map[string]interface{})["tasks"].([]interface{})
	assert.Len(t, tasks, 1)
}

func TestGraphQL_MutationAddMemory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	memRepo := &graphql.TestMemoryRepo{}
	querySvc := svcdashboard.NewQueryService(nil, memRepo, nil, nil, nil)
	commandSvc := svcdashboard.NewCommandService(nil, memRepo, nil)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Test", Status: "ok"},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
		MemoryRepo: memRepo,
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "mutation ($content: String!) { addMemory(content: $content) { success } }", "variables": {"content": "Important fact"}}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	addMem := data["addMemory"].(map[string]interface{})
	assert.Equal(t, true, addMem["success"])
}

func TestGraphQL_MutationSearchMemory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	memRepo := &graphql.TestMemoryRepo{}
	querySvc := svcdashboard.NewQueryService(nil, memRepo, nil, nil, nil)
	commandSvc := svcdashboard.NewCommandService(nil, memRepo, nil)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Test", Status: "ok"},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
		MemoryRepo: memRepo,
	})
	handler := graphql.NewGraphQLServer(deps)

	addReq := `{"query": "mutation ($content: String!) { addMemory(content: $content) { success } }", "variables": {"content": "Go is a programming language"}}`
	addReqHTTP := httptest.NewRequest("POST", "/graphql", strings.NewReader(addReq))
	addReqHTTP.Header.Set("Content-Type", "application/json")
	addResp := httptest.NewRecorder()
	handler.ServeHTTP(addResp, addReqHTTP)

	searchReq := `{"query": "query ($query: String!) { searchMemory(query: $query) { success result } }", "variables": {"query": "programming"}}`
	searchReqHTTP := httptest.NewRequest("POST", "/graphql", strings.NewReader(searchReq))
	searchReqHTTP.Header.Set("Content-Type", "application/json")
	searchResp := httptest.NewRecorder()
	handler.ServeHTTP(searchResp, searchReqHTTP)

	t.Logf("Response body: %s", searchResp.Body.String())

	var resp map[string]interface{}
	err := json.Unmarshal(searchResp.Body.Bytes(), &resp)
	require.NoError(t, err, "should parse JSON")

	if resp["data"] == nil {
		t.Logf("Response: %+v", resp)
	}

	data := resp["data"].(map[string]interface{})
	search, ok := data["searchMemory"].(map[string]interface{})
	require.True(t, ok, "searchMemory should be in response")
	assert.Equal(t, true, search["success"])
}

func TestGraphQL_QueryChannels(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:   "Test",
			Status: "ok",
			Channels: []dto.ChannelStatus{
				{Type: "telegram", Enabled: true},
				{Type: "discord", Enabled: false},
			},
		},
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "query { channels { type enabled } }"}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data := resp["data"].(map[string]interface{})
	channels := data["channels"].([]interface{})
	assert.Len(t, channels, 2)
}

func TestGraphQL_StatusQuery(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	taskRepo := repositories.NewTaskRepository(db.GormDB())
	querySvc := svcdashboard.NewQueryService(taskRepo, nil, nil, nil, nil)
	commandSvc := svcdashboard.NewCommandService(taskRepo, nil, nil)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:       "TestAgent",
			AIProvider: "openai",
			Status:     "ok",
			Channels:   []dto.ChannelStatus{{Type: "telegram", Enabled: true}},
		},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
		TaskRepo:   taskRepo,
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "query { status { agent { name } health { status } tasks { id } } }"}`
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
}

func TestGraphQL_MultipleQueries(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	taskRepo := repositories.NewTaskRepository(db.GormDB())
	memRepo := &graphql.TestMemoryRepo{}
	querySvc := svcdashboard.NewQueryService(taskRepo, memRepo, nil, nil, nil)
	commandSvc := svcdashboard.NewCommandService(taskRepo, memRepo, nil)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:       "TestAgent",
			AIProvider: "openai",
			Status:     "ok",
			Channels:   []dto.ChannelStatus{{Type: "telegram", Enabled: true}},
		},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
		TaskRepo:   taskRepo,
		MemoryRepo: memRepo,
	})
	handler := graphql.NewGraphQLServer(deps)

	_ = taskRepo.Add(context.Background(), &models.Task{
		ID:     "task-1",
		Prompt: "Test task",
		Status: "pending",
	})

	reqBody := `{"query": "query { agent { name } channels { type } tasks { id prompt } }"}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	assert.Contains(t, data, "agent")
}

func TestGraphQL_ErrorHandling(t *testing.T) {
	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{Name: "Test", Status: "ok"},
	})
	handler := graphql.NewGraphQLServer(deps)

	// Invalid GraphQL: field does not exist. GraphQL returns 200 with errors in body, or 422 for parse errors.
	reqBody := `{"query": "query { nonexistentField }"}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// GraphQL may return 200 (with errors) or 422 for validation failures
	assert.True(t, w.Code == http.StatusOK || w.Code == 422, "expected 200 or 422, got %d", w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Response should contain either "data" or "errors"
	assert.True(t, resp["data"] != nil || resp["errors"] != nil, "response should have data or errors: %+v", resp)
}

func TestGraphQL_NestedFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mockGraphRepo := &graphql.TestGraphRepo{
		GetUserGraphFunc: func(ctx context.Context, userID string) (ports.Graph, error) {
			return ports.Graph{
				Nodes: []ports.GraphNode{
					{ID: "user:1", Label: "User", Type: "user", Value: userID},
					{ID: "fact:1", Label: "Fact", Type: "fact", Value: "Test fact"},
				},
				Edges: []ports.GraphEdge{
					{Source: "user:1", Target: "fact:1", Label: "HAS_FACT"},
				},
			}, nil
		},
		QueryGraphFunc: func(ctx context.Context, cypher string) (ports.GraphResult, error) {
			return ports.GraphResult{Data: []map[string]interface{}{}}, nil
		},
	}
	querySvc := svcdashboard.NewQueryService(nil, nil, mockGraphRepo, nil, nil)
	commandSvc := svcdashboard.NewCommandService(nil, nil, mockGraphRepo)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent: &dto.AgentSnapshot{
			Name:       "TestAgent",
			AIProvider: "openai",
			Status:     "ok",
			Channels:   []dto.ChannelStatus{{Type: "telegram", Enabled: true}},
		},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "query { userGraph(userId: \"user1\") { success nodes { id } edges { source } } }"}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	userGraph := data["userGraph"].(map[string]interface{})
	assert.Equal(t, true, userGraph["success"])
	assert.NotNil(t, userGraph["nodes"])
}

func TestGraphQL_MutationAddRelation(t *testing.T) {
	mockGraphRepo := &graphql.TestGraphRepo{}
	querySvc := svcdashboard.NewQueryService(nil, nil, mockGraphRepo, nil, nil)
	commandSvc := svcdashboard.NewCommandService(nil, nil, mockGraphRepo)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Test", Status: "ok"},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "mutation ($from: String!, $to: String!, $relationType: String!) { addRelation(from: $from, to: $to, relationType: $relationType) { success } }", "variables": {"from": "entity1", "to": "entity2", "relationType": "KNOWS"}}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	addRel := data["addRelation"].(map[string]interface{})
	assert.Equal(t, true, addRel["success"])
}

func TestGraphQL_QueryCypher(t *testing.T) {
	mockGraphRepo := &graphql.TestGraphRepo{}
	querySvc := svcdashboard.NewQueryService(nil, nil, mockGraphRepo, nil, nil)
	commandSvc := svcdashboard.NewCommandService(nil, nil, mockGraphRepo)

	deps := graphql.NewTestDeps(graphql.TestDepsOpts{
		Agent:      &dto.AgentSnapshot{Name: "Test", Status: "ok"},
		QuerySvc:   querySvc,
		CommandSvc: commandSvc,
	})
	handler := graphql.NewGraphQLServer(deps)

	reqBody := `{"query": "mutation ($cypher: String!) { executeCypher(cypher: $cypher) { success } }", "variables": {"cypher": "MATCH (n) RETURN n"}}`
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	cypherResult := data["executeCypher"].(map[string]interface{})
	assert.Equal(t, true, cypherResult["success"])
}
