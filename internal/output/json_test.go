package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

func TestRenderJSON(t *testing.T) {
	g := graph.New()

	node1 := &graph.Node{
		ID:      "node-1",
		Type:    "LoadBalancer",
		ARN:     "arn:aws:elb:us-east-1:123456789012:loadbalancer/test",
		Name:    "test-lb",
		Region:  "us-east-1",
		Account: "123456789012",
		Tags: map[string]string{
			"Environment": "test",
		},
		Metadata: map[string]any{
			"dnsName": "test-lb.elb.amazonaws.com",
		},
	}

	node2 := &graph.Node{
		ID:      "node-2",
		Type:    "TargetGroup",
		Name:    "test-tg",
		Region:  "us-east-1",
		Account: "123456789012",
	}

	g.AddNode(node1)
	g.AddNode(node2)

	g.AddEdge(&graph.Edge{
		From:         node1.ID,
		To:           node2.ID,
		RelationType: "forwards-to",
		Evidence: graph.Evidence{
			APICall: "DescribeTargetGroups",
			Fields: map[string]any{
				"TargetGroupArn": "arn:aws:elb:us-east-1:123456789012:targetgroup/test",
			},
		},
	})

	var buf bytes.Buffer
	err := RenderJSON(&buf, g)
	if err != nil {
		t.Fatalf("RenderJSON() error = %v", err)
	}

	// Parse JSON to verify it's valid
	var result GraphJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("RenderJSON() produced invalid JSON: %v", err)
	}

	// Verify structure
	if len(result.Nodes) != 2 {
		t.Errorf("RenderJSON() expected 2 nodes, got %d", len(result.Nodes))
	}

	if len(result.Edges) != 1 {
		t.Errorf("RenderJSON() expected 1 edge, got %d", len(result.Edges))
	}

	// Verify first node
	if result.Nodes[0].Type != "LoadBalancer" && result.Nodes[1].Type != "LoadBalancer" {
		t.Error("RenderJSON() missing LoadBalancer node")
	}

	// Verify edge
	if result.Edges[0].RelationType != "forwards-to" {
		t.Errorf("RenderJSON() edge RelationType = %v, want forwards-to", result.Edges[0].RelationType)
	}
}
