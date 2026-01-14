package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

func TestRenderDOT(t *testing.T) {
	g := graph.New()

	node1 := &graph.Node{
		ID:     "node-1",
		Type:   "LoadBalancer",
		Name:   "test-lb",
		Region: "us-east-1",
	}

	node2 := &graph.Node{
		ID:     "node-2",
		Type:   "TargetGroup",
		Name:   "test-tg",
		Region: "us-east-1",
	}

	g.AddNode(node1)
	g.AddNode(node2)

	g.AddEdge(&graph.Edge{
		From:         node1.ID,
		To:           node2.ID,
		RelationType: "forwards-to",
		Evidence: graph.Evidence{
			Heuristic: false,
		},
	})

	var buf bytes.Buffer
	err := RenderDOT(&buf, g)
	if err != nil {
		t.Fatalf("RenderDOT() error = %v", err)
	}

	output := buf.String()

	// Check DOT structure
	expectedStrings := []string{
		"digraph blast_radius {",
		"rankdir=LR;",
		"LoadBalancer",
		"test-lb",
		"TargetGroup",
		"test-tg",
		"forwards-to",
		"}",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("RenderDOT() output missing expected string: %q\nGot:\n%s", expected, output)
		}
	}
}

func TestRenderDOTHeuristic(t *testing.T) {
	g := graph.New()

	node1 := &graph.Node{
		ID:   "node-1",
		Type: "RDS",
		Name: "test-db",
	}

	node2 := &graph.Node{
		ID:   "node-2",
		Type: "ECSService",
		Name: "test-service",
	}

	g.AddNode(node1)
	g.AddNode(node2)

	g.AddEdge(&graph.Edge{
		From:         node2.ID,
		To:           node1.ID,
		RelationType: "connects-to",
		Evidence: graph.Evidence{
			Heuristic: true,
		},
	})

	var buf bytes.Buffer
	err := RenderDOT(&buf, g)
	if err != nil {
		t.Fatalf("RenderDOT() error = %v", err)
	}

	output := buf.String()

	// Check that heuristic edges are dashed
	if !strings.Contains(output, "style=dashed") {
		t.Error("RenderDOT() heuristic edge should be dashed")
	}
	if !strings.Contains(output, "(heuristic)") {
		t.Error("RenderDOT() heuristic edge should have (heuristic) label")
	}
}
