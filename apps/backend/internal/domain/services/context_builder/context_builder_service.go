package context_builder

import (
	"context"
	"strings"

	toon "github.com/toon-format/toon-go"

	"github.com/neirth/openlobster/internal/domain/ports"
)

// Service builds agent context from workspace files and memory.
type Service struct {
	agentsMD     string
	soulMD       string
	identityMD   string
	mcpServers   []string
	tools        []string
	memoryPort   ports.MemoryPort
	memoryDigest *MemoryDigestService
}

// NewService creates a ContextBuilderService.
func NewService(
	agentsMD string,
	soulMD string,
	identityMD string,
	mcpServers []string,
	tools []string,
	memoryPort ports.MemoryPort,
	memoryDigest *MemoryDigestService,
) *Service {
	return &Service{
		agentsMD:     agentsMD,
		soulMD:       soulMD,
		identityMD:   identityMD,
		mcpServers:   mcpServers,
		tools:        tools,
		memoryPort:   memoryPort,
		memoryDigest: memoryDigest,
	}
}

// Build builds the context string for a user.
func (s *Service) Build(ctx context.Context, userID string) (string, error) {
	var sb strings.Builder

	if s.agentsMD != "" {
		sb.WriteString(s.agentsMD)
		sb.WriteString("\n\n")
	}

	if s.soulMD != "" {
		sb.WriteString(s.soulMD)
		sb.WriteString("\n\n")
	}

	if s.identityMD != "" {
		sb.WriteString(s.identityMD)
		sb.WriteString("\n\n")
	}

	if len(s.mcpServers) > 0 {
		sb.WriteString("Available MCPs:\n")
		for _, mcp := range s.mcpServers {
			sb.WriteString("- ")
			sb.WriteString(mcp)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(s.tools) > 0 {
		sb.WriteString("Available tools:\n")
		for _, tool := range s.tools {
			sb.WriteString("- ")
			sb.WriteString(tool)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if s.memoryDigest != nil && userID != "" {
		memoryContent, err := s.memoryDigest.GetOrRebuild(ctx, userID)
		if err == nil && memoryContent != "" {
			sb.WriteString("User memory:\n")
			sb.WriteString(memoryContent)
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// BuildForGroup builds context for multiple users.
func (s *Service) BuildForGroup(ctx context.Context, userIDs []string) (string, error) {
	var contexts []string

	for _, userID := range userIDs {
		ctxStr, err := s.Build(ctx, userID)
		if err != nil {
			continue
		}
		contexts = append(contexts, ctxStr)
	}

	return strings.Join(contexts, "\n---\n"), nil
}

// MemoryDigestService builds digests of user memory graphs.
type MemoryDigestService struct {
	backend ports.MemoryPort
	cache   MemoryDigestCache
	ttl     int
}

// MemoryDigestCache is the cache interface for memory digests.
type MemoryDigestCache interface {
	Get(userID string) (string, bool)
	Set(userID, content string)
	Invalidate(userID string)
}

// NewMemoryDigestService creates a MemoryDigestService.
func NewMemoryDigestService(backend ports.MemoryPort, cache MemoryDigestCache, ttl int) *MemoryDigestService {
	return &MemoryDigestService{
		backend: backend,
		cache:   cache,
		ttl:     ttl,
	}
}

// GetOrRebuild gets cached digest or rebuilds from graph.
func (s *MemoryDigestService) GetOrRebuild(ctx context.Context, userID string) (string, error) {
	if content, ok := s.cache.Get(userID); ok {
		return content, nil
	}

	graph, err := s.backend.GetUserGraph(ctx, userID)
	if err != nil {
		return "", err
	}

	if len(graph.Nodes) == 0 {
		return "", nil
	}

	summary := s.summarizeGraph(graph)
	s.cache.Set(userID, summary)

	return summary, nil
}

func (s *MemoryDigestService) summarizeGraph(graph ports.Graph) string {
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

	// Only include the subject user's own node and nodes with direct knowledge
	// relationships (facts, persons, places, etc.). Other User nodes that appear
	// due to User-User relations (e.g. FRIEND_OF) represent people the user
	// knows, not personal facts — injecting them pollutes the AI context with
	// data that belongs to other users.
	includedIDs := make(map[string]bool)
	filteredNodes := make([]toonNode, 0, len(graph.Nodes))
	for _, n := range graph.Nodes {
		// Keep the subject user node (synthetic "user:<id>" prefix) and all
		// non-user entity nodes. Exclude other User nodes.
		if strings.HasPrefix(n.ID, "user:") || n.Type != "user" {
			filteredNodes = append(filteredNodes, toonNode{ID: n.ID, Label: n.Label, Type: n.Type, Value: n.Value})
			includedIDs[n.ID] = true
		}
	}

	filteredEdges := make([]toonEdge, 0, len(graph.Edges))
	for _, e := range graph.Edges {
		if includedIDs[e.Source] && includedIDs[e.Target] {
			filteredEdges = append(filteredEdges, toonEdge{Source: e.Source, Target: e.Target, Label: e.Label})
		}
	}

	out, err := toon.MarshalString(toonGraph{Nodes: filteredNodes, Edges: filteredEdges})
	if err != nil {
		return ""
	}
	return out
}

// Invalidate invalidates the cache for a user.
func (s *MemoryDigestService) Invalidate(userID string) {
	s.cache.Invalidate(userID)
}
