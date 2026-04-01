package dependencies

import (
	"sort"
	"strings"

	"github.com/saintedlama/archscout/common"
)

// Item represents one file import dependency.
type Item struct {
	Ref               common.Ref
	ImportPath        string
	WithinWorkspace   bool
	External          bool
	StandardLibrary   bool
	TargetPackageName string
}

// MatchFunc is a function type that matches dependency entries.
type MatchFunc func(Item) bool

// Collection stores dependency entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// TreeNode represents one node in the dependency import-path hierarchy.
//
// Dependencies are attached to the node that exactly matches their full import path.
// Intermediate nodes are path segments used for grouping.
type TreeNode struct {
	Name         string
	Path         string
	Dependencies []Item
	Children     []TreeNode
}

type treeNodeBuilder struct {
	name         string
	path         string
	dependencies []Item
	children     map[string]*treeNodeBuilder
}

func newTreeNodeBuilder(name, path string) *treeNodeBuilder {
	return &treeNodeBuilder{
		name:     name,
		path:     path,
		children: make(map[string]*treeNodeBuilder),
	}
}

func (builder *treeNodeBuilder) child(name, path string) *treeNodeBuilder {
	next, ok := builder.children[name]
	if ok {
		return next
	}

	next = newTreeNodeBuilder(name, path)
	builder.children[name] = next
	return next
}

func (builder *treeNodeBuilder) Build() TreeNode {
	childNames := make([]string, 0, len(builder.children))
	for name := range builder.children {
		childNames = append(childNames, name)
	}
	sort.Strings(childNames)

	children := make([]TreeNode, 0, len(childNames))
	for _, name := range childNames {
		children = append(children, builder.children[name].Build())
	}

	return TreeNode{
		Name:         builder.name,
		Path:         builder.path,
		Dependencies: append([]Item(nil), builder.dependencies...),
		Children:     children,
	}
}

// NewCollection constructs an immutable dependency collection snapshot.
func NewCollection(items []Item) Collection {
	return Collection{items: append([]Item(nil), items...)}
}

// All returns a snapshot of all dependency entries.
func (c Collection) All() []Item {
	return append([]Item(nil), c.items...)
}

// Len returns number of dependency entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Tree builds an import-path hierarchy tree from dependencies in the collection.
//
// Example import path "github.com/acme/service" becomes:
// root -> "github.com" -> "acme" -> "service" (leaf with dependency entries).
func (c Collection) Tree() TreeNode {
	root := newTreeNodeBuilder("", "")

	for _, item := range c.items {
		if item.ImportPath == "" {
			continue
		}

		parts := strings.Split(item.ImportPath, "/")
		current := root
		path := ""

		for i, part := range parts {
			if part == "" {
				continue
			}

			if path == "" {
				path = part
			} else {
				path += "/" + part
			}

			current = current.child(part, path)

			if i == len(parts)-1 {
				current.dependencies = append(current.dependencies, item)
			}
		}
	}

	return root.Build()
}

// InPackage returns a filtered collection containing only items in matching package patterns.
// A pattern ending in "/..." matches the base package and all of its sub-packages.
func (c Collection) InPackage(patterns ...string) Collection {
	if len(patterns) == 0 {
		return c
	}

	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !common.PackageMatchesAny(item.Ref.PackageID, patterns...) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// NotInPackage returns a filtered collection excluding items in matching package patterns.
// A pattern ending in "/..." matches the base package and all of its sub-packages.
func (c Collection) NotInPackage(patterns ...string) Collection {
	if len(patterns) == 0 {
		return c
	}

	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if common.PackageMatchesAny(item.Ref.PackageID, patterns...) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsTest returns a filtered collection containing only items from _test.go files.
func (c Collection) IsTest() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !common.IsTestFilename(item.Ref.Filename) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsNotTest returns a filtered collection excluding items from _test.go files.
func (c Collection) IsNotTest() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if common.IsTestFilename(item.Ref.Filename) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsWithinWorkspace returns a filtered collection with dependencies targeting analyzed workspace packages.
func (c Collection) IsWithinWorkspace() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !item.WithinWorkspace {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsExternal returns a filtered collection with dependencies targeting packages outside the analyzed workspace.
func (c Collection) IsExternal() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !item.External {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsStandardLibrary returns a filtered collection with dependencies targeting Go standard library packages.
func (c Collection) IsStandardLibrary() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !item.StandardLibrary {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsThirdParty returns a filtered collection with dependencies targeting third-party packages
// (external packages that are not part of the Go standard library).
func (c Collection) IsThirdParty() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !item.External || item.StandardLibrary {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// DependOn returns a filtered collection containing only items whose import path matches any pattern.
// A pattern ending in "/..." matches the base path and all sub-paths.
func (c Collection) DependOn(patterns ...string) Collection {
	if len(patterns) == 0 {
		return c
	}

	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !common.PackageMatchesAny(item.ImportPath, patterns...) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// DoNotDependOn returns a filtered collection excluding items whose import path matches any pattern.
// A pattern ending in "/..." matches the base path and all sub-paths.
func (c Collection) DoNotDependOn(patterns ...string) Collection {
	if len(patterns) == 0 {
		return c
	}

	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if common.PackageMatchesAny(item.ImportPath, patterns...) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// Match applies matcher to all dependency entries and converts matches into code refs.
func (c Collection) Match(matcher MatchFunc) common.Refs {
	if matcher == nil {
		return nil
	}

	var refs common.Refs
	for _, item := range c.items {
		if !matcher(item) {
			continue
		}
		refs = append(refs, item.Ref)
	}

	return refs
}
