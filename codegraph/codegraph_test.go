package codegraph_test

import (
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/codegraph"
	"github.com/saintedlama/archscout/internaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildGraphForFixture(t *testing.T, fixture string, opts ...archscout.LoadWorkspaceOption) *codegraph.Graph {
	t.Helper()
	ws := internaltest.LoadFixtureWorkspace(t, fixture, opts...)
	var implGraph = archscout.BuildImplementsGraph(ws)
	return codegraph.Build(codegraph.Input{
		ModuleRoot:    ws.ModuleRoot(),
		Packages:      ws.Packages,
		Files:         ws.Files,
		Types:         ws.Types,
		Functions:     ws.Functions,
		FunctionCalls: ws.FunctionCalls,
		Dependencies:  ws.Dependencies,
		Implements:    implGraph,
	})
}

func TestBuild_NodeHierarchy(t *testing.T) {
	graph := buildGraphForFixture(t, "fixturemod")
	require.NotNil(t, graph)

	// Check Module Node
	modNode, ok := graph.Node(codegraph.ModuleNodeID("example.com/fixturemod"))
	require.True(t, ok, "module node should exist")
	assert.Equal(t, codegraph.NodeKindModule, modNode.Kind)
	assert.True(t, modNode.WithinWorkspace)

	// Check Package Nodes
	pkgNode, ok := graph.Node(codegraph.PackageNodeID("example.com/fixturemod/domain"))
	require.True(t, ok, "package node should exist")
	assert.Equal(t, codegraph.NodeKindPackage, pkgNode.Kind)
	assert.Equal(t, modNode.ID, pkgNode.ParentID)

	// Check Module -> Package contains edge
	assert.True(t, graph.DirectlyReaches(modNode.ID, pkgNode.ID, codegraph.EdgeKindContains))

	// Check Package -> File contains edges
	fileNodes := graph.DirectDependencies(pkgNode.ID, codegraph.EdgeKindContains)
	assert.NotEmpty(t, fileNodes)
	assert.Equal(t, codegraph.NodeKindFile, fileNodes[0].Kind)
}

func TestBuild_PackageImports(t *testing.T) {
	graph := buildGraphForFixture(t, "fixturemod")
	require.NotNil(t, graph)

	appPkgID := codegraph.PackageNodeID("example.com/fixturemod/application")
	domainPkgID := codegraph.PackageNodeID("example.com/fixturemod/domain")

	// application imports domain
	assert.True(t, graph.DirectlyReaches(appPkgID, domainPkgID, codegraph.EdgeKindImports))
	assert.False(t, graph.DirectlyReaches(domainPkgID, appPkgID, codegraph.EdgeKindImports))
}

func TestBuild_FunctionCallsAndTypeInfo(t *testing.T) {
	graph := buildGraphForFixture(t, "typeinfofixture", archscout.WithTypeInfo())
	require.NotNil(t, graph)

	mainFuncID := codegraph.FunctionNodeID("example.com/typeinfofixture.main")
	runFuncID := codegraph.FunctionNodeID("example.com/typeinfofixture/api.Service.Run")
	fmtPkgID := codegraph.FunctionNodeID("fmt.Println")

	// main() calls api.Service.Run
	assert.True(t, graph.DirectlyReaches(mainFuncID, runFuncID, codegraph.EdgeKindCalls))

	// main() calls fmt.Println
	assert.True(t, graph.DirectlyReaches(mainFuncID, fmtPkgID, codegraph.EdgeKindCalls))
}

func TestBuild_ImplementsEdges(t *testing.T) {
	graph := buildGraphForFixture(t, "typeinfofixture", archscout.WithTypeInfo())
	require.NotNil(t, graph)

	greeterIfaceID := codegraph.TypeNodeID("example.com/typeinfofixture/api.Greeter")
	ptrGreeterTypeID := codegraph.TypeNodeID("example.com/typeinfofixture.PointerGreeter")
	defaultGreeterTypeID := codegraph.TypeNodeID("example.com/typeinfofixture.defaultGreeter")

	assert.True(t, graph.DirectlyReaches(ptrGreeterTypeID, greeterIfaceID, codegraph.EdgeKindImplements))
	assert.True(t, graph.DirectlyReaches(defaultGreeterTypeID, greeterIfaceID, codegraph.EdgeKindImplements))
}

func TestBuild_TransitiveReachability(t *testing.T) {
	graph := buildGraphForFixture(t, "fixturemod")
	require.NotNil(t, graph)

	mainPkgID := codegraph.PackageNodeID("example.com/fixturemod")
	domainPkgID := codegraph.PackageNodeID("example.com/fixturemod/domain")

	// main -> application -> domain
	assert.True(t, graph.TransitivelyReaches(mainPkgID, domainPkgID, codegraph.EdgeKindImports))
}

func TestBuild_InPackageSubgraph(t *testing.T) {
	graph := buildGraphForFixture(t, "fixturemod")
	require.NotNil(t, graph)

	sub := graph.InPackage("example.com/fixturemod/domain/...")
	require.NotNil(t, sub)

	for _, node := range sub.Nodes() {
		assert.Contains(t, node.PackageID, "example.com/fixturemod/domain")
	}
}
