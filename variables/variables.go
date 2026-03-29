package variables

import (
	"go/ast"

	"github.com/saintedlama/goarch/common"
)

// Item represents a variable/constant declaration entry.
type Item struct {
	Ref  common.Ref
	Name string
	Kind string
	Node *ast.Ident
}

// MatchFunc is a function type that matches variable entries.
type MatchFunc func(Item) bool

// Collection stores variable entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// NewCollection constructs an immutable variable collection snapshot.
func NewCollection(items []Item) Collection {
	return Collection{items: append([]Item(nil), items...)}
}

// All returns a snapshot of all variable entries.
func (c Collection) All() []Item {
	return append([]Item(nil), c.items...)
}

// Len returns number of variable entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Match applies matcher to all variable entries and converts matches into code refs.
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
