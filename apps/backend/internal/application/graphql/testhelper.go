package graphql

import (
	"context"
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
	GetUserGraphFunc func(ctx context.Context, userID string) (ports.Graph, error)
	QueryGraphFunc   func(ctx context.Context, cypher string) (ports.GraphResult, error)
	AddRelationFunc  func(ctx context.Context, from, to, relType string) error
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

// TestMemoryRepo implements ports.MemoryPort for tests.
type TestMemoryRepo struct {
	store map[string][]string // userID -> contents for SearchSimilar
}

func (t *TestMemoryRepo) AddKnowledge(ctx context.Context, userID, content, label, relation string, embedding []float64) error {
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
