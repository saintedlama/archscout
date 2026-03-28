package types_test

import (
	"testing"

	"github.com/saintedlama/goarch/analysis/internaltest"
	"github.com/saintedlama/goarch/analysis/types"
)

func TestTypes_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Types.Match(types.MatchFunc(func(typ types.Item) bool {
		return typ.Name == "Widget"
	}))
	if len(refs) != 1 {
		t.Fatalf("expected 1 type ref, got %d", len(refs))
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
