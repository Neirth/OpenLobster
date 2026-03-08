package memory

import (
	"context"
	"strings"
	"time"
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
	CreatedAt interface{}
}

type Result struct {
	Data   []map[string]interface{}
	Errors []error
}

type AIProvider interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

type ChatRequest struct {
	Model    string
	Messages []ChatMessage
	Tools    []interface{}
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
	var sb strings.Builder
	sb.WriteString("User knowledge graph:\n")

	// Group nodes by type for clarity.
	byType := make(map[string][]Node)
	for _, node := range graph.Nodes {
		byType[node.Type] = append(byType[node.Type], node)
	}

	// Facts first.
	for _, node := range byType["fact"] {
		sb.WriteString("- ")
		sb.WriteString(node.Value)
		sb.WriteString("\n")
	}
	// Then all other node types.
	for nodeType, nodes := range byType {
		if nodeType == "fact" {
			continue
		}
		for _, node := range nodes {
			sb.WriteString("- [")
			sb.WriteString(nodeType)
			sb.WriteString("] ")
			if node.Label != "" {
				sb.WriteString(node.Label)
				sb.WriteString(": ")
			}
			sb.WriteString(node.Value)
			sb.WriteString("\n")
		}
	}

	// Include edges as relationship statements.
	if len(graph.Edges) > 0 {
		sb.WriteString("Relations:\n")
		for _, edge := range graph.Edges {
			sb.WriteString("- ")
			sb.WriteString(edge.Source)
			sb.WriteString(" --[")
			sb.WriteString(edge.Label)
			sb.WriteString("]--> ")
			sb.WriteString(edge.Target)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (s *MemoryDigestService) Invalidate(userID string) error {
	return s.Cache.Invalidate(userID)
}
