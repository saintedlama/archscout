package implementsgraph_test

import (
	"sort"
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/internaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild_FindsImplementersOfInterface(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())
	graph := archscout.BuildImplementsGraph(ws)

	got := graph.Implementers("example.com/typeinfofixture/api.Greeter")
	sort.Strings(got)

	assert.Equal(t,
		[]string{
			"example.com/typeinfofixture.PointerGreeter",
			"example.com/typeinfofixture.defaultGreeter",
		},
		got,
		"both value-receiver and pointer-only implementers should be reported",
	)
}

func TestBuild_FindsInterfacesSatisfiedByType(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())
	graph := archscout.BuildImplementsGraph(ws)

	got := graph.Interfaces("example.com/typeinfofixture.PointerGreeter")
	assert.Contains(t, got, "example.com/typeinfofixture/api.Greeter")
}

func TestBuild_PatternFilterScopesByPackage(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())
	graph := archscout.BuildImplementsGraph(ws)

	got := graph.Implementers(
		"example.com/typeinfofixture/api.Greeter",
		"example.com/typeinfofixture/api/...",
	)
	assert.Empty(t, got)

	got = graph.Implementers(
		"example.com/typeinfofixture/api.Greeter",
		"example.com/typeinfofixture",
	)
	sort.Strings(got)
	assert.Equal(t,
		[]string{
			"example.com/typeinfofixture.PointerGreeter",
			"example.com/typeinfofixture.defaultGreeter",
		},
		got,
	)
}

func TestBuild_SkipsEmptyInterfaces(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())
	graph := archscout.BuildImplementsGraph(ws)

	got := graph.Implementers("any")
	assert.Empty(t, got, "empty interfaces are intentionally not indexed")
}

func TestBuild_ReturnsEmptyGraphWithoutTypeInfo(t *testing.T) {
	// Without WithTypeInfo, ws.TypedPackages() is nil and BuildImplementsGraph returns an empty graph.
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture")
	require.Nil(t, ws.TypedPackages())

	graph := archscout.BuildImplementsGraph(ws)
	assert.Empty(t, graph.Implementers("example.com/typeinfofixture/api.Greeter"))
	assert.Empty(t, graph.Interfaces("example.com/typeinfofixture.PointerGreeter"))
}

func TestBuild_SafeOnNilWorkspace(t *testing.T) {
	graph := archscout.BuildImplementsGraph(nil)
	assert.Empty(t, graph.Implementers("anything"))
	assert.Empty(t, graph.Interfaces("anything"))
}
