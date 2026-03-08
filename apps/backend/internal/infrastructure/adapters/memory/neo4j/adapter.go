package neo4j

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/neirth/openlobster/internal/domain/ports"
	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Adapter struct {
	driver neo4j.DriverWithContext
	mu     sync.RWMutex
	config Config
}

type Config struct {
	URI      string
	Username string
	Password string
}

func NewAdapter(config Config) (*Adapter, error) {
	driver, err := neo4j.NewDriverWithContext(config.URI, neo4j.BasicAuth(config.Username, config.Password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("neo4j not reachable: %w", err)
	}

	return &Adapter{
		driver: driver,
		config: config,
	}, nil
}

func (a *Adapter) AddKnowledge(ctx context.Context, userID string, content string, label string, relation string, embedding []float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())

	if label == "" {
		label = uuid.New().String()
	}
	rel := relation
	if rel == "" {
		rel = "HAS_FACT"
	}

	// Deduplication: if a Fact node with the same label already exists for
	// this user (case-insensitive), update its content in-place.
	updateResult, err := session.Run(ctx,
		"MATCH (u:User {id: $userID})-[]->(f:Fact) WHERE toLower(f.label) = toLower($label) SET f.content = $content RETURN f.id AS id LIMIT 1",
		map[string]interface{}{"userID": userID, "label": label, "content": content},
	)
	if err == nil {
		if rec, recErr := updateResult.Single(ctx); recErr == nil && rec != nil {
			return nil
		}
	}

	// No existing fact found — create user node (if needed), a new Fact node
	// and the relationship.
	factID := uuid.New().String()
	createResult, err := session.Run(ctx,
		"MERGE (u:User {id: $userID}) "+
			"CREATE (f:Fact {id: $factID, label: $label, content: $content, createdAt: timestamp()}) "+
			"CREATE (u)-[r:"+rel+"]->(f)",
		map[string]interface{}{"userID": userID, "factID": factID, "label": label, "content": content},
	)
	if err != nil {
		return err
	}
	createResult.Consume(ctx)
	return nil
}

func (a *Adapter) UpdateUserLabel(ctx context.Context, userID, displayName string) error {
	if displayName == "" {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())

	result, err := session.Run(ctx,
		"MERGE (u:User {id: $userID}) SET u.displayName = $displayName",
		map[string]interface{}{"userID": userID, "displayName": displayName},
	)
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
}

func (a *Adapter) SearchSimilar(ctx context.Context, query string, limit int) ([]ports.Knowledge, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session := a.driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(context.Background())

	if limit <= 0 {
		limit = 10
	}

	result, err := session.Run(ctx,
		"MATCH (f:Fact) WHERE f.content CONTAINS $query RETURN f.id AS id, f.content AS content LIMIT $limit",
		map[string]interface{}{"query": query, "limit": limit},
	)
	if err != nil {
		return nil, err
	}

	knowledge := make([]ports.Knowledge, 0)
	for result.Next(ctx) {
		record := result.Record()
		if id, ok := record.Get("id"); ok {
			content, _ := record.Get("content")
			knowledge = append(knowledge, ports.Knowledge{
				ID:      id.(string),
				Content: content.(string),
			})
		}
	}

	return knowledge, result.Err()
}

func (a *Adapter) GetUserGraph(ctx context.Context, userID string) (ports.Graph, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session := a.driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(context.Background())

	// userID vacío o "*" = grafo completo (para el dashboard que muestra todas las memorias).
	if userID == "" || userID == "*" {
		return a.getFullGraph(ctx, session)
	}

	result, err := session.Run(ctx,
		"MATCH (u:User {id: $userID})-[r]-(n) RETURN labels(n) AS labels, n.id AS id, n.content AS content, n.name AS name, type(r) AS relType",
		map[string]interface{}{"userID": userID},
	)
	if err != nil {
		return ports.Graph{}, err
	}

	nodes := make(map[string]ports.GraphNode)
	edges := make([]ports.GraphEdge, 0)

	userNodeID := fmt.Sprintf("user:%s", userID)
	nodes[userNodeID] = ports.GraphNode{ID: userNodeID, Label: "User", Type: "user", Value: userID}

	nodeCounter := 0
	for result.Next(ctx) {
		record := result.Record()
		labels, _ := record.Get("labels")
		id, _ := record.Get("id")
		relType, _ := record.Get("relType")

		if idVal, ok := id.(string); ok && idVal != "" {
			labelStr := "Node"
			if l, ok := labels.([]interface{}); ok && len(l) > 0 {
				if s, ok := l[0].(string); ok {
					labelStr = s
				}
			}
			nodeID := fmt.Sprintf("%s:%d", labelStr, nodeCounter)
			nodeCounter++

			var value string
			if c, ok := record.Get("content"); ok {
				value = fmt.Sprintf("%v", c)
			} else if n, ok := record.Get("name"); ok {
				value = fmt.Sprintf("%v", n)
			}

			nodes[idVal] = ports.GraphNode{ID: nodeID, Label: labelStr, Type: strings.ToLower(labelStr), Value: value}
			edges = append(edges, ports.GraphEdge{Source: userNodeID, Target: nodeID, Label: fmt.Sprintf("%v", relType)})
		}
	}

	graphNodes := make([]ports.GraphNode, 0, len(nodes))
	for _, n := range nodes {
		graphNodes = append(graphNodes, n)
	}

	return ports.Graph{Nodes: graphNodes, Edges: edges}, result.Err()
}

// getFullGraph returns all User and Fact nodes with their relationships (dashboard full memory view).
func (a *Adapter) getFullGraph(ctx context.Context, session neo4j.SessionWithContext) (ports.Graph, error) {
	result, err := session.Run(ctx,
		"MATCH (u:User)-[r]->(n) RETURN u.id AS userId, labels(n) AS labels, n.id AS id, n.content AS content, n.name AS name, type(r) AS relType",
		nil,
	)
	if err != nil {
		return ports.Graph{}, err
	}

	nodes := make(map[string]ports.GraphNode)
	edges := make([]ports.GraphEdge, 0)
	nodeCounter := 0

	for result.Next(ctx) {
		record := result.Record()
		userId, _ := record.Get("userId")
		labels, _ := record.Get("labels")
		id, _ := record.Get("id")
		relType, _ := record.Get("relType")

		userIDStr := ""
		if uid, ok := userId.(string); ok {
			userIDStr = uid
		}
		userNodeID := fmt.Sprintf("user:%s", userIDStr)
		if _, exists := nodes[userNodeID]; !exists {
			nodes[userNodeID] = ports.GraphNode{ID: userNodeID, Label: "User", Type: "user", Value: userIDStr}
		}

		if idVal, ok := id.(string); ok && idVal != "" {
			labelStr := "Node"
			if l, ok := labels.([]interface{}); ok && len(l) > 0 {
				if s, ok := l[0].(string); ok {
					labelStr = s
				}
			}
			nodeID := fmt.Sprintf("%s:%s:%d", labelStr, idVal, nodeCounter)
			nodeCounter++

			var value string
			if c, ok := record.Get("content"); ok {
				value = fmt.Sprintf("%v", c)
			} else if n, ok := record.Get("name"); ok {
				value = fmt.Sprintf("%v", n)
			}

			nodes[nodeID] = ports.GraphNode{ID: nodeID, Label: labelStr, Type: strings.ToLower(labelStr), Value: value}
			edges = append(edges, ports.GraphEdge{Source: userNodeID, Target: nodeID, Label: fmt.Sprintf("%v", relType)})
		}
	}

	graphNodes := make([]ports.GraphNode, 0, len(nodes))
	for _, n := range nodes {
		graphNodes = append(graphNodes, n)
	}

	return ports.Graph{Nodes: graphNodes, Edges: edges}, result.Err()
}

func (a *Adapter) AddRelation(ctx context.Context, from, to string, relType string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())

	result, err := session.Run(ctx,
		"MERGE (a:Entity {id: $from}) MERGE (b:Entity {id: $to}) CREATE (a)-[r:RELATES {type: $relType}]->(b) RETURN r",
		map[string]interface{}{"from": from, "to": to, "relType": relType},
	)
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
}

func (a *Adapter) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	session := a.driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(context.Background())

	result, err := session.Run(ctx, cypher, nil)
	if err != nil {
		return ports.GraphResult{Errors: []error{err}}, nil
	}

	data := make([]map[string]interface{}, 0)
	for result.Next(ctx) {
		record := result.Record()
		row := make(map[string]interface{})
		for i, key := range record.Keys {
			row[key] = record.Values[i]
		}
		data = append(data, row)
	}

	return ports.GraphResult{Data: data}, result.Err()
}

func (a *Adapter) InvalidateMemoryCache(ctx context.Context, userID string) error {
	return nil
}

// SetUserProperty upserts a key/value property on the User node for the given userID.
// Uses MERGE to find or create the user node, then SET the property dynamically.
func (a *Adapter) SetUserProperty(ctx context.Context, userID, key, value string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	cypher := fmt.Sprintf(`MERGE (u:User {id: $userID}) SET u.%s = $value`, key)
	_, err := session.Run(ctx, cypher, map[string]interface{}{
		"userID": userID,
		"value":  value,
	})
	return err
}

// EditMemoryNode updates the content of a Fact node owned by userID.
func (a *Adapter) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (u:User {id: $userID})-[]->(f:Fact {id: $nodeID}) SET f.content = $newValue RETURN f",
		map[string]interface{}{"userID": userID, "nodeID": nodeID, "newValue": newValue},
	)
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
}

// DeleteMemoryNode removes a Fact node owned by userID.
func (a *Adapter) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (u:User {id: $userID})-[]->(f:Fact {id: $nodeID}) DETACH DELETE f",
		map[string]interface{}{"userID": userID, "nodeID": nodeID},
	)
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
}

func (a *Adapter) Close() error {
	return a.driver.Close(context.Background())
}

type Neo4jMemoryBackend struct {
	*Adapter
}

func NewNeo4jMemoryBackend(uri, username, password string) (*Neo4jMemoryBackend, error) {
	adapter, err := NewAdapter(Config{URI: uri, Username: username, Password: password})
	if err != nil {
		return nil, err
	}
	return &Neo4jMemoryBackend{Adapter: adapter}, nil
}

func (b *Neo4jMemoryBackend) GetUserFacts(ctx context.Context, userID string) ([]string, error) {
	session := b.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (u:User {id: $userID})-[:HAS_FACT]->(f:Fact) RETURN f.content AS content",
		map[string]interface{}{"userID": userID},
	)
	if err != nil {
		return nil, err
	}

	facts := make([]string, 0)
	for result.Next(ctx) {
		record := result.Record()
		if c, ok := record.Get("content"); ok {
			facts = append(facts, c.(string))
		}
	}
	return facts, result.Err()
}

func (b *Neo4jMemoryBackend) DeleteFact(ctx context.Context, factID string) error {
	session := b.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.Run(ctx, "MATCH (f:Fact {id: $factID}) DETACH DELETE f", map[string]interface{}{"factID": factID})
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
}

func (b *Neo4jMemoryBackend) GetRelatedEntities(ctx context.Context, entityID string, relationType string) ([]string, error) {
	session := b.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	cypher := fmt.Sprintf("MATCH (e:Entity {id: $entityId})-[r:%s]->(related:Entity) RETURN related.id AS id", strings.ToUpper(relationType))
	result, err := session.Run(ctx, cypher, map[string]interface{}{"entityId": entityID})
	if err != nil {
		return nil, err
	}

	entities := make([]string, 0)
	for result.Next(ctx) {
		record := result.Record()
		if id, ok := record.Get("id"); ok {
			entities = append(entities, id.(string))
		}
	}
	return entities, result.Err()
}

var _ ports.MemoryPort = (*Adapter)(nil)
