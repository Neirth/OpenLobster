package file

import (
	"context"
	"testing"
)

func TestExecuteCypherInMemory_MatchNode(t *testing.T) {
	graph := &InMemoryGraph{
		Nodes: map[int]*Node{
			1: {ID: 1, Label: "Alice", Type: "user", Value: "user1"},
			2: {ID: 2, Label: "Valencia", Type: "fact", Value: "User lives in Valencia"},
		},
		Edges: []*Edge{
			{Source: 1, Target: 2, Label: "LIVES_IN"},
		},
	}

	data, err := executeCypherInMemory(graph, "MATCH (n) RETURN n")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 2 {
		t.Errorf("expected 2 rows, got %d", len(data))
	}
}

func TestExecuteCypherInMemory_MatchNodeUser(t *testing.T) {
	graph := &InMemoryGraph{
		Nodes: map[int]*Node{
			1: {ID: 1, Label: "Alice", Type: "user", Value: "user1"},
			2: {ID: 2, Label: "Valencia", Type: "fact", Value: "User lives in Valencia"},
		},
		Edges: []*Edge{},
	}

	data, err := executeCypherInMemory(graph, "MATCH (n:User) RETURN n")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 {
		t.Errorf("expected 1 row (User), got %d", len(data))
	}
	row := data[0]["n"].(map[string]interface{})
	if row["type"] != "user" {
		t.Errorf("expected type user, got %v", row["type"])
	}
}

func TestExecuteCypherInMemory_MatchNodeFact(t *testing.T) {
	graph := &InMemoryGraph{
		Nodes: map[int]*Node{
			1: {ID: 1, Label: "Alice", Type: "user", Value: "user1"},
			2: {ID: 2, Label: "Valencia", Type: "fact", Value: "User lives in Valencia"},
		},
		Edges: []*Edge{},
	}

	data, err := executeCypherInMemory(graph, "MATCH (n:Fact) RETURN n")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 {
		t.Errorf("expected 1 row (Fact), got %d", len(data))
	}
	row := data[0]["n"].(map[string]interface{})
	if row["label"] != "Valencia" {
		t.Errorf("expected label Valencia, got %v", row["label"])
	}
}

func TestExecuteCypherInMemory_MatchEdge(t *testing.T) {
	graph := &InMemoryGraph{
		Nodes: map[int]*Node{
			1: {ID: 1, Label: "Alice", Type: "user", Value: "user1"},
			2: {ID: 2, Label: "Valencia", Type: "fact", Value: "User lives in Valencia"},
		},
		Edges: []*Edge{
			{Source: 1, Target: 2, Label: "LIVES_IN"},
		},
	}

	data, err := executeCypherInMemory(graph, "MATCH (a)-[r]->(b) RETURN a, r, b")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 {
		t.Errorf("expected 1 row (one edge), got %d", len(data))
	}
	row := data[0]
	r := row["r"].(map[string]interface{})
	if r["label"] != "LIVES_IN" {
		t.Errorf("expected edge label LIVES_IN, got %v", r["label"])
	}
}

func TestExecuteCypherInMemory_Unsupported(t *testing.T) {
	graph := &InMemoryGraph{Nodes: map[int]*Node{}, Edges: []*Edge{}}
	_, err := executeCypherInMemory(graph, "CREATE (n) RETURN n")
	if err == nil {
		t.Error("expected error for unsupported pattern")
	}
}

func TestGMLBackend_QueryGraph_Integration(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/test.gml")
	defer b.Close()

	ctx := context.Background()
	if err := b.AddKnowledge(ctx, "user1", "User lives in Madrid", "Madrid", "LIVES_IN", "place", nil); err != nil {
		t.Fatalf("AddKnowledge failed: %v", err)
	}
	if err := b.AddKnowledge(ctx, "user1", "User likes music", "Music", "LIKES", "thing", nil); err != nil {
		t.Fatalf("AddKnowledge failed: %v", err)
	}

	result, err := b.QueryGraph(ctx, "MATCH (n) RETURN n")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors[0])
	}
	// user node + 2 fact nodes
	if len(result.Data) < 3 {
		t.Errorf("expected at least 3 nodes, got %d", len(result.Data))
	}
}
