package implementsgraph

import (
	gotypes "go/types"
	"sort"

	"github.com/saintedlama/archscout/common"
)

// Graph stores interface-implementation edges over a fixed set of go/types
// packages. Edges are computed once at Build time.
type Graph struct {
	implementers map[string][]implementer
	interfaces   map[string][]string
}

// implementer pairs a concrete type's qname with its defining package path.
type implementer struct {
	qname       string
	packagePath string
}

// Build constructs an interface-implementation graph from the supplied
// go/types packages. Workspaces loaded without archscout.WithTypeInfo() pass
// nil here; the resulting graph is empty. Empty interfaces (interface{} / any) are skipped.
func Build(pkgs []*gotypes.Package) *Graph {
	g := &Graph{
		implementers: make(map[string][]implementer),
		interfaces:   make(map[string][]string),
	}
	if len(pkgs) == 0 {
		return g
	}

	type namedType struct {
		named *gotypes.Named
	}
	var interfaces, concretes []namedType

	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}
		scope := pkg.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			tn, ok := obj.(*gotypes.TypeName)
			if !ok {
				continue
			}
			named, ok := tn.Type().(*gotypes.Named)
			if !ok {
				continue
			}
			switch named.Underlying().(type) {
			case *gotypes.Interface:
				interfaces = append(interfaces, namedType{named: named})
			default:
				concretes = append(concretes, namedType{named: named})
			}
		}
	}

	for _, iface := range interfaces {
		ifaceTy, _ := iface.named.Underlying().(*gotypes.Interface)
		if ifaceTy == nil || ifaceTy.NumMethods() == 0 {
			continue
		}
		ifaceQName := qnameOf(iface.named)

		seen := make(map[string]struct{})
		for _, c := range concretes {
			if !typeImplements(c.named, ifaceTy) {
				continue
			}
			cQName := qnameOf(c.named)
			if _, dup := seen[cQName]; dup {
				continue
			}
			seen[cQName] = struct{}{}

			g.implementers[ifaceQName] = append(g.implementers[ifaceQName], implementer{
				qname:       cQName,
				packagePath: packagePathOf(c.named),
			})
			g.interfaces[cQName] = append(g.interfaces[cQName], ifaceQName)
		}
	}

	for k := range g.implementers {
		sort.Slice(g.implementers[k], func(i, j int) bool {
			return g.implementers[k][i].qname < g.implementers[k][j].qname
		})
	}
	for k := range g.interfaces {
		sort.Strings(g.interfaces[k])
	}
	return g
}

// Implementers returns the sorted, deduplicated list of concrete-type qnames
// that satisfy the given interface. Filters the result to types whose
// defining package matches any of the supplied patterns; an empty pattern
// list matches everything. Supports the /... glob convention.
//
// A type that satisfies the interface only via its pointer method set is
// still listed. Callers that need to distinguish value- from pointer-only
// implementations should inspect the underlying *types.Named themselves.
func (g *Graph) Implementers(ifaceQName string, patterns ...string) []string {
	if g == nil {
		return nil
	}
	impls, ok := g.implementers[ifaceQName]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(impls))
	for _, impl := range impls {
		if len(patterns) > 0 && !common.PackageMatchesAny(impl.packagePath, patterns...) {
			continue
		}
		out = append(out, impl.qname)
	}
	return out
}

// Interfaces returns the sorted list of interface qnames satisfied by the
// given concrete type.
func (g *Graph) Interfaces(typeQName string) []string {
	if g == nil {
		return nil
	}
	return append([]string(nil), g.interfaces[typeQName]...)
}

// typeImplements reports whether the named type satisfies the interface
// either directly (value-receiver method set) or via its pointer type.
func typeImplements(t *gotypes.Named, iface *gotypes.Interface) bool {
	if gotypes.Implements(t, iface) {
		return true
	}
	return gotypes.Implements(gotypes.NewPointer(t), iface)
}

// qnameOf renders a named type as "<importpath>.<TypeName>". When the type
// belongs to the universe scope (e.g. error, builtin types), the import
// path is omitted.
func qnameOf(named *gotypes.Named) string {
	obj := named.Obj()
	if obj.Pkg() == nil {
		return obj.Name()
	}
	return obj.Pkg().Path() + "." + obj.Name()
}

// packagePathOf returns the import path of the package that defines named.
// Empty string for universe-scope types.
func packagePathOf(named *gotypes.Named) string {
	obj := named.Obj()
	if obj.Pkg() == nil {
		return ""
	}
	return obj.Pkg().Path()
}
