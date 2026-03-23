// Copyright (c) OpenLobster contributors. See LICENSE for details.

package file

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// NewGMLBackend + Load
// ---------------------------------------------------------------------------

func TestNewGMLBackend_EmptyGraph(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/graph.gml")
	defer b.Close()
	assert.NotNil(t, b)
	assert.NotNil(t, b.graph)
}

func TestGMLBackend_Load_NonExistentFile(t *testing.T) {
	path := t.TempDir() + "/new.gml"
	b := NewGMLBackend(path)
	err := b.Load()
	assert.NoError(t, err)
	// File should have been created.
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr)
}

func TestGMLBackend_Load_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/graph.gml"
	b := NewGMLBackend(path)
	defer b.Close()

	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "alice likes cats", "cats", "LIKES", "thing", nil))
	// Flush by closing and reopening.
	require.NoError(t, b.Close())

	b2 := NewGMLBackend(path)
	defer b2.Close()
	require.NoError(t, b2.Load())

	graph, err := b2.GetUserGraph(ctx, "alice")
	require.NoError(t, err)
	// At least the user node and the fact node should be there.
	assert.GreaterOrEqual(t, len(graph.Nodes), 1)
}

// ---------------------------------------------------------------------------
// AddKnowledge — entity type normalization
// ---------------------------------------------------------------------------

func TestAddKnowledge_EntityTypeNormalization(t *testing.T) {
	cases := []struct {
		raw      string
		expected string
	}{
		{"person", "person"},
		{"PERSON", "person"},
		{"place", "place"},
		{"thing", "thing"},
		{"story", "story"},
		{"event", "event"},
		{"organization", "organization"},
		{"fact", "fact"},
		{"", "fact"},
		{"unknown_type", "fact"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			b := NewGMLBackend(t.TempDir() + "/g.gml")
			defer b.Close()
			ctx := context.Background()
			err := b.AddKnowledge(ctx, "user1", "some content", "label", "HAS_FACT", tc.raw, nil)
			require.NoError(t, err)

			graph, err := b.GetUserGraph(ctx, "user1")
			require.NoError(t, err)

			found := false
			for _, n := range graph.Nodes {
				if n.Type == tc.expected && n.Label == "label" {
					found = true
					break
				}
			}
			assert.True(t, found, "expected node with type %q", tc.expected)
		})
	}
}

func TestAddKnowledge_LabelFallback_FromContent(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	// Empty label — should derive from first 4 words of content.
	err := b.AddKnowledge(ctx, "alice", "The quick brown fox jumped over", "", "HAS_FACT", "fact", nil)
	require.NoError(t, err)

	graph, err := b.GetUserGraph(ctx, "alice")
	require.NoError(t, err)
	found := false
	for _, n := range graph.Nodes {
		if n.Type == "fact" && n.Label == "The_quick_brown_fox" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestAddKnowledge_DefaultEdgeLabel(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	// Empty relation — should use HAS_FACT.
	err := b.AddKnowledge(ctx, "bob", "content", "label", "", "fact", nil)
	require.NoError(t, err)

	graph, err := b.GetUserGraph(ctx, "bob")
	require.NoError(t, err)
	found := false
	for _, e := range graph.Edges {
		if e.Label == "HAS_FACT" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestAddKnowledge_Idempotent(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := b.AddKnowledge(ctx, "user1", "some content", "label", "HAS_FACT", "fact", nil)
		require.NoError(t, err)
	}

	graph, err := b.GetUserGraph(ctx, "user1")
	require.NoError(t, err)
	count := 0
	for _, e := range graph.Edges {
		if e.Label == "HAS_FACT" {
			count++
		}
	}
	assert.Equal(t, 1, count, "duplicate calls should not create duplicate edges")
}

// ---------------------------------------------------------------------------
// GetUserGraph — wildcard
// ---------------------------------------------------------------------------

func TestGetUserGraph_Wildcard(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()

	require.NoError(t, b.AddKnowledge(ctx, "alice", "likes cats", "cats", "LIKES", "thing", nil))
	require.NoError(t, b.AddKnowledge(ctx, "bob", "likes dogs", "dogs", "LIKES", "thing", nil))

	graph, err := b.GetUserGraph(ctx, "*")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(graph.Nodes), 4) // 2 users + 2 fact nodes
}

func TestGetUserGraph_EmptyUserID(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "likes cats", "cats", "LIKES", "thing", nil))

	graph, err := b.GetUserGraph(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(graph.Nodes), 1)
}

// ---------------------------------------------------------------------------
// UpdateNode
// ---------------------------------------------------------------------------

func TestUpdateNode_ByNumericID(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "original value", "my-label", "HAS_FACT", "fact", nil))

	// Find the fact node ID.
	graph, err := b.GetUserGraph(ctx, "alice")
	require.NoError(t, err)
	var factID string
	for _, n := range graph.Nodes {
		if n.Type == "fact" {
			factID = n.ID
			break
		}
	}
	require.NotEmpty(t, factID)

	err = b.UpdateNode(ctx, factID, "new-label", "fact", "updated value", map[string]string{"key": "val"})
	require.NoError(t, err)

	graph2, err := b.GetUserGraph(ctx, "alice")
	require.NoError(t, err)
	found := false
	for _, n := range graph2.Nodes {
		if n.ID == factID && n.Value == "updated value" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestUpdateNode_UserPrefix(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "content", "label", "HAS_FACT", "fact", nil))

	err := b.UpdateNode(ctx, "user:alice", "Alice Smith", "", "", nil)
	require.NoError(t, err)
}

func TestUpdateNode_UserPrefix_LabelFromValue(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "bob", "content", "label", "HAS_FACT", "fact", nil))

	// Label="" but value="Bob Updated" — should use value as label.
	err := b.UpdateNode(ctx, "user:bob", "", "", "Bob Updated", nil)
	require.NoError(t, err)
}

func TestUpdateNode_InvalidNumericID(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.UpdateNode(context.Background(), "not-a-number", "", "", "", nil)
	assert.Error(t, err)
}

func TestUpdateNode_NumericIDNotFound(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.UpdateNode(context.Background(), "9999", "", "", "", nil)
	assert.Error(t, err)
}

func TestUpdateNode_DeleteProperty(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "content", "label", "HAS_FACT", "fact", nil))

	graph, err := b.GetUserGraph(ctx, "alice")
	require.NoError(t, err)
	var factID string
	for _, n := range graph.Nodes {
		if n.Type == "fact" {
			factID = n.ID
			break
		}
	}
	require.NotEmpty(t, factID)

	// Set then delete a property by passing empty value.
	err = b.UpdateNode(ctx, factID, "", "", "", map[string]string{"toDelete": "exists"})
	require.NoError(t, err)
	err = b.UpdateNode(ctx, factID, "", "", "", map[string]string{"toDelete": ""})
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// DeleteNode
// ---------------------------------------------------------------------------

func TestDeleteNode_InvalidNumericID(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.DeleteNode(context.Background(), "abc")
	assert.Error(t, err)
}

func TestDeleteNode_NumericIDNotFound(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.DeleteNode(context.Background(), "9999")
	assert.Error(t, err)
}

func TestDeleteNode_UserPrefix_NotFound(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.DeleteNode(context.Background(), "user:nonexistent")
	assert.Error(t, err)
}

func TestDeleteNode_NumericID_RemovesEdges(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "likes cats", "cats", "LIKES", "thing", nil))

	graph, err := b.GetUserGraph(ctx, "alice")
	require.NoError(t, err)
	var factID string
	for _, n := range graph.Nodes {
		if n.Type == "thing" {
			factID = n.ID
			break
		}
	}
	require.NotEmpty(t, factID)

	err = b.DeleteNode(ctx, factID)
	require.NoError(t, err)

	graph2, err := b.GetUserGraph(ctx, "alice")
	require.NoError(t, err)
	for _, e := range graph2.Edges {
		assert.NotEqual(t, factID, e.Target)
	}
}

// ---------------------------------------------------------------------------
// EditMemoryNode
// ---------------------------------------------------------------------------

func TestEditMemoryNode_Success(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "original", "my-fact", "HAS_FACT", "fact", nil))

	graph, err := b.GetUserGraph(ctx, "alice")
	require.NoError(t, err)
	var factID string
	for _, n := range graph.Nodes {
		if n.Type == "fact" {
			factID = n.ID
			break
		}
	}
	require.NotEmpty(t, factID)

	err = b.EditMemoryNode(ctx, "alice", factID, "updated value")
	require.NoError(t, err)
}

func TestEditMemoryNode_UserNotFound(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.EditMemoryNode(context.Background(), "nonexistent", "1", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestEditMemoryNode_InvalidNodeID(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "content", "label", "HAS_FACT", "fact", nil))

	err := b.EditMemoryNode(ctx, "alice", "not-a-number", "value")
	assert.Error(t, err)
}

func TestEditMemoryNode_NotOwned(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "content", "label", "HAS_FACT", "fact", nil))
	require.NoError(t, b.AddKnowledge(ctx, "bob", "bob content", "bob-label", "HAS_FACT", "fact", nil))

	// Find alice's fact node.
	graph, _ := b.GetUserGraph(ctx, "alice")
	var aliceFactID string
	for _, n := range graph.Nodes {
		if n.Type == "fact" {
			aliceFactID = n.ID
			break
		}
	}
	require.NotEmpty(t, aliceFactID)

	// Bob should not be able to edit alice's fact.
	err := b.EditMemoryNode(ctx, "bob", aliceFactID, "malicious")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a fact belonging to this user")
}

// ---------------------------------------------------------------------------
// DeleteMemoryNode
// ---------------------------------------------------------------------------

func TestDeleteMemoryNode_Success(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "content", "my-fact", "HAS_FACT", "fact", nil))

	graph, _ := b.GetUserGraph(ctx, "alice")
	var factID string
	for _, n := range graph.Nodes {
		if n.Type == "fact" {
			factID = n.ID
			break
		}
	}
	require.NotEmpty(t, factID)

	err := b.DeleteMemoryNode(ctx, "alice", factID)
	require.NoError(t, err)
}

func TestDeleteMemoryNode_UserNotFound(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.DeleteMemoryNode(context.Background(), "ghost", "1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestDeleteMemoryNode_InvalidNodeID(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "content", "label", "HAS_FACT", "fact", nil))

	err := b.DeleteMemoryNode(ctx, "alice", "INVALID")
	assert.Error(t, err)
}

func TestDeleteMemoryNode_NotOwned(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()
	require.NoError(t, b.AddKnowledge(ctx, "alice", "content", "label", "HAS_FACT", "fact", nil))
	require.NoError(t, b.AddKnowledge(ctx, "bob", "content", "label2", "HAS_FACT", "fact", nil))

	graph, _ := b.GetUserGraph(ctx, "alice")
	var aliceFactID string
	for _, n := range graph.Nodes {
		if n.Type == "fact" {
			aliceFactID = n.ID
			break
		}
	}
	require.NotEmpty(t, aliceFactID)

	err := b.DeleteMemoryNode(ctx, "bob", aliceFactID)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// SetUserProperty
// ---------------------------------------------------------------------------

func TestSetUserProperty_CreatesAndUpdates(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()

	err := b.SetUserProperty(ctx, "alice", "lang", "en")
	require.NoError(t, err)

	err = b.SetUserProperty(ctx, "alice", "lang", "es")
	require.NoError(t, err)

	b.mu.RLock()
	userNode := b.findUserNodeInGraph(b.graph, "alice")
	b.mu.RUnlock()
	require.NotNil(t, userNode)
	assert.Equal(t, "es", userNode.Properties["lang"])
}

// ---------------------------------------------------------------------------
// UpdateUserLabel
// ---------------------------------------------------------------------------

func TestUpdateUserLabel_EmptyDisplayName_NoOp(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.UpdateUserLabel(context.Background(), "alice", "")
	assert.NoError(t, err)
}

func TestUpdateUserLabel_SetsLabel(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()

	err := b.UpdateUserLabel(ctx, "alice", "Alice Wonderland")
	require.NoError(t, err)

	b.mu.RLock()
	userNode := b.findUserNodeInGraph(b.graph, "alice")
	b.mu.RUnlock()
	require.NotNil(t, userNode)
	assert.Equal(t, "Alice Wonderland", userNode.Label)
}

// ---------------------------------------------------------------------------
// InvalidateMemoryCache — always nil
// ---------------------------------------------------------------------------

func TestInvalidateMemoryCache(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	err := b.InvalidateMemoryCache(context.Background(), "alice")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// QueryGraph — empty query
// ---------------------------------------------------------------------------

func TestQueryGraph_EmptyQuery(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	result, err := b.QueryGraph(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "empty")
}

// ---------------------------------------------------------------------------
// executeC­ypherInMemory — undirected edge match
// ---------------------------------------------------------------------------

func TestExecuteCypherInMemory_UndirectedEdge(t *testing.T) {
	graph := &InMemoryGraph{
		Nodes: map[int]*Node{
			1: {ID: 1, Label: "Alice", Type: "user"},
			2: {ID: 2, Label: "Bob", Type: "user"},
		},
		Edges: []*Edge{
			{Source: 1, Target: 2, Label: "FRIEND_OF"},
		},
	}
	data, err := executeCypherInMemory(graph, "MATCH (a)-[r]-(b) RETURN a, r, b")
	require.NoError(t, err)
	assert.Len(t, data, 1)
}

// ---------------------------------------------------------------------------
// serializeGML + parseGML round-trip
// ---------------------------------------------------------------------------

func TestSerializeParseGML_RoundTrip(t *testing.T) {
	graph := &InMemoryGraph{
		Nodes: map[int]*Node{
			0: {ID: 0, Label: "Alice", Type: "user", Value: "alice123"},
			1: {ID: 1, Label: "cats", Type: "thing", Value: "Alice likes cats", Properties: map[string]string{"source": "manual"}},
		},
		Edges: []*Edge{
			{Source: 0, Target: 1, Label: "LIKES"},
		},
		NextID: 2,
	}

	data := serializeGML(graph)
	assert.Contains(t, string(data), "graph [")
	assert.Contains(t, string(data), "Alice")
	assert.Contains(t, string(data), "LIKES")

	parsed, err := parseGML(data)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.Len(t, parsed.Nodes, 2)
	assert.Len(t, parsed.Edges, 1)
	assert.Equal(t, "LIKES", parsed.Edges[0].Label)
}

func TestParseGML_EmptyContent(t *testing.T) {
	graph, err := parseGML([]byte(""))
	require.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Empty(t, graph.Nodes)
}

func TestParseGML_MissingLabel_FallsBackToNodeID(t *testing.T) {
	// Node without a label should get a generated "node:N" label.
	raw := []byte(`graph [
  node [
    id 5
    type "fact"
    value "some value"
  ]
]
`)
	graph, err := parseGML(raw)
	require.NoError(t, err)
	n, ok := graph.Nodes[5]
	require.True(t, ok)
	assert.Equal(t, "node:5", n.Label)
}

// ---------------------------------------------------------------------------
// InMemoryGraph.Copy
// ---------------------------------------------------------------------------

func TestInMemoryGraph_Copy_IsDeep(t *testing.T) {
	orig := &InMemoryGraph{
		Nodes: map[int]*Node{
			0: {ID: 0, Label: "A", Properties: map[string]string{"k": "v"}},
		},
		Edges:  []*Edge{{Source: 0, Target: 1, Label: "REL"}},
		NextID: 2,
	}

	copy := orig.Copy()

	// Mutate copy and verify original is unchanged.
	copy.Nodes[0].Label = "B"
	copy.Nodes[0].Properties["k"] = "changed"
	copy.Edges[0].Label = "OTHER"

	assert.Equal(t, "A", orig.Nodes[0].Label)
	assert.Equal(t, "v", orig.Nodes[0].Properties["k"])
	assert.Equal(t, "REL", orig.Edges[0].Label)
}

// ---------------------------------------------------------------------------
// responseNodeID
// ---------------------------------------------------------------------------

func TestResponseNodeID(t *testing.T) {
	assert.Equal(t, "user:u123", responseNodeID(&Node{Type: "user", Value: "u123"}))
	assert.Equal(t, "42", responseNodeID(&Node{ID: 42, Type: "fact"}))
	assert.Equal(t, "", responseNodeID(nil))
}

// ---------------------------------------------------------------------------
// nodeToRow / edgeToRow
// ---------------------------------------------------------------------------

func TestNodeToRow(t *testing.T) {
	n := &Node{ID: 1, Label: "label", Type: "fact", Value: "val", Properties: map[string]string{"a": "b"}}
	row := nodeToRow(n)
	assert.Equal(t, "1", row["id"])
	assert.Equal(t, "label", row["label"])
	assert.Equal(t, "fact", row["type"])
	assert.Equal(t, "val", row["value"])
	props := row["properties"].(map[string]interface{})
	assert.Equal(t, "b", props["a"])
}

func TestNodeToRow_NilProperties(t *testing.T) {
	n := &Node{ID: 2, Label: "l", Type: "fact", Value: "v"}
	row := nodeToRow(n)
	assert.NotNil(t, row["properties"])
}

func TestEdgeToRow(t *testing.T) {
	e := &Edge{Source: 1, Target: 2, Label: "KNOWS"}
	row := edgeToRow(e)
	assert.Equal(t, "1", row["source"])
	assert.Equal(t, "2", row["target"])
	assert.Equal(t, "KNOWS", row["label"])
}

// ---------------------------------------------------------------------------
// parseReturnList
// ---------------------------------------------------------------------------

func TestParseReturnList(t *testing.T) {
	out := parseReturnList("a, b, c")
	assert.Equal(t, []string{"a", "b", "c"}, out)
}

func TestParseReturnList_Single(t *testing.T) {
	out := parseReturnList("n")
	assert.Equal(t, []string{"n"}, out)
}

// ---------------------------------------------------------------------------
// SearchSimilar
// ---------------------------------------------------------------------------

func TestSearchSimilar_FindsMatchingValue(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()

	require.NoError(t, b.AddKnowledge(ctx, "alice", "Alice likes pizza", "pizza", "LIKES", "thing", nil))
	require.NoError(t, b.AddKnowledge(ctx, "alice", "Alice likes cats", "cats", "LIKES", "thing", nil))

	results, err := b.SearchSimilar(ctx, "pizza", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "pizza")
}

func TestSearchSimilar_LimitRespected(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		require.NoError(t, b.AddKnowledge(ctx, "alice",
			"Alice likes things", "thing"+string(rune('a'+i)), "LIKES", "thing", nil))
	}

	results, err := b.SearchSimilar(ctx, "Alice likes", 2)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 2)
}

func TestSearchSimilar_UserNodeExcluded(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/g.gml")
	defer b.Close()
	ctx := context.Background()

	// User node value contains the search term.
	require.NoError(t, b.AddKnowledge(ctx, "alice", "alice is a user", "user-content", "HAS_FACT", "fact", nil))

	// "alice" appears in the user node's value, but we exclude user nodes.
	results, err := b.SearchSimilar(ctx, "alice", 10)
	require.NoError(t, err)
	// Should only include the fact node whose Value contains "alice".
	for _, r := range results {
		assert.NotEmpty(t, r.Content)
	}
}
