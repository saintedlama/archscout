package functions_test

import (
	"testing"

	"github.com/saintedlama/archscout/functions"
	"github.com/saintedlama/archscout/internaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctions_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Functions.Match(func(fn functions.Item) bool {
		return fn.Name == "NewOrder"
	})
	require.Len(t, refs, 1, "expected 1 function ref")

	for _, f := range refs {
		assert.NotEmpty(t, f.PackageName, "ref package should not be empty")
		assert.Greater(t, f.Line, 0, "ref line should be > 0")
	}
}
