package goarch_test

import (
	"context"
	"strings"
	"testing"

	"github.com/saintedlama/goarch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectionPackages are the sub-packages that each define the
// Item / Collection / MatchFunc triad and must follow the collection pattern.
var collectionPackages = []string{
	"github.com/saintedlama/goarch/functions",
	"github.com/saintedlama/goarch/functioncalls",
	"github.com/saintedlama/goarch/packages",
	"github.com/saintedlama/goarch/types",
	"github.com/saintedlama/goarch/variables",
}

func loadSelf(t *testing.T) *goarch.Workspace {
	t.Helper()

	ws, err := goarch.LoadWorkspace(context.Background(), ".")
	require.NoError(t, err, "failed to load goarch workspace")

	return ws
}

// TestArch_AllCollectionPackagesExist verifies that each expected collection
// sub-package is present in the workspace.
func TestArch_AllCollectionPackagesExist(t *testing.T) {
	ws := loadSelf(t)

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
	ws := loadSelf(t)

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
// sub-package has Add, All, Len, and Match methods on its Collection type.
func TestArch_CollectionPackagesDefineRequiredMethods(t *testing.T) {
	ws := loadSelf(t)

	required := []string{"Add", "All", "Len", "Match"}

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
	ws := loadSelf(t)

	forbidden := []string{"panic", "os.Exit"}

	for _, callee := range forbidden {
		refs := ws.MatchFunctionCalls(func(fc goarch.FunctionCall) bool {
			if fc.Callee != callee {
				return false
			}
			if strings.Contains(fc.Ref.PackageID, "/internal") {
				return false
			}
			if strings.HasSuffix(fc.Ref.Filename, "_test.go") {
				return false
			}
			return true
		})
		for _, ref := range refs {
			assert.Fail(t, "forbidden call in library code",
				"package %q calls %s at %s:%d", ref.PackageID, callee, ref.Filename, ref.Line)
		}
	}
}
