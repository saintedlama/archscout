package types

import (
	"go/ast"

	"github.com/saintedlama/goarch/analysis/common"
)

// Item represents a type declaration entry.
type Item struct {
	Ref  common.Ref
	Name string
	Kind string
	Node *ast.TypeSpec
}

// Matcher matches type entries and provides a finding message.
type Matcher interface {
	MatchType(Item) (bool, string)
}

// MatchFunc adapts a function into a Matcher.
type MatchFunc func(Item) (bool, string)

func (f MatchFunc) MatchType(i Item) (bool, string) {
	return f(i)
}

// Collection stores type entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// All returns all type entries.
func (c Collection) All() []Item {
	return c.items
}

// Len returns number of type entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Match applies matcher to all type entries and converts matches into findings.
func (c Collection) Match(matcher Matcher) []common.Finding {
	if matcher == nil {
		return nil
	}

	var findings []common.Finding
	for _, item := range c.items {
		ok, msg := matcher.MatchType(item)
		if !ok {
			continue
		}
		findings = append(findings, common.FindingFromRef(item.Ref, common.MessageOrDefault(msg, "type matched")))
	}

	return findings
}

// Add appends an entry to the collection.
func (c *Collection) Add(item Item) {
	c.items = append(c.items, item)
}
