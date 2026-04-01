package packages_test

import (
	"testing"

	"github.com/saintedlama/archscout/internaltest"
	"github.com/saintedlama/archscout/packages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackages_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Packages.Match(func(pkg packages.Item) bool {
		return pkg.Name == "main"
	})
	require.NotEmpty(t, refs, "expected at least one package ref")

	for _, f := range refs {
		assert.NotEmpty(t, f.PackageName, "ref package should not be empty")
		assert.Greater(t, f.Line, 0, "ref line should be > 0")
	}
}
