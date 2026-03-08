// Copyright (c) OpenLobster contributors. See LICENSE for details.

package dashboard

import (
	"context"
	"strings"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/services/mcp"
	"github.com/neirth/openlobster/internal/domain/ports"
)

// QueryService aggregates data for the GraphQL dashboard.
type QueryService struct {
	taskRepo   ports.TaskRepositoryPort
	memoryRepo ports.MemoryPort
	graphRepo  GraphQueryPort
	mcpTools   []mcp.ToolDefinition
	mcps       []mcp.ServerConfig
}

// GraphQueryPort is the graph query interface for the dashboard.
type GraphQueryPort interface {
	GetUserGraph(ctx context.Context, userID string) (ports.Graph, error)
	QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error)
}

// NewQueryService creates a DashboardQueryService.
func NewQueryService(
	taskRepo ports.TaskRepositoryPort,
	memoryRepo ports.MemoryPort,
	graphRepo GraphQueryPort,
	mcpTools []mcp.ToolDefinition,
	mcps []mcp.ServerConfig,
) *QueryService {
	return &QueryService{
		taskRepo:   taskRepo,
		memoryRepo: memoryRepo,
		graphRepo:  graphRepo,
		mcpTools:   mcpTools,
		mcps:       mcps,
	}
}

// GetTasks returns all tasks.
func (s *QueryService) GetTasks(ctx context.Context) ([]models.Task, error) {
	if s.taskRepo != nil {
		return s.taskRepo.ListAll(ctx)
	}
	return []models.Task{}, nil
}

// SearchMemory searches memory for the given query.
func (s *QueryService) SearchMemory(ctx context.Context, userID, query string) (string, error) {
	if s.memoryRepo == nil {
		return "", nil
	}
	results, err := s.memoryRepo.SearchSimilar(ctx, query, 10)
	if err != nil {
		return "", err
	}
	var content []string
	for _, r := range results {
		content = append(content, r.Content)
	}
	return strings.Join(content, "\n"), nil
}

// GetUserGraph returns the memory graph for a user.
func (s *QueryService) GetUserGraph(ctx context.Context, userID string) (PortsGraph, error) {
	if s.graphRepo == nil {
		return PortsGraph{}, nil
	}
	graph, err := s.graphRepo.GetUserGraph(ctx, userID)
	if err != nil {
		return PortsGraph{}, err
	}
	return PortsGraph{Nodes: graph.Nodes, Edges: graph.Edges}, nil
}

// ExecuteCypher executes a Cypher query on the graph.
func (s *QueryService) ExecuteCypher(ctx context.Context, cypher string) (PortsGraphResult, error) {
	if s.graphRepo == nil {
		return PortsGraphResult{}, nil
	}
	result, err := s.graphRepo.QueryGraph(ctx, cypher)
	if err != nil {
		return PortsGraphResult{}, err
	}
	return PortsGraphResult{Data: result.Data}, nil
}

// PortsGraph wraps ports.Graph for dashboard use.
type PortsGraph struct {
	Nodes []ports.GraphNode
	Edges []ports.GraphEdge
}

// PortsGraphResult wraps graph query results.
type PortsGraphResult struct {
	Data []map[string]interface{}
}

// CommandService handles dashboard mutations.
type CommandService struct {
	taskRepo     ports.TaskRepositoryPort
	memoryRepo   ports.MemoryPort
	graphRepo    GraphCommandPort
	taskNotifier func()
}

// GraphCommandPort is the graph command interface.
type GraphCommandPort interface {
	AddRelation(ctx context.Context, from, to string, relType string) error
}

// NodeMutatorPort is optionally implemented by memory backends.
type NodeMutatorPort interface {
	UpdateNode(ctx context.Context, id, label, typ, value string, properties map[string]string) error
	DeleteNode(ctx context.Context, id string) error
}

// NewCommandService creates a DashboardCommandService.
func NewCommandService(
	taskRepo ports.TaskRepositoryPort,
	memoryRepo ports.MemoryPort,
	graphRepo GraphCommandPort,
) *CommandService {
	return &CommandService{
		taskRepo:   taskRepo,
		memoryRepo: memoryRepo,
		graphRepo:  graphRepo,
	}
}

// SetTaskNotifier registers a callback invoked after a task is successfully added.
// Use this to wake the scheduler immediately so the new task fires on time.
func (s *CommandService) SetTaskNotifier(notify func()) {
	s.taskNotifier = notify
}

// AddTask adds a new task.
func (s *CommandService) AddTask(ctx context.Context, prompt, schedule string) (string, error) {
	if s.taskRepo == nil {
		return "", nil
	}
	task := models.NewTask(prompt, schedule)
	if err := s.taskRepo.Add(ctx, task); err != nil {
		return "", err
	}
	if s.taskNotifier != nil {
		s.taskNotifier()
	}
	return task.ID, nil
}

// CompleteTask marks a task as done.
func (s *CommandService) CompleteTask(ctx context.Context, taskID string) error {
	if s.taskRepo == nil {
		return nil
	}
	return s.taskRepo.MarkDone(ctx, taskID)
}

// DeleteTask removes a task.
func (s *CommandService) DeleteTask(ctx context.Context, taskID string) error {
	if s.taskRepo == nil {
		return nil
	}
	return s.taskRepo.Delete(ctx, taskID)
}

// UpdateTask updates a task.
func (s *CommandService) UpdateTask(ctx context.Context, id, prompt, schedule string) error {
	if s.taskRepo == nil {
		return nil
	}
	task := &models.Task{
		ID:       id,
		Prompt:   prompt,
		Schedule: schedule,
		TaskType: models.ComputeTaskType(schedule),
	}
	return s.taskRepo.Update(ctx, task)
}

// ToggleTask enables or disables a task.
func (s *CommandService) ToggleTask(ctx context.Context, id string, enabled bool) error {
	if s.taskRepo == nil {
		return nil
	}
	return s.taskRepo.SetEnabled(ctx, id, enabled)
}

// AddMemory adds knowledge to a user's memory.
func (s *CommandService) AddMemory(ctx context.Context, userID, content string) error {
	if s.memoryRepo == nil {
		return nil
	}
	return s.memoryRepo.AddKnowledge(ctx, userID, content, "", "", nil)
}

// AddRelation adds a relation in the graph.
func (s *CommandService) AddRelation(ctx context.Context, from, to, relType string) error {
	if s.graphRepo == nil {
		return nil
	}
	return s.graphRepo.AddRelation(ctx, from, to, relType)
}

// UpdateNode updates a memory node.
func (s *CommandService) UpdateNode(ctx context.Context, id, label, typ, value string, properties map[string]string) error {
	if m, ok := s.memoryRepo.(NodeMutatorPort); ok {
		return m.UpdateNode(ctx, id, label, typ, value, properties)
	}
	return nil
}

// DeleteNode removes a memory node.
func (s *CommandService) DeleteNode(ctx context.Context, id string) error {
	if m, ok := s.memoryRepo.(NodeMutatorPort); ok {
		return m.DeleteNode(ctx, id)
	}
	return nil
}
