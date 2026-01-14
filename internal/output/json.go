package output

import (
	"encoding/json"
	"io"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// GraphJSON represents the graph in JSON format
type GraphJSON struct {
	Nodes []*graph.Node `json:"nodes"`
	Edges []*graph.Edge `json:"edges"`
}

// RenderJSON renders the graph as JSON
func RenderJSON(w io.Writer, g *graph.Graph) error {
	output := GraphJSON{
		Nodes: g.Nodes(),
		Edges: g.Edges(),
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
