package functions

import (
	"go/ast"

	"github.com/saintedlama/goarch/analysis/common"
)

// Item represents a function or method declaration entry.
type Item struct {
	Ref      common.Ref
	Name     string
	Receiver string
	Node     *ast.FuncDecl
}

// Matcher matches function entries and provides a finding message.
type Matcher interface {
	MatchFunction(Item) (bool, string)
}

// MatchFunc adapts a function into a Matcher.
type MatchFunc func(Item) (bool, string)

func (f MatchFunc) MatchFunction(i Item) (bool, string) {
	return f(i)
}

// Collection stores function entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// All returns all function entries.
func (c Collection) All() []Item {
	return c.items
}

// Len returns number of function entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Match applies matcher to all function entries and converts matches into findings.
func (c Collection) Match(matcher Matcher) []common.Finding {
	if matcher == nil {
		return nil
	}

	var findings []common.Finding
	for _, item := range c.items {
		ok, msg := matcher.MatchFunction(item)
		if !ok {
			continue
		}
		findings = append(findings, common.FindingFromRef(item.Ref, common.MessageOrDefault(msg, "function matched")))
	}

	return findings
}

// Add appends an entry to the collection.
func (c *Collection) Add(item Item) {
	c.items = append(c.items, item)
}
