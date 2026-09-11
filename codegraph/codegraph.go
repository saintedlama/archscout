package codegraph

import (
	"slices"
	"sort"

	"github.com/saintedlama/archscout/common"
)

// NodeKind identifies the architectural level or entity category of a node.
type NodeKind string

const (
	NodeKindModule   NodeKind = "module"
	NodeKindPackage  NodeKind = "package"
	NodeKindFile     NodeKind = "file"
	NodeKindType     NodeKind = "type"
	NodeKindFunction NodeKind = "function"
)

// EdgeKind identifies the semantic relationship represented by a directed edge.
type EdgeKind string

const (
	// Containment hierarchy
	EdgeKindContains EdgeKind = "contains" // Module -> Package, Package -> File, File -> Type/Func, Type -> Method

	// Import and dependency relationships
	EdgeKindImports   EdgeKind = "imports"    // File -> Package, Package -> Package
	EdgeKindDependsOn EdgeKind = "depends_on" // Module -> Module

	// Invocations
	EdgeKindCalls EdgeKind = "calls" // Function -> Function, Function -> Package

	// Type relationships
	EdgeKindImplements     EdgeKind = "implements"      // Concrete Type -> Interface
	EdgeKindEmbeds         EdgeKind = "embeds"          // Struct -> Struct, Interface -> Interface
	EdgeKindReferencesType EdgeKind = "references_type" // Struct -> Field Type, Func -> Param/Return Type
)

// Node represents an entity in the codebase graph.
type Node struct {
	ID              string     // Canonical unique ID, e.g. "package:github.com/foo/bar"
	Kind            NodeKind   // NodeKindModule, NodeKindPackage, etc.
	Name            string     // Short identifier (e.g. package name, filename, func name)
	QName           string     // Canonical qualified name for types and functions
	PackageID       string     // Defining package ID / import path
	ParentID        string     // Immediate container node ID in containment hierarchy
	Ref             common.Ref // Source location reference
	WithinWorkspace bool       // True if entity belongs to analyzed workspace
}

// Edge represents a directed relationship between two nodes in the graph.
type Edge struct {
	Source string     // Source node ID
	Target string     // Target node ID
	Kind   EdgeKind   // Relationship kind
	Ref    common.Ref // Source location where edge originates
}

// Graph is an indexed directed graph containing all code entities and relationships.
type Graph struct {
	nodes       map[string]Node
	sortedNodes []string
	outEdges    map[string][]Edge
	inEdges     map[string][]Edge
	nodesByKind map[NodeKind][]string
}

// NodeID builds a canonical prefixed node ID.
func NodeID(kind NodeKind, key string) string {
	return string(kind) + ":" + key
}

// ModuleNodeID returns the canonical node ID for a module.
func ModuleNodeID(modulePath string) string {
	return NodeID(NodeKindModule, modulePath)
}

// PackageNodeID returns the canonical node ID for a package.
func PackageNodeID(packageID string) string {
	return NodeID(NodeKindPackage, packageID)
}

// FileNodeID returns the canonical node ID for a file.
func FileNodeID(filename string) string {
	return NodeID(NodeKindFile, filename)
}

// TypeNodeID returns the canonical node ID for a type.
func TypeNodeID(qname string) string {
	return NodeID(NodeKindType, qname)
}

// FunctionNodeID returns the canonical node ID for a function or method.
func FunctionNodeID(qname string) string {
	return NodeID(NodeKindFunction, qname)
}

// Node returns the node with the given ID and reports whether it was found.
func (g *Graph) Node(id string) (Node, bool) {
	if g == nil {
		return Node{}, false
	}
	n, ok := g.nodes[id]
	return n, ok
}

// HasNode reports whether a node with the given ID exists in the graph.
func (g *Graph) HasNode(id string) bool {
	if g == nil {
		return false
	}
	_, ok := g.nodes[id]
	return ok
}

// Nodes returns all nodes in the graph, optionally filtered by NodeKind.
// The result is sorted deterministically by node ID.
func (g *Graph) Nodes(kinds ...NodeKind) []Node {
	if g == nil || len(g.nodes) == 0 {
		return nil
	}

	if len(kinds) == 0 {
		out := make([]Node, 0, len(g.sortedNodes))
		for _, id := range g.sortedNodes {
			out = append(out, g.nodes[id])
		}
		return out
	}

	var out []Node
	for _, id := range g.sortedNodes {
		n := g.nodes[id]
		if slices.Contains(kinds, n.Kind) {
			out = append(out, n)
		}
	}
	return out
}

// Edges returns all edges in the graph, optionally filtered by EdgeKind.
func (g *Graph) Edges(kinds ...EdgeKind) []Edge {
	if g == nil {
		return nil
	}

	var out []Edge
	for _, src := range g.sortedNodes {
		for _, edge := range g.outEdges[src] {
			if matchEdgeKind(edge.Kind, kinds) {
				out = append(out, edge)
			}
		}
	}
	return out
}

// OutEdges returns outgoing edges from nodeID, optionally filtered by EdgeKind.
func (g *Graph) OutEdges(nodeID string, kinds ...EdgeKind) []Edge {
	if g == nil {
		return nil
	}
	edges := g.outEdges[nodeID]
	if len(kinds) == 0 {
		return append([]Edge(nil), edges...)
	}

	var out []Edge
	for _, edge := range edges {
		if matchEdgeKind(edge.Kind, kinds) {
			out = append(out, edge)
		}
	}
	return out
}

// InEdges returns incoming edges to nodeID, optionally filtered by EdgeKind.
func (g *Graph) InEdges(nodeID string, kinds ...EdgeKind) []Edge {
	if g == nil {
		return nil
	}
	edges := g.inEdges[nodeID]
	if len(kinds) == 0 {
		return append([]Edge(nil), edges...)
	}

	var out []Edge
	for _, edge := range edges {
		if matchEdgeKind(edge.Kind, kinds) {
			out = append(out, edge)
		}
	}
	return out
}

// DirectDependencies returns the target nodes directly connected from nodeID
// via outgoing edges of the specified kinds.
func (g *Graph) DirectDependencies(nodeID string, kinds ...EdgeKind) []Node {
	if g == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, edge := range g.OutEdges(nodeID, kinds...) {
		if _, dup := seen[edge.Target]; !dup {
			seen[edge.Target] = struct{}{}
			ids = append(ids, edge.Target)
		}
	}
	sort.Strings(ids)

	out := make([]Node, 0, len(ids))
	for _, id := range ids {
		if n, ok := g.nodes[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

// DirectDependents returns the source nodes directly connected to nodeID
// via incoming edges of the specified kinds.
func (g *Graph) DirectDependents(nodeID string, kinds ...EdgeKind) []Node {
	if g == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var ids []string
	for _, edge := range g.InEdges(nodeID, kinds...) {
		if _, dup := seen[edge.Source]; !dup {
			seen[edge.Source] = struct{}{}
			ids = append(ids, edge.Source)
		}
	}
	sort.Strings(ids)

	out := make([]Node, 0, len(ids))
	for _, id := range ids {
		if n, ok := g.nodes[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

// TransitiveDependencies performs a BFS from nodeID along outgoing edges of kinds
// and returns all reachable target nodes in deterministic order.
func (g *Graph) TransitiveDependencies(nodeID string, kinds ...EdgeKind) []Node {
	if g == nil {
		return nil
	}

	visited := make(map[string]struct{})
	queue := []string{nodeID}
	visited[nodeID] = struct{}{}

	var reached []string
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, edge := range g.OutEdges(curr, kinds...) {
			if _, ok := visited[edge.Target]; !ok {
				visited[edge.Target] = struct{}{}
				queue = append(queue, edge.Target)
				reached = append(reached, edge.Target)
			}
		}
	}
	sort.Strings(reached)

	out := make([]Node, 0, len(reached))
	for _, id := range reached {
		if n, ok := g.nodes[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

// TransitiveDependents performs a reverse BFS from nodeID along incoming edges of kinds
// and returns all nodes that can reach nodeID in deterministic order.
func (g *Graph) TransitiveDependents(nodeID string, kinds ...EdgeKind) []Node {
	if g == nil {
		return nil
	}

	visited := make(map[string]struct{})
	queue := []string{nodeID}
	visited[nodeID] = struct{}{}

	var reached []string
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		for _, edge := range g.InEdges(curr, kinds...) {
			if _, ok := visited[edge.Source]; !ok {
				visited[edge.Source] = struct{}{}
				queue = append(queue, edge.Source)
				reached = append(reached, edge.Source)
			}
		}
	}
	sort.Strings(reached)

	out := make([]Node, 0, len(reached))
	for _, id := range reached {
		if n, ok := g.nodes[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

// DirectlyReaches reports whether sourceID has an outgoing edge to targetID of kinds.
func (g *Graph) DirectlyReaches(sourceID, targetID string, kinds ...EdgeKind) bool {
	if g == nil {
		return false
	}
	for _, edge := range g.OutEdges(sourceID, kinds...) {
		if edge.Target == targetID {
			return true
		}
	}
	return false
}

// TransitivelyReaches reports whether targetID is reachable from sourceID along edges of kinds.
func (g *Graph) TransitivelyReaches(sourceID, targetID string, kinds ...EdgeKind) bool {
	for _, dep := range g.TransitiveDependencies(sourceID, kinds...) {
		if dep.ID == targetID {
			return true
		}
	}
	return false
}

// InPackage returns a sub-graph containing only nodes whose PackageID matches
// any of the provided patterns (supports /... glob convention), and the edges
// connecting those retained nodes.
func (g *Graph) InPackage(patterns ...string) *Graph {
	if g == nil {
		return nil
	}
	if len(patterns) == 0 {
		return g
	}

	sub := newGraph()
	for _, n := range g.nodes {
		if common.PackageMatchesAny(n.PackageID, patterns...) {
			sub.addNode(n)
		}
	}

	for _, edges := range g.outEdges {
		for _, edge := range edges {
			if sub.HasNode(edge.Source) && sub.HasNode(edge.Target) {
				sub.addEdge(edge)
			}
		}
	}
	sub.finalize()
	return sub
}

func matchEdgeKind(kind EdgeKind, kinds []EdgeKind) bool {
	if len(kinds) == 0 {
		return true
	}
	return slices.Contains(kinds, kind)
}

func newGraph() *Graph {
	return &Graph{
		nodes:       make(map[string]Node),
		outEdges:    make(map[string][]Edge),
		inEdges:     make(map[string][]Edge),
		nodesByKind: make(map[NodeKind][]string),
	}
}

func (g *Graph) addNode(n Node) {
	if _, exists := g.nodes[n.ID]; exists {
		return
	}
	g.nodes[n.ID] = n
	g.nodesByKind[n.Kind] = append(g.nodesByKind[n.Kind], n.ID)
}

func (g *Graph) addEdge(e Edge) {
	g.outEdges[e.Source] = append(g.outEdges[e.Source], e)
	g.inEdges[e.Target] = append(g.inEdges[e.Target], e)
}

func (g *Graph) finalize() {
	g.sortedNodes = make([]string, 0, len(g.nodes))
	for id := range g.nodes {
		g.sortedNodes = append(g.sortedNodes, id)
	}
	sort.Strings(g.sortedNodes)

	for k := range g.nodesByKind {
		sort.Strings(g.nodesByKind[k])
	}

	edgeLess := func(a, b Edge) bool {
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.Kind < b.Kind
	}

	for src := range g.outEdges {
		sort.Slice(g.outEdges[src], func(i, j int) bool {
			return edgeLess(g.outEdges[src][i], g.outEdges[src][j])
		})
	}
	for dst := range g.inEdges {
		sort.Slice(g.inEdges[dst], func(i, j int) bool {
			return edgeLess(g.inEdges[dst][i], g.inEdges[dst][j])
		})
	}
}
