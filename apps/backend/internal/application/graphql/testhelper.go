package graphql

import (
	"context"
	"fmt"
	"strings"

	"github.com/neirth/openlobster/internal/application/graphql/dto"
	"github.com/neirth/openlobster/internal/application/graphql/resolvers"
	"github.com/neirth/openlobster/internal/application/registry"
	"github.com/neirth/openlobster/internal/domain/ports"
	svcdashboard "github.com/neirth/openlobster/internal/domain/services/dashboard"
)

// NewTestDeps builds minimal Deps for tests. Pass nil for optional services to use defaults.
func NewTestDeps(opts TestDepsOpts) *resolvers.Deps {
	reg := registry.NewAgentRegistry()
	if opts.Agent != nil {
		reg.UpdateAgent(opts.Agent)
	}
	if len(opts.Channels) > 0 {
		reg.UpdateAgentChannels(opts.Channels)
	}

	deps := &resolvers.Deps{AgentRegistry: reg}

	if opts.QuerySvc != nil {
		deps.QuerySvc = opts.QuerySvc
	}
	if opts.CommandSvc != nil {
		deps.CommandSvc = opts.CommandSvc
	}
	if opts.TaskRepo != nil {
		deps.TaskRepo = opts.TaskRepo
	}
	if opts.MemoryRepo != nil {
		deps.MemoryRepo = opts.MemoryRepo
	}

	return deps
}

// TestDepsOpts configures NewTestDeps.
type TestDepsOpts struct {
	Agent      *dto.AgentSnapshot
	Channels   []dto.ChannelStatus
	QuerySvc   *svcdashboard.QueryService
	CommandSvc *svcdashboard.CommandService
	TaskRepo   ports.TaskRepositoryPort
	MemoryRepo ports.MemoryPort
}

// TestGraphRepo implements svcdashboard.GraphQueryPort and GraphCommandPort for tests.
type TestGraphRepo struct {
	GetUserGraphFunc    func(ctx context.Context, userID string) (ports.Graph, error)
	QueryGraphFunc      func(ctx context.Context, cypher string) (ports.GraphResult, error)
	AddRelationFunc     func(ctx context.Context, from, to, relType string) error
	DeleteRelationFunc  func(ctx context.Context, from, to string) error
}

func (t *TestGraphRepo) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	if t.GetUserGraphFunc != nil {
		return t.GetUserGraphFunc(ctx, userID)
	}
	return ports.Graph{Nodes: []ports.GraphNode{}, Edges: []ports.GraphEdge{}}, nil
}

func (t *TestGraphRepo) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	if t.QueryGraphFunc != nil {
		return t.QueryGraphFunc(ctx, cypher)
	}
	return ports.GraphResult{Data: []map[string]interface{}{}}, nil
}

func (t *TestGraphRepo) AddRelation(ctx context.Context, from, to, relType string) error {
	if t.AddRelationFunc != nil {
		return t.AddRelationFunc(ctx, from, to, relType)
	}
	return nil
}

func (t *TestGraphRepo) DeleteRelation(ctx context.Context, from, to string) error {
	if t.DeleteRelationFunc != nil {
		return t.DeleteRelationFunc(ctx, from, to)
	}
	return nil
}

// TestMemoryRepo implements ports.MemoryPort for tests.
type TestMemoryRepo struct {
	store map[string][]string // userID -> contents for SearchSimilar
}

func (t *TestMemoryRepo) AddKnowledge(ctx context.Context, userID, content, label, relation, entityType string, embedding []float64) error {
	if t.store == nil {
		t.store = make(map[string][]string)
	}
	t.store[userID] = append(t.store[userID], content)
	return nil
}
func (t *TestMemoryRepo) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	if t.store == nil {
		return nil, nil
	}
	var results []ports.Knowledge
	q := strings.ToLower(query)
	for userID, contents := range t.store {
		for _, c := range contents {
			if strings.Contains(strings.ToLower(c), q) {
				results = append(results, ports.Knowledge{UserID: userID, Content: c})
				if len(results) >= limit {
					return results, nil
				}
			}
		}
	}
	return results, nil
}
func (t *TestMemoryRepo) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	return ports.Graph{}, nil
}
func (t *TestMemoryRepo) AddRelation(ctx context.Context, from, to, relType string) error {
	return nil
}
func (t *TestMemoryRepo) DeleteRelation(ctx context.Context, from, to string) error {
	return nil
}
func (t *TestMemoryRepo) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	return ports.GraphResult{}, nil
}
func (t *TestMemoryRepo) InvalidateMemoryCache(ctx context.Context, userID string) error {
	return nil
}
func (t *TestMemoryRepo) SetUserProperty(ctx context.Context, userID, key, value string) error {
	return nil
}
func (t *TestMemoryRepo) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	return nil
}
func (t *TestMemoryRepo) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	return nil
}
func (t *TestMemoryRepo) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	return nil
}

// TestMemoryRepoWithState implements ports.MemoryPort with actual state tracking.
// Unlike TestMemoryRepo, this verifies post-conditions for delete operations.
type TestMemoryRepoWithState struct {
	nodes     map[string]nodeRec
	relations map[string][]relationRec
	deleted   map[string]bool
}

func NewTestMemoryRepoWithState() *TestMemoryRepoWithState {
	return &TestMemoryRepoWithState{
		nodes:     make(map[string]nodeRec),
		relations: make(map[string][]relationRec),
		deleted:   make(map[string]bool),
	}
}

type nodeRec struct{ Label, Value, Content string }

func (t *TestMemoryRepoWithState) AddKnowledge(ctx context.Context, userID, content, label, relation, entityType string, embedding []float64) error {
	nodeID := label
	t.nodes[nodeID] = nodeRec{Label: label, Value: content, Content: content}
	return nil
}

func (t *TestMemoryRepoWithState) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	var results []ports.Knowledge
	q := strings.ToLower(query)
	for id, n := range t.nodes {
		if t.deleted[id] {
			continue
		}
		rec := n // nodeRec
		if strings.Contains(strings.ToLower(rec.Content), q) {
			results = append(results, ports.Knowledge{UserID: id, Content: rec.Content})
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (t *TestMemoryRepoWithState) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	var nodes []ports.GraphNode
	var edges []ports.GraphEdge
	for id, n := range t.nodes {
		if t.deleted[id] {
			continue
		}
		rec := n // nodeRec
		nodes = append(nodes, ports.GraphNode{ID: id, Label: rec.Label, Value: rec.Value})
	}
	for _, rels := range t.relations {
		for _, r := range rels {
			if t.deleted[r.From] || t.deleted[r.To] {
				continue
			}
			edges = append(edges, ports.GraphEdge{Source: r.From, Target: r.To, Label: r.Type})
		}
	}
	return ports.Graph{Nodes: nodes, Edges: edges}, nil
}

func (t *TestMemoryRepoWithState) AddRelation(ctx context.Context, from, to, relType string) error {
	if t.relations == nil {
		t.relations = make(map[string][]relationRec)
	}
	rec := relationRec{From: from, To: to, Type: relType}
	t.relations[from] = append(t.relations[from], rec)
	return nil
}

type relationRec struct{ From, To, Type string }

func (t *TestMemoryRepoWithState) DeleteRelation(ctx context.Context, from, to string) error {
	rels, ok := t.relations[from]
	if !ok {
		return nil
	}
	filtered := rels[:0]
	for _, r := range rels {
		if r.To != to {
			filtered = append(filtered, r)
		}
	}
	t.relations[from] = filtered
	return nil
}

func (t *TestMemoryRepoWithState) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	return ports.GraphResult{}, nil
}

func (t *TestMemoryRepoWithState) InvalidateMemoryCache(ctx context.Context, userID string) error {
	return nil
}

func (t *TestMemoryRepoWithState) SetUserProperty(ctx context.Context, userID, key, value string) error {
	return nil
}

func (t *TestMemoryRepoWithState) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	if n, ok := t.nodes[nodeID]; ok {
		t.nodes[nodeID] = struct{ Label, Value, Content string }{Label: n.Label, Value: newValue, Content: newValue}
	}
	return nil
}

func (t *TestMemoryRepoWithState) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	if _, exists := t.nodes[nodeID]; !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	t.deleted[nodeID] = true
	return nil
}

func (t *TestMemoryRepoWithState) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	return nil
}

// AddNode adds a test node directly (for test setup)
func (t *TestMemoryRepoWithState) AddNode(id, label, value, content string) {
	t.nodes[id] = struct{ Label, Value, Content string }{Label: label, Value: value, Content: content}
}

// IsDeleted returns true if node was deleted
func (t *TestMemoryRepoWithState) IsDeleted(nodeID string) bool {
	return t.deleted[nodeID]
}

// HasNode returns true if node exists (including deleted)
func (t *TestMemoryRepoWithState) HasNode(nodeID string) bool {
	_, exists := t.nodes[nodeID]
	return exists
}
