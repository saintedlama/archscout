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
	require.Len(t, refs, 3, "expected 3 fmt.Errorf refs")

	var sawApp, sawInfra, sawSub bool
	for _, f := range refs {
		normalized := strings.ReplaceAll(f.Filename, "\\", "/")
		if strings.HasSuffix(normalized, "/application/service.go") {
			sawApp = true
		}
		if strings.HasSuffix(normalized, "/infrastructure/repo.go") {
			sawInfra = true
		}
		if strings.HasSuffix(normalized, "/subpkg/sub.go") {
			sawSub = true
		}
	}

	assert.True(t, sawApp, "did not find fmt.Errorf in application/service.go")
	assert.True(t, sawInfra, "did not find fmt.Errorf in infrastructure/repo.go")
	assert.True(t, sawSub, "did not find fmt.Errorf in subpkg/sub.go")
}

func TestFunctionCalls_InPackageAndNotInPackage_CanBeChained(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	// Only application-layer fmt.Errorf calls (excluding infrastructure and subpkg).
	refs := workspace.FunctionCalls.
		InPackage("example.com/fixturemod/application").
		Match(func(call functioncalls.Item) bool {
			return call.Callee == "fmt.Errorf"
		})

	require.Len(t, refs, 1, "expected 1 fmt.Errorf call in application layer")

	normalized := strings.ReplaceAll(refs[0].Filename, "\\", "/")
	assert.True(t, strings.HasSuffix(normalized, "/application/service.go"), "expected ref in application/service.go")
}

func TestFunctionCalls_InTestAndNotInTest_FilterByTestFilenames(t *testing.T) {
	calls := functioncalls.NewCollection([]functioncalls.Item{
		{Ref: common.Ref{Filename: "pkg/file.go"}, Callee: "fmt.Println"},
		{Ref: common.Ref{Filename: "pkg/file_test.go"}, Callee: "fmt.Println"},
	})

	assert.Len(t, calls.IsTest().All(), 1, "expected one test call entry")
	assert.Len(t, calls.IsNotTest().All(), 1, "expected one non-test call entry")
}
