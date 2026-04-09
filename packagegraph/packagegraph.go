package packagegraph

import (
	"sort"

	"github.com/saintedlama/archscout/common"
	"github.com/saintedlama/archscout/dependencies"
)

// PackageGraph is a directed graph of workspace-internal package dependencies.
// Each node is a package ID; edges represent direct imports.
//
// Construct via BuildGraph, not directly.
type PackageGraph struct {
	// edges maps source package ID → sorted slice of direct dependency IDs.
	edges map[string][]string
	// nodes is the sorted set of all package IDs that appear as source or target.
	nodes []string
}

// BuildGraph constructs a PackageGraph from the provided dependency collection.
// Only workspace-internal dependencies (WithinWorkspace == true) contribute edges;
// call IsNotTest() or other filters on the collection before passing it in if you
// want to restrict what is included:
//
//	graph := packagegraph.BuildGraph(ws.Dependencies.IsNotTest().IsWithinWorkspace())
func BuildGraph(c dependencies.Collection) *PackageGraph {
	edgeSet := make(map[string]map[string]struct{})
	nodeSet := make(map[string]struct{})

	for _, item := range c.All() {
		if !item.WithinWorkspace {
			continue
		}
		src := item.Ref.PackageID
		dst := item.ImportPath

		nodeSet[src] = struct{}{}
		nodeSet[dst] = struct{}{}

		if _, ok := edgeSet[src]; !ok {
			edgeSet[src] = make(map[string]struct{})
		}
		edgeSet[src][dst] = struct{}{}
	}

	// Materialise sorted slices for deterministic iteration.
	edges := make(map[string][]string, len(edgeSet))
	for src, dstSet := range edgeSet {
		dsts := make([]string, 0, len(dstSet))
		for dst := range dstSet {
			dsts = append(dsts, dst)
		}
		sort.Strings(dsts)
		edges[src] = dsts
	}

	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	return &PackageGraph{edges: edges, nodes: nodes}
}

// Packages returns the sorted set of all package IDs in the graph (both sources
// and targets).
func (g *PackageGraph) Packages() []string {
	return append([]string(nil), g.nodes...)
}

// DirectDependencies returns the sorted list of packages directly imported by
// any source package matching patterns. Supports the /... glob convention.
func (g *PackageGraph) DirectDependencies(patterns ...string) []string {
	seen := make(map[string]struct{})
	for src, dsts := range g.edges {
		if !common.PackageMatchesAny(src, patterns...) {
			continue
		}
		for _, dst := range dsts {
			seen[dst] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

// TransitiveDependencies returns all packages reachable from any source package
// matching patterns via one or more import hops. The originating packages
// themselves are not included. Supports the /... glob convention.
func (g *PackageGraph) TransitiveDependencies(patterns ...string) []string {
	// Seed the frontier with the direct dependencies of matching sources.
	visited := make(map[string]struct{})
	queue := []string{}

	for src, dsts := range g.edges {
		if !common.PackageMatchesAny(src, patterns...) {
			continue
		}
		for _, dst := range dsts {
			if _, ok := visited[dst]; !ok {
				visited[dst] = struct{}{}
				queue = append(queue, dst)
			}
		}
	}

	// BFS over the graph.
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, dst := range g.edges[node] {
			if _, ok := visited[dst]; !ok {
				visited[dst] = struct{}{}
				queue = append(queue, dst)
			}
		}
	}

	return sortedKeys(visited)
}

// TransitivelyReaches reports whether any package matching fromPatterns can
// reach any package matching toPatterns via one or more import hops.
// Supports the /... glob convention.
//
//	graph.TransitivelyReaches(mod.Pkg("domain/..."), mod.Pkg("infrastructure/..."))
func (g *PackageGraph) TransitivelyReaches(fromPatterns []string, toPatterns []string) bool {
	for _, reachable := range g.TransitiveDependencies(fromPatterns...) {
		if common.PackageMatchesAny(reachable, toPatterns...) {
			return true
		}
	}
	return false
}

// DirectlyReaches reports whether any package matching fromPatterns directly
// imports any package matching toPatterns (single hop only).
func (g *PackageGraph) DirectlyReaches(fromPatterns []string, toPatterns []string) bool {
	for _, direct := range g.DirectDependencies(fromPatterns...) {
		if common.PackageMatchesAny(direct, toPatterns...) {
			return true
		}
	}
	return false
}

// Importers returns the sorted list of packages that directly import any package
// matching patterns. This is the reverse edge query. Supports /... glob convention.
func (g *PackageGraph) Importers(patterns ...string) []string {
	seen := make(map[string]struct{})
	for src, dsts := range g.edges {
		for _, dst := range dsts {
			if common.PackageMatchesAny(dst, patterns...) {
				seen[src] = struct{}{}
				break
			}
		}
	}
	return sortedKeys(seen)
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
