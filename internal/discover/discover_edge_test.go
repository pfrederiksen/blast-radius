package discover

import (
	"testing"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// TestParseARN_EdgeCases tests ARN parsing with edge cases
func TestParseARN_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		arn          string
		expectError  bool
		expectedType string
	}{
		{
			name:        "Invalid ARN - too few parts",
			arn:         "arn:aws:service",
			expectError: true,
		},
		{
			name:        "Invalid ARN - not starting with arn",
			arn:         "notarn:aws:service:region:account:resource",
			expectError: true,
		},
		{
			name:         "Valid ALB ARN",
			arn:          "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123",
			expectError:  false,
			expectedType: ResourceTypeLoadBalancer,
		},
		{
			name:         "Valid NLB ARN",
			arn:          "arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/net/my-nlb/def456",
			expectError:  false,
			expectedType: ResourceTypeLoadBalancer,
		},
		{
			name:         "Valid ECS Service ARN",
			arn:          "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-service",
			expectError:  false,
			expectedType: ResourceTypeECSService,
		},
		{
			name:         "Valid Lambda ARN",
			arn:          "arn:aws:lambda:us-east-1:123456789012:function:my-function",
			expectError:  false,
			expectedType: ResourceTypeLambda,
		},
		{
			name:         "Valid RDS Instance ARN",
			arn:          "arn:aws:rds:us-east-1:123456789012:db:my-database",
			expectError:  false,
			expectedType: ResourceTypeRDSInstance,
		},
		{
			name:         "Valid RDS Cluster ARN",
			arn:          "arn:aws:rds:us-east-1:123456789012:cluster:my-cluster",
			expectError:  false,
			expectedType: ResourceTypeRDSCluster,
		},
		{
			name:        "Unsupported service",
			arn:         "arn:aws:s3:us-east-1:123456789012:bucket/my-bucket",
			expectError: true,
		},
		{
			name:         "ECS task ARN (not service) - no type set",
			arn:          "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123",
			expectError:  false,
			expectedType: "", // Type won't be set for unsupported ECS resource types
		},
		{
			name:         "Lambda ARN with alias",
			arn:          "arn:aws:lambda:us-east-1:123456789012:function:my-function:prod",
			expectError:  false,
			expectedType: ResourceTypeLambda,
		},
		{
			name:         "ARN with colons in resource part",
			arn:          "arn:aws:lambda:us-east-1:123456789012:function:my-function:$LATEST",
			expectError:  false,
			expectedType: ResourceTypeLambda,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Discoverer{}
			node, err := d.parseARN(tt.arn)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none for ARN: %s", tt.arn)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for ARN %s: %v", tt.arn, err)
				return
			}

			if node == nil {
				t.Errorf("Expected node but got nil for ARN: %s", tt.arn)
				return
			}

			if tt.expectedType != "" && node.Type != tt.expectedType {
				t.Errorf("Expected type %s but got %s for ARN: %s", tt.expectedType, node.Type, tt.arn)
			}

			// Verify ARN is set correctly
			if node.ARN != tt.arn {
				t.Errorf("Expected ARN %s but got %s", tt.arn, node.ARN)
			}
		})
	}
}

// TestHasHeuristic_EmptyOptions tests heuristic checking with nil options
func TestHasHeuristic_EmptyOptions(t *testing.T) {
	d := &Discoverer{
		opts: &Options{},
	}

	if d.hasHeuristic("any-heuristic") {
		t.Error("Expected false for heuristic with empty options")
	}
}

// TestHasHeuristic_NilSlice tests heuristic checking with nil heuristics slice
func TestHasHeuristic_NilSlice(t *testing.T) {
	d := &Discoverer{
		opts: &Options{
			Heuristics: nil,
		},
	}

	if d.hasHeuristic("any-heuristic") {
		t.Error("Expected false for heuristic with nil heuristics slice")
	}
}

// TestGraphOperations tests basic graph operations
func TestGraphOperations(t *testing.T) {
	g := graph.New()

	// Test adding nodes
	node1 := &graph.Node{
		ID:   "node-1",
		Type: "TestType",
		Name: "Test Node 1",
	}
	g.AddNode(node1)

	if !g.HasNode("node-1") {
		t.Error("Expected graph to have node-1")
	}

	retrievedNode, ok := g.GetNode("node-1")
	if !ok {
		t.Error("Expected to retrieve node-1")
	}
	if retrievedNode.Name != "Test Node 1" {
		t.Errorf("Expected node name 'Test Node 1' but got %q", retrievedNode.Name)
	}

	// Test adding duplicate node (should not duplicate)
	g.AddNode(node1)
	if g.NodeCount() != 1 {
		t.Errorf("Expected 1 node but got %d", g.NodeCount())
	}

	// Test adding edge
	node2 := &graph.Node{
		ID:   "node-2",
		Type: "TestType",
		Name: "Test Node 2",
	}
	g.AddNode(node2)

	edge := &graph.Edge{
		From:         "node-1",
		To:           "node-2",
		RelationType: "test-relation",
	}
	g.AddEdge(edge)

	if g.EdgeCount() != 1 {
		t.Errorf("Expected 1 edge but got %d", g.EdgeCount())
	}

	// Test non-existent node
	if g.HasNode("non-existent") {
		t.Error("Expected graph to not have non-existent node")
	}

	_, ok = g.GetNode("non-existent")
	if ok {
		t.Error("Expected to not retrieve non-existent node")
	}
}
