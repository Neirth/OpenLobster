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
		words := strings.Fields(content)
		if len(words) > 4 {
			words = words[:4]
		}
		label = strings.Join(words, "_")
		if label == "" {
			label = uuid.New().String()
		}
	}
	rel := relation
	if rel == "" {
		rel = "HAS_FACT"
	}

	// Step 1: search for an existing concept node globally (shared across users).
	var existingFactID string
	searchResult, searchErr := session.Run(ctx,
		"MATCH (f:Fact) WHERE toLower(f.label) = toLower($label) RETURN f.id AS id LIMIT 1",
		map[string]interface{}{"label": label},
	)
	if searchErr == nil {
		if rec, recErr := searchResult.Single(ctx); recErr == nil && rec != nil {
			if id, ok := rec.Get("id"); ok {
				existingFactID, _ = id.(string)
			}
		}
	}

	if existingFactID != "" {
		// Concept already exists: update its content, then upsert this user's
		// relation without touching other users' relations to the same node.
		upsertResult, err := session.Run(ctx,
			"MATCH (f:Fact {id: $factID}) SET f.content = $content "+
				"WITH f MERGE (u:User {id: $userID}) "+
				"OPTIONAL MATCH (u)-[oldRel]->(f) DELETE oldRel "+
				"WITH u, f CREATE (u)-[r:"+rel+"]->(f)",
			map[string]interface{}{"factID": existingFactID, "content": content, "userID": userID},
		)
		if err != nil {
			return err
		}
		upsertResult.Consume(ctx)
		return nil
	}

	// No existing concept — create a new Fact node shared across users and
	// attach this user's relation.
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
		"MATCH (u:User {id: $userID})-[r]-(n) RETURN labels(n) AS labels, n.id AS id, n.content AS content, n.name AS name, n.label AS nodeLabel, type(r) AS relType, properties(u) AS userProps, properties(n) AS nodeProps",
		map[string]interface{}{"userID": userID},
	)
	if err != nil {
		return ports.Graph{}, err
	}

	nodes := make(map[string]ports.GraphNode)
	edges := make([]ports.GraphEdge, 0)

	userNodeID := fmt.Sprintf("user:%s", userID)
	userNode := ports.GraphNode{ID: userNodeID, Label: userLabel, Type: "user", Value: userVal}

	for result.Next(ctx) {
		record := result.Record()
		labels, _ := record.Get("labels")
		id, _ := record.Get("id")
		relType, _ := record.Get("relType")

		// Populate user node properties from the first row that has them.
		if _, exists := nodes[userNodeID]; !exists {
			if up, ok := record.Get("userProps"); ok {
				userNode.Properties = neo4jPropsToMap(up, "id", "displayName")
			}
			nodes[userNodeID] = userNode
		}

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
			np, _ := record.Get("nodeProps")
			// Use real Neo4j node id so GUI delete/update work.
			// Exclude "label" since it is already surfaced as the Label field on GraphNode.
			nodes[idVal] = ports.GraphNode{ID: idVal, Label: displayLabel, Type: strings.ToLower(typeStr), Value: value, Properties: neo4jPropsToMap(np, "id", "displayName", "label")}
			edges = append(edges, ports.GraphEdge{Source: userNodeID, Target: idVal, Label: fmt.Sprintf("%v", relType)})
		}
	}
	// Ensure user node is present even when there are no connected nodes.
	if _, exists := nodes[userNodeID]; !exists {
		nodes[userNodeID] = userNode
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
		"MATCH (u:User)-[r]->(n) RETURN u.id AS userId, u.displayName AS userDisplayName, labels(n) AS labels, n.id AS id, n.content AS content, n.name AS name, n.label AS nodeLabel, n.displayName AS targetDisplayName, type(r) AS relType, properties(u) AS userProps, properties(n) AS nodeProps",
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
			up, _ := record.Get("userProps")
			nodes[userNodeID] = ports.GraphNode{ID: userNodeID, Label: userLabel, Type: "user", Value: userVal, Properties: neo4jPropsToMap(up, "id", "displayName")}
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
			np, _ := record.Get("nodeProps")
			// Use real Neo4j node id so GUI delete/update work.
			// Exclude "label" since it is already surfaced as the Label field on GraphNode.
			nodes[idVal] = ports.GraphNode{ID: idVal, Label: displayLabel, Type: strings.ToLower(typeStr), Value: value, Properties: neo4jPropsToMap(np, "id", "displayName", "label")}
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
// The result always starts with a letter: a "REL_" prefix is added when the first retained
// character would otherwise be a digit, which is invalid in Cypher relationship type names.
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
	result := b.String()
	if result[0] >= '0' && result[0] <= '9' {
		return "REL_" + result
	}
	return result
}

// AddRelation creates a typed relationship between two User nodes (e.g. FRIEND_OF, KNOWS).
// Used by add_user_relation so that "user A is friend of user B" is stored as (User)-[:FRIEND_OF]->(User).
func (a *Adapter) AddRelation(ctx context.Context, from, to string, relType string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())

	rel := sanitizeRelType(relType)
	cypher := fmt.Sprintf("MERGE (a:User {id: $from}) MERGE (b:User {id: $to}) MERGE (a)-[r:%s]->(b) RETURN r", rel)
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
	result, err := session.Run(ctx, cypher, map[string]interface{}{
		"userID": userID,
		"value":  value,
	})
	if err != nil {
		return err
	}
	result.Consume(ctx)
	return nil
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

// UpdateNode implements NodeMutatorPort for the dashboard: updates a node by id.
// User nodes are identified by the synthetic "user:<userId>" prefix used when
// building graph responses. Fact nodes are matched by their UUID id property.
func (a *Adapter) UpdateNode(ctx context.Context, id, label, typ, value string, properties map[string]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session := a.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// User nodes use the synthetic "user:<userId>" prefix — strip it to get the real userId.
	if strings.HasPrefix(id, "user:") {
		userID := strings.TrimPrefix(id, "user:")
		cypher := "MERGE (u:User {id: $userID})"
		params := map[string]interface{}{"userID": userID}
		if label != "" {
			cypher += " SET u.displayName = $label"
			params["label"] = label
		} else if value != "" {
			cypher += " SET u.displayName = $value"
			params["value"] = value
		}
		result, err := session.Run(ctx, cypher, params)
		if err != nil {
			return err
		}
		result.Consume(ctx)
		return nil
	}

	// Default: Fact node — MATCH by UUID id, set content and optionally the label property.
	cypher := "MATCH (f:Fact {id: $id}) SET f.content = $value"
	params := map[string]interface{}{"id": id, "value": value}
	if label != "" {
		cypher += ", f.label = $label"
		params["label"] = label
	}
	// Apply extra properties from the map, skipping reserved/already-handled keys.
	for k, v := range properties {
		safe := sanitizePropKey(k)
		if safe == "" || safe == "id" || safe == "content" || safe == "label" {
			continue
		}
		paramKey := "prop_" + safe
		cypher += fmt.Sprintf(", f.%s = $%s", safe, paramKey)
		params[paramKey] = v
	}
	cypher += " RETURN f"
	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return err
	}
	if !result.Next(ctx) {
		return fmt.Errorf("neo4j: UpdateNode: no node found with id %q", id)
	}
	result.Consume(ctx)
	return nil
}

// sanitizePropKey returns a safe Cypher property key: starts with a letter, contains only
// letters, digits, and underscores. Returns empty string if no valid characters remain.
func sanitizePropKey(key string) string {
	var b strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (b.Len() > 0 && r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
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

// neo4jPropsToMap converts the result of Cypher's properties(n) to a
// map[string]string suitable for GraphNode.Properties, skipping any keys
// listed in exclude (typically "id" and "displayName" which are already
// surfaced as dedicated fields on GraphNode).
func neo4jPropsToMap(raw interface{}, exclude ...string) map[string]string {
	propsMap, ok := raw.(map[string]interface{})
	if !ok || len(propsMap) == 0 {
		return nil
	}
	skip := make(map[string]bool, len(exclude))
	for _, k := range exclude {
		skip[k] = true
	}
	result := make(map[string]string, len(propsMap))
	for k, v := range propsMap {
		if skip[k] || v == nil {
			continue
		}
		result[k] = fmt.Sprintf("%v", v)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

var _ ports.MemoryPort = (*Adapter)(nil)
