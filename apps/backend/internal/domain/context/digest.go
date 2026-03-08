package context

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/domain/ports"
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

type inMemoryDigestCache struct {
	digests map[string]*MemoryDigest
	mu      sync.RWMutex
	ttl     time.Duration
}

func NewInMemoryDigestCache(ttl time.Duration) MemoryDigestCache {
	return &inMemoryDigestCache{
		digests: make(map[string]*MemoryDigest),
		ttl:     ttl,
	}
}

func (c *inMemoryDigestCache) Get(userID string) (*MemoryDigest, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	digest, ok := c.digests[userID]
	if !ok {
		return nil, nil
	}

	if time.Now().After(digest.ExpiresAt) {
		return nil, nil
	}

	if digest.Stale {
		return nil, nil
	}

	return digest, nil
}

func (c *inMemoryDigestCache) Set(digest *MemoryDigest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	digest.ExpiresAt = time.Now().Add(c.ttl)
	digest.Stale = false
	digest.CreatedAt = time.Now()
	c.digests[digest.UserID] = digest

	return nil
}

func (c *inMemoryDigestCache) Invalidate(userID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if digest, ok := c.digests[userID]; ok {
		digest.Stale = true
	}

	return nil
}

type MemoryDigestService struct {
	backend    ports.MemoryPort
	cache      MemoryDigestCache
	aiProvider ports.AIProviderPort
	ttl        time.Duration
}

func NewMemoryDigestService(
	backend ports.MemoryPort,
	cache MemoryDigestCache,
	aiProvider ports.AIProviderPort,
	ttl time.Duration,
) *MemoryDigestService {
	if ttl == 0 {
		ttl = 1 * time.Hour
	}
	return &MemoryDigestService{
		backend:    backend,
		cache:      cache,
		aiProvider: aiProvider,
		ttl:        ttl,
	}
}

func (s *MemoryDigestService) GetOrRebuild(ctx context.Context, userID string) (string, error) {
	digest, err := s.cache.Get(userID)
	if err == nil && digest != nil {
		return digest.Content, nil
	}

	if s.backend == nil {
		return "", nil
	}

	graph, err := s.backend.GetUserGraph(ctx, userID)
	if err != nil {
		return "", err
	}

	if len(graph.Nodes) == 0 {
		return "", nil
	}

	var summary string
	if s.aiProvider != nil {
		summary = s.summarizeGraph(ctx, &graph)
	} else {
		summary = formatGraphAsText(&graph)
	}

	newDigest := &MemoryDigest{
		UserID:  userID,
		Content: summary,
	}
	_ = s.cache.Set(newDigest)

	return summary, nil
}

func (s *MemoryDigestService) Invalidate(userID string) error {
	return s.cache.Invalidate(userID)
}

func (s *MemoryDigestService) summarizeGraph(ctx context.Context, graph *ports.Graph) string {
	if len(graph.Nodes) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("User knowledge:\n")

	nodeMap := make(map[string]ports.GraphNode)
	for _, node := range graph.Nodes {
		nodeMap[node.ID] = node
	}

	for _, edge := range graph.Edges {
		source, ok := nodeMap[edge.Source]
		if !ok {
			continue
		}
		target, ok := nodeMap[edge.Target]
		if !ok {
			continue
		}

		if source.Type == "user" && target.Type == "fact" {
			b.WriteString("- ")
			b.WriteString(target.Value)
			b.WriteString("\n")
		}
	}

	return b.String()
}
