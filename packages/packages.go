package packages

import (
	"go/ast"
	"go/token"

	"github.com/saintedlama/archscout/common"
	"github.com/saintedlama/archscout/dependencies"

	toolspackages "golang.org/x/tools/go/packages"
)

// File wraps one parsed Go source file in a package.
type File struct {
	Filename string
	Node     *ast.File
}

// Item represents one loaded package entry.
type Item struct {
	ID      string
	Name    string
	FileSet *token.FileSet
	Files   []File
	Errors  []toolspackages.Error
	deps    dependencies.Collection
}

// Dependencies returns dependency entries originating from files in this package.
func (item Item) Dependencies() dependencies.Collection {
	return item.deps
}

// WithDependencies returns a copy of the item with the provided dependencies attached.
func (item Item) WithDependencies(items []dependencies.Item) Item {
	item.deps = dependencies.NewCollection(items)
	return item
}

// MatchFunc is a function type that matches package entries.
type MatchFunc func(Item) bool

// Collection stores package entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// NewCollection constructs an immutable package collection snapshot.
func NewCollection(items []Item) Collection {
	return Collection{items: append([]Item(nil), items...)}
}

// All returns a snapshot of all package entries.
func (c Collection) All() []Item {
	return append([]Item(nil), c.items...)
}

// Len returns number of package entries.
func (c Collection) Len() int {
	return len(c.items)
}

// InPackage returns a filtered collection containing only items in matching package patterns.
// A pattern ending in "/..." matches the base package and all of its sub-packages.
func (c Collection) InPackage(patterns ...string) Collection {
	if len(patterns) == 0 {
		return c
	}

	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !common.PackageMatchesAny(item.ID, patterns...) {
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
		if common.PackageMatchesAny(item.ID, patterns...) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsTest returns a filtered collection containing only items with at least one _test.go file.
func (c Collection) IsTest() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !packageHasTestFile(item) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsNotTest returns a filtered collection excluding items that contain _test.go files.
func (c Collection) IsNotTest() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if packageHasTestFile(item) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// InTest is an alias for IsTest kept for backward compatibility.
func (c Collection) InTest() Collection {
	return c.IsTest()
}

// NotInTest is an alias for IsNotTest kept for backward compatibility.
func (c Collection) NotInTest() Collection {
	return c.IsNotTest()
}

// Match applies matcher to all package entries and converts matches into code refs.
func (c Collection) Match(matcher MatchFunc) common.Refs {
	if matcher == nil {
		return nil
	}

	var refs common.Refs
	for _, item := range c.items {
		if !matcher(item) {
			continue
		}
		refs = append(refs, packageRef(item))
	}

	return refs
}

// GroupBy partitions the collection into sub-collections keyed by the return
// value of key. Items for which key returns an empty string are silently
// dropped. The returned map is never nil.
func (c Collection) GroupBy(key func(Item) string) map[string]Collection {
	groups := make(map[string][]Item)
	for _, item := range c.items {
		k := key(item)
		if k == "" {
			continue
		}
		groups[k] = append(groups[k], item)
	}

	result := make(map[string]Collection, len(groups))
	for k, items := range groups {
		result[k] = Collection{items: items}
	}

	return result
}

func packageRef(item Item) common.Ref {
	ref := common.Ref{
		PackageID:   item.ID,
		PackageName: item.Name,
		Kind:        common.RefKindPackage,
		Match:       "package " + item.Name,
	}

	if len(item.Files) > 0 {
		ref.Filename = item.Files[0].Filename
	}

	if item.FileSet != nil && len(item.Files) > 0 && item.Files[0].Node != nil {
		pos := item.FileSet.PositionFor(item.Files[0].Node.Name.Pos(), true)
		if pos.Filename != "" {
			ref.Filename = pos.Filename
		}
		if pos.Line > 0 {
			ref.Line = pos.Line
			ref.Column = pos.Column
		}
	}

	if ref.Line == 0 {
		ref.Line = 1
		ref.Column = 1
	}

	return ref
}

func packageHasTestFile(item Item) bool {
	for _, file := range item.Files {
		if common.IsTestFilename(file.Filename) {
			return true
		}
	}

	return false
}
