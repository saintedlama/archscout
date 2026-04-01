package files_test

import (
	"strings"
	"testing"

	"github.com/saintedlama/archscout/files"
	"github.com/saintedlama/archscout/internaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiles_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Files.Match(func(f files.Item) bool {
		normalized := strings.ReplaceAll(f.Filename, "\\", "/")
		return strings.HasSuffix(normalized, "/main.go")
	})
	require.NotEmpty(t, refs, "expected to find fixture main.go")

	for _, ref := range refs {
		assert.NotEmpty(t, ref.PackageName, "ref package should not be empty")
		assert.Greater(t, ref.Line, 0, "ref line should be > 0")
	}
}
