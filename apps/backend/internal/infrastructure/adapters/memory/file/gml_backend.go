package file

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/neirth/openlobster/internal/domain/models"
	"github.com/neirth/openlobster/internal/domain/ports"
)

type Node struct {
	ID         int
	Label      string
	Type       string
	Value      string
	Properties map[string]string
}

type Edge struct {
	Source int
	Target int
	Label  string
}

type InMemoryGraph struct {
	Nodes  map[int]*Node
	Edges  []*Edge
	NextID int
}

func (g *InMemoryGraph) Copy() *InMemoryGraph {
	newNodes := make(map[int]*Node, len(g.Nodes))
	for k, v := range g.Nodes {
		props := make(map[string]string, len(v.Properties))
		for pk, pv := range v.Properties {
			props[pk] = pv
		}
		newNodes[k] = &Node{
			ID:         v.ID,
			Label:      v.Label,
			Type:       v.Type,
			Value:      v.Value,
			Properties: props,
		}
	}
	newEdges := make([]*Edge, len(g.Edges))
	for i, e := range g.Edges {
		newEdges[i] = &Edge{
			Source: e.Source,
			Target: e.Target,
			Label:  e.Label,
		}
	}
	return &InMemoryGraph{
		Nodes:  newNodes,
		Edges:  newEdges,
		NextID: g.NextID,
	}
}

type GMLBackend struct {
	path        string
	graph       *InMemoryGraph
	mu          sync.RWMutex
	dirty       bool
	persistCh   chan *InMemoryGraph
	persistDone chan struct{}
	loopDone    chan struct{}
}

func NewGMLBackend(path string) *GMLBackend {
	b := &GMLBackend{
		path:        path,
		graph:       &InMemoryGraph{Nodes: make(map[int]*Node)},
		persistCh:   make(chan *InMemoryGraph, 1),
		persistDone: make(chan struct{}, 1),
		loopDone:    make(chan struct{}),
	}
	go b.persistLoop()
	return b
}

func (b *GMLBackend) persistLoop() {
	defer close(b.loopDone)
	for graph := range b.persistCh {
		b.doPersist(graph)
		b.mu.Lock()
		b.dirty = false
		b.mu.Unlock()
		select {
		case b.persistDone <- struct{}{}:
		default:
		}
	}
}

// Close flushes any pending in-memory state to disk and waits for the persist
// goroutine to finish. Should be called on graceful shutdown.
func (b *GMLBackend) Close() error {
	b.mu.RLock()
	dirty := b.dirty
	var snapshot *InMemoryGraph
	if dirty {
		snapshot = b.graph.Copy()
	}
	b.mu.RUnlock()

	if dirty && snapshot != nil {
		b.schedulePersist(snapshot)
	}
	close(b.persistCh)
	<-b.loopDone
	return nil
}

// schedulePersist drains any pending snapshot from persistCh (already stale
// since b.graph has been updated in-place) and enqueues the latest copy.
// Must be called without b.mu held.
func (b *GMLBackend) schedulePersist(snapshot *InMemoryGraph) {
	select {
	case <-b.persistCh:
	default:
	}
	b.persistCh <- snapshot
}

func (b *GMLBackend) doPersist(graph *InMemoryGraph) error {
	data := serializeGML(graph)
	dir := b.path[:strings.LastIndex(b.path, "/")]
	if dir != "" && dir != b.path {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(b.path, data, 0644)
}

func (b *GMLBackend) Load() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	data, err := os.ReadFile(b.path)
	if err != nil && os.IsNotExist(err) {
		b.graph = &InMemoryGraph{Nodes: make(map[int]*Node)}
		b.doPersist(b.graph)
		return nil
	}
	if err != nil {
		return err
	}
	b.graph, err = parseGML(data)
	return err
}

func (b *GMLBackend) AddKnowledge(_ context.Context, userID string, content string, label string, relation string, entityType string, _ []float64) error {
	b.mu.Lock()

	graphCopy := b.graph.Copy()

	// Use the provided label for the fact node; fall back to a slug of the first
	// few words of the content so the graph is always human-readable.
	factLabel := label
	if factLabel == "" {
		words := strings.Fields(content)
		if len(words) > 4 {
			words = words[:4]
		}
		factLabel = strings.Join(words, "_")
	}

	edgeLabel := relation
	if edgeLabel == "" {
		edgeLabel = "HAS_FACT"
	}

	userNode := b.findOrCreateUserInGraph(graphCopy, userID)

	// Normalize entityType for GML node typing.
	etype := strings.ToLower(strings.TrimSpace(entityType))
	switch etype {
	case "person":
		entityType = "person"
	case "place":
		entityType = "place"
	case "thing":
		entityType = "thing"
	case "story":
		entityType = "story"
	case "event":
		entityType = "event"
	case "organization":
		entityType = "organization"
	case "", "fact":
		entityType = "fact"
	default:
		entityType = "fact"
	}

	// Step 1: find an existing node owned by this user.
	// We dedupe by (userID, nodeType, factLabel, edgeLabel) so that:
	// - cross-user overwrites do not happen (no global concept nodes)
	// - "permanent" relations are not destroyed (no edge label replacement)
	existingFactID := -1
	for _, edge := range graphCopy.Edges {
		if edge.Source != userNode.ID || edge.Label != edgeLabel {
			continue
		}
		if tgt, ok := graphCopy.Nodes[edge.Target]; ok && tgt != nil {
			if tgt.Type == entityType &&
				strings.EqualFold(tgt.Label, factLabel) {
				existingFactID = tgt.ID
				break
			}
		}
	}

	if existingFactID >= 0 {
		// Relation already exists for this exact fact evidence.
		b.mu.Unlock()
		return nil
	}

	// No existing fact for this user+relation+evidence — create a new node and edge.
	factID := graphCopy.NextID
	graphCopy.NextID++
	graphCopy.Nodes[factID] = &Node{
		ID:    factID,
		Label: factLabel,
		Type:  entityType,
		Value: content,
	}
	graphCopy.Edges = append(graphCopy.Edges, &Edge{
		Source: userNode.ID,
		Target: factID,
		Label:  edgeLabel,
	})

	b.graph = graphCopy
	b.dirty = true

	b.mu.Unlock()

	b.schedulePersist(graphCopy.Copy())

	return nil
}

func (b *GMLBackend) SearchSimilar(_ context.Context, query string, limit int) ([]ports.Knowledge, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var results []ports.Knowledge

	for id, node := range b.graph.Nodes {
		if node.Type != "user" && strings.Contains(node.Value, query) {
			results = append(results, ports.Knowledge{
				ID:      strconv.Itoa(id),
				Content: node.Value,
			})
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func responseNodeID(n *Node) string {
	if n != nil && n.Type == "user" {
		return "user:" + n.Value
	}
	if n == nil {
		return ""
	}
	return strconv.Itoa(n.ID)
}

func (b *GMLBackend) GetUserGraph(_ context.Context, userID string) (ports.Graph, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var nodes []models.Node
	var edges []models.Edge

	// Empty userID or "*" returns the full graph (for the dashboard that shows all memories).
	if userID == "" || userID == "*" {
		idMap := make(map[int]string, len(b.graph.Nodes))
		for _, n := range b.graph.Nodes {
			rid := responseNodeID(n)
			idMap[n.ID] = rid
			nodes = append(nodes, models.Node{
				ID:         rid,
				Label:      n.Label,
				Type:       n.Type,
				Value:      n.Value,
				Properties: n.Properties,
			})
		}
		for _, edge := range b.graph.Edges {
			edges = append(edges, models.Edge{
				Source: idMap[edge.Source],
				Target: idMap[edge.Target],
				Label:  edge.Label,
			})
		}
		graphNodes := make([]ports.GraphNode, len(nodes))
		graphEdges := make([]ports.GraphEdge, len(edges))
		for i, n := range nodes {
			graphNodes[i] = ports.GraphNode{
				ID:         n.ID,
				Label:      n.Label,
				Type:       n.Type,
				Value:      n.Value,
				Properties: n.Properties,
			}
		}
		for i, e := range edges {
			graphEdges[i] = ports.GraphEdge{
				Source: e.Source,
				Target: e.Target,
				Label:  e.Label,
			}
		}
		return ports.Graph{Nodes: graphNodes, Edges: graphEdges}, nil
	}

	userNode := b.findUserNodeInGraph(b.graph, userID)
	if userNode == nil {
		// Keep parity with Neo4j: return a synthetic user node even when the
		// user has no relations yet.
		return ports.Graph{
			Nodes: []ports.GraphNode{{
				ID:    "user:" + userID,
				Label: "User",
				Type:  "user",
				Value: userID,
			}},
			Edges: []ports.GraphEdge{},
		}, nil
	}

	nodesMap := make(map[int]bool)
	nodesMap[userNode.ID] = true

	for _, edge := range b.graph.Edges {
		if edge.Source == userNode.ID || edge.Target == userNode.ID {
			srcNode := b.graph.Nodes[edge.Source]
			tgtNode := b.graph.Nodes[edge.Target]
			srcID := strconv.Itoa(edge.Source)
			tgtID := strconv.Itoa(edge.Target)
			if srcNode != nil {
				srcID = responseNodeID(srcNode)
			}
			if tgtNode != nil {
				tgtID = responseNodeID(tgtNode)
			}
			edges = append(edges, models.Edge{
				Source: srcID,
				Target: tgtID,
				Label:  edge.Label,
			})
			nodesMap[edge.Source] = true
			nodesMap[edge.Target] = true
		}
	}

	for id := range nodesMap {
		if n, ok := b.graph.Nodes[id]; ok {
			nodes = append(nodes, models.Node{
				ID:         responseNodeID(n),
				Label:      n.Label,
				Type:       n.Type,
				Value:      n.Value,
				Properties: n.Properties,
			})
		}
	}

	graphNodes := make([]ports.GraphNode, len(nodes))
	graphEdges := make([]ports.GraphEdge, len(edges))
	for i, n := range nodes {
		graphNodes[i] = ports.GraphNode{
			ID:         n.ID,
			Label:      n.Label,
			Type:       n.Type,
			Value:      n.Value,
			Properties: n.Properties,
		}
	}
	for i, e := range edges {
		graphEdges[i] = ports.GraphEdge{
			Source: e.Source,
			Target: e.Target,
			Label:  e.Label,
		}
	}

	return ports.Graph{
		Nodes: graphNodes,
		Edges: graphEdges,
	}, nil
}

func (b *GMLBackend) AddRelation(_ context.Context, from, to string, relType string) error {
	b.mu.Lock()

	graphCopy := b.graph.Copy()

	fromNode := b.findOrCreateUserInGraph(graphCopy, from)
	toNode := b.findOrCreateUserInGraph(graphCopy, to)

	// Keep parity with Neo4j MERGE semantics: avoid duplicate user-user edges.
	for _, e := range graphCopy.Edges {
		if e.Source == fromNode.ID && e.Target == toNode.ID && e.Label == relType {
			b.graph = graphCopy
			b.dirty = true
			b.mu.Unlock()
			b.schedulePersist(graphCopy.Copy())
			return nil
		}
	}
	graphCopy.Edges = append(graphCopy.Edges, &Edge{Source: fromNode.ID, Target: toNode.ID, Label: relType})

	b.graph = graphCopy
	b.dirty = true

	b.mu.Unlock()

	b.schedulePersist(graphCopy.Copy())

	return nil
}

// Regexes for minimal Cypher parsing (Cypher-like syntax, in-memory exploration).
var (
	reMatchNode      = regexp.MustCompile(`(?i)MATCH\s*\(\s*(\w+)\s*(?::(\w+))?\s*\)\s*RETURN\s+(.+)`)
	reMatchEdgeDir   = regexp.MustCompile(`(?i)MATCH\s*\(\s*(\w+)\s*\)\s*-\s*\[\s*(\w+)\s*\]\s*->\s*\(\s*(\w+)\s*\)\s*RETURN\s+(.+)`)
	reMatchEdgeUndir = regexp.MustCompile(`(?i)MATCH\s*\(\s*(\w+)\s*\)\s*-\s*\[\s*(\w+)\s*\]\s*-\s*\(\s*(\w+)\s*\)\s*RETURN\s+(.+)`)
)

func nodeToRow(n *Node) map[string]interface{} {
	props := make(map[string]interface{})
	if n.Properties != nil {
		for k, v := range n.Properties {
			props[k] = v
		}
	}
	return map[string]interface{}{
		"id":         strconv.Itoa(n.ID),
		"label":      n.Label,
		"type":       n.Type,
		"value":      n.Value,
		"properties": props,
	}
}

func edgeToRow(e *Edge) map[string]interface{} {
	return map[string]interface{}{
		"source": strconv.Itoa(e.Source),
		"target": strconv.Itoa(e.Target),
		"label":  e.Label,
	}
}

func parseReturnList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

// executeCypherInMemory runs a minimal Cypher subset against a copy of the graph.
func executeCypherInMemory(graph *InMemoryGraph, cypher string) ([]map[string]interface{}, error) {
	q := strings.TrimSpace(cypher)
	if q == "" {
		return nil, fmt.Errorf("empty query")
	}

	// MATCH (a)-[r]->(b) RETURN a, r, b
	if m := reMatchEdgeDir.FindStringSubmatch(q); len(m) == 5 {
		sourceVar, edgeVar, targetVar := m[1], m[2], m[3]
		returnVars := parseReturnList(m[4])
		var data []map[string]interface{}
		for _, e := range graph.Edges {
			src, okSrc := graph.Nodes[e.Source]
			tgt, okTgt := graph.Nodes[e.Target]
			if !okSrc || !okTgt {
				continue
			}
			row := make(map[string]interface{})
			for _, v := range returnVars {
				switch v {
				case sourceVar:
					row[v] = nodeToRow(src)
				case edgeVar:
					row[v] = edgeToRow(e)
				case targetVar:
					row[v] = nodeToRow(tgt)
				}
			}
			data = append(data, row)
		}
		return data, nil
	}

	// MATCH (a)-[r]-(b) RETURN a, r, b (undirected)
	if m := reMatchEdgeUndir.FindStringSubmatch(q); len(m) == 5 {
		sourceVar, edgeVar, targetVar := m[1], m[2], m[3]
		returnVars := parseReturnList(m[4])
		var data []map[string]interface{}
		for _, e := range graph.Edges {
			src, okSrc := graph.Nodes[e.Source]
			tgt, okTgt := graph.Nodes[e.Target]
			if !okSrc || !okTgt {
				continue
			}
			row := make(map[string]interface{})
			for _, v := range returnVars {
				switch v {
				case sourceVar:
					row[v] = nodeToRow(src)
				case edgeVar:
					row[v] = edgeToRow(e)
				case targetVar:
					row[v] = nodeToRow(tgt)
				}
			}
			data = append(data, row)
		}
		return data, nil
	}

	// MATCH (n) RETURN n  or  MATCH (n:User) RETURN n  or  MATCH (n:Fact) RETURN n
	if m := reMatchNode.FindStringSubmatch(q); len(m) >= 3 {
		nodeVar := m[1]
		labelFilter := ""
		if len(m) > 2 && m[2] != "" {
			labelFilter = strings.ToLower(m[2])
		}
		returnVars := parseReturnList(m[3])
		var data []map[string]interface{}
		for _, n := range graph.Nodes {
			if labelFilter != "" && strings.ToLower(n.Type) != labelFilter {
				continue
			}
			row := make(map[string]interface{})
			for _, v := range returnVars {
				if v == nodeVar {
					row[v] = nodeToRow(n)
					break
				}
			}
			if len(row) > 0 {
				data = append(data, row)
			}
		}
		return data, nil
	}

	return nil, fmt.Errorf("unsupported Cypher pattern (supported: MATCH (n) RETURN n, MATCH (n:User)/MATCH (n:Fact) RETURN n, MATCH (a)-[r]->(b) RETURN a,r,b)")
}

// QueryGraph runs a minimal Cypher-like query against the in-memory graph and returns
// rows as []map[string]interface{} (kept in parity with Neo4j). It supports:
//   - MATCH (n) RETURN n
//   - MATCH (n:User) RETURN n  /  MATCH (n:Fact) RETURN n
//   - MATCH (a)-[r]->(b) RETURN a, r, b  (directed)
//   - MATCH (a)-[r]-(b) RETURN a, r, b  (undirected)
func (b *GMLBackend) QueryGraph(ctx context.Context, cypher string) (ports.GraphResult, error) {
	b.mu.RLock()
	graph := b.graph.Copy()
	b.mu.RUnlock()

	result, err := executeCypherInMemory(graph, cypher)
	if err != nil {
		return ports.GraphResult{Errors: []error{err}}, nil
	}
	return ports.GraphResult{Data: result}, nil
}

func (b *GMLBackend) UpdateNode(_ context.Context, id, label, typ, value string, properties map[string]string) error {
	if strings.HasPrefix(id, "user:") {
		userID := strings.TrimPrefix(id, "user:")
		b.mu.Lock()
		graphCopy := b.graph.Copy()
		node := b.findOrCreateUserInGraph(graphCopy, userID)
		if label != "" {
			node.Label = label
		} else if value != "" {
			node.Label = value
		}
		b.graph = graphCopy
		b.dirty = true
		b.mu.Unlock()
		b.schedulePersist(graphCopy.Copy())
		return nil
	}

	nodeID, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid node id: %s", id)
	}

	b.mu.Lock()
	graphCopy := b.graph.Copy()
	node, ok := graphCopy.Nodes[nodeID]
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("node not found: %s", id)
	}
	if label != "" {
		node.Label = label
	}
	if typ != "" {
		node.Type = typ
	}
	node.Value = value
	if properties != nil {
		if node.Properties == nil {
			node.Properties = make(map[string]string)
		}
		for k, v := range properties {
			if v == "" {
				delete(node.Properties, k)
			} else {
				node.Properties[k] = v
			}
		}
	}
	b.graph = graphCopy
	b.dirty = true
	b.mu.Unlock()

	b.schedulePersist(graphCopy.Copy())
	return nil
}

func (b *GMLBackend) DeleteNode(_ context.Context, id string) error {
	if strings.HasPrefix(id, "user:") {
		userID := strings.TrimPrefix(id, "user:")
		b.mu.Lock()
		graphCopy := b.graph.Copy()
		node := b.findUserNodeInGraph(graphCopy, userID)
		if node == nil {
			b.mu.Unlock()
			return fmt.Errorf("node not found: %s", id)
		}
		nodeID := node.ID
		delete(graphCopy.Nodes, nodeID)
		var newEdges []*Edge
		for _, e := range graphCopy.Edges {
			if e.Source != nodeID && e.Target != nodeID {
				newEdges = append(newEdges, e)
			}
		}
		graphCopy.Edges = newEdges
		b.graph = graphCopy
		b.dirty = true
		b.mu.Unlock()
		b.schedulePersist(graphCopy.Copy())
		return nil
	}

	nodeID, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid node id: %s", id)
	}

	b.mu.Lock()
	graphCopy := b.graph.Copy()
	if _, ok := graphCopy.Nodes[nodeID]; !ok {
		b.mu.Unlock()
		return fmt.Errorf("node not found: %s", id)
	}
	delete(graphCopy.Nodes, nodeID)
	var newEdges []*Edge
	for _, e := range graphCopy.Edges {
		if e.Source != nodeID && e.Target != nodeID {
			newEdges = append(newEdges, e)
		}
	}
	graphCopy.Edges = newEdges
	b.graph = graphCopy
	b.dirty = true
	b.mu.Unlock()

	b.schedulePersist(graphCopy.Copy())
	return nil
}

func (b *GMLBackend) InvalidateMemoryCache(_ context.Context, userID string) error {
	return nil
}

// EditMemoryNode updates the value of a fact node, verifying it belongs to userID.
func (b *GMLBackend) EditMemoryNode(ctx context.Context, userID, nodeID, newValue string) error {
	b.mu.RLock()
	userNode := b.findUserNodeInGraph(b.graph, userID)
	if userNode == nil {
		b.mu.RUnlock()
		return fmt.Errorf("user not found in memory graph")
	}
	// Verify the target node is connected to the user via HAS_FACT.
	nodeIDInt, err := strconv.Atoi(nodeID)
	if err != nil {
		b.mu.RUnlock()
		return fmt.Errorf("invalid node id: %s", nodeID)
	}
	owned := false
	for _, e := range b.graph.Edges {
		if e.Source == userNode.ID && e.Target == nodeIDInt {
			owned = true
			break
		}
	}
	b.mu.RUnlock()
	if !owned {
		return fmt.Errorf("node %s is not a fact belonging to this user", nodeID)
	}
	// Preserve the existing node type; only update the textual content.
	return b.UpdateNode(ctx, nodeID, "", "", newValue, nil)
}

// DeleteMemoryNode removes a fact node, verifying it belongs to userID.
func (b *GMLBackend) DeleteMemoryNode(ctx context.Context, userID, nodeID string) error {
	b.mu.RLock()
	userNode := b.findUserNodeInGraph(b.graph, userID)
	if userNode == nil {
		b.mu.RUnlock()
		return fmt.Errorf("user not found in memory graph")
	}
	nodeIDInt, err := strconv.Atoi(nodeID)
	if err != nil {
		b.mu.RUnlock()
		return fmt.Errorf("invalid node id: %s", nodeID)
	}
	owned := false
	for _, e := range b.graph.Edges {
		if e.Source == userNode.ID && e.Target == nodeIDInt {
			owned = true
			break
		}
	}
	b.mu.RUnlock()
	if !owned {
		return fmt.Errorf("node %s is not a fact belonging to this user", nodeID)
	}
	return b.DeleteNode(ctx, nodeID)
}

// SetUserProperty upserts a key/value pair on the user node's Properties map.
// The property is persisted to the GML file alongside the rest of the graph.
func (b *GMLBackend) SetUserProperty(_ context.Context, userID, key, value string) error {
	b.mu.Lock()

	graphCopy := b.graph.Copy()
	userNode := b.findOrCreateUserInGraph(graphCopy, userID)
	if userNode.Properties == nil {
		userNode.Properties = make(map[string]string)
	}
	userNode.Properties[key] = value
	b.graph = graphCopy
	b.dirty = true

	b.mu.Unlock()

	b.schedulePersist(graphCopy.Copy())
	return nil
}

// UpdateUserLabel sets the Label of the user node to the human-readable display
// name so the graph dashboard shows the real name instead of the internal UUID.
func (b *GMLBackend) UpdateUserLabel(_ context.Context, userID, displayName string) error {
	if displayName == "" {
		return nil
	}
	b.mu.Lock()
	graphCopy := b.graph.Copy()
	userNode := b.findOrCreateUserInGraph(graphCopy, userID)
	userNode.Label = displayName
	b.graph = graphCopy
	b.dirty = true
	b.mu.Unlock()
	b.schedulePersist(graphCopy.Copy())
	return nil
}

func (b *GMLBackend) findOrCreateUserInGraph(graph *InMemoryGraph, userID string) *Node {
	if node := b.findUserNodeInGraph(graph, userID); node != nil {
		return node
	}

	userIDInt := graph.NextID
	graph.NextID++
	userNode := &Node{
		ID:    userIDInt,
		Label: userID,
		Type:  "user",
		Value: userID,
	}
	graph.Nodes[userIDInt] = userNode
	return userNode
}

func (b *GMLBackend) findUserNodeInGraph(graph *InMemoryGraph, userID string) *Node {
	for _, node := range graph.Nodes {
		if node.Type == "user" && node.Value == userID {
			return node
		}
	}
	return nil
}

func parseGML(data []byte) (*InMemoryGraph, error) {
	graph := &InMemoryGraph{
		Nodes: make(map[int]*Node),
		Edges: make([]*Edge, 0),
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var currentNode *Node
	var currentEdge *Edge
	nodeRegex := regexp.MustCompile(`^\s*node\s+\[`)
	edgeRegex := regexp.MustCompile(`^\s*edge\s+\[`)
	closeBracketRegex := regexp.MustCompile(`^\s*\]`)
	keyValRegex := regexp.MustCompile(`^\s*(\w+)\s+(.+)$`)
	directedRegex := regexp.MustCompile(`^\s*directed\s+(\d+)`)

	for _, line := range lines {
		if directedRegex.MatchString(line) {
			continue
		}

		if nodeRegex.MatchString(line) {
			currentNode = &Node{Properties: make(map[string]string)}
			continue
		}

		if edgeRegex.MatchString(line) {
			currentEdge = &Edge{}
			currentNode = nil
			continue
		}

		if closeBracketRegex.MatchString(line) {
			if currentNode != nil && currentNode.ID >= 0 {
				graph.Nodes[currentNode.ID] = currentNode
				if currentNode.ID >= graph.NextID {
					graph.NextID = currentNode.ID + 1
				}
				currentNode = nil
			} else if currentEdge != nil {
				graph.Edges = append(graph.Edges, currentEdge)
				currentEdge = nil
			}
			continue
		}

		if currentNode != nil {
			matches := keyValRegex.FindStringSubmatch(line)
			if len(matches) == 3 {
				key := matches[1]
				val := strings.Trim(matches[2], "\"")

				switch key {
				case "id":
					currentNode.ID, _ = strconv.Atoi(val)
				case "label":
					currentNode.Label = val
				case "type":
					currentNode.Type = val
				case "value":
					currentNode.Value = val
				default:
					currentNode.Properties[key] = val
				}
			}
		} else if currentEdge != nil {
			matches := keyValRegex.FindStringSubmatch(line)
			if len(matches) == 3 {
				key := matches[1]
				val := strings.Trim(matches[2], "\"")

				switch key {
				case "source":
					currentEdge.Source, _ = strconv.Atoi(val)
				case "target":
					currentEdge.Target, _ = strconv.Atoi(val)
				case "label", "relation":
					currentEdge.Label = val
				}
			}
		}
	}

	for _, node := range graph.Nodes {
		if node.Label == "" {
			node.Label = fmt.Sprintf("node:%d", node.ID)
		}
	}

	return graph, nil
}

var _ ports.MemoryPort = (*GMLBackend)(nil)

func serializeGML(graph *InMemoryGraph) []byte {
	var sb strings.Builder
	sb.WriteString("graph [\n")
	sb.WriteString("  directed 1\n")

	for _, node := range graph.Nodes {
		sb.WriteString("  node [\n")
		fmt.Fprintf(&sb, "    id %d\n", node.ID)
		fmt.Fprintf(&sb, "    label \"%s\"\n", node.Label)
		if node.Type != "" {
			fmt.Fprintf(&sb, "    type \"%s\"\n", node.Type)
		}
		if node.Value != "" {
			fmt.Fprintf(&sb, "    value \"%s\"\n", node.Value)
		}
		// Write custom properties
		for key, val := range node.Properties {
			fmt.Fprintf(&sb, "    %s \"%s\"\n", key, val)
		}
		sb.WriteString("  ]\n")
	}

	for _, edge := range graph.Edges {
		sb.WriteString("  edge [\n")
		fmt.Fprintf(&sb, "    source %d\n", edge.Source)
		fmt.Fprintf(&sb, "    target %d\n", edge.Target)
		fmt.Fprintf(&sb, "    label \"%s\"\n", edge.Label)
		sb.WriteString("  ]\n")
	}

	sb.WriteString("]\n")
	return []byte(sb.String())
}
