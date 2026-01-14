package graph

import (
	"sync"
)

// Node represents a resource in the dependency graph
type Node struct {
	ID       string            // Unique identifier (ARN or ID)
	Type     string            // Resource type (LoadBalancer, ECSService, etc.)
	ARN      string            // Full ARN if available
	Name     string            // Human-readable name
	Region   string            // AWS region
	Account  string            // AWS account ID
	Tags     map[string]string // Resource tags
	Metadata map[string]any    // Additional metadata
}

// Edge represents a relationship between two resources
type Edge struct {
	From         string   // Source node ID
	To           string   // Target node ID
	RelationType string   // Type of relationship (forward, uses, member-of, etc.)
	Evidence     Evidence // How this relationship was discovered
}

// Evidence tracks how a relationship was discovered
type Evidence struct {
	APICall   string         // AWS API call that revealed this relationship
	Fields    map[string]any // Key fields from the API response
	Heuristic bool           // Whether this was discovered via heuristic
}

// Graph represents the complete dependency graph
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]*Node // Node ID -> Node
	edges []*Edge          // All edges
}

// New creates a new empty graph
func New() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
		edges: make([]*Edge, 0),
	}
}

// AddNode adds or updates a node in the graph
func (g *Graph) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[node.ID] = node
}

// AddEdge adds an edge to the graph
func (g *Graph) AddEdge(edge *Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.edges = append(g.edges, edge)
}

// GetNode retrieves a node by ID
func (g *Graph) GetNode(id string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, ok := g.nodes[id]
	return node, ok
}

// HasNode checks if a node exists in the graph
func (g *Graph) HasNode(id string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.nodes[id]
	return ok
}

// Nodes returns all nodes in the graph
func (g *Graph) Nodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]*Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// Edges returns all edges in the graph
func (g *Graph) Edges() []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges := make([]*Edge, len(g.edges))
	copy(edges, g.edges)
	return edges
}

// EdgesFrom returns all edges originating from a node
func (g *Graph) EdgesFrom(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*Edge
	for _, edge := range g.edges {
		if edge.From == nodeID {
			result = append(result, edge)
		}
	}
	return result
}

// EdgesTo returns all edges pointing to a node
func (g *Graph) EdgesTo(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*Edge
	for _, edge := range g.edges {
		if edge.To == nodeID {
			result = append(result, edge)
		}
	}
	return result
}

// NodeCount returns the number of nodes
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// EdgeCount returns the number of edges
func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.edges)
}
