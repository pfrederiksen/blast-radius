package graph

import (
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := New()
	if g == nil {
		t.Fatal("New() returned nil")
	}
	if g.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestAddNode(t *testing.T) {
	g := New()
	node := &Node{
		ID:   "test-1",
		Type: "TestResource",
		Name: "test-resource",
	}

	g.AddNode(node)

	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}

	retrieved, ok := g.GetNode("test-1")
	if !ok {
		t.Fatal("node not found")
	}
	if retrieved.ID != node.ID {
		t.Errorf("expected ID %s, got %s", node.ID, retrieved.ID)
	}
}

func TestAddEdge(t *testing.T) {
	g := New()
	edge := &Edge{
		From:         "node-1",
		To:           "node-2",
		RelationType: "depends-on",
	}

	g.AddEdge(edge)

	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}

	edges := g.Edges()
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].From != "node-1" {
		t.Errorf("expected From=node-1, got %s", edges[0].From)
	}
}

func TestEdgesFrom(t *testing.T) {
	g := New()
	g.AddEdge(&Edge{From: "A", To: "B", RelationType: "uses"})
	g.AddEdge(&Edge{From: "A", To: "C", RelationType: "uses"})
	g.AddEdge(&Edge{From: "B", To: "C", RelationType: "uses"})

	edges := g.EdgesFrom("A")
	if len(edges) != 2 {
		t.Errorf("expected 2 edges from A, got %d", len(edges))
	}

	edges = g.EdgesFrom("B")
	if len(edges) != 1 {
		t.Errorf("expected 1 edge from B, got %d", len(edges))
	}

	edges = g.EdgesFrom("C")
	if len(edges) != 0 {
		t.Errorf("expected 0 edges from C, got %d", len(edges))
	}
}

func TestEdgesTo(t *testing.T) {
	g := New()
	g.AddEdge(&Edge{From: "A", To: "C", RelationType: "uses"})
	g.AddEdge(&Edge{From: "B", To: "C", RelationType: "uses"})

	edges := g.EdgesTo("C")
	if len(edges) != 2 {
		t.Errorf("expected 2 edges to C, got %d", len(edges))
	}

	edges = g.EdgesTo("A")
	if len(edges) != 0 {
		t.Errorf("expected 0 edges to A, got %d", len(edges))
	}
}

func TestHasNode(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "test-1"})

	if !g.HasNode("test-1") {
		t.Error("expected HasNode to return true for test-1")
	}
	if g.HasNode("test-2") {
		t.Error("expected HasNode to return false for test-2")
	}
}
