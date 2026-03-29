package variables_test

import (
	"testing"

	"github.com/saintedlama/goarch/internaltest"
	"github.com/saintedlama/goarch/variables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariables_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Variables.Match(func(v variables.Item) bool {
		return v.Name == "GlobalCounter"
	})
	require.Len(t, refs, 1, "expected 1 variable ref")

	for _, f := range refs {
		assert.NotEmpty(t, f.PackageName, "ref package should not be empty")
		assert.Greater(t, f.Line, 0, "ref line should be > 0")
	}
}
