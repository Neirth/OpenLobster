package context_builder

import (
	"context"
	"errors"
	"testing"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/stretchr/testify/assert"
)

type testMemoryPort struct{}

func (m *testMemoryPort) AddKnowledge(ctx context.Context, userID string, content string, label string, relation string, entityType string, embedding []float64) error {
	return nil
}
func (m *testMemoryPort) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	return nil
}
func (m *testMemoryPort) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	return nil, nil
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

type testMemoryDigestCache struct {
	data map[string]string
}

func (c *testMemoryDigestCache) Get(userID string) (string, bool) {
	if val, ok := c.data[userID]; ok {
		return val, true
	}
	return "", false
}

func (c *testMemoryDigestCache) Set(userID, content string) {
	if c.data == nil {
		c.data = make(map[string]string)
	}
	c.data[userID] = content
}

func (c *testMemoryDigestCache) Invalidate(userID string) {
	delete(c.data, userID)
}

func TestNewService(t *testing.T) {
	builder := NewService("agents", "soul", "identity", []string{"mcp1"}, []string{"tool1"}, nil, nil)
	assert.NotNil(t, builder)
}

func TestService_Build(t *testing.T) {
	builder := NewService("agents", "soul", "identity", []string{"mcp1"}, []string{"tool1"}, nil, nil)

	ctx, err := builder.Build(context.Background(), "user1")

	assert.NoError(t, err)
	assert.Contains(t, ctx, "agents")
	assert.Contains(t, ctx, "soul")
	assert.Contains(t, ctx, "identity")
	assert.Contains(t, ctx, "mcp1")
	assert.Contains(t, ctx, "tool1")
}

func TestService_Build_WithMemory(t *testing.T) {
	cache := &testMemoryDigestCache{}
	cache.Set("user1", "User memory content")

	memoryPort := &testMemoryPort{}
	memoryDigest := NewMemoryDigestService(memoryPort, cache, 3600)

	builder := NewService("", "", "", []string{}, []string{}, memoryPort, memoryDigest)

	ctx, err := builder.Build(context.Background(), "user1")

	assert.NoError(t, err)
	assert.Contains(t, ctx, "User memory content")
}

func TestMemoryDigestService_GetOrRebuild_EmptyCache(t *testing.T) {
	memoryPort := &testMemoryPort{}
	cache := &testMemoryDigestCache{}

	memoryDigest := NewMemoryDigestService(memoryPort, cache, 3600)

	content, err := memoryDigest.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.Empty(t, content)
}

func TestMemoryDigestService_GetOrRebuild_WithCache(t *testing.T) {
	memoryPort := &testMemoryPort{}
	cache := &testMemoryDigestCache{}
	cache.Set("user1", "Cached content")

	memoryDigest := NewMemoryDigestService(memoryPort, cache, 3600)

	content, err := memoryDigest.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.Equal(t, "Cached content", content)
}

func TestMemoryDigestService_Invalidate(t *testing.T) {
	cache := &testMemoryDigestCache{}
	cache.Set("user1", "Cached content")

	memoryDigest := NewMemoryDigestService(&testMemoryPort{}, cache, 3600)
	memoryDigest.Invalidate("user1")

	_, ok := cache.Get("user1")
	assert.False(t, ok)
}

func TestService_Build_EmptyUserID(t *testing.T) {
	builder := NewService("a", "b", "c", nil, nil, nil, nil)
	ctx, err := builder.Build(context.Background(), "")
	assert.NoError(t, err)
	assert.Contains(t, ctx, "a")
}

func TestService_BuildForGroup(t *testing.T) {
	builder := NewService("x", "", "", nil, nil, nil, nil)
	ctx, err := builder.BuildForGroup(context.Background(), []string{"u1", "u2"})
	assert.NoError(t, err)
	assert.Contains(t, ctx, "x")
	assert.Contains(t, ctx, "---")
}

func TestService_BuildForGroup_EmptyBuilder(t *testing.T) {
	builder := NewService("", "", "", nil, nil, nil, nil)
	ctx, err := builder.BuildForGroup(context.Background(), []string{"u1"})
	assert.NoError(t, err)
	// With empty builder and no memory, result can be empty
	_ = ctx
}

func TestMemoryDigestService_GetOrRebuild_WithGraph(t *testing.T) {
	memoryPort := &testMemoryPortWithGraph{
		graph: ports.Graph{
			Nodes: []ports.GraphNode{
				{ID: "n1", Type: "fact", Value: "Likes coding"},
				{ID: "n2", Type: "user"},
			},
		},
	}
	cache := &testMemoryDigestCache{}

	memoryDigest := NewMemoryDigestService(memoryPort, cache, 3600)
	content, err := memoryDigest.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.Contains(t, content, "Likes coding")
	assert.Contains(t, content, "nodes[")
}

func TestMemoryDigestService_GetOrRebuild_BackendError(t *testing.T) {
	memoryPort := &testMemoryPortErr{}
	cache := &testMemoryDigestCache{}
	memoryDigest := NewMemoryDigestService(memoryPort, cache, 3600)

	content, err := memoryDigest.GetOrRebuild(context.Background(), "user1")

	assert.Error(t, err)
	assert.Empty(t, content)
}

type testMemoryPortWithGraph struct {
	graph ports.Graph
}

func (m *testMemoryPortWithGraph) AddKnowledge(context.Context, string, string, string, string, string, []float64) error {
	return nil
}
func (m *testMemoryPortWithGraph) UpdateUserLabel(context.Context, string, string) error { return nil }
func (m *testMemoryPortWithGraph) SearchSimilar(context.Context, string, int) ([]ports.Knowledge, error) {
	return nil, nil
}
func (m *testMemoryPortWithGraph) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	return m.graph, nil
}
func (m *testMemoryPortWithGraph) AddRelation(context.Context, string, string, string) error {
	return nil
}
func (m *testMemoryPortWithGraph) QueryGraph(context.Context, string) (ports.GraphResult, error) {
	return ports.GraphResult{}, nil
}
func (m *testMemoryPortWithGraph) InvalidateMemoryCache(context.Context, string) error { return nil }
func (m *testMemoryPortWithGraph) SetUserProperty(context.Context, string, string, string) error {
	return nil
}
func (m *testMemoryPortWithGraph) EditMemoryNode(context.Context, string, string, string) error {
	return nil
}
func (m *testMemoryPortWithGraph) DeleteMemoryNode(context.Context, string, string) error { return nil }

type testMemoryPortErr struct{}

func (m *testMemoryPortErr) AddKnowledge(context.Context, string, string, string, string, string, []float64) error {
	return nil
}
func (m *testMemoryPortErr) UpdateUserLabel(context.Context, string, string) error { return nil }
func (m *testMemoryPortErr) SearchSimilar(context.Context, string, int) ([]ports.Knowledge, error) {
	return nil, nil
}
func (m *testMemoryPortErr) GetUserGraph(context.Context, string) (ports.Graph, error) {
	return ports.Graph{}, errors.New("backend error")
}
func (m *testMemoryPortErr) AddRelation(context.Context, string, string, string) error { return nil }
func (m *testMemoryPortErr) QueryGraph(context.Context, string) (ports.GraphResult, error) {
	return ports.GraphResult{}, nil
}
func (m *testMemoryPortErr) InvalidateMemoryCache(context.Context, string) error { return nil }
func (m *testMemoryPortErr) SetUserProperty(context.Context, string, string, string) error {
	return nil
}
func (m *testMemoryPortErr) EditMemoryNode(context.Context, string, string, string) error { return nil }
func (m *testMemoryPortErr) DeleteMemoryNode(context.Context, string, string) error       { return nil }
