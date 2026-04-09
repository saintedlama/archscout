package packagegraph_test

import (
	"testing"

	"github.com/saintedlama/archscout/internaltest"
	"github.com/saintedlama/archscout/packagegraph"
	"github.com/stretchr/testify/assert"
)

const (
	rootPkg   = "example.com/fixturemod"
	appPkg    = "example.com/fixturemod/application"
	domainPkg = "example.com/fixturemod/domain"
	infraPkg  = "example.com/fixturemod/infrastructure"
)

func buildFixtureGraph(t *testing.T) *packagegraph.PackageGraph {
	t.Helper()
	ws := internaltest.LoadFixtureWorkspace(t, "fixturemod")
	return packagegraph.BuildGraph(ws.Dependencies)
}

func TestPackageGraph_Packages_ContainsAllInternalPackages(t *testing.T) {
	g := buildFixtureGraph(t)

	pkgs := g.Packages()

	assert.Contains(t, pkgs, rootPkg)
	assert.Contains(t, pkgs, appPkg)
	assert.Contains(t, pkgs, infraPkg)
	assert.Contains(t, pkgs, domainPkg)
}

func TestPackageGraph_Packages_DoesNotContainExternalPackages(t *testing.T) {
	g := buildFixtureGraph(t)

	pkgs := g.Packages()

	assert.NotContains(t, pkgs, "fmt")
	assert.NotContains(t, pkgs, "errors")
}

func TestPackageGraph_DirectDependencies_RootImportsAppAndInfra(t *testing.T) {
	g := buildFixtureGraph(t)

	deps := g.DirectDependencies(rootPkg)

	assert.ElementsMatch(t, []string{appPkg, infraPkg}, deps)
}

func TestPackageGraph_DirectDependencies_ApplicationImportsDomain(t *testing.T) {
	g := buildFixtureGraph(t)

	deps := g.DirectDependencies(appPkg)

	assert.Equal(t, []string{domainPkg}, deps)
}

func TestPackageGraph_DirectDependencies_DomainHasNone(t *testing.T) {
	g := buildFixtureGraph(t)

	deps := g.DirectDependencies(domainPkg)

	assert.Empty(t, deps)
}

func TestPackageGraph_TransitiveDependencies_RootReachesAllInternalPackages(t *testing.T) {
	g := buildFixtureGraph(t)

	deps := g.TransitiveDependencies(rootPkg)

	assert.Contains(t, deps, appPkg)
	assert.Contains(t, deps, infraPkg)
	assert.Contains(t, deps, domainPkg)
}

func TestPackageGraph_TransitiveDependencies_DomainHasNone(t *testing.T) {
	g := buildFixtureGraph(t)

	deps := g.TransitiveDependencies(domainPkg)

	assert.Empty(t, deps)
}

func TestPackageGraph_TransitiveDependencies_GlobPatternMatchesMultipleSources(t *testing.T) {
	g := buildFixtureGraph(t)

	// application/... and infrastructure/... both transitively depend on domain
	deps := g.TransitiveDependencies("example.com/fixturemod/...")

	assert.Contains(t, deps, domainPkg)
}

func TestPackageGraph_TransitivelyReaches_TrueWhenPathExists(t *testing.T) {
	g := buildFixtureGraph(t)

	// root → application → domain (two hops)
	assert.True(t, g.TransitivelyReaches([]string{rootPkg}, []string{domainPkg}))
}

func TestPackageGraph_TransitivelyReaches_FalseWhenNoPath(t *testing.T) {
	g := buildFixtureGraph(t)

	// domain has no outgoing edges
	assert.False(t, g.TransitivelyReaches([]string{domainPkg}, []string{rootPkg}))
}

func TestPackageGraph_DirectlyReaches_TrueForSingleHop(t *testing.T) {
	g := buildFixtureGraph(t)

	assert.True(t, g.DirectlyReaches([]string{rootPkg}, []string{appPkg}))
}

func TestPackageGraph_DirectlyReaches_FalseForTwoHops(t *testing.T) {
	g := buildFixtureGraph(t)

	// main does NOT directly import domain — only via application/infrastructure
	assert.False(t, g.DirectlyReaches([]string{rootPkg}, []string{domainPkg}))
}

func TestPackageGraph_Importers_ReturnsBothImportersOfDomain(t *testing.T) {
	g := buildFixtureGraph(t)

	importers := g.Importers(domainPkg)

	assert.ElementsMatch(t, []string{appPkg, infraPkg}, importers)
}

func TestPackageGraph_Importers_GlobPatternMatchesDomainAndSubPackages(t *testing.T) {
	g := buildFixtureGraph(t)

	// /... should match example.com/fixturemod/domain itself
	importers := g.Importers("example.com/fixturemod/domain/...")

	assert.ElementsMatch(t, []string{appPkg, infraPkg}, importers)
}

func TestPackageGraph_Importers_NobodyImportsRoot(t *testing.T) {
	g := buildFixtureGraph(t)

	importers := g.Importers(rootPkg)

	assert.Empty(t, importers)
}
