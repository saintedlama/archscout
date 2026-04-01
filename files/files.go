package files

import (
	"go/ast"

	"github.com/saintedlama/archscout/common"
	"github.com/saintedlama/archscout/dependencies"
)

// Item represents a parsed Go source file entry.
type Item struct {
	Ref      common.Ref
	Filename string
	Node     *ast.File
	deps     dependencies.Collection
}

// Dependencies returns dependency entries originating from this file.
func (item Item) Dependencies() dependencies.Collection {
	return item.deps
}

// WithDependencies returns a copy of the item with the provided dependencies attached.
func (item Item) WithDependencies(items []dependencies.Item) Item {
	item.deps = dependencies.NewCollection(items)
	return item
}

// MatchFunc is a function type that matches file entries.
type MatchFunc func(Item) bool

// Collection stores file entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// NewCollection constructs an immutable file collection snapshot.
func NewCollection(items []Item) Collection {
	return Collection{items: append([]Item(nil), items...)}
}

// All returns a snapshot of all file entries.
func (c Collection) All() []Item {
	return append([]Item(nil), c.items...)
}

// Len returns number of file entries.
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
		if !common.IsTestFilename(item.Filename) {
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
		if common.IsTestFilename(item.Filename) {
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

// Match applies matcher to all file entries and converts matches into code refs.
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
