package codegraph

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// ExportConfig configures the serialization of a CodeGraph.
type ExportConfig struct {
	NodeKinds []NodeKind
	EdgeKinds []EdgeKind
	Direction string // For Mermaid: "TD" (default) or "LR"
	Title     string // Optional diagram title
}

// ExportOption mutates an ExportConfig.
type ExportOption func(*ExportConfig)

// WithNodeKinds filters the exported graph to the specified node kinds.
func WithNodeKinds(kinds ...NodeKind) ExportOption {
	return func(c *ExportConfig) {
		c.NodeKinds = kinds
	}
}

// WithEdgeKinds filters the exported graph to the specified edge kinds.
func WithEdgeKinds(kinds ...EdgeKind) ExportOption {
	return func(c *ExportConfig) {
		c.EdgeKinds = kinds
	}
}

// WithDirection sets the Mermaid diagram layout direction ("TD", "LR", etc.).
func WithDirection(direction string) ExportOption {
	return func(c *ExportConfig) {
		c.Direction = direction
	}
}

// WithTitle sets an optional diagram title.
func WithTitle(title string) ExportOption {
	return func(c *ExportConfig) {
		c.Title = title
	}
}

func applyExportOptions(opts []ExportOption) ExportConfig {
	cfg := ExportConfig{
		Direction: "TD",
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// ToMermaid generates Mermaid diagram syntax representing the graph.
func (g *Graph) ToMermaid(opts ...ExportOption) string {
	if g == nil || len(g.nodes) == 0 {
		return "graph TD\n"
	}

	cfg := applyExportOptions(opts)
	var sb strings.Builder

	if cfg.Title != "" {
		sb.WriteString(fmt.Sprintf("---\ntitle: %s\n---\n", cfg.Title))
	}

	dir := cfg.Direction
	if dir == "" {
		dir = "TD"
	}
	sb.WriteString("graph " + dir + "\n")

	// Filter nodes
	visibleNodes := make(map[string]bool)
	idMap := make(map[string]string)
	counter := 0

	for _, id := range g.sortedNodes {
		n := g.nodes[id]
		if len(cfg.NodeKinds) > 0 && !slices.Contains(cfg.NodeKinds, n.Kind) {
			continue
		}
		visibleNodes[id] = true
		counter++
		nodeVar := fmt.Sprintf("n%d", counter)
		idMap[id] = nodeVar

		label := n.Name
		if n.Kind == NodeKindPackage {
			label = n.PackageID
		} else if (n.Kind == NodeKindType || n.Kind == NodeKindFunction) && n.QName != "" {
			label = n.QName
		}
		label = escapeMermaidLabel(fmt.Sprintf("[%s] %s", n.Kind, label))
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeVar, label))
	}

	// Filter and render edges
	for _, id := range g.sortedNodes {
		if !visibleNodes[id] {
			continue
		}
		for _, edge := range g.outEdges[id] {
			if !visibleNodes[edge.Target] {
				continue
			}
			if len(cfg.EdgeKinds) > 0 && !slices.Contains(cfg.EdgeKinds, edge.Kind) {
				continue
			}

			srcVar := idMap[edge.Source]
			dstVar := idMap[edge.Target]

			switch edge.Kind {
			case EdgeKindImplements:
				sb.WriteString(fmt.Sprintf("    %s -.->|implements| %s\n", srcVar, dstVar))
			case EdgeKindContains:
				sb.WriteString(fmt.Sprintf("    %s -->|contains| %s\n", srcVar, dstVar))
			default:
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", srcVar, edge.Kind, dstVar))
			}
		}
	}

	return sb.String()
}

// ToDOT generates Graphviz DOT language syntax representing the graph.
func (g *Graph) ToDOT(opts ...ExportOption) string {
	if g == nil {
		return "digraph G {\n}\n"
	}

	cfg := applyExportOptions(opts)
	var sb strings.Builder

	sb.WriteString("digraph G {\n")
	sb.WriteString("    rankdir=" + cfg.Direction + ";\n")
	sb.WriteString("    node [fontsize=10, fontname=\"Helvetica\"];\n")
	sb.WriteString("    edge [fontsize=9, fontname=\"Helvetica\"];\n")

	if cfg.Title != "" {
		sb.WriteString(fmt.Sprintf("    label=\"%s\";\n    labelloc=top;\n", escapeDOT(cfg.Title)))
	}

	visibleNodes := make(map[string]bool)
	for _, id := range g.sortedNodes {
		n := g.nodes[id]
		if len(cfg.NodeKinds) > 0 && !slices.Contains(cfg.NodeKinds, n.Kind) {
			continue
		}
		visibleNodes[id] = true

		shape := "box"
		switch n.Kind {
		case NodeKindModule:
			shape = "component"
		case NodeKindPackage:
			shape = "box3d"
		case NodeKindFile:
			shape = "note"
		case NodeKindType:
			shape = "oval"
		case NodeKindFunction:
			shape = "ellipse"
		}

		label := n.Name
		if n.Kind == NodeKindPackage {
			label = n.PackageID
		} else if (n.Kind == NodeKindType || n.Kind == NodeKindFunction) && n.QName != "" {
			label = n.QName
		}

		color := "black"
		if !n.WithinWorkspace {
			color = "gray60"
		}

		sb.WriteString(fmt.Sprintf("    \"%s\" [label=\"[%s]\\n%s\", shape=%s, color=%s];\n",
			escapeDOT(id), n.Kind, escapeDOT(label), shape, color))
	}

	for _, id := range g.sortedNodes {
		if !visibleNodes[id] {
			continue
		}
		for _, edge := range g.outEdges[id] {
			if !visibleNodes[edge.Target] {
				continue
			}
			if len(cfg.EdgeKinds) > 0 && !slices.Contains(cfg.EdgeKinds, edge.Kind) {
				continue
			}

			style := "solid"
			color := "black"
			if edge.Kind == EdgeKindImplements {
				style = "dashed"
			} else if edge.Kind == EdgeKindContains {
				color = "gray50"
			}

			sb.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\" [label=\"%s\", style=%s, color=%s];\n",
				escapeDOT(edge.Source), escapeDOT(edge.Target), edge.Kind, style, color))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

// JSONGraph represents the serializable structure of a CodeGraph.
type JSONGraph struct {
	Nodes []JSONNode `json:"nodes"`
	Edges []JSONEdge `json:"edges"`
}

// JSONNode represents a serializable graph node.
type JSONNode struct {
	ID              string   `json:"id"`
	Kind            NodeKind `json:"kind"`
	Name            string   `json:"name"`
	QName           string   `json:"qname,omitempty"`
	PackageID       string   `json:"packageId,omitempty"`
	ParentID        string   `json:"parentId,omitempty"`
	WithinWorkspace bool     `json:"withinWorkspace"`
	Ref             *JSONRef `json:"ref,omitempty"`
}

// JSONRef represents a source location reference.
type JSONRef struct {
	Filename string `json:"filename,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

// JSONEdge represents a serializable directed relationship.
type JSONEdge struct {
	Source string   `json:"source"`
	Target string   `json:"target"`
	Kind   EdgeKind `json:"kind"`
	Ref    *JSONRef `json:"ref,omitempty"`
}

// ToJSON serializes the graph to JSON bytes, optionally indented.
func (g *Graph) ToJSON(opts ...ExportOption) ([]byte, error) {
	jg := g.toJSONGraph(opts...)
	return json.MarshalIndent(jg, "", "  ")
}

// ToJSONString serializes the graph to a JSON string.
func (g *Graph) ToJSONString(opts ...ExportOption) string {
	b, err := g.ToJSON(opts...)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (g *Graph) toJSONGraph(opts ...ExportOption) JSONGraph {
	if g == nil {
		return JSONGraph{}
	}

	cfg := applyExportOptions(opts)
	visibleNodes := make(map[string]bool)

	var jnodes []JSONNode
	for _, id := range g.sortedNodes {
		n := g.nodes[id]
		if len(cfg.NodeKinds) > 0 && !slices.Contains(cfg.NodeKinds, n.Kind) {
			continue
		}
		visibleNodes[id] = true

		var jref *JSONRef
		if n.Ref.Filename != "" {
			jref = &JSONRef{
				Filename: n.Ref.Filename,
				Line:     n.Ref.Line,
				Column:   n.Ref.Column,
			}
		}

		jnodes = append(jnodes, JSONNode{
			ID:              n.ID,
			Kind:            n.Kind,
			Name:            n.Name,
			QName:           n.QName,
			PackageID:       n.PackageID,
			ParentID:        n.ParentID,
			WithinWorkspace: n.WithinWorkspace,
			Ref:             jref,
		})
	}

	var jedges []JSONEdge
	for _, id := range g.sortedNodes {
		if !visibleNodes[id] {
			continue
		}
		for _, edge := range g.outEdges[id] {
			if !visibleNodes[edge.Target] {
				continue
			}
			if len(cfg.EdgeKinds) > 0 && !slices.Contains(cfg.EdgeKinds, edge.Kind) {
				continue
			}

			var jref *JSONRef
			if edge.Ref.Filename != "" {
				jref = &JSONRef{
					Filename: edge.Ref.Filename,
					Line:     edge.Ref.Line,
					Column:   edge.Ref.Column,
				}
			}

			jedges = append(jedges, JSONEdge{
				Source: edge.Source,
				Target: edge.Target,
				Kind:   edge.Kind,
				Ref:    jref,
			})
		}
	}

	return JSONGraph{
		Nodes: jnodes,
		Edges: jedges,
	}
}

func escapeMermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "\"", "#quot;")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func escapeDOT(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
