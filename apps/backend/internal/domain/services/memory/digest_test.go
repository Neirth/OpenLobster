package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMemoryDigestService_GetOrRebuild_EmptyCache(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	backend.On("GetUserGraph", "user1").Return(Graph{
		Nodes: []Node{{ID: "1", Type: "fact", Value: "Test fact"}},
		Edges: []Edge{},
	}, nil)
	cache.On("Get", "user1").Return(nil, nil)
	cache.On("Set", mock.Anything).Return(nil)

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	result, err := service.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	backend.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestMemoryDigestService_GetOrRebuild_ValidCache(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	expTime := time.Now().Add(time.Hour)
	cache.On("Get", "user1").Return(&MemoryDigest{
		UserID:    "user1",
		Content:   "Cached content",
		ExpiresAt: expTime,
		Stale:     false,
	}, nil)

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	result, err := service.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.Equal(t, "Cached content", result)
	backend.AssertNotCalled(t, "GetUserGraph")
}

func TestMemoryDigestService_GetOrRebuild_StaleCache(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	cache.On("Get", "user1").Return(&MemoryDigest{
		UserID:    "user1",
		Content:   "Old content",
		ExpiresAt: time.Now().Add(time.Hour),
		Stale:     true,
	}, nil)
	backend.On("GetUserGraph", "user1").Return(Graph{
		Nodes: []Node{{ID: "1", Type: "fact", Value: "New fact"}},
		Edges: []Edge{},
	}, nil)
	cache.On("Set", mock.Anything).Return(nil)

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	result, err := service.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	backend.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestMemoryDigestService_GetOrRebuild_EmptyGraph(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	cache.On("Get", "user1").Return(nil, nil)
	backend.On("GetUserGraph", "user1").Return(Graph{
		Nodes: []Node{},
		Edges: []Edge{},
	}, nil)

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	result, err := service.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestMemoryDigestService_Invalidate(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	cache.On("Invalidate", "user1").Return(nil)

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	err := service.Invalidate("user1")

	assert.NoError(t, err)
	cache.AssertExpectations(t)
}

func TestMemoryDigestService_summarizeGraph(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	graph := Graph{
		Nodes: []Node{
			{ID: "1", Type: "fact", Value: "Likes pizza"},
			{ID: "2", Type: "fact", Value: "Works at Google"},
			{ID: "3", Type: "user", Value: "user1"},
		},
		Edges: []Edge{},
	}

	result := service.summarizeGraph(graph)

	assert.Contains(t, result, "Likes pizza")
	assert.Contains(t, result, "Works at Google")
	// Non-fact nodes (e.g. "user" type) are also included in the graph summary.
	assert.Contains(t, result, "user1")
}

func TestMemoryDigestService_summarizeGraph_WithEdges(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	graph := Graph{
		Nodes: []Node{
			{ID: "a", Type: "entity", Label: "Person", Value: "Alice"},
			{ID: "b", Type: "entity", Label: "Person", Value: "Bob"},
		},
		Edges: []Edge{
			{Source: "a", Target: "b", Label: "knows"},
		},
	}

	result := service.summarizeGraph(graph)
	assert.Contains(t, result, "edges[1]{source,target,label}:")
	assert.Contains(t, result, "a,b,knows")
}

func TestMemoryDigestService_GetOrRebuild_ExpiredCache(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	cache.On("Get", "user1").Return(&MemoryDigest{
		UserID:    "user1",
		Content:   "Old content",
		ExpiresAt: time.Now().Add(-time.Minute),
		Stale:     false,
	}, nil)
	backend.On("GetUserGraph", "user1").Return(Graph{
		Nodes: []Node{{ID: "1", Type: "fact", Value: "New fact"}},
		Edges: []Edge{},
	}, nil)
	cache.On("Set", mock.Anything).Return(nil)

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	result, err := service.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	backend.AssertExpectations(t)
}

func TestMemoryDigestService_GetOrRebuild_CacheError(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	cache.On("Get", "user1").Return(nil, assert.AnError)
	backend.On("GetUserGraph", "user1").Return(Graph{
		Nodes: []Node{{ID: "1", Type: "fact", Value: "Fact"}},
		Edges: []Edge{},
	}, nil)
	cache.On("Set", mock.Anything).Return(nil)

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	result, err := service.GetOrRebuild(context.Background(), "user1")

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestMemoryDigestService_GetOrRebuild_BackendError(t *testing.T) {
	backend := &MockMemoryBackend{}
	cache := &MockMemoryDigestCache{}
	aiProvider := &MockAIProvider{}

	cache.On("Get", "user1").Return(nil, nil)
	backend.On("GetUserGraph", "user1").Return(Graph{}, assert.AnError)

	service := NewMemoryDigestService(backend, cache, aiProvider, time.Hour)

	_, err := service.GetOrRebuild(context.Background(), "user1")

	assert.Error(t, err)
}

type MockMemoryBackend struct {
	mock.Mock
}

func (m *MockMemoryBackend) AddKnowledge(userID string, content string, embedding []float64) error {
	args := m.Called(userID, content, embedding)
	return args.Error(0)
}

func (m *MockMemoryBackend) SearchSimilar(query string, limit int) ([]Knowledge, error) {
	args := m.Called(query, limit)
	return args.Get(0).([]Knowledge), args.Error(1)
}

func (m *MockMemoryBackend) GetUserGraph(userID string) (Graph, error) {
	args := m.Called(userID)
	return args.Get(0).(Graph), args.Error(1)
}

func (m *MockMemoryBackend) AddRelation(from, to string, relType string) error {
	args := m.Called(from, to, relType)
	return args.Error(0)
}

func (m *MockMemoryBackend) QueryGraph(cypher string) (Result, error) {
	args := m.Called(cypher)
	return args.Get(0).(Result), args.Error(1)
}

func (m *MockMemoryBackend) InvalidateMemoryCache(userID string) error {
	args := m.Called(userID)
	return args.Error(0)
}

type MockMemoryDigestCache struct {
	mock.Mock
}

func (m *MockMemoryDigestCache) Get(userID string) (*MemoryDigest, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MemoryDigest), args.Error(1)
}

func (m *MockMemoryDigestCache) Set(digest *MemoryDigest) error {
	args := m.Called(digest)
	return args.Error(0)
}

func (m *MockMemoryDigestCache) Invalidate(userID string) error {
	args := m.Called(userID)
	return args.Error(0)
}

type MockAIProvider struct {
	mock.Mock
}

func (m *MockAIProvider) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(ChatResponse), args.Error(1)
}
