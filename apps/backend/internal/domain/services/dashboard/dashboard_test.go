package dashboard

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

type testTaskRepo struct {
	tasks []models.Task
	err   error
}

func (r *testTaskRepo) GetPending(ctx context.Context) ([]models.Task, error) {
	return r.tasks, r.err
}
func (r *testTaskRepo) ListAll(ctx context.Context) ([]models.Task, error) {
	return r.tasks, r.err
}
func (r *testTaskRepo) Add(ctx context.Context, task *models.Task) error {
	if r.err != nil {
		return r.err
	}
	r.tasks = append(r.tasks, *task)
	return nil
}
func (r *testTaskRepo) MarkDone(ctx context.Context, id string) error {
	return r.err
}
func (r *testTaskRepo) Delete(ctx context.Context, id string) error {
	if r.err != nil {
		return r.err
	}
	for i, t := range r.tasks {
		if t.ID == id {
			r.tasks = append(r.tasks[:i], r.tasks[i+1:]...)
			break
		}
	}
	return nil
}
func (r *testTaskRepo) Update(ctx context.Context, task *models.Task) error {
	if r.err != nil {
		return r.err
	}
	for i := range r.tasks {
		if r.tasks[i].ID == task.ID {
			r.tasks[i] = *task
			break
		}
	}
	return nil
}
func (r *testTaskRepo) SetEnabled(ctx context.Context, id string, enabled bool) error {
	if r.err != nil {
		return r.err
	}
	for i := range r.tasks {
		if r.tasks[i].ID == id {
			r.tasks[i].Enabled = enabled
			break
		}
	}
	return nil
}

func (r *testTaskRepo) SetStatus(ctx context.Context, id string, status string) error {
	if r.err != nil {
		return r.err
	}
	for i := range r.tasks {
		if r.tasks[i].ID == id {
			r.tasks[i].Status = status
			break
		}
	}
	return nil
}

type testGraphCommandPort struct {
	err error
}

func (g *testGraphCommandPort) AddRelation(ctx context.Context, from, to, relType string) error {
	return g.err
}

type testMemoryPort struct{}

func (m *testMemoryPort) AddKnowledge(ctx context.Context, userID string, content string, label string, relation string, entityType string, embedding []float64) error {
	return nil
}
func (m *testMemoryPort) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	return nil
}
func (m *testMemoryPort) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	return []ports.Knowledge{{Content: "found"}}, nil
}
func (m *testMemoryPort) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	return ports.Graph{}, nil
}
func (m *testMemoryPort) AddRelation(ctx context.Context, from, to string, relType string) error {
	return nil
}
func (m *testMemoryPort) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	return ports.GraphResult{}, nil
}
func (m *testMemoryPort) InvalidateMemoryCache(ctx context.Context, userID string) error {
	return nil
}
func (m *testMemoryPort) SetUserProperty(ctx context.Context, userID, key, value string) error {
	return nil
}
func (m *testMemoryPort) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	return nil
}
func (m *testMemoryPort) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	return nil
}

func TestNewQueryService(t *testing.T) {
	svc := NewQueryService(nil, nil, nil, nil, nil)
	assert.NotNil(t, svc)
}

func TestQueryService_GetTasks_NilRepo(t *testing.T) {
	svc := NewQueryService(nil, nil, nil, nil, nil)
	tasks, err := svc.GetTasks(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestQueryService_GetTasks_WithRepo(t *testing.T) {
	now := time.Now()
	repo := &testTaskRepo{
		tasks: []models.Task{{ID: "t1", Prompt: "Do it", Status: "pending", AddedAt: now}},
	}
	svc := NewQueryService(repo, nil, nil, nil, nil)
	tasks, err := svc.GetTasks(context.Background())
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "t1", tasks[0].ID)
}

func TestQueryService_SearchMemory_NilRepo(t *testing.T) {
	svc := NewQueryService(nil, nil, nil, nil, nil)
	result, err := svc.SearchMemory(context.Background(), "u1", "query")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestQueryService_SearchMemory_WithResults(t *testing.T) {
	svc := NewQueryService(nil, &testMemoryPort{}, nil, nil, nil)
	result, err := svc.SearchMemory(context.Background(), "u1", "query")
	assert.NoError(t, err)
	assert.Contains(t, result, "found")
}

func TestQueryService_GetUserGraph_NilRepo(t *testing.T) {
	svc := NewQueryService(nil, nil, nil, nil, nil)
	graph, err := svc.GetUserGraph(context.Background(), "u1")
	assert.NoError(t, err)
	assert.Empty(t, graph.Nodes)
}

func TestQueryService_ExecuteCypher_NilRepo(t *testing.T) {
	svc := NewQueryService(nil, nil, nil, nil, nil)
	result, err := svc.ExecuteCypher(context.Background(), "MATCH (n) RETURN n")
	assert.NoError(t, err)
	assert.Empty(t, result.Data)
}

func TestQueryService_GetUserGraph_WithRepo(t *testing.T) {
	graphRepo := &testGraphQueryPort{graph: ports.Graph{
		Nodes: []ports.GraphNode{{ID: "n1", Value: "fact1"}},
		Edges: []ports.GraphEdge{},
	}}
	svc := NewQueryService(nil, nil, graphRepo, nil, nil)
	graph, err := svc.GetUserGraph(context.Background(), "u1")
	assert.NoError(t, err)
	assert.Len(t, graph.Nodes, 1)
	assert.Equal(t, "fact1", graph.Nodes[0].Value)
}

func TestQueryService_ExecuteCypher_WithRepo(t *testing.T) {
	graphRepo := &testGraphQueryPort{data: []map[string]interface{}{{"n": 1}}}
	svc := NewQueryService(nil, nil, graphRepo, nil, nil)
	result, err := svc.ExecuteCypher(context.Background(), "MATCH (n) RETURN n")
	assert.NoError(t, err)
	assert.Len(t, result.Data, 1)
}

func TestNewCommandService(t *testing.T) {
	svc := NewCommandService(nil, nil, nil)
	assert.NotNil(t, svc)
}

func TestCommandService_AddTask_NilRepo(t *testing.T) {
	svc := NewCommandService(nil, nil, nil)
	id, err := svc.AddTask(context.Background(), "prompt", "")
	assert.NoError(t, err)
	assert.Empty(t, id)
}

func TestCommandService_AddTask_WithRepo(t *testing.T) {
	repo := &testTaskRepo{}
	svc := NewCommandService(repo, nil, nil)
	id, err := svc.AddTask(context.Background(), "Do work", "")
	assert.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Len(t, repo.tasks, 1)
	assert.Equal(t, "Do work", repo.tasks[0].Prompt)
}

func TestCommandService_AddTask_RepoError(t *testing.T) {
	repo := &testTaskRepo{err: fmt.Errorf("db full")}
	svc := NewCommandService(repo, nil, nil)
	_, err := svc.AddTask(context.Background(), "x", "")
	assert.Error(t, err)
}

func TestCommandService_AddMemory_WithRepo(t *testing.T) {
	svc := NewCommandService(nil, &testMemoryPort{}, nil)
	err := svc.AddMemory(context.Background(), "u1", "fact")
	assert.NoError(t, err)
}

func TestCommandService_AddRelation_WithRepo(t *testing.T) {
	svc := NewCommandService(nil, nil, &testGraphCommandPort{})
	err := svc.AddRelation(context.Background(), "a", "b", "KNOWS")
	assert.NoError(t, err)
}

func TestCommandService_CompleteTask(t *testing.T) {
	repo := &testTaskRepo{}
	svc := NewCommandService(repo, nil, nil)
	_, _ = svc.AddTask(context.Background(), "task", "")
	err := svc.CompleteTask(context.Background(), repo.tasks[0].ID)
	assert.NoError(t, err)
}

func TestCommandService_DeleteTask(t *testing.T) {
	repo := &testTaskRepo{}
	svc := NewCommandService(repo, nil, nil)
	_, _ = svc.AddTask(context.Background(), "task", "")
	err := svc.DeleteTask(context.Background(), repo.tasks[0].ID)
	assert.NoError(t, err)
}

func TestCommandService_UpdateTask(t *testing.T) {
	repo := &testTaskRepo{}
	svc := NewCommandService(repo, nil, nil)
	_, _ = svc.AddTask(context.Background(), "old", "")
	err := svc.UpdateTask(context.Background(), repo.tasks[0].ID, "new prompt", "0 9 * * *")
	assert.NoError(t, err)
	assert.Equal(t, "new prompt", repo.tasks[0].Prompt)
}

func TestCommandService_ToggleTask(t *testing.T) {
	repo := &testTaskRepo{}
	svc := NewCommandService(repo, nil, nil)
	_, _ = svc.AddTask(context.Background(), "task", "")
	err := svc.ToggleTask(context.Background(), repo.tasks[0].ID, false)
	assert.NoError(t, err)
}

type testGraphQueryPort struct {
	graph ports.Graph
	data  []map[string]interface{}
	err   error
}

func (g *testGraphQueryPort) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	return g.graph, g.err
}
func (g *testGraphQueryPort) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	return ports.GraphResult{Data: g.data}, g.err
}

func TestQueryService_GetUserGraph_WithData(t *testing.T) {
	graph := ports.Graph{
		Nodes: []ports.GraphNode{{ID: "n1", Label: "Node1"}},
		Edges: []ports.GraphEdge{{Source: "n1", Target: "n2", Label: "REL"}},
	}
	svc := NewQueryService(nil, nil, &testGraphQueryPort{graph: graph}, nil, nil)
	result, err := svc.GetUserGraph(context.Background(), "u1")
	assert.NoError(t, err)
	assert.Len(t, result.Nodes, 1)
	assert.Len(t, result.Edges, 1)
}

func TestQueryService_ExecuteCypher_WithData(t *testing.T) {
	data := []map[string]interface{}{{"n": "value"}}
	svc := NewQueryService(nil, nil, &testGraphQueryPort{data: data}, nil, nil)
	result, err := svc.ExecuteCypher(context.Background(), "MATCH (n) RETURN n")
	assert.NoError(t, err)
	assert.Len(t, result.Data, 1)
}

func TestQueryService_GetUserGraph_Error(t *testing.T) {
	graphRepo := &testGraphQueryPort{err: fmt.Errorf("graph error")}
	svc := NewQueryService(nil, nil, graphRepo, nil, nil)
	graph, err := svc.GetUserGraph(context.Background(), "u1")
	assert.Error(t, err)
	assert.Empty(t, graph.Nodes)
}

func TestQueryService_ExecuteCypher_Error(t *testing.T) {
	graphRepo := &testGraphQueryPort{err: fmt.Errorf("cypher error")}
	svc := NewQueryService(nil, nil, graphRepo, nil, nil)
	_, err := svc.ExecuteCypher(context.Background(), "INVALID")
	assert.Error(t, err)
}

func TestQueryService_SearchMemory_Error(t *testing.T) {
	mem := &testMemoryPortWithError{err: fmt.Errorf("search failed")}
	svc := NewQueryService(nil, mem, nil, nil, nil)
	_, err := svc.SearchMemory(context.Background(), "u1", "query")
	assert.Error(t, err)
}

type testMemoryPortWithError struct {
	err error
}

func (m *testMemoryPortWithError) AddKnowledge(ctx context.Context, userID string, content string, label string, relation string, entityType string, embedding []float64) error {
	return m.err
}
func (m *testMemoryPortWithError) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	return nil, m.err
}
func (m *testMemoryPortWithError) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	return ports.Graph{}, m.err
}
func (m *testMemoryPortWithError) AddRelation(ctx context.Context, from, to string, relType string) error {
	return m.err
}
func (m *testMemoryPortWithError) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	return ports.GraphResult{}, m.err
}
func (m *testMemoryPortWithError) InvalidateMemoryCache(ctx context.Context, userID string) error {
	return m.err
}
func (m *testMemoryPortWithError) SetUserProperty(ctx context.Context, userID, key, value string) error {
	return m.err
}
func (m *testMemoryPortWithError) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	return m.err
}
func (m *testMemoryPortWithError) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	return m.err
}
func (m *testMemoryPortWithError) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	return m.err
}

func TestCommandService_UpdateNode_DeleteNode_NoMutator(t *testing.T) {
	svc := NewCommandService(nil, &testMemoryPort{}, nil)
	err := svc.UpdateNode(context.Background(), "n1", "l", "t", "v", nil)
	assert.NoError(t, err)
	err = svc.DeleteNode(context.Background(), "n1")
	assert.NoError(t, err)
}

func TestCommandService_UpdateNode_DeleteNode_WithMutator(t *testing.T) {
	mutator := &testNodeMutator{}
	svc := NewCommandService(nil, mutator, nil)
	err := svc.UpdateNode(context.Background(), "n1", "l", "t", "v", map[string]string{"k": "v"})
	assert.NoError(t, err)
	assert.True(t, mutator.updated)
	err = svc.DeleteNode(context.Background(), "n1")
	assert.NoError(t, err)
	assert.True(t, mutator.deleted)
}

type testNodeMutator struct {
	updated, deleted bool
}

func (t *testNodeMutator) AddKnowledge(ctx context.Context, userID string, content string, label string, relation string, entityType string, embedding []float64) error {
	return nil
}
func (t *testNodeMutator) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	return nil, nil
}
func (t *testNodeMutator) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	return ports.Graph{}, nil
}
func (t *testNodeMutator) AddRelation(ctx context.Context, from, to string, relType string) error {
	return nil
}
func (t *testNodeMutator) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	return ports.GraphResult{}, nil
}
func (t *testNodeMutator) InvalidateMemoryCache(ctx context.Context, userID string) error {
	return nil
}
func (t *testNodeMutator) SetUserProperty(ctx context.Context, userID, key, value string) error {
	return nil
}
func (t *testNodeMutator) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	return nil
}
func (t *testNodeMutator) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	return nil
}
func (t *testNodeMutator) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	return nil
}
func (t *testNodeMutator) UpdateNode(ctx context.Context, id, label, typ, value string, properties map[string]string) error {
	t.updated = true
	return nil
}
func (t *testNodeMutator) DeleteNode(ctx context.Context, id string) error {
	t.deleted = true
	return nil
}
