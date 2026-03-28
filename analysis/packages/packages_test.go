package packages_test

import (
	"testing"

	"github.com/saintedlama/goarch/analysis/internaltest"
	"github.com/saintedlama/goarch/analysis/packages"
)

func TestPackages_MatchBuildsRefsFromPredicates(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.Packages.Match(packages.MatchFunc(func(pkg packages.Item) bool {
		return pkg.Name == "main"
	}))
	if len(refs) == 0 {
		t.Fatalf("expected package refs")
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
