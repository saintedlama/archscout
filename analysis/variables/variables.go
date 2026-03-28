package variables

import (
	"go/ast"

	"github.com/saintedlama/goarch/analysis/common"
)

// Item represents a variable/constant declaration entry.
type Item struct {
	Ref  common.Ref
	Name string
	Kind string
	Node *ast.Ident
}

// Matcher matches variable entries and provides a finding message.
type Matcher interface {
	MatchVariable(Item) (bool, string)
}

// MatchFunc adapts a function into a Matcher.
type MatchFunc func(Item) (bool, string)

func (f MatchFunc) MatchVariable(i Item) (bool, string) {
	return f(i)
}

// Collection stores variable entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// All returns all variable entries.
func (c Collection) All() []Item {
	return c.items
}

// Len returns number of variable entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Match applies matcher to all variable entries and converts matches into findings.
func (c Collection) Match(matcher Matcher) []common.Finding {
	if matcher == nil {
		return nil
	}

	var findings []common.Finding
	for _, item := range c.items {
		ok, msg := matcher.MatchVariable(item)
		if !ok {
			continue
		}
		findings = append(findings, common.FindingFromRef(item.Ref, common.MessageOrDefault(msg, "variable matched")))
	}

	return findings
}

// Add appends an entry to the collection.
func (c *Collection) Add(item Item) {
	c.items = append(c.items, item)
}
