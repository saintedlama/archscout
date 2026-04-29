package types

import (
	"go/ast"
	"regexp"

	"github.com/saintedlama/archscout/common"
)

// Item represents a type declaration entry.
//
// QName is the canonical fully-qualified name "<importpath>.<Name>" and
// is always populated. It's the join key against
// functions.Item.Receiver / functions.Item.QName, types.Item.Embeds, and
// types.Item.Methods (which use it for the receiver-prefix component).
//
// Fields lists the declared fields of a struct, including embedded
// entries (Embedded == true, Name == "").
//
// Methods lists the methods declared directly on an interface. Methods
// contributed by embedded interfaces are not flattened into this list;
// follow Embeds to discover them.
//
// Embeds lists the type identifiers of embedded fields (for structs) or
// embedded interfaces (for interfaces). When the workspace was loaded
// with archscout.WithTypeInfo() the entries are fully-qualified
// (e.g. "io.Reader"); otherwise they are syntactic ("Reader" or
// "io.Reader" depending on how the source wrote the embed).
type Item struct {
	Ref     common.Ref
	Name    string
	QName   string
	Kind    string
	Fields  []FieldInfo
	Methods []MethodInfo
	Embeds  []string
	Node    *ast.TypeSpec
}

// FieldInfo describes a single struct field.
type FieldInfo struct {
	Name      string
	TypeName  string
	TypeQName string
	Tag       string
	Embedded  bool
}

// MethodInfo describes a method declared directly on an interface. QName is
// populated only when the workspace was loaded with archscout.WithTypeInfo().
type MethodInfo struct {
	Name  string
	QName string
}

// MatchFunc is a function type that matches type entries.
type MatchFunc func(Item) bool

// Collection stores type entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// NewCollection constructs an immutable type collection snapshot.
func NewCollection(items []Item) Collection {
	return Collection{items: append([]Item(nil), items...)}
}

// All returns a snapshot of all type entries.
func (c Collection) All() []Item {
	return append([]Item(nil), c.items...)
}

// Len returns number of type entries.
func (c Collection) Len() int {
	return len(c.items)
}

// InPackage returns a filtered collection containing only items in matching package patterns.
// A pattern ending in "/..." matches the base package and all of its sub-packages.
func (c Collection) InPackage(patterns ...string) Collection {
	if len(patterns) == 0 {
		return c
	}

	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !common.PackageMatchesAny(item.Ref.PackageID, patterns...) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// NotInPackage returns a filtered collection excluding items in matching package patterns.
// A pattern ending in "/..." matches the base package and all of its sub-packages.
func (c Collection) NotInPackage(patterns ...string) Collection {
	if len(patterns) == 0 {
		return c
	}

	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if common.PackageMatchesAny(item.Ref.PackageID, patterns...) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsTest returns a filtered collection containing only items from _test.go files.
func (c Collection) IsTest() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !common.IsTestFilename(item.Ref.Filename) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// IsNotTest returns a filtered collection excluding items from _test.go files.
func (c Collection) IsNotTest() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if common.IsTestFilename(item.Ref.Filename) {
			continue
		}
		filtered = append(filtered, item)
	}

	return Collection{items: filtered}
}

// InTest is an alias for IsTest kept for backward compatibility.
func (c Collection) InTest() Collection {
	return c.IsTest()
}

// NotInTest is an alias for IsNotTest kept for backward compatibility.
func (c Collection) NotInTest() Collection {
	return c.IsNotTest()
}

// IsExported returns a filtered collection containing only exported types
// (names starting with an uppercase letter).
func (c Collection) IsExported() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !common.IsExportedName(item.Name) {
			continue
		}
		filtered = append(filtered, item)
	}
	return Collection{items: filtered}
}

// IsUnexported returns a filtered collection containing only unexported types
// (names starting with a lowercase letter).
func (c Collection) IsUnexported() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if common.IsExportedName(item.Name) {
			continue
		}
		filtered = append(filtered, item)
	}
	return Collection{items: filtered}
}

// NameMatches returns a filtered collection containing only items whose name satisfies fn.
func (c Collection) NameMatches(fn func(string) bool) Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !fn(item.Name) {
			continue
		}
		filtered = append(filtered, item)
	}
	return Collection{items: filtered}
}

// NameMatchesRegex returns a filtered collection containing only items whose name
// matches the regular expression. Panics if the pattern is not valid.
func (c Collection) NameMatchesRegex(pattern string) Collection {
	return c.NameMatches(regexp.MustCompile(pattern).MatchString)
}

// Match applies matcher to all type entries and converts matches into code refs.
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
