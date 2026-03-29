package packages

import (
	"go/ast"
	"go/token"

	"github.com/saintedlama/goarch/common"

	toolspackages "golang.org/x/tools/go/packages"
)

// File wraps one parsed Go source file in a package.
type File struct {
	Filename string
	Node     *ast.File
}

// Item represents one loaded package entry.
type Item struct {
	ID      string
	Name    string
	FileSet *token.FileSet
	Files   []File
	Errors  []toolspackages.Error
}

// MatchFunc is a function type that matches package entries.
type MatchFunc func(Item) bool

// Collection stores package entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// NewCollection constructs an immutable package collection snapshot.
func NewCollection(items []Item) Collection {
	return Collection{items: append([]Item(nil), items...)}
}

// All returns a snapshot of all package entries.
func (c Collection) All() []Item {
	return append([]Item(nil), c.items...)
}

// Len returns number of package entries.
func (c Collection) Len() int {
	return len(c.items)
}

// Match applies matcher to all package entries and converts matches into code refs.
func (c Collection) Match(matcher MatchFunc) common.Refs {
	if matcher == nil {
		return nil
	}

	var refs common.Refs
	for _, item := range c.items {
		if !matcher(item) {
			continue
		}
		refs = append(refs, packageRef(item))
	}

	return refs
}

func packageRef(item Item) common.Ref {
	ref := common.Ref{
		PackageID:   item.ID,
		PackageName: item.Name,
		Kind:        common.RefKindPackage,
		Match:       "package " + item.Name,
	}

	if len(item.Files) > 0 {
		ref.Filename = item.Files[0].Filename
	}

	if item.FileSet != nil && len(item.Files) > 0 && item.Files[0].Node != nil {
		pos := item.FileSet.PositionFor(item.Files[0].Node.Name.Pos(), true)
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
