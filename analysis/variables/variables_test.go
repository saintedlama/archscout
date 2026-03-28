package variables_test

import (
	"testing"

	"github.com/saintedlama/goarch/analysis/internaltest"
	"github.com/saintedlama/goarch/analysis/variables"
)

func TestVariables_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Variables.Match(variables.MatchFunc(func(v variables.Item) bool {
		return v.Name == "GlobalCounter"
	}))
	if len(refs) != 1 {
		t.Fatalf("expected 1 variable ref, got %d", len(refs))
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
