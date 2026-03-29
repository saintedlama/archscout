package files

import (
	"go/ast"

	"github.com/saintedlama/goarch/common"
)

// Item represents a parsed Go source file entry.
type Item struct {
	Ref      common.Ref
	Filename string
	Node     *ast.File
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
