package dependencies_test

import (
	"testing"

	"github.com/saintedlama/goarch/common"
	"github.com/saintedlama/goarch/dependencies"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDependencies_FiltersAndMatch(t *testing.T) {
	collection := dependencies.NewCollection([]dependencies.Item{
		{
			Ref:             common.Ref{PackageID: "example.com/fixturemod", Filename: "main.go"},
			ImportPath:      "fmt",
			WithinWorkspace: false,
			External:        true,
			StandardLibrary: true,
		},
		{
			Ref:             common.Ref{PackageID: "example.com/fixturemod/subpkg", Filename: "sub.go"},
			ImportPath:      "example.com/fixturemod/internalpkg",
			WithinWorkspace: true,
			External:        false,
			StandardLibrary: false,
		},
		{
			Ref:             common.Ref{PackageID: "example.com/fixturemod", Filename: "main_test.go"},
			ImportPath:      "testing",
			WithinWorkspace: false,
			External:        true,
			StandardLibrary: true,
		},
		{
			Ref:             common.Ref{PackageID: "example.com/fixturemod", Filename: "main.go"},
			ImportPath:      "github.com/someorg/somepkg",
			WithinWorkspace: false,
			External:        true,
			StandardLibrary: false,
		},
	})

	assert.Len(t, collection.InPackage("example.com/fixturemod/...").All(), 4)
	assert.Len(t, collection.NotInPackage("example.com/fixturemod/subpkg/...").All(), 3)
	assert.Len(t, collection.IsTest().All(), 1)
	assert.Len(t, collection.IsNotTest().All(), 3)
	assert.Len(t, collection.IsWithinWorkspace().All(), 1)
	assert.Len(t, collection.IsExternal().All(), 3)
	assert.Len(t, collection.IsStandardLibrary().All(), 2)
	assert.Len(t, collection.IsThirdParty().All(), 1)
	assert.Len(t, collection.DependOn("fmt").All(), 1)
	assert.Len(t, collection.DependOn("github.com/someorg/...").All(), 1)
	assert.Len(t, collection.DoNotDependOn("fmt").All(), 3)

	refs := collection.Match(func(item dependencies.Item) bool {
		return item.ImportPath == "fmt"
	})
	require.Len(t, refs, 1)
	assert.Equal(t, "main.go", refs[0].Filename)
}
