package neo4j

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/neirth/openlobster/internal/domain/ports"
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

	// Empty userID or "*" returns the full graph (for the dashboard that shows all memories).
	if userID == "" || userID == "*" {
		return a.getFullGraph(ctx, session)
	}

	// Resolve user display name for the node label (same as getFullGraph).
	userLabel := "User"
	userVal := userID
	if dispResult, dispErr := session.Run(ctx,
		"MATCH (u:User {id: $userID}) RETURN u.displayName AS displayName",
		map[string]interface{}{"userID": userID},
	); dispErr == nil {
		if rec, recErr := dispResult.Single(ctx); recErr == nil && rec != nil {
			if dn, ok := rec.Get("displayName"); ok && dn != nil {
				if s := fmt.Sprintf("%v", dn); s != "" {
					userLabel = s
					userVal = s
				}
			}
		}
	}

	result, err := session.Run(ctx,
		"MATCH (u:User {id: $userID})-[r]-(n) RETURN labels(n) AS labels, n.id AS id, n.content AS content, n.name AS name, n.label AS nodeLabel, type(r) AS relType",
		map[string]interface{}{"userID": userID},
	)
	if err != nil {
		return ports.Graph{}, err
	}

	nodes := make(map[string]ports.GraphNode)
	edges := make([]ports.GraphEdge, 0)

	userNodeID := fmt.Sprintf("user:%s", userID)
	nodes[userNodeID] = ports.GraphNode{ID: userNodeID, Label: userLabel, Type: "user", Value: userVal}

	for result.Next(ctx) {
		record := result.Record()
		labels, _ := record.Get("labels")
		id, _ := record.Get("id")
		relType, _ := record.Get("relType")

		if idVal, ok := id.(string); ok && idVal != "" {
			typeStr := "Node"
			if l, ok := labels.([]interface{}); ok && len(l) > 0 {
				if s, ok := l[0].(string); ok {
					typeStr = s
				}
			}
			// Display label: Fact's "label" property (e.g. "Real Name"); fallback to type.
			displayLabel := typeStr
			if nl, ok := record.Get("nodeLabel"); ok && nl != nil && fmt.Sprintf("%v", nl) != "" {
				displayLabel = fmt.Sprintf("%v", nl)
			}
			var value string
			if c, ok := record.Get("content"); ok && c != nil {
				value = fmt.Sprintf("%v", c)
			} else if n, ok := record.Get("name"); ok && n != nil {
				value = fmt.Sprintf("%v", n)
			}
			// Use real Neo4j node id so GUI delete/update work.
			nodes[idVal] = ports.GraphNode{ID: idVal, Label: displayLabel, Type: strings.ToLower(typeStr), Value: value}
			edges = append(edges, ports.GraphEdge{Source: userNodeID, Target: idVal, Label: fmt.Sprintf("%v", relType)})
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
		"MATCH (u:User)-[r]->(n) RETURN u.id AS userId, u.displayName AS userDisplayName, labels(n) AS labels, n.id AS id, n.content AS content, n.name AS name, n.label AS nodeLabel, n.displayName AS targetDisplayName, type(r) AS relType",
		nil,
	)
	if err != nil {
		return ports.Graph{}, err
	}

	nodes := make(map[string]ports.GraphNode)
	edges := make([]ports.GraphEdge, 0)

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
			userLabel := "User"
			userVal := userIDStr
			if dn, ok := record.Get("userDisplayName"); ok && dn != nil && fmt.Sprintf("%v", dn) != "" {
				userLabel = fmt.Sprintf("%v", dn)
				userVal = userLabel
			}
			nodes[userNodeID] = ports.GraphNode{ID: userNodeID, Label: userLabel, Type: "user", Value: userVal}
		}

		if idVal, ok := id.(string); ok && idVal != "" {
			typeStr := "Node"
			if l, ok := labels.([]interface{}); ok && len(l) > 0 {
				if s, ok := l[0].(string); ok {
					typeStr = s
				}
			}
			displayLabel := typeStr
			if nl, ok := record.Get("nodeLabel"); ok && nl != nil && fmt.Sprintf("%v", nl) != "" {
				displayLabel = fmt.Sprintf("%v", nl)
			}
			var value string
			if typeStr == "User" {
				if td, ok := record.Get("targetDisplayName"); ok && td != nil && fmt.Sprintf("%v", td) != "" {
					value = fmt.Sprintf("%v", td)
				} else {
					value = idVal
				}
			} else if c, ok := record.Get("content"); ok && c != nil {
				value = fmt.Sprintf("%v", c)
			} else if n, ok := record.Get("name"); ok && n != nil {
				value = fmt.Sprintf("%v", n)
			}
			// Use real Neo4j node id so GUI delete/update work.
			nodes[idVal] = ports.GraphNode{ID: idVal, Label: displayLabel, Type: strings.ToLower(typeStr), Value: value}
			edges = append(edges, ports.GraphEdge{Source: userNodeID, Target: idVal, Label: fmt.Sprintf("%v", relType)})
		}
	}

	graphNodes := make([]ports.GraphNode, 0, len(nodes))
	for _, n := range nodes {
		graphNodes = append(graphNodes, n)
	}

	return ports.Graph{Nodes: graphNodes, Edges: edges}, result.Err()
}

// sanitizeRelType returns a safe Cypher relationship type (uppercase alphanumeric + underscore).
func sanitizeRelType(relType string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(relType) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "RELATES"
	}
	return b.String()
}

// AddRelation creates a typed relationship between two User nodes (e.g. FRIEND_OF, KNOWS).
// Used by add_user_relation so that "user A is friend of user B" is stored as (User)-[:FRIEND_OF]->(User).
func (a *Adapter) AddRelation(ctx context.Context, from, to string, relType string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())

	rel := sanitizeRelType(relType)
	cypher := fmt.Sprintf("MERGE (a:User {id: $from}) MERGE (b:User {id: $to}) CREATE (a)-[r:%s]->(b) RETURN r", rel)
	result, err := session.Run(ctx, cypher, map[string]interface{}{"from": from, "to": to})
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

// EditMemoryNode updates the content (and optionally label) of a Fact node.
// When userID is empty, matches any User (dashboard flow).
func (a *Adapter) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	var cypher string
	params := map[string]interface{}{"nodeID": nodeID, "newValue": newValue}
	if userID != "" {
		cypher = "MATCH (u:User {id: $userID})-[]->(f:Fact {id: $nodeID}) SET f.content = $newValue RETURN f"
		params["userID"] = userID
	} else {
		cypher = "MATCH (f:Fact {id: $nodeID}) SET f.content = $newValue RETURN f"
	}
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
}

// DeleteMemoryNode removes a Fact node. When userID is empty, deletes the Fact by id (dashboard flow).
func (a *Adapter) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	var cypher string
	params := map[string]interface{}{"nodeID": nodeID}
	if userID != "" {
		cypher = "MATCH (u:User {id: $userID})-[]->(f:Fact {id: $nodeID}) DETACH DELETE f"
		params["userID"] = userID
	} else {
		cypher = "MATCH (f:Fact {id: $nodeID}) DETACH DELETE f"
	}
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
}

// UpdateNode implements NodeMutatorPort for the dashboard: updates a Fact by id (content and optional label).
// userID is not available in the dashboard flow; the adapter matches the Fact by id only.
func (a *Adapter) UpdateNode(ctx context.Context, id, label, typ, value string, properties map[string]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// MATCH Fact by id; set content and optionally the label property (display name in GUI).
	cypher := "MATCH (f:Fact {id: $id}) SET f.content = $value"
	params := map[string]interface{}{"id": id, "value": value}
	if label != "" {
		cypher += ", f.label = $label"
		params["label"] = label
	}
	cypher += " RETURN f"
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
}

// DeleteNode implements NodeMutatorPort for the dashboard: deletes a Fact by id (no userID in flow).
func (a *Adapter) DeleteNode(ctx context.Context, id string) error {
	return a.DeleteMemoryNode(ctx, "", id)
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
