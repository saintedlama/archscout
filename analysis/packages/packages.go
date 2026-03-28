package packages

import (
	"go/ast"
	"go/token"

	"github.com/saintedlama/goarch/analysis/common"

	toolspackages "golang.org/x/tools/go/packages"
)

// File wraps one parsed Go source file in a package.
type File struct {
	Filename string
	Node     *ast.File
}

// Item represents one loaded package entry.
type Item struct {
	ID     string
	Name   string
	Fset   *token.FileSet
	Files  []File
	Errors []toolspackages.Error
}

// Matcher matches package entries.
type Matcher interface {
	MatchPackage(Item) bool
}

// MatchFunc adapts a function into a Matcher.
type MatchFunc func(Item) bool

func (f MatchFunc) MatchPackage(i Item) bool {
	return f(i)
}

// Collection stores package entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// All returns all package entries.
func (c Collection) All() []Item {
	return c.items
}

// Len returns number of package entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Match applies matcher to all package entries and converts matches into code refs.
func (c Collection) Match(matcher Matcher) []common.Ref {
	if matcher == nil {
		return nil
	}

	var refs []common.Ref
	for _, item := range c.items {
		if !matcher.MatchPackage(item) {
			continue
		}
		refs = append(refs, packageRef(item))
	}

	return refs
}

// Add appends an entry to the collection.
func (c *Collection) Add(item Item) {
	c.items = append(c.items, item)
}

func packageRef(item Item) common.Ref {
	ref := common.Ref{PackageID: item.ID, PackageName: item.Name}

	if len(item.Files) > 0 {
		ref.Filename = item.Files[0].Filename
	}

	if item.Fset != nil && len(item.Files) > 0 && item.Files[0].Node != nil {
		pos := item.Fset.PositionFor(item.Files[0].Node.Name.Pos(), true)
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
