package functioncalls_test

import (
	"strings"
	"testing"

	"github.com/saintedlama/goarch/functioncalls"
	"github.com/saintedlama/goarch/internaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionCalls_FindsExpectedFmtErrorfCalls(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.FunctionCalls.Match(func(call functioncalls.Item) bool {
		return call.Callee == "fmt.Errorf"
	})
	require.Len(t, refs, 2, "expected 2 fmt.Errorf refs")

	var sawRoot, sawSub bool
	for _, f := range refs {
		normalized := strings.ReplaceAll(f.Filename, "\\", "/")
		if strings.HasSuffix(normalized, "/main.go") {
			sawRoot = true
		}
		if strings.HasSuffix(normalized, "/subpkg/sub.go") {
			sawSub = true
		}
	}

	assert.True(t, sawRoot, "did not find fmt.Errorf in fixture main.go")
	assert.True(t, sawSub, "did not find fmt.Errorf in fixture subpkg/sub.go")
}
