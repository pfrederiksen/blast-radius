package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// RenderDOT renders the graph in Graphviz DOT format
func RenderDOT(w io.Writer, g *graph.Graph) error {
	fmt.Fprintln(w, "digraph blast_radius {")
	fmt.Fprintln(w, "  rankdir=LR;")
	fmt.Fprintln(w, "  node [shape=box, style=rounded];")
	fmt.Fprintln(w, "")

	// Render nodes
	for _, node := range g.Nodes() {
		label := formatNodeLabel(node)
		nodeID := sanitizeID(node.ID)
		fmt.Fprintf(w, "  %s [label=\"%s\"];\n", nodeID, label)
	}

	fmt.Fprintln(w, "")

	// Render edges
	for _, edge := range g.Edges() {
		fromID := sanitizeID(edge.From)
		toID := sanitizeID(edge.To)
		label := edge.RelationType

		if edge.Evidence.Heuristic {
			label += " (heuristic)"
			fmt.Fprintf(w, "  %s -> %s [label=\"%s\", style=dashed];\n", fromID, toID, label)
		} else {
			fmt.Fprintf(w, "  %s -> %s [label=\"%s\"];\n", fromID, toID, label)
		}
	}

	fmt.Fprintln(w, "}")
	return nil
}

func formatNodeLabel(node *graph.Node) string {
	label := fmt.Sprintf("%s\\n%s", node.Type, node.Name)
	if node.Region != "" {
		label += fmt.Sprintf("\\n(%s)", node.Region)
	}
	return label
}

func sanitizeID(id string) string {
	// Replace characters that are invalid in DOT identifiers
	id = strings.ReplaceAll(id, ":", "_")
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "-", "_")
	return "\"" + id + "\""
}
