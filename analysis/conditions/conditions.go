package conditions

import (
	"go/ast"

	"github.com/saintedlama/goarch/analysis/common"
)

// Item represents a conditional control-flow entry.
type Item struct {
	Ref  common.Ref
	Kind string
	Node ast.Node
}

// Matcher matches condition entries.
type Matcher interface {
	MatchCondition(Item) bool
}

// MatchFunc adapts a function into a Matcher.
type MatchFunc func(Item) bool

func (f MatchFunc) MatchCondition(i Item) bool {
	return f(i)
}

// Collection stores condition entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// All returns all condition entries.
func (c Collection) All() []Item {
	return c.items
}

// Len returns number of condition entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Match applies matcher to all condition entries and converts matches into code refs.
func (c Collection) Match(matcher Matcher) []common.Ref {
	if matcher == nil {
		return nil
	}

	var refs []common.Ref
	for _, item := range c.items {
		if !matcher.MatchCondition(item) {
			continue
		}
		refs = append(refs, item.Ref)
	}

	return refs
}

// Add appends an entry to the collection.
func (c *Collection) Add(item Item) {
	c.items = append(c.items, item)
}
