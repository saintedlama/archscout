package dependencies_test

import (
	"testing"

	"github.com/saintedlama/goarch/common"
	"github.com/saintedlama/goarch/dependencies"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findChild(t *testing.T, node dependencies.TreeNode, name string) dependencies.TreeNode {
	t.Helper()
	for _, child := range node.Children {
		if child.Name == name {
			return child
		}
	}
	t.Fatalf("child %q not found under %q", name, node.Path)
	return dependencies.TreeNode{}
}

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

func TestDependencies_Tree_GroupsByImportPathHierarchy(t *testing.T) {
	collection := dependencies.NewCollection([]dependencies.Item{
		{ImportPath: "fmt"},
		{ImportPath: "github.com/someorg/somepkg"},
		{ImportPath: "github.com/someorg/otherpkg"},
		{ImportPath: "example.com/fixturemod/domain"},
		{ImportPath: "example.com/fixturemod/domain"},
	})

	tree := collection.Tree()
	require.Empty(t, tree.Name)
	require.Empty(t, tree.Path)

	// Root children are sorted by segment name.
	require.Len(t, tree.Children, 3)
	assert.Equal(t, "example.com", tree.Children[0].Name)
	assert.Equal(t, "fmt", tree.Children[1].Name)
	assert.Equal(t, "github.com", tree.Children[2].Name)

	fmtNode := findChild(t, tree, "fmt")
	assert.Equal(t, "fmt", fmtNode.Path)
	require.Len(t, fmtNode.Dependencies, 1)
	assert.Empty(t, fmtNode.Children)

	exampleNode := findChild(t, tree, "example.com")
	fixtureNode := findChild(t, exampleNode, "fixturemod")
	domainNode := findChild(t, fixtureNode, "domain")
	assert.Equal(t, "example.com/fixturemod/domain", domainNode.Path)
	assert.Len(t, domainNode.Dependencies, 2)

	githubNode := findChild(t, tree, "github.com")
	someOrgNode := findChild(t, githubNode, "someorg")
	require.Len(t, someOrgNode.Children, 2)
	assert.Equal(t, "otherpkg", someOrgNode.Children[0].Name)
	assert.Equal(t, "somepkg", someOrgNode.Children[1].Name)
}
