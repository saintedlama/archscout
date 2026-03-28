package functioncalls_test

import (
	"strings"
	"testing"

	"github.com/saintedlama/goarch/analysis/functioncalls"
	"github.com/saintedlama/goarch/analysis/internaltest"
)

func TestFunctionCalls_FindsExpectedFmtErrorfCalls(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.FunctionCalls.Match(functioncalls.MatchFunc(func(call functioncalls.Item) bool {
		if call.Callee != "fmt.Errorf" {
			return false
		}
		return true
	}))
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}

	var sawRoot, sawSub bool
	for _, f := range refs {
		if strings.HasSuffix(strings.ReplaceAll(f.Filename, "\\", "/"), "/main.go") {
			sawRoot = true
		}
		if strings.HasSuffix(strings.ReplaceAll(f.Filename, "\\", "/"), "/subpkg/sub.go") {
			sawSub = true
		}
	}

	if !sawRoot {
		t.Fatalf("did not find fmt.Errorf in fixture main.go")
	}
	if !sawSub {
		t.Fatalf("did not find fmt.Errorf in fixture subpkg/sub.go")
	}
}
