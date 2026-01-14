package discover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pfrederiksen/blast-radius/internal/awsx"
	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// Options configures the discovery process
type Options struct {
	MaxDepth   int
	MaxNodes   int
	Heuristics []string
}

// Discoverer orchestrates resource discovery
type Discoverer struct {
	clients *awsx.Clients
	opts    *Options
}

// New creates a new Discoverer
func New(clients *awsx.Clients, opts *Options) *Discoverer {
	return &Discoverer{
		clients: clients,
		opts:    opts,
	}
}

// Discover starts the discovery process from a resource identifier
func (d *Discoverer) Discover(ctx context.Context, resourceID string, g *graph.Graph) error {
	slog.Debug("Starting discovery", "resourceID", resourceID)

	// Parse resource identifier to determine type
	startNode, err := d.identifyResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("failed to identify resource: %w", err)
	}

	g.AddNode(startNode)
	slog.Info("Identified starting resource",
		"type", startNode.Type,
		"id", startNode.ID,
		"name", startNode.Name)

	// BFS traversal
	visited := make(map[string]bool)
	queue := []string{startNode.ID}
	visited[startNode.ID] = true
	currentDepth := 0

	for len(queue) > 0 && currentDepth <= d.opts.MaxDepth {
		levelSize := len(queue)
		slog.Debug("Processing BFS level",
			"depth", currentDepth,
			"queueSize", levelSize,
			"totalNodes", g.NodeCount())

		for i := 0; i < levelSize; i++ {
			if g.NodeCount() >= d.opts.MaxNodes {
				slog.Warn("Reached max nodes limit", "maxNodes", d.opts.MaxNodes)
				return nil
			}

			nodeID := queue[0]
			queue = queue[1:]

			node, ok := g.GetNode(nodeID)
			if !ok {
				continue
			}

			// Discover dependencies for this node
			neighbors, err := d.discoverNode(ctx, node, g)
			if err != nil {
				slog.Warn("Discovery error for node",
					"nodeID", nodeID,
					"error", err)
				// Continue despite errors
			}

			// Add new neighbors to queue
			for _, neighborID := range neighbors {
				if !visited[neighborID] {
					visited[neighborID] = true
					queue = append(queue, neighborID)
				}
			}
		}

		currentDepth++
	}

	slog.Info("Discovery complete",
		"finalDepth", currentDepth,
		"nodes", g.NodeCount(),
		"edges", g.EdgeCount())

	return nil
}

// identifyResource determines the resource type and creates initial node
func (d *Discoverer) identifyResource(ctx context.Context, resourceID string) (*graph.Node, error) {
	// Check if it's an ARN
	if strings.HasPrefix(resourceID, "arn:") {
		return d.parseARN(resourceID)
	}

	// Try to resolve as a friendly name
	// For MVP, try common patterns

	// Try as load balancer name
	if node, err := d.resolveLoadBalancerByName(ctx, resourceID); err == nil {
		return node, nil
	}

	// Try as ECS service (format: cluster/service)
	if strings.Contains(resourceID, "/") {
		parts := strings.Split(resourceID, "/")
		if len(parts) == 2 {
			if node, err := d.resolveECSService(ctx, parts[0], parts[1]); err == nil {
				return node, nil
			}
		}
	}

	// Try as Lambda function name
	if node, err := d.resolveLambdaFunction(ctx, resourceID); err == nil {
		return node, nil
	}

	// Try as RDS instance identifier
	if node, err := d.resolveRDSInstance(ctx, resourceID); err == nil {
		return node, nil
	}

	return nil, fmt.Errorf("unable to identify resource: %s", resourceID)
}

// discoverNode discovers dependencies for a specific node
func (d *Discoverer) discoverNode(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering dependencies", "nodeType", node.Type, "nodeID", node.ID)

	switch node.Type {
	case "LoadBalancer":
		return d.discoverLoadBalancer(ctx, node, g)
	case "ECSService":
		return d.discoverECSService(ctx, node, g)
	case "Lambda":
		return d.discoverLambda(ctx, node, g)
	case "RDSInstance", "RDSCluster":
		return d.discoverRDS(ctx, node, g)
	default:
		slog.Debug("No discovery handler for node type", "type", node.Type)
		return nil, nil
	}
}

// parseARN parses an ARN and creates a node
func (d *Discoverer) parseARN(arn string) (*graph.Node, error) {
	// ARN format: arn:partition:service:region:account:resource-type/resource-id
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid ARN format: %s", arn)
	}

	service := parts[2]
	region := parts[3]
	account := parts[4]
	resource := strings.Join(parts[5:], ":")

	node := &graph.Node{
		ID:       arn,
		ARN:      arn,
		Region:   region,
		Account:  account,
		Metadata: make(map[string]any),
	}

	// Determine type from service and resource
	switch service {
	case "elasticloadbalancing":
		node.Type = "LoadBalancer"
		if strings.Contains(resource, "/") {
			parts := strings.Split(resource, "/")
			if len(parts) >= 2 {
				node.Name = parts[len(parts)-2]
			}
		}
	case "ecs":
		if strings.HasPrefix(resource, "service/") {
			node.Type = "ECSService"
			parts := strings.Split(resource, "/")
			if len(parts) >= 3 {
				node.Name = parts[len(parts)-1]
				node.Metadata["cluster"] = parts[1]
			}
		}
	case "lambda":
		node.Type = "Lambda"
		if strings.HasPrefix(resource, "function:") {
			node.Name = strings.TrimPrefix(resource, "function:")
		}
	case "rds":
		if strings.HasPrefix(resource, "db:") {
			node.Type = "RDSInstance"
			node.Name = strings.TrimPrefix(resource, "db:")
		} else if strings.HasPrefix(resource, "cluster:") {
			node.Type = "RDSCluster"
			node.Name = strings.TrimPrefix(resource, "cluster:")
		}
	default:
		return nil, fmt.Errorf("unsupported service in ARN: %s", service)
	}

	return node, nil
}

// Placeholder resolution functions (will be implemented in discovery modules)
func (d *Discoverer) resolveECSService(ctx context.Context, cluster, service string) (*graph.Node, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (d *Discoverer) resolveLambdaFunction(ctx context.Context, name string) (*graph.Node, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func (d *Discoverer) resolveRDSInstance(ctx context.Context, identifier string) (*graph.Node, error) {
	return nil, fmt.Errorf("not implemented yet")
}

// Placeholder discovery functions (will be implemented in separate files)
func (d *Discoverer) discoverECSService(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	return nil, nil
}

func (d *Discoverer) discoverLambda(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	return nil, nil
}

func (d *Discoverer) discoverRDS(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	return nil, nil
}
