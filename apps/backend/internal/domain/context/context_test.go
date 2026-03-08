package context

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/stretchr/testify/assert"
)

func TestNewContextInjector(t *testing.T) {
	injector := NewContextInjector(
		"",
		"./agents.md",
		"./soul.md",
		"./identity.md",
		"",
		nil,
		nil,
	)
	assert.NotNil(t, injector)
}

func TestContextInjector_BuildContext_NoMemory(t *testing.T) {
	injector := NewContextInjector(
		"",
		"",
		"",
		"",
		"",
		nil,
		nil,
	)

	ctx := context.Background()
	agentCtx, err := injector.BuildContext(ctx, "user123", "session123")

	assert.NoError(t, err)
	assert.NotNil(t, agentCtx)
	assert.Empty(t, agentCtx.UserMemory)
}

func TestContextInjector_BuildContext_WithMemory(t *testing.T) {
	mockMemory := &mockMemoryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{
				{ID: "user:1", Type: "user", Label: "user:1"},
				{ID: "fact:1", Type: "fact", Value: "Likes pizza"},
			},
			Edges: []ports.GraphEdge{
				{Source: "user:1", Target: "fact:1", Label: "HAS_FACT"},
			},
		},
	}

	injector := NewContextInjector(
		"",
		"",
		"",
		"",
		"",
		mockMemory,
		nil,
	)

	ctx := context.Background()
	agentCtx, err := injector.BuildContext(ctx, "user123", "session123")

	assert.NoError(t, err)
	assert.NotNil(t, agentCtx)
	assert.Contains(t, agentCtx.UserMemory, "Likes pizza")
}

func TestContextInjector_GetUserMemory(t *testing.T) {
	mockMemory := &mockMemoryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{{ID: "1", Type: "user"}},
			Edges: []ports.GraphEdge{},
		},
	}

	injector := NewContextInjector("", "", "", "", "", mockMemory, nil)

	graph, err := injector.GetUserMemory(context.Background(), "user123")

	assert.NoError(t, err)
	assert.Len(t, graph.Nodes, 1)
}

func TestContextInjector_GetGroupMemories(t *testing.T) {
	mockMemory := &mockMemoryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{{ID: "1", Type: "user"}},
			Edges: []ports.GraphEdge{},
		},
	}

	injector := NewContextInjector("", "", "", "", "", mockMemory, nil)

	graphs, err := injector.GetGroupMemories(context.Background(), []string{"user1", "user2"})

	assert.NoError(t, err)
	assert.Len(t, graphs, 2)
}

type mockMemoryPort struct {
	graph ports.Graph
	err   error
}

func (m *mockMemoryPort) AddKnowledge(ctx context.Context, userID string, content string, label string, relation string, embedding []float64) error {
	return m.err
}
func (m *mockMemoryPort) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	return m.err
}

func (m *mockMemoryPort) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	return nil, m.err
}

func (m *mockMemoryPort) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	return m.graph, m.err
}

func (m *mockMemoryPort) AddRelation(ctx context.Context, from, to string, relType string) error {
	return m.err
}

func (m *mockMemoryPort) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	return ports.GraphResult{}, m.err
}

func (m *mockMemoryPort) InvalidateMemoryCache(ctx context.Context, userID string) error {
	return m.err
}

func (m *mockMemoryPort) SetUserProperty(ctx context.Context, userID, key, value string) error {
	return m.err
}

func (m *mockMemoryPort) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	return m.err
}

func (m *mockMemoryPort) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	return m.err
}

func TestInMemoryDigestCache_Get_NotFound(t *testing.T) {
	cache := NewInMemoryDigestCache(1 * time.Hour)

	digest, err := cache.Get("nonexistent")

	assert.NoError(t, err)
	assert.Nil(t, digest)
}

func TestInMemoryDigestCache_SetAndGet(t *testing.T) {
	cache := NewInMemoryDigestCache(1 * time.Hour)

	digest := &MemoryDigest{
		UserID:  "user123",
		Content: "Some memory content",
	}

	err := cache.Set(digest)
	assert.NoError(t, err)

	retrieved, err := cache.Get("user123")
	assert.NoError(t, err)
	assert.Equal(t, "user123", retrieved.UserID)
	assert.Equal(t, "Some memory content", retrieved.Content)
}

func TestInMemoryDigestCache_Get_Expired(t *testing.T) {
	cache := NewInMemoryDigestCache(0) // No TTL (already expired)

	digest := &MemoryDigest{
		UserID:  "user123",
		Content: "Some memory content",
	}

	_ = cache.Set(digest)

	retrieved, err := cache.Get("user123")
	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestInMemoryDigestCache_Invalidate(t *testing.T) {
	cache := NewInMemoryDigestCache(1 * time.Hour)

	digest := &MemoryDigest{
		UserID:  "user123",
		Content: "Some memory content",
	}

	_ = cache.Set(digest)
	_ = cache.Invalidate("user123")

	retrieved, err := cache.Get("user123")
	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestMemoryDigestService_GetOrRebuild_NoBackend(t *testing.T) {
	cache := NewInMemoryDigestCache(1 * time.Hour)
	service := NewMemoryDigestService(nil, cache, nil, 0)

	content, err := service.GetOrRebuild(context.Background(), "user123")

	assert.NoError(t, err)
	assert.Empty(t, content)
}

func TestMemoryDigestService_GetOrRebuild_WithCache(t *testing.T) {
	cache := NewInMemoryDigestCache(1 * time.Hour)
	mockMemory := &mockMemoryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{{ID: "1", Type: "user"}},
			Edges: []ports.GraphEdge{},
		},
	}

	service := NewMemoryDigestService(mockMemory, cache, nil, 1*time.Hour)

	_, _ = service.GetOrRebuild(context.Background(), "user123")

	content, err := service.GetOrRebuild(context.Background(), "user123")
	assert.NoError(t, err)
	assert.Contains(t, content, "User memory")
}

func TestMemoryDigestService_Invalidate(t *testing.T) {
	cache := NewInMemoryDigestCache(1 * time.Hour)
	mockMemory := &mockMemoryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{
				{ID: "user:1", Type: "user"},
				{ID: "fact:1", Type: "fact", Value: "Test fact"},
			},
			Edges: []ports.GraphEdge{
				{Source: "user:1", Target: "fact:1", Label: "HAS_FACT"},
			},
		},
	}

	service := NewMemoryDigestService(mockMemory, cache, nil, 1*time.Hour)

	content1, _ := service.GetOrRebuild(context.Background(), "user123")
	assert.Contains(t, content1, "Test fact")

	_ = service.Invalidate("user123")

	content2, _ := service.GetOrRebuild(context.Background(), "user123")
	assert.Contains(t, content2, "Test fact")
}

func TestSplitToolName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"simple", "tool", []string{"tool"}},
		{"with colon", "server:tool", []string{"server", "tool"}},
		{"multiple colons", "a:b:c", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitToolName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatGraphAsText_Empty(t *testing.T) {
	result := formatGraphAsText(nil)
	assert.Empty(t, result)

	result = formatGraphAsText(&ports.Graph{Nodes: []ports.GraphNode{}})
	assert.Empty(t, result)
}

func TestFormatGraphAsText_WithFacts(t *testing.T) {
	graph := &ports.Graph{
		Nodes: []ports.GraphNode{
			{ID: "u1", Type: "user", Value: "John"},
			{ID: "f1", Type: "fact", Value: "Likes coding"},
			{ID: "f2", Type: "fact", Value: "Likes pizza"},
		},
		Edges: []ports.GraphEdge{
			{Source: "u1", Target: "f1", Label: "HAS_FACT"},
			{Source: "u1", Target: "f2", Label: "HAS_FACT"},
		},
	}

	result := formatGraphAsText(graph)

	assert.Contains(t, result, "Likes coding")
	assert.Contains(t, result, "Likes pizza")
}

func TestFormatGraphAsText_WithUserProperties(t *testing.T) {
	graph := &ports.Graph{
		Nodes: []ports.GraphNode{
			{ID: "u1", Type: "user", Properties: map[string]string{"name": "Alice", "role": "admin"}},
			{ID: "f1", Type: "fact", Value: "Fact1"},
		},
		Edges: []ports.GraphEdge{
			{Source: "u1", Target: "f1", Label: "HAS_FACT"},
		},
	}

	result := formatGraphAsText(graph)
	assert.Contains(t, result, "User profile properties")
	assert.Contains(t, result, "name: Alice")
	assert.Contains(t, result, "role: admin")
	assert.Contains(t, result, "Fact1")
}

func TestContextInjector_BuildContext_WithToolRegistry(t *testing.T) {
	reg := mcp.NewToolRegistry(false, nil)
	reg.RegisterInternal("internal_tool", &fakeInternalTool{name: "internal_tool", desc: "Internal"})
	reg.RegisterInternal("mcp_server:external", &fakeInternalTool{name: "mcp_server:external", desc: "MCP tool"})

	injector := NewContextInjector("", "", "", "", "", nil, reg)
	ctx := context.Background()
	agentCtx, err := injector.BuildContext(ctx, "", "")
	assert.NoError(t, err)
	assert.NotNil(t, agentCtx)
	assert.Len(t, agentCtx.MCPs, 1)
	assert.Equal(t, "mcp_server", agentCtx.MCPs[0].Name)
	assert.Contains(t, agentCtx.MCPs[0].Tools, "mcp_server:external")
	assert.Len(t, agentCtx.Tools, 2)
}

type fakeInternalTool struct {
	name, desc string
}

func (f *fakeInternalTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{Name: f.name, Description: f.desc}
}

func (f *fakeInternalTool) Execute(ctx context.Context, params map[string]interface{}) (json.RawMessage, error) {
	return json.RawMessage("{}"), nil
}

func TestContextInjector_GetGroupMemories_PartialError(t *testing.T) {
	mockMemory := &mockMemoryPortPartial{
		user1Graph: ports.Graph{Nodes: []ports.GraphNode{{ID: "1", Type: "user"}}, Edges: []ports.GraphEdge{}},
		failFor:    "user2",
	}
	injector := NewContextInjector("", "", "", "", "", mockMemory, nil)
	graphs, err := injector.GetGroupMemories(context.Background(), []string{"user1", "user2"})
	assert.NoError(t, err)
	assert.Len(t, graphs, 1)
}

type mockMemoryPortPartial struct {
	user1Graph ports.Graph
	failFor    string
}

func (m *mockMemoryPortPartial) GetUserGraph(_ context.Context, userID string) (ports.Graph, error) {
	if userID == m.failFor {
		return ports.Graph{}, fmt.Errorf("not found")
	}
	return m.user1Graph, nil
}
func (m *mockMemoryPortPartial) AddKnowledge(context.Context, string, string, string, string, []float64) error {
	return nil
}
func (m *mockMemoryPortPartial) UpdateUserLabel(context.Context, string, string) error { return nil }
func (m *mockMemoryPortPartial) SearchSimilar(context.Context, string, int) ([]ports.Knowledge, error) {
	return nil, nil
}
func (m *mockMemoryPortPartial) AddRelation(context.Context, string, string, string) error {
	return nil
}
func (m *mockMemoryPortPartial) QueryGraph(context.Context, string) (ports.GraphResult, error) {
	return ports.GraphResult{}, nil
}
func (m *mockMemoryPortPartial) InvalidateMemoryCache(context.Context, string) error { return nil }
func (m *mockMemoryPortPartial) SetUserProperty(context.Context, string, string, string) error {
	return nil
}
func (m *mockMemoryPortPartial) EditMemoryNode(context.Context, string, string, string) error {
	return nil
}
func (m *mockMemoryPortPartial) DeleteMemoryNode(context.Context, string, string) error { return nil }

func TestContextInjector_GetUserMemory_BackendError(t *testing.T) {
	mockMemory := &mockMemoryPort{
		graph: ports.Graph{},
		err:   fmt.Errorf("backend error"),
	}
	injector := NewContextInjector("", "", "", "", "", mockMemory, nil)

	graph, err := injector.GetUserMemory(context.Background(), "user123")
	assert.Error(t, err)
	assert.Nil(t, graph)
}

func TestContextInjector_QueryUserMemory(t *testing.T) {
	mockMemory := &mockMemoryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{{ID: "1", Type: "user"}},
			Edges: []ports.GraphEdge{},
		},
	}

	injector := NewContextInjector("", "", "", "", "", mockMemory, nil)

	graph, err := injector.QueryUserMemory(context.Background(), "requester", "target")

	assert.NoError(t, err)
	assert.Len(t, graph.Nodes, 1)
}

func TestContextInjector_BuildContext_SystemFiles(t *testing.T) {
	mockMemory := &mockMemoryPort{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{
				{ID: "user:1", Type: "user", Label: "user:1"},
				{ID: "fact:1", Type: "fact", Value: "Test fact"},
			},
			Edges: []ports.GraphEdge{
				{Source: "user:1", Target: "fact:1", Label: "HAS_FACT"},
			},
		},
	}

	injector := NewContextInjector(
		"",
		"./agents.md",
		"./soul.md",
		"./identity.md",
		"",
		mockMemory,
		nil,
	)

	ctx := context.Background()
	agentCtx, err := injector.BuildContext(ctx, "user123", "session123")

	assert.NoError(t, err)
	assert.NotNil(t, agentCtx)
	assert.Contains(t, agentCtx.UserMemory, "Test fact")
}

func TestContextInjector_GetGroupMemories_Empty(t *testing.T) {
	injector := NewContextInjector("", "", "", "", "", nil, nil)

	graphs, err := injector.GetGroupMemories(context.Background(), []string{})

	assert.NoError(t, err)
	assert.Len(t, graphs, 0)
}

func TestContextInjector_GetUserMemory_NilBackend(t *testing.T) {
	injector := NewContextInjector("", "", "", "", "", nil, nil)

	graph, err := injector.GetUserMemory(context.Background(), "user123")

	assert.NoError(t, err)
	assert.NotNil(t, graph)
}
