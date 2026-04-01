package functioncalls_test

import (
	"strings"
	"testing"

	"github.com/saintedlama/goarch/common"
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

func TestFunctionCalls_InPackageAndNotInPackage_CanBeChained(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.FunctionCalls.
		InPackage("example.com/fixturemod/...").
		NotInPackage("example.com/fixturemod/subpkg/...").
		Match(func(call functioncalls.Item) bool {
			return call.Callee == "fmt.Errorf"
		})

	require.Len(t, refs, 1, "expected only root package fmt.Errorf call after excluding subpkg")

	normalized := strings.ReplaceAll(refs[0].Filename, "\\", "/")
	assert.True(t, strings.HasSuffix(normalized, "/main.go"), "expected remaining ref to be in fixture main.go")
}

func TestFunctionCalls_InTestAndNotInTest_FilterByTestFilenames(t *testing.T) {
	calls := functioncalls.NewCollection([]functioncalls.Item{
		{Ref: common.Ref{Filename: "pkg/file.go"}, Callee: "fmt.Println"},
		{Ref: common.Ref{Filename: "pkg/file_test.go"}, Callee: "fmt.Println"},
	})

	assert.Len(t, calls.IsTest().All(), 1, "expected one test call entry")
	assert.Len(t, calls.IsNotTest().All(), 1, "expected one non-test call entry")
}
