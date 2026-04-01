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
