package goarch_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/saintedlama/goarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectionPackages are the sub-packages that each define the
// Item / Collection / MatchFunc triad and must follow the read-only collection pattern.
var collectionPackages = []string{
	"github.com/saintedlama/goarch/files",
	"github.com/saintedlama/goarch/functions",
	"github.com/saintedlama/goarch/functioncalls",
	"github.com/saintedlama/goarch/packages",
	"github.com/saintedlama/goarch/types",
	"github.com/saintedlama/goarch/variables",
}

func loadWorkspace(t *testing.T) *goarch.Workspace {
	t.Helper()

	ws, err := goarch.LoadWorkspace(context.Background(), ".", goarch.WithInMemoryCache())
	require.NoError(t, err, "failed to load goarch workspace")

	return ws
}

// TestArch_AllCollectionPackagesExist verifies that each expected collection
// sub-package is present in the workspace.
func TestArch_AllCollectionPackagesExist(t *testing.T) {
	ws := loadWorkspace(t)

	for _, want := range collectionPackages {
		refs := ws.MatchPackages(func(pkg goarch.Package) bool {
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
			refs := ws.MatchTypes(func(typ goarch.Type) bool {
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
			refs := ws.MatchFunctions(func(fn goarch.Function) bool {
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

	refs := ws.FunctionCalls.
		InPackage("github.com/saintedlama/goarch/...").
		NotInPackage("github.com/saintedlama/goarch/internal/...").
		IsNotTest().
		Match(func(fc goarch.FunctionCall) bool {
			if !slices.Contains(forbidden, fc.Callee) {
				return false
			}

			return true
		})

	assert.Empty(t, refs, "panic and os.Exit forbidden in library code violated:\n%s", refs.Format())
}
