package archscout_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/stretchr/testify/assert"
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

	for _, want := range collectionPackages {
		refs := ws.MatchPackages(func(pkg archscout.Package) bool {
			return pkg.ID == want
		})
		assert.NotEmpty(t, refs, "expected package %q not found in workspace", want)
	}
}

// TestArch_CollectionPackagesDefineRequiredTypes verifies that every collection
// sub-package exports Item, Collection, and MatchFunc types.
func TestArch_CollectionPackagesDefineRequiredTypes(t *testing.T) {
	ws := loadWorkspace(t)

	required := []string{"Item", "Collection", "MatchFunc"}

	for _, pkg := range collectionPackages {
		for _, typeName := range required {
			refs := ws.MatchTypes(func(typ archscout.Type) bool {
				return typ.Ref.PackageID == pkg && typ.Name == typeName
			})
			assert.NotEmpty(t, refs, "package %q is missing required exported type %q", pkg, typeName)
		}
	}
}

// TestArch_CollectionPackagesDefineRequiredMethods verifies that every collection
// sub-package has All, Len, and Match methods on its Collection type.
func TestArch_CollectionPackagesDefineRequiredMethods(t *testing.T) {
	ws := loadWorkspace(t)

	required := []string{"All", "Len", "Match"}

	for _, pkg := range collectionPackages {
		for _, method := range required {
			refs := ws.MatchFunctions(func(fn archscout.Function) bool {
				return fn.Ref.PackageID == pkg &&
					fn.Name == method &&
					strings.Contains(fn.Receiver, "Collection")
			})
			assert.NotEmpty(t, refs, "package %q Collection is missing required method %q", pkg, method)
		}
	}
}

// TestArch_LibraryCodeDoesNotCallPanicOrExit verifies that non-internal, non-test
// library packages never call panic or os.Exit.
func TestArch_LibraryCodeDoesNotCallPanicOrExit(t *testing.T) {
	ws := loadWorkspace(t)

	forbidden := []string{"panic", "os.Exit"}
	rule := archscout.Rule("panic and os.Exit forbidden in library code").
		FunctionCalls().
		InPackage("github.com/saintedlama/archscout/...").
		NotInPackage("github.com/saintedlama/archscout/internal/...").
		IsNotTest().
		Match(func(fc archscout.FunctionCall) bool {
			if !slices.Contains(forbidden, fc.Callee) {
				return false
			}

			return true
		})

	rule.Test(t, ws)
}
