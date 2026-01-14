package graph

import (
	"testing"
)

func TestBFS(t *testing.T) {
	g := New()

	// Create a simple graph:
	//     A
	//    / \
	//   B   C
	//   |
	//   D

	g.AddNode(&Node{ID: "A", Name: "Node A"})
	g.AddNode(&Node{ID: "B", Name: "Node B"})
	g.AddNode(&Node{ID: "C", Name: "Node C"})
	g.AddNode(&Node{ID: "D", Name: "Node D"})

	g.AddEdge(&Edge{From: "A", To: "B"})
	g.AddEdge(&Edge{From: "A", To: "C"})
	g.AddEdge(&Edge{From: "B", To: "D"})

	levels := g.BFS("A")

	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}

	// Level 0: A
	if len(levels[0].Nodes) != 1 || levels[0].Nodes[0].ID != "A" {
		t.Errorf("level 0 should contain only A")
	}

	// Level 1: B, C
	if len(levels[1].Nodes) != 2 {
		t.Errorf("level 1 should contain 2 nodes, got %d", len(levels[1].Nodes))
	}

	// Level 2: D
	if len(levels[2].Nodes) != 1 || levels[2].Nodes[0].ID != "D" {
		t.Errorf("level 2 should contain only D")
	}
}

func TestBFSNonexistentStart(t *testing.T) {
	g := New()
	g.AddNode(&Node{ID: "A"})

	levels := g.BFS("nonexistent")
	if levels != nil {
		t.Error("expected nil for nonexistent start node")
	}
}

func TestBFSCycle(t *testing.T) {
	g := New()

	// Create a graph with a cycle:
	//     A -> B -> C
	//     ^---------/

	g.AddNode(&Node{ID: "A", Name: "Node A"})
	g.AddNode(&Node{ID: "B", Name: "Node B"})
	g.AddNode(&Node{ID: "C", Name: "Node C"})

	g.AddEdge(&Edge{From: "A", To: "B"})
	g.AddEdge(&Edge{From: "B", To: "C"})
	g.AddEdge(&Edge{From: "C", To: "A"}) // Cycle

	levels := g.BFS("A")

	// Should still visit each node only once
	totalNodes := 0
	for _, level := range levels {
		totalNodes += len(level.Nodes)
	}

	if totalNodes != 3 {
		t.Errorf("expected to visit 3 nodes exactly once, got %d", totalNodes)
	}
}
