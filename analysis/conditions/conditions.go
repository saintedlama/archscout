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

// Matcher matches condition entries and provides a finding message.
type Matcher interface {
	MatchCondition(Item) (bool, string)
}

// MatchFunc adapts a function into a Matcher.
type MatchFunc func(Item) (bool, string)

func (f MatchFunc) MatchCondition(i Item) (bool, string) {
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

// Match applies matcher to all condition entries and converts matches into findings.
func (c Collection) Match(matcher Matcher) []common.Finding {
	if matcher == nil {
		return nil
	}

	var findings []common.Finding
	for _, item := range c.items {
		ok, msg := matcher.MatchCondition(item)
		if !ok {
			continue
		}
		findings = append(findings, common.FindingFromRef(item.Ref, common.MessageOrDefault(msg, "condition matched")))
	}

	return findings
}

// Add appends an entry to the collection.
func (c *Collection) Add(item Item) {
	c.items = append(c.items, item)
}
