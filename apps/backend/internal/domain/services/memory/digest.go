package memory

import (
	"context"
	"time"

	toon "github.com/toon-format/toon-go"
)

type MemoryDigest struct {
	UserID    string
	Content   string
	CreatedAt time.Time
	ExpiresAt time.Time
	Stale     bool
}

type MemoryDigestCache interface {
	Get(userID string) (*MemoryDigest, error)
	Set(digest *MemoryDigest) error
	Invalidate(userID string) error
}

type Graph struct {
	Nodes []Node
	Edges []Edge
}

type Node struct {
	ID    string
	Label string
	Type  string
	Value string
}

type Edge struct {
	Source string
	Target string
	Label  string
}

type MemoryBackend interface {
	AddKnowledge(userID string, content string, embedding []float64) error
	SearchSimilar(query string, limit int) ([]Knowledge, error)
	GetUserGraph(userID string) (Graph, error)
	AddRelation(from, to string, relType string) error
	QueryGraph(cypher string) (Result, error)
	InvalidateMemoryCache(userID string) error
}

type Knowledge struct {
	ID        string
	UserID    string
	Content   string
	Embedding []float64
	CreatedAt any
}

type Result struct {
	Data   []map[string]any
	Errors []error
}

type AIProvider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

type ChatRequest struct {
	Model    string
	Messages []ChatMessage
	Tools    []any
}

type ChatMessage struct {
	Role    string
	Content string
}

type ChatResponse struct {
	Content    string
	ToolCalls  string
	StopReason string
	Audio      []byte
}

type MemoryDigestService struct {
	Backend    MemoryBackend
	Cache      MemoryDigestCache
	AIProvider AIProvider
	TTL        time.Duration
}

func NewMemoryDigestService(backend MemoryBackend, cache MemoryDigestCache, aiProvider AIProvider, ttl time.Duration) *MemoryDigestService {
	return &MemoryDigestService{
		Backend:    backend,
		Cache:      cache,
		AIProvider: aiProvider,
		TTL:        ttl,
	}
}

func (s *MemoryDigestService) GetOrRebuild(ctx context.Context, userID string) (string, error) {
	digest, err := s.Cache.Get(userID)
	if err == nil && digest != nil && !digest.Stale && time.Now().Before(digest.ExpiresAt) {
		return digest.Content, nil
	}

	graph, err := s.Backend.GetUserGraph(userID)
	if err != nil {
		return "", err
	}

	if len(graph.Nodes) == 0 {
		return "", nil
	}

	summary := s.summarizeGraph(graph)

	newDigest := &MemoryDigest{
		UserID:    userID,
		Content:   summary,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.TTL),
		Stale:     false,
	}
	_ = s.Cache.Set(newDigest)

	return summary, nil
}

func (s *MemoryDigestService) summarizeGraph(graph Graph) string {
	type toonNode struct {
		ID    string `toon:"id"`
		Label string `toon:"label"`
		Type  string `toon:"type"`
		Value string `toon:"value"`
	}
	type toonEdge struct {
		Source string `toon:"source"`
		Target string `toon:"target"`
		Label  string `toon:"label"`
	}
	type toonGraph struct {
		Nodes []toonNode `toon:"nodes"`
		Edges []toonEdge `toon:"edges"`
	}

	nodes := make([]toonNode, len(graph.Nodes))
	for i, n := range graph.Nodes {
		nodes[i] = toonNode(n)
	}
	edges := make([]toonEdge, len(graph.Edges))
	for i, e := range graph.Edges {
		edges[i] = toonEdge(e)
	}
	out, err := toon.MarshalString(toonGraph{Nodes: nodes, Edges: edges})
	if err != nil {
		return ""
	}
	return out
}

func (s *MemoryDigestService) Invalidate(userID string) error {
	return s.Cache.Invalidate(userID)
}
