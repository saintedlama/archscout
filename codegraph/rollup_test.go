package codegraph_test

import (
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/codegraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollup_Ancestor(t *testing.T) {
	graph := buildGraphForFixture(t, "typeinfofixture", archscout.WithTypeInfo())
	require.NotNil(t, graph)

	mainFuncID := codegraph.FunctionNodeID("example.com/typeinfofixture.main")

	// Ancestor File
	fileNode, ok := graph.Ancestor(mainFuncID, codegraph.NodeKindFile)
	require.True(t, ok)
	assert.Equal(t, codegraph.NodeKindFile, fileNode.Kind)

	// Ancestor Package
	pkgNode, ok := graph.Ancestor(mainFuncID, codegraph.NodeKindPackage)
	require.True(t, ok)
	assert.Equal(t, codegraph.PackageNodeID("example.com/typeinfofixture"), pkgNode.ID)

	// Ancestor Module
	modNode, ok := graph.Ancestor(mainFuncID, codegraph.NodeKindModule)
	require.True(t, ok)
	assert.Equal(t, codegraph.ModuleNodeID("example.com/typeinfofixture"), modNode.ID)

	// External function ancestor
	fmtFuncID := codegraph.FunctionNodeID("fmt.Println")
	extModNode, ok := graph.Ancestor(fmtFuncID, codegraph.NodeKindModule)
	require.True(t, ok)
	assert.Equal(t, "module:std", extModNode.ID)
}

func TestRollup_RollupToPackage(t *testing.T) {
	graph := buildGraphForFixture(t, "typeinfofixture", archscout.WithTypeInfo())
	require.NotNil(t, graph)

	rolled := graph.Rollup(codegraph.NodeKindPackage, codegraph.EdgeKindCalls)
	require.NotNil(t, rolled)

	// All nodes in rolled graph must be Packages
	for _, n := range rolled.Nodes() {
		assert.Equal(t, codegraph.NodeKindPackage, n.Kind)
	}

	// example.com/typeinfofixture calls example.com/typeinfofixture/api
	mainPkgID := codegraph.PackageNodeID("example.com/typeinfofixture")
	apiPkgID := codegraph.PackageNodeID("example.com/typeinfofixture/api")
	assert.True(t, rolled.DirectlyReaches(mainPkgID, apiPkgID, codegraph.EdgeKindCalls))

	// example.com/typeinfofixture calls fmt
	fmtPkgID := codegraph.PackageNodeID("fmt")
	assert.True(t, rolled.DirectlyReaches(mainPkgID, fmtPkgID, codegraph.EdgeKindCalls))
}

func TestRollup_RollupToModule(t *testing.T) {
	graph := buildGraphForFixture(t, "typeinfofixture", archscout.WithTypeInfo())
	require.NotNil(t, graph)

	rolled := graph.Rollup(codegraph.NodeKindModule, codegraph.EdgeKindCalls)
	require.NotNil(t, rolled)

	for _, n := range rolled.Nodes() {
		assert.Equal(t, codegraph.NodeKindModule, n.Kind)
	}

	wsModID := codegraph.ModuleNodeID("example.com/typeinfofixture")
	stdModID := codegraph.ModuleNodeID("std")

	// Workspace module calls std module
	assert.True(t, rolled.DirectlyReaches(wsModID, stdModID, codegraph.EdgeKindCalls))
}

func TestRollup_DependenciesOf(t *testing.T) {
	graph := buildGraphForFixture(t, "typeinfofixture", archscout.WithTypeInfo())
	require.NotNil(t, graph)

	mainFuncID := codegraph.FunctionNodeID("example.com/typeinfofixture.main")

	// Which modules does main() call?
	calledModules := graph.DependenciesOf(mainFuncID, codegraph.NodeKindModule, codegraph.EdgeKindCalls)
	require.NotEmpty(t, calledModules)

	var modNames []string
	for _, m := range calledModules {
		modNames = append(modNames, m.Name)
	}
	assert.Contains(t, modNames, "std")

	// Which packages does main() call?
	calledPackages := graph.DependenciesOf(mainFuncID, codegraph.NodeKindPackage, codegraph.EdgeKindCalls)
	require.NotEmpty(t, calledPackages)

	var pkgIDs []string
	for _, p := range calledPackages {
		pkgIDs = append(pkgIDs, p.PackageID)
	}
	assert.Contains(t, pkgIDs, "fmt")
	assert.Contains(t, pkgIDs, "example.com/typeinfofixture/api")
}

func TestRollup_DependentsOf(t *testing.T) {
	graph := buildGraphForFixture(t, "typeinfofixture", archscout.WithTypeInfo())
	require.NotNil(t, graph)

	apiPkgID := codegraph.PackageNodeID("example.com/typeinfofixture/api")

	// Which functions call into the api package?
	callingFuncs := graph.DependentsOf(apiPkgID, codegraph.NodeKindFunction, codegraph.EdgeKindCalls)
	require.NotEmpty(t, callingFuncs)

	var funcQNames []string
	for _, fn := range callingFuncs {
		funcQNames = append(funcQNames, fn.QName)
	}
	assert.Contains(t, funcQNames, "example.com/typeinfofixture.main")
}

func TestRollup_Paths(t *testing.T) {
	graph := buildGraphForFixture(t, "fixturemod")
	require.NotNil(t, graph)

	mainPkgID := codegraph.PackageNodeID("example.com/fixturemod")
	domainPkgID := codegraph.PackageNodeID("example.com/fixturemod/domain")

	// main -> application -> domain
	paths := graph.Paths(mainPkgID, domainPkgID, 5, codegraph.EdgeKindImports)
	require.NotEmpty(t, paths)
	assert.Equal(t, mainPkgID, paths[0][0])
	assert.Equal(t, domainPkgID, paths[0][len(paths[0])-1])
}

func TestRollup_Cycles(t *testing.T) {
	graph := buildGraphForFixture(t, "fixturemod")
	require.NotNil(t, graph)

	// fixturemod imports should be acyclic
	cycles := graph.Cycles(codegraph.EdgeKindImports)
	assert.Empty(t, cycles, "expected no import cycles in fixturemod")
}
