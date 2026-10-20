package codegraph_test

import (
	"encoding/json"
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/codegraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExport_ToMermaid(t *testing.T) {
	graph := buildGraphForFixture(t, "fixturemod")
	require.NotNil(t, graph)

	mermaid := graph.ToMermaid(
		codegraph.WithNodeKinds(codegraph.NodeKindPackage),
		codegraph.WithEdgeKinds(codegraph.EdgeKindImports),
		codegraph.WithTitle("Package Imports"),
	)

	assert.Contains(t, mermaid, "title: Package Imports")
	assert.Contains(t, mermaid, "graph TD")
	assert.Contains(t, mermaid, "example.com/fixturemod")
	assert.Contains(t, mermaid, "-->|imports|")
}

func TestExport_ToDOT(t *testing.T) {
	graph := buildGraphForFixture(t, "fixturemod")
	require.NotNil(t, graph)

	dot := graph.ToDOT(
		codegraph.WithNodeKinds(codegraph.NodeKindPackage),
		codegraph.WithEdgeKinds(codegraph.EdgeKindImports),
		codegraph.WithTitle("Package Graph"),
	)

	assert.Contains(t, dot, "digraph G {")
	assert.Contains(t, dot, "label=\"Package Graph\"")
	assert.Contains(t, dot, "package:example.com/fixturemod")
	assert.Contains(t, dot, "shape=box3d")
	assert.Contains(t, dot, "->")
}

func TestExport_ToJSON(t *testing.T) {
	graph := buildGraphForFixture(t, "typeinfofixture", archscout.WithTypeInfo())
	require.NotNil(t, graph)

	jsonStr := graph.ToJSONString(
		codegraph.WithNodeKinds(codegraph.NodeKindPackage, codegraph.NodeKindFunction),
		codegraph.WithEdgeKinds(codegraph.EdgeKindCalls),
	)

	require.NotEmpty(t, jsonStr)

	var parsed struct {
		Nodes []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"nodes"`
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Kind   string `json:"kind"`
		} `json:"edges"`
	}

	err := json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)

	assert.NotEmpty(t, parsed.Nodes)
	assert.NotEmpty(t, parsed.Edges)

	hasCalls := false
	for _, e := range parsed.Edges {
		if e.Kind == "calls" {
			hasCalls = true
			break
		}
	}
	assert.True(t, hasCalls, "expected calls edges in JSON export")
}
