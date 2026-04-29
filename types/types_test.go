package types_test

import (
	"testing"

	"github.com/saintedlama/archscout/internaltest"
	"github.com/saintedlama/archscout/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypes_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Types.Match(func(typ types.Item) bool {
		return typ.Name == "Order"
	})
	require.Len(t, refs, 1, "expected 1 type ref")

	for _, f := range refs {
		assert.NotEmpty(t, f.PackageName, "ref package should not be empty")
		assert.Greater(t, f.Line, 0, "ref line should be > 0")
	}
}

func TestTypes_QName_ComposedFromImportPathAndName(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	var sawOrder bool
	for _, ti := range workspace.Types.All() {
		if ti.Name == "Order" {
			sawOrder = true
			assert.Equal(t, "example.com/fixturemod/domain.Order", ti.QName)
		}
	}
	assert.True(t, sawOrder, "expected to see the Order type")
}

func TestTypes_IsExported_ReturnsOnlyExportedTypes(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	all := workspace.Types.All()
	exported := workspace.Types.IsExported().All()
	unexported := workspace.Types.IsUnexported().All()

	assert.NotEmpty(t, exported)
	assert.Equal(t, len(all), len(exported)+len(unexported), "exported + unexported should cover all types")
	for _, item := range exported {
		assert.True(t, item.Name[0] >= 'A' && item.Name[0] <= 'Z', "expected exported name, got %q", item.Name)
	}
}

func TestTypes_NameMatchesRegex_FiltersOnPattern(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Types.NameMatchesRegex(`^Order`).All()

	require.NotEmpty(t, refs)
	for _, item := range refs {
		assert.Contains(t, item.Name, "Order")
	}
}

func TestTypes_NameMatches_FiltersOnPredicate(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	items := workspace.Types.NameMatches(func(name string) bool {
		return name == "Order"
	}).All()

	require.Len(t, items, 1)
	assert.Equal(t, "Order", items[0].Name)
}
