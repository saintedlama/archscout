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
		extModID := ModuleNodeID(externalModuleOf(start.PackageID))
		if extMod, exists := g.nodes[extModID]; exists {
			return extMod, true
		}
		mod := externalModuleOf(start.PackageID)
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

// ExternalModuleOf derives the external module identifier or "std" for standard library packages.
func ExternalModuleOf(pkgID string) string {
	return externalModuleOf(pkgID)
}

func externalModuleOf(pkgID string) string {
	if !strings.Contains(pkgID, ".") {
		return "std"
	}
	parts := strings.Split(pkgID, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return pkgID
}
