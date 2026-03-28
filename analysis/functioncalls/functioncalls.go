package functioncalls

import (
	"go/ast"

	"github.com/saintedlama/goarch/analysis/common"
)

// Item represents a function call entry.
type Item struct {
	Ref    common.Ref
	Callee string
	Node   *ast.CallExpr
}

// Matcher matches function call entries and provides a finding message.
type Matcher interface {
	MatchFunctionCall(Item) (bool, string)
}

// MatchFunc adapts a function into a Matcher.
type MatchFunc func(Item) (bool, string)

func (f MatchFunc) MatchFunctionCall(i Item) (bool, string) {
	return f(i)
}

// Collection stores call entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// All returns all function call entries.
func (c Collection) All() []Item {
	return c.items
}

// Len returns number of function call entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Match applies matcher to all function call entries and converts matches into findings.
func (c Collection) Match(matcher Matcher) []common.Finding {
	if matcher == nil {
		return nil
	}

	var findings []common.Finding
	for _, item := range c.items {
		ok, msg := matcher.MatchFunctionCall(item)
		if !ok {
			continue
		}
		findings = append(findings, common.FindingFromRef(item.Ref, common.MessageOrDefault(msg, "function call matched")))
	}

	return findings
}

// Add appends an entry to the collection.
func (c *Collection) Add(item Item) {
	c.items = append(c.items, item)
}
