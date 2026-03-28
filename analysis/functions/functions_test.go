package functions_test

import (
	"testing"

	"github.com/saintedlama/goarch/analysis/functions"
	"github.com/saintedlama/goarch/analysis/internaltest"
)

func TestFunctions_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Functions.Match(functions.MatchFunc(func(fn functions.Item) bool {
		return fn.Name == "RootErr"
	}))
	if len(refs) != 1 {
		t.Fatalf("expected 1 function ref, got %d", len(refs))
	}

	for _, f := range refs {
		if f.PackageName == "" {
			t.Fatalf("ref package should not be empty")
		}
		if f.Line <= 0 {
			t.Fatalf("ref line should be > 0")
		}
	}
}
