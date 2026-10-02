package codegraph_test

import (
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/codegraph"
	"github.com/saintedlama/archscout/common"
	"github.com/saintedlama/archscout/functioncalls"
	"github.com/saintedlama/archscout/functions"
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

func TestExternalModuleOf(t *testing.T) {
	cases := map[string]string{
		"fmt":                                "std",
		"net/http":                           "std",
		"github.com/stretchr/testify/assert": "github.com/stretchr/testify",
		"github.com/org/repo/v2/sub":         "github.com/org/repo/v2",
		"golang.org/x/tools/go/packages":     "golang.org/x/tools",
		"go.uber.org/zap/zapcore":            "go.uber.org/zap",
		"go.uber.org/zap/v2/zapcore":         "go.uber.org/zap/v2",
		"k8s.io/client-go/kubernetes":        "k8s.io/client-go",
		"google.golang.org/grpc/codes":       "google.golang.org/grpc",
		"gopkg.in/yaml.v3":                   "gopkg.in/yaml.v3",
		"modernc.org/sqlite":                 "modernc.org/sqlite",
		"github.com/org":                     "github.com/org",
	}
	for pkg, want := range cases {
		assert.Equal(t, want, codegraph.ExternalModuleOf(pkg), pkg)
	}
}

func TestAncestor_UsesKnownModules(t *testing.T) {
	ref := common.Ref{PackageID: "example.com/m", Filename: "/m/main.go"}
	graph := codegraph.Build(codegraph.Input{
		ModuleRoot: "example.com/m",
		Functions: functions.NewCollection([]functions.Item{
			{Ref: ref, Name: "main", QName: "example.com/m.main"},
		}),
		FunctionCalls: functioncalls.NewCollection([]functioncalls.Item{
			{Ref: ref, CallerQName: "example.com/m.main", CalleeQName: "cloud.google.com/go/storage/internal.X", CalleePackage: "cloud.google.com/go/storage/internal"},
		}),
		Modules: []string{"example.com/m", "cloud.google.com/go", "cloud.google.com/go/storage"},
	})

	mod, ok := graph.Ancestor(codegraph.FunctionNodeID("cloud.google.com/go/storage/internal.X"), codegraph.NodeKindModule)
	require.True(t, ok)
	assert.Equal(t, codegraph.ModuleNodeID("cloud.google.com/go/storage"), mod.ID)
}
