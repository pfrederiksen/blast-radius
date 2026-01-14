package graph

// BFSLevel represents nodes at a specific depth level
type BFSLevel struct {
	Depth int
	Nodes []*Node
}

// BFS performs breadth-first traversal from a starting node
func (g *Graph) BFS(startID string) []BFSLevel {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[startID]; !ok {
		return nil
	}

	visited := make(map[string]bool)
	levels := make([]BFSLevel, 0)
	queue := []string{startID}
	visited[startID] = true
	currentDepth := 0

	for len(queue) > 0 {
		levelSize := len(queue)
		level := BFSLevel{
			Depth: currentDepth,
			Nodes: make([]*Node, 0),
		}

		for i := 0; i < levelSize; i++ {
			nodeID := queue[0]
			queue = queue[1:]

			node := g.nodes[nodeID]
			level.Nodes = append(level.Nodes, node)

			// Find all neighbors (nodes connected by outgoing edges)
			for _, edge := range g.edges {
				if edge.From == nodeID && !visited[edge.To] {
					visited[edge.To] = true
					queue = append(queue, edge.To)
				}
			}
		}

		levels = append(levels, level)
		currentDepth++
	}

	return levels
}
