package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

func TestRenderTree(t *testing.T) {
	g := graph.New()

	// Build a sample graph
	rootNode := &graph.Node{
		ID:      "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-alb/abc123",
		Type:    "LoadBalancer",
		ARN:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-alb/abc123",
		Name:    "test-alb",
		Region:  "us-east-1",
		Account: "123456789012",
		Metadata: map[string]any{
			"dnsName": "test-alb-123456789.us-east-1.elb.amazonaws.com",
		},
	}

	listenerNode := &graph.Node{
		ID:      "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/test-alb/abc123/def456",
		Type:    "Listener",
		ARN:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/test-alb/abc123/def456",
		Name:    "HTTPS:443",
		Region:  "us-east-1",
		Account: "123456789012",
	}

	tgNode := &graph.Node{
		ID:      "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/test-tg/ghi789",
		Type:    "TargetGroup",
		ARN:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/test-tg/ghi789",
		Name:    "test-tg",
		Region:  "us-east-1",
		Account: "123456789012",
	}

	g.AddNode(rootNode)
	g.AddNode(listenerNode)
	g.AddNode(tgNode)

	g.AddEdge(&graph.Edge{
		From:         rootNode.ID,
		To:           listenerNode.ID,
		RelationType: "has-listener",
	})

	g.AddEdge(&graph.Edge{
		From:         listenerNode.ID,
		To:           tgNode.ID,
		RelationType: "forwards-to",
	})

	var buf bytes.Buffer
	err := RenderTree(&buf, g, rootNode.ID)
	if err != nil {
		t.Fatalf("RenderTree() error = %v", err)
	}

	output := buf.String()

	// Check that output contains expected elements
	expectedStrings := []string{
		"[Level 0] Root",
		"LoadBalancer: test-alb",
		"[Level 1] Direct Dependencies",
		"Listener: HTTPS:443",
		"[Level 2] Transitive Dependencies",
		"TargetGroup: test-tg",
		"Summary:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("RenderTree() output missing expected string: %q\nGot:\n%s", expected, output)
		}
	}
}

func TestRenderTreeNonexistentStart(t *testing.T) {
	g := graph.New()
	g.AddNode(&graph.Node{ID: "test-1", Type: "Test", Name: "test"})

	var buf bytes.Buffer
	err := RenderTree(&buf, g, "nonexistent")
	if err == nil {
		t.Error("RenderTree() expected error for nonexistent start node, got nil")
	}
}
