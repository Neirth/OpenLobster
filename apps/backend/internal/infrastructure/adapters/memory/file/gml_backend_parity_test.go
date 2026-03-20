package file

import (
	"context"
	"testing"
)

func TestGML_GetUserGraph_MissingUser_ReturnsSyntheticUserNode(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/graph.gml")
	defer b.Close()

	graph, err := b.GetUserGraph(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(graph.Nodes))
	}
	if graph.Nodes[0].ID != "user:alice" {
		t.Fatalf("expected synthetic user id user:alice, got %q", graph.Nodes[0].ID)
	}
}

func TestGML_GetUserGraph_UserEdge_UsesSyntheticUserID(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/graph.gml")
	defer b.Close()
	ctx := context.Background()
	if err := b.AddKnowledge(ctx, "alice", "likes pizza", "pizza", "LIKES", "thing", nil); err != nil {
		t.Fatal(err)
	}

	graph, err := b.GetUserGraph(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Edges) == 0 {
		t.Fatalf("expected at least one edge")
	}
	if graph.Edges[0].Source != "user:alice" {
		t.Fatalf("expected source user:alice, got %q", graph.Edges[0].Source)
	}
}

func TestGML_AddRelation_DeduplicatesSameEdge(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/graph.gml")
	defer b.Close()
	ctx := context.Background()

	if err := b.AddRelation(ctx, "alice", "bob", "FRIEND_OF"); err != nil {
		t.Fatal(err)
	}
	if err := b.AddRelation(ctx, "alice", "bob", "FRIEND_OF"); err != nil {
		t.Fatal(err)
	}

	b.mu.RLock()
	defer b.mu.RUnlock()
	count := 0
	for _, e := range b.graph.Edges {
		if e.Label == "FRIEND_OF" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 FRIEND_OF edge, got %d", count)
	}
}

func TestGML_DeleteNode_UserPrefix_DeletesUserNode(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/graph.gml")
	defer b.Close()
	ctx := context.Background()
	if err := b.AddKnowledge(ctx, "alice", "likes pizza", "pizza", "LIKES", "thing", nil); err != nil {
		t.Fatal(err)
	}

	if err := b.DeleteNode(ctx, "user:alice"); err != nil {
		t.Fatal(err)
	}

	graph, err := b.GetUserGraph(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range graph.Nodes {
		if n.ID == "user:alice" {
			t.Fatalf("expected alice user node to be deleted")
		}
	}
}

func TestGML_AddKnowledge_DoesNotOverwriteEdgesForSameLabel(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/graph.gml")
	defer b.Close()

	ctx := context.Background()

	if err := b.AddKnowledge(ctx, "sergio", "Sergio was born in Elche", "Elche", "BORN_IN", "place", nil); err != nil {
		t.Fatal(err)
	}
	if err := b.AddKnowledge(ctx, "sergio", "Sergio lives in Elche", "Elche", "LIVES_IN", "place", nil); err != nil {
		t.Fatal(err)
	}

	graph, err := b.GetUserGraph(ctx, "sergio")
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	for _, e := range graph.Edges {
		if e.Source == "user:sergio" {
			counts[e.Label]++
		}
	}

	if counts["BORN_IN"] != 1 {
		t.Fatalf("expected 1 BORN_IN edge, got %d", counts["BORN_IN"])
	}
	if counts["LIVES_IN"] != 1 {
		t.Fatalf("expected 1 LIVES_IN edge, got %d", counts["LIVES_IN"])
	}
}

func TestGML_AddKnowledge_DoesNotOverwriteCrossUserFactContent(t *testing.T) {
	b := NewGMLBackend(t.TempDir() + "/graph.gml")
	defer b.Close()

	ctx := context.Background()

	aliceContent := "Alice was born in Elche"
	bobContent := "Bob was born in Elche"

	if err := b.AddKnowledge(ctx, "alice", aliceContent, "Elche", "BORN_IN", "place", nil); err != nil {
		t.Fatal(err)
	}
	if err := b.AddKnowledge(ctx, "bob", bobContent, "Elche", "BORN_IN", "place", nil); err != nil {
		t.Fatal(err)
	}

	graphAlice, err := b.GetUserGraph(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, n := range graphAlice.Nodes {
		if n.Type == "place" && n.Label == "Elche" {
			found = true
			if n.Value != aliceContent {
				t.Fatalf("expected alice place content %q, got %q", aliceContent, n.Value)
			}
		}
	}
	if !found {
		t.Fatalf("expected to find alice's Elche place node")
	}
}

