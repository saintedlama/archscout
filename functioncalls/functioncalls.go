package functioncalls

import (
	"go/ast"

	"github.com/saintedlama/archscout/common"
)

// Item represents a function call entry.
//
// Callee is the syntactic callee text taken from the source
// (e.g. "fmt.Errorf", "c.Sign"). It is always populated.
//
// CalleePackage, CalleeQName and CalleeIsMethod are populated only when the
// workspace was loaded with archscout.WithTypeInfo(). They carry the resolved
// import path of the defining package and the fully-qualified name of the
// callee. For methods, CalleeQName has the form
// "<importpath>.<TypeName>.<MethodName>" with any pointer indirection on the
// receiver stripped. For plain functions it is "<importpath>.<FuncName>".
// CalleePackage is empty for callees defined in the universe scope.
//
// CallerName and CallerReceiver identify the function declaration that
// lexically encloses the call site. They are empty when the call appears
// at package level (e.g. inside a var/const initializer). For a method,
// CallerReceiver mirrors the raw receiver text from the function entry —
// for example, "*System" or "System".
//
// CallerQName is the canonical fully-qualified name of the enclosing
// function, composed identically to functions.Item.QName. It is empty
// for calls at package level. CallerQName is what consumers should
// match against to attribute calls to functions; CallerName and
// CallerReceiver remain for callers that need the unqualified pieces.
type Item struct {
	Ref            common.Ref
	Callee         string
	CalleePackage  string
	CalleeQName    string
	CalleeIsMethod bool
	CallerName     string
	CallerReceiver string
	CallerQName    string
	Node           *ast.CallExpr
}

// MatchFunc is a function type that matches function call entries.
type MatchFunc func(Item) bool

// Collection stores call entries and provides convenience query APIs.
type Collection struct {
	items []Item
}

// NewCollection constructs an immutable function call collection snapshot.
func NewCollection(items []Item) Collection {
	return Collection{items: append([]Item(nil), items...)}
}

// All returns a snapshot of all function call entries.
func (c Collection) All() []Item {
	return append([]Item(nil), c.items...)
}

// Len returns number of function call entries.
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

// Match applies matcher to all function call entries and converts matches into code refs.
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
