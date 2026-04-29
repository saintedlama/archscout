package functions

import (
	"go/ast"
	"regexp"
	"strings"

	"github.com/saintedlama/archscout/common"
)

// Item represents a function or method declaration entry.
//
// QName is the canonical fully-qualified name composed by archscout from
// the package import path, the receiver's named type (if any), and the
// function name:
//
//	plain function:  "<importpath>.<Name>"
//	method:          "<importpath>.<RecvType>.<Name>"  (pointer indirection on the receiver is stripped)
//
// QName is always populated. It's the join key for cross-collection
// references: callees in functioncalls.Item.CalleeQName, methods listed
// in types.Item.Methods, and callers stored in
// functioncalls.Item.CallerQName all match this string.
type Item struct {
	Ref      common.Ref
	Name     string
	QName    string
	Receiver string
	Node     *ast.FuncDecl
}

// MatchFunc is a function type that matches function entries.
type MatchFunc func(Item) bool

// Collection stores function entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// NewCollection constructs an immutable function collection snapshot.
func NewCollection(items []Item) Collection {
	return Collection{items: append([]Item(nil), items...)}
}

// All returns a snapshot of all function entries.
func (c Collection) All() []Item {
	return append([]Item(nil), c.items...)
}

// Len returns number of function entries.
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

// IsExported returns a filtered collection containing only exported functions and methods
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

// IsUnexported returns a filtered collection containing only unexported functions and methods
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

// IsMethod returns a filtered collection containing only method declarations
// (those with a non-empty receiver).
func (c Collection) IsMethod() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if item.Receiver == "" {
			continue
		}
		filtered = append(filtered, item)
	}
	return Collection{items: filtered}
}

// IsFunction returns a filtered collection containing only free-function declarations
// (those without a receiver).
func (c Collection) IsFunction() Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if item.Receiver != "" {
			continue
		}
		filtered = append(filtered, item)
	}
	return Collection{items: filtered}
}

// HasReceiver returns a filtered collection containing only methods whose receiver
// type contains pattern. For example, HasReceiver("Service") matches both
// "Service" and "*Service".
func (c Collection) HasReceiver(pattern string) Collection {
	filtered := make([]Item, 0, len(c.items))
	for _, item := range c.items {
		if !strings.Contains(item.Receiver, pattern) {
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

// Match applies matcher to all function entries and converts matches into code refs.
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
