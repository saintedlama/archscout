package codegraph

import (
	"sort"
	"strings"
)

// Ancestor returns the ancestor of nodeID that matches targetKind.
// If the node itself is of targetKind, it returns (node, true).
// If no ancestor of targetKind is found, it returns (Node{}, false).
func (g *Graph) Ancestor(nodeID string, targetKind NodeKind) (Node, bool) {
	if g == nil {
		return Node{}, false
	}
	start, ok := g.nodes[nodeID]
	if !ok {
		return Node{}, false
	}
	if start.Kind == targetKind {
		return start, true
	}

	// Shortcut for Package: if target is Package and node has PackageID
	if targetKind == NodeKindPackage && start.PackageID != "" {
		if pkgNode, exists := g.Node(PackageNodeID(start.PackageID)); exists {
			return pkgNode, true
		}
	}

	curr := start
	for curr.ParentID != "" {
		parent, exists := g.nodes[curr.ParentID]
		if !exists {
			break
		}
		if parent.Kind == targetKind {
			return parent, true
		}
		curr = parent
	}

	// If target is Module and node is within workspace
	if targetKind == NodeKindModule && start.WithinWorkspace {
		for _, mID := range g.nodesByKind[NodeKindModule] {
			if m, exists := g.nodes[mID]; exists && m.WithinWorkspace {
				return m, true
			}
		}
	}

	// If target is Module and node is external
	if targetKind == NodeKindModule && !start.WithinWorkspace && start.PackageID != "" {
		mod := g.moduleOf(start.PackageID)
		extModID := ModuleNodeID(mod)
		if extMod, exists := g.nodes[extModID]; exists {
			return extMod, true
		}
		return Node{ID: extModID, Kind: NodeKindModule, Name: mod, WithinWorkspace: false}, true
	}

	return Node{}, false
}

// Rollup projects low-level edges up to an architectural tier specified by targetKind.
//
// For example, rolling up to NodeKindPackage condenses all function calls, type references,
// and file imports into a Package-to-Package dependency graph.
// Rolling up to NodeKindModule creates a Module-to-Module graph.
func (g *Graph) Rollup(targetKind NodeKind, edgeKinds ...EdgeKind) *Graph {
	if g == nil {
		return nil
	}

	rolled := newGraph()
	// Pre-seed with existing targetKind nodes
	for _, id := range g.nodesByKind[targetKind] {
		rolled.addNode(g.nodes[id])
	}

	seenEdges := make(map[string]struct{})
	for _, edge := range g.Edges(edgeKinds...) {
		if edge.Kind == EdgeKindContains {
			continue
		}

		srcAncestor, srcOk := g.Ancestor(edge.Source, targetKind)
		dstAncestor, dstOk := g.Ancestor(edge.Target, targetKind)

		if !srcOk || !dstOk || srcAncestor.ID == dstAncestor.ID {
			continue
		}

		rolled.addNode(srcAncestor)
		rolled.addNode(dstAncestor)

		key := srcAncestor.ID + "|" + dstAncestor.ID + "|" + string(edge.Kind)
		if _, dup := seenEdges[key]; !dup {
			seenEdges[key] = struct{}{}
			rolled.addEdge(Edge{
				Source: srcAncestor.ID,
				Target: dstAncestor.ID,
				Kind:   edge.Kind,
				Ref:    edge.Ref,
			})
		}
	}

	rolled.finalize()
	return rolled
}

// DependenciesOf returns all nodes of targetKind that nodeID depends on,
// across outgoing edges of the specified edgeKinds (direct or rolled up from children).
//
// For example:
//   - DependenciesOf("func:pkg.Service.Run", NodeKindModule, EdgeKindCalls) returns the modules called.
//   - DependenciesOf("file:service.go", NodeKindPackage, EdgeKindImports) returns imported packages.
func (g *Graph) DependenciesOf(nodeID string, targetKind NodeKind, edgeKinds ...EdgeKind) []Node {
	if g == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var result []Node

	add := func(n Node) {
		if _, dup := seen[n.ID]; !dup {
			seen[n.ID] = struct{}{}
			result = append(result, n)
		}
	}

	checkEdges := func(sourceID string) {
		for _, edge := range g.OutEdges(sourceID, edgeKinds...) {
			if edge.Kind == EdgeKindContains {
				continue
			}
			if target, ok := g.Ancestor(edge.Target, targetKind); ok {
				add(target)
			}
		}
	}

	// Direct edges from nodeID
	checkEdges(nodeID)

	// Edges from any contained descendants (e.g. package contains files, file contains functions)
	containedChildren := g.TransitiveDependencies(nodeID, EdgeKindContains)
	for _, child := range containedChildren {
		checkEdges(child.ID)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// DependentsOf returns all nodes of sourceKind that depend on nodeID,
// across incoming edges of the specified edgeKinds (direct or rolled up from children).
//
// For example:
//   - DependentsOf("package:domain", NodeKindFunction, EdgeKindCalls) returns functions calling domain.
func (g *Graph) DependentsOf(nodeID string, sourceKind NodeKind, edgeKinds ...EdgeKind) []Node {
	if g == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var result []Node

	add := func(n Node) {
		if _, dup := seen[n.ID]; !dup {
			seen[n.ID] = struct{}{}
			result = append(result, n)
		}
	}

	checkInEdges := func(targetID string) {
		for _, edge := range g.InEdges(targetID, edgeKinds...) {
			if edge.Kind == EdgeKindContains {
				continue
			}
			if src, ok := g.Ancestor(edge.Source, sourceKind); ok {
				add(src)
			}
		}
	}

	// Direct incoming edges
	checkInEdges(nodeID)

	// Incoming edges to any contained descendants
	containedChildren := g.TransitiveDependencies(nodeID, EdgeKindContains)
	for _, child := range containedChildren {
		checkInEdges(child.ID)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// Paths returns all simple paths from sourceID to targetID along edges of edgeKinds,
// up to maxDepth hops. If maxDepth <= 0, a default of 10 hops is enforced.
func (g *Graph) Paths(sourceID, targetID string, maxDepth int, edgeKinds ...EdgeKind) [][]string {
	if g == nil || !g.HasNode(sourceID) || !g.HasNode(targetID) {
		return nil
	}
	if maxDepth <= 0 {
		maxDepth = 10
	}

	var paths [][]string
	visited := make(map[string]bool)

	var dfs func(curr string, path []string, depth int)
	dfs = func(curr string, path []string, depth int) {
		if depth > maxDepth {
			return
		}
		if curr == targetID {
			fullPath := make([]string, len(path))
			copy(fullPath, path)
			paths = append(paths, fullPath)
			return
		}

		visited[curr] = true
		defer func() { visited[curr] = false }()

		for _, edge := range g.OutEdges(curr, edgeKinds...) {
			if edge.Kind == EdgeKindContains {
				continue
			}
			if !visited[edge.Target] {
				dfs(edge.Target, append(path, edge.Target), depth+1)
			}
		}
	}

	dfs(sourceID, []string{sourceID}, 0)
	return paths
}

// Cycles finds all directed cycles among the specified edge kinds.
// Returns a slice of cycle paths, where each cycle is a slice of node IDs.
func (g *Graph) Cycles(edgeKinds ...EdgeKind) [][]string {
	if g == nil {
		return nil
	}

	var cycles [][]string
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var stack []string

	var dfs func(curr string)
	dfs = func(curr string) {
		visited[curr] = true
		inStack[curr] = true
		stack = append(stack, curr)

		for _, edge := range g.OutEdges(curr, edgeKinds...) {
			if edge.Kind == EdgeKindContains {
				continue
			}
			nxt := edge.Target
			if inStack[nxt] {
				// Found a cycle: extract segment from nxt to curr + nxt
				idx := -1
				for i, id := range stack {
					if id == nxt {
						idx = i
						break
					}
				}
				if idx >= 0 {
					cycle := make([]string, len(stack)-idx+1)
					copy(cycle, stack[idx:])
					cycle[len(cycle)-1] = nxt
					cycles = append(cycles, cycle)
				}
			} else if !visited[nxt] {
				dfs(nxt)
			}
		}

		stack = stack[:len(stack)-1]
		inStack[curr] = false
	}

	for _, nodeID := range g.sortedNodes {
		if !visited[nodeID] {
			dfs(nodeID)
		}
	}

	return cycles
}

// ExternalModuleOf derives the module path of an external package, or "std" for
// standard library packages, from the import path alone. It knows the layout of
// common hosts (github.com, golang.org/x, gopkg.in, ...) and major version
// suffixes; for other domains it assumes a two-segment module path. Graphs built
// with Input.Modules use the actual module list first.
func ExternalModuleOf(pkgID string) string {
	return externalModuleOf(pkgID)
}

// moduleOf returns the longest known module path that contains pkgID, falling
// back to ExternalModuleOf.
func (g *Graph) moduleOf(pkgID string) string {
	for _, mod := range g.modules {
		if pkgID == mod || strings.HasPrefix(pkgID, mod+"/") {
			return mod
		}
	}
	return externalModuleOf(pkgID)
}

func externalModuleOf(pkgID string) string {
	parts := strings.Split(pkgID, "/")
	if !strings.Contains(parts[0], ".") {
		return "std"
	}

	n := 2 // vanity domains: go.uber.org/zap, k8s.io/client-go, google.golang.org/grpc
	switch parts[0] {
	case "github.com", "gitlab.com", "bitbucket.org", "golang.org", "codeberg.org":
		n = 3
	case "gopkg.in":
		n = 2
	}
	if n > len(parts) {
		return pkgID
	}
	// Major version suffix: github.com/org/repo/v2, go.uber.org/zap/v2.
	if n < len(parts) && isMajorVersion(parts[n]) {
		n++
	}
	return strings.Join(parts[:n], "/")
}

func isMajorVersion(segment string) bool {
	if len(segment) < 2 || segment[0] != 'v' {
		return false
	}
	for _, r := range segment[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func sortModulesLongestFirst(modules []string) []string {
	out := make([]string, 0, len(modules))
	for _, m := range modules {
		if m != "" {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}
