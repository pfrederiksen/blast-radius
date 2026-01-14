package output

import (
	"fmt"
	"io"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// RenderTree renders the graph as a tree structure
func RenderTree(w io.Writer, g *graph.Graph, startID string) error {
	levels := g.BFS(startID)
	if len(levels) == 0 {
		return fmt.Errorf("starting node not found: %s", startID)
	}

	for _, level := range levels {
		fmt.Fprintf(w, "\n[Level %d] ", level.Depth)
		if level.Depth == 0 {
			fmt.Fprintf(w, "Root\n")
		} else if level.Depth == 1 {
			fmt.Fprintf(w, "Direct Dependencies\n")
		} else {
			fmt.Fprintf(w, "Transitive Dependencies\n")
		}

		for i, node := range level.Nodes {
			prefix := "└─"
			if i < len(level.Nodes)-1 {
				prefix = "├─"
			}

			// Find incoming edges to show relationship
			edges := g.EdgesTo(node.ID)
			relType := ""
			if len(edges) > 0 {
				relType = fmt.Sprintf(" [%s]", edges[0].RelationType)
			}

			fmt.Fprintf(w, "%s %s: %s%s\n",
				prefix,
				node.Type,
				node.Name,
				relType)

			// Show ARN if different from name
			if node.ARN != "" && node.ARN != node.ID {
				fmt.Fprintf(w, "   ARN: %s\n", node.ARN)
			}

			// Show metadata if present
			if len(node.Metadata) > 0 {
				for k, v := range node.Metadata {
					fmt.Fprintf(w, "   %s: %v\n", k, v)
				}
			}
		}
	}

	fmt.Fprintf(w, "\nSummary: %d nodes, %d edges\n", g.NodeCount(), g.EdgeCount())
	return nil
}
