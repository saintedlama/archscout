package archscout_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/stretchr/testify/require"
)

// collectionPackages are the sub-packages that each define the
// Item / Collection / MatchFunc triad and must follow the read-only collection pattern.
var collectionPackages = []string{
	"github.com/saintedlama/archscout/files",
	"github.com/saintedlama/archscout/functions",
	"github.com/saintedlama/archscout/functioncalls",
	"github.com/saintedlama/archscout/packages",
	"github.com/saintedlama/archscout/types",
	"github.com/saintedlama/archscout/variables",
}

func loadWorkspace(t *testing.T) *archscout.Workspace {
	t.Helper()

	ws, err := archscout.LoadWorkspace(context.Background(), ".", archscout.WithInMemoryCache())
	require.NoError(t, err, "failed to load archscout workspace")

	return ws
}

// TestArch_AllCollectionPackagesExist verifies that each expected collection
// sub-package is present in the workspace.
func TestArch_AllCollectionPackagesExist(t *testing.T) {
	ws := loadWorkspace(t)

	for _, pkg := range collectionPackages {
		t.Run(pkg, func(t *testing.T) {
			archscout.Rule(fmt.Sprintf("package %q should exist in workspace", pkg)).
				Packages().
				InPackage(pkg).
				ShouldExist().
				Test(t, ws)
		})
	}
}

// TestArch_CollectionPackagesDefineRequiredTypes verifies that every collection
// sub-package exports Item, Collection, and MatchFunc types.
func TestArch_CollectionPackagesDefineRequiredTypes(t *testing.T) {
	ws := loadWorkspace(t)

	type typeExpectation struct {
		pkg      string
		typeName string
	}

	var expectations []typeExpectation
	for _, pkg := range collectionPackages {
		for _, typeName := range []string{"Item", "Collection", "MatchFunc"} {
			expectations = append(expectations, typeExpectation{pkg: pkg, typeName: typeName})
		}
	}

	for _, tc := range expectations {
		t.Run(tc.pkg+"/"+tc.typeName, func(t *testing.T) {
			archscout.Rule(fmt.Sprintf("package %q should define type %q", tc.pkg, tc.typeName)).
				Types().
				InPackage(tc.pkg).
				ShouldExist().
				Match(func(typ archscout.Type) bool {
					return typ.Name == tc.typeName
				}).
				Test(t, ws)
		})
	}
}

// TestArch_CollectionPackagesDefineRequiredMethods verifies that every collection
// sub-package has All, Len, and Match methods on its Collection type.
func TestArch_CollectionPackagesDefineRequiredMethods(t *testing.T) {
	ws := loadWorkspace(t)

	type methodExpectation struct {
		pkg    string
		method string
	}

	var expectations []methodExpectation
	for _, pkg := range collectionPackages {
		for _, method := range []string{"All", "Len", "Match"} {
			expectations = append(expectations, methodExpectation{pkg: pkg, method: method})
		}
	}

	for _, tc := range expectations {
		t.Run(tc.pkg+"/"+tc.method, func(t *testing.T) {
			archscout.Rule(fmt.Sprintf("package %q Collection should define method %q", tc.pkg, tc.method)).
				Functions().
				InPackage(tc.pkg).
				ShouldExist().
				Match(func(fn archscout.Function) bool {
					return fn.Name == tc.method && strings.Contains(fn.Receiver, "Collection")
				}).
				Test(t, ws)
		})
	}
}

// TestArch_LibraryCodeDoesNotCallPanicOrExit verifies that non-internal, non-test
// library packages never call panic or os.Exit.
func TestArch_LibraryCodeDoesNotCallPanicOrExit(t *testing.T) {
	ws := loadWorkspace(t)

	forbidden := []string{"panic", "os.Exit"}
	rule := archscout.Rule("library code should not panic or os.Exit").
		FunctionCalls().
		InPackage("github.com/saintedlama/archscout/...").
		NotInPackage("github.com/saintedlama/archscout/internal/...").
		IsNotTest().
		Match(func(fc archscout.FunctionCall) bool {
			return slices.Contains(forbidden, fc.Callee)
		})

	rule.Test(t, ws)
}
