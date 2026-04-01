package goarch

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFunctionCallRuleEvaluate_RequiresMatcher(t *testing.T) {
	rule := Rule("missing matcher").FunctionCalls()

	_, err := rule.evaluate(&Workspace{})
	if err == nil {
		t.Fatal("expected error when matcher is missing")
	}
}

func TestFunctionCallRuleEvaluate_AppliesConfiguredFilters(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	rule := Rule("root-only fmt.Errorf").
		FunctionCalls().
		InPackage("example.com/fixturemod/...").
		NotInPackage("example.com/fixturemod/subpkg/...").
		Match(func(fc FunctionCall) bool {
			return fc.Callee == "fmt.Errorf"
		})

	refs, err := rule.evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}

	normalized := strings.ReplaceAll(refs[0].Filename, "\\", "/")
	if !strings.HasSuffix(normalized, "/main.go") {
		t.Fatalf("expected ref in /main.go, got %q", normalized)
	}
}

func loadFixtureWorkspace(t *testing.T, fixtureName string) *Workspace {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	dir := filepath.Join(filepath.Dir(filename), "testdata", fixtureName)
	workspace, err := LoadWorkspace(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadWorkspace(%q) failed: %v", dir, err)
	}

	return workspace
}
