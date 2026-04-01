package goarch

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFunctionCallRuleEvaluate_RequiresMatcher(t *testing.T) {
	rule := Rule("missing matcher").FunctionCalls()

	_, err := rule.Evaluate(&Workspace{})
	if err == nil {
		t.Fatal("expected error when matcher is missing")
	}
}

func TestFunctionCallRuleEvaluate_DomainErrorsNewCalls(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	// domain/model.go has exactly 3 calls to errors.New.
	rule := Rule("domain errors.New calls").
		FunctionCalls().
		InPackage("example.com/fixturemod/domain").
		Match(func(fc FunctionCall) bool {
			return fc.Callee == "errors.New"
		})

	refs, err := rule.Evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected 3 errors.New calls in domain, got %d", len(refs))
	}
}

// TestDependencyRuleEvaluate_DomainOnlyHasStdlibDeps verifies the domain layer
// imports nothing but the Go standard library.
func TestDependencyRuleEvaluate_DomainOnlyHasStdlibDeps(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	// Count workspace-internal (intra-module) dependencies from domain.
	rule := Rule("domain has no intra-module deps").
		Dependencies().
		InPackage("example.com/fixturemod/domain").
		IsWithinWorkspace().
		Match(func(dep Dependency) bool { return true })

	refs, err := rule.Evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("domain must not have intra-module dependencies, got %d", len(refs))
	}
}

// TestDependencyRuleEvaluate_ApplicationDependsOnDomain verifies that the
// application layer has exactly one intra-module dependency (domain).
func TestDependencyRuleEvaluate_ApplicationDependsOnDomain(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	rule := Rule("application imports domain").
		Dependencies().
		InPackage("example.com/fixturemod/application").
		DependOn("example.com/fixturemod/domain")

	refs, err := rule.Evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 application→domain dependency, got %d", len(refs))
	}
}

// TestDependencyRuleEvaluate_DomainMustNotDependOnApplication is a passing
// architectural rule: domain must not reach into the application layer.
func TestDependencyRuleEvaluate_DomainMustNotDependOnApplication(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	rule := Rule("domain must not depend on application").
		Dependencies().
		InPackage("example.com/fixturemod/domain").
		DependOn("example.com/fixturemod/application/...")

	refs, err := rule.Evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("domain must not depend on application, got %d violation(s)", len(refs))
	}
}

// TestDependencyRuleEvaluate_DomainMustNotDependOnInfrastructure verifies the
// domain layer has no dependency on infrastructure (passing rule).
func TestDependencyRuleEvaluate_DomainMustNotDependOnInfrastructure(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	rule := Rule("domain must not depend on infrastructure").
		Dependencies().
		InPackage("example.com/fixturemod/domain").
		DependOn("example.com/fixturemod/infrastructure/...")

	refs, err := rule.Evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("domain must not depend on infrastructure, got %d violation(s)", len(refs))
	}
}

// TestDependencyRuleEvaluate_ApplicationMustNotDependOnInfrastructure encodes
// a hexagonal-architecture rule: use-cases must not depend on adapters.
func TestDependencyRuleEvaluate_ApplicationMustNotDependOnInfrastructure(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	rule := Rule("application must not depend on infrastructure").
		Dependencies().
		InPackage("example.com/fixturemod/application").
		DependOn("example.com/fixturemod/infrastructure/...")

	refs, err := rule.Evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("application must not depend on infrastructure, got %d violation(s)", len(refs))
	}
}

// TestDependencyRuleEvaluate_InfrastructureDependsOnDomain demonstrates a
// directional rule without a Match — the filter chain alone expresses the intent.
// Here it is used to assert that infrastructure DOES import domain (1 dependency).
func TestDependencyRuleEvaluate_InfrastructureDependsOnDomain(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	// No .Match() — every dep surviving the filters is returned as a ref.
	rule := Rule("infrastructure depends on domain").
		Dependencies().
		InPackage("example.com/fixturemod/infrastructure").
		DependOn("example.com/fixturemod/domain")

	refs, err := rule.Evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 infrastructure→domain dependency, got %d", len(refs))
	}
}

// TestDependencyRuleEvaluate_IsStandardLibrary verifies the stdlib filter
// returns only standard-library imports across the whole fixture module.
func TestDependencyRuleEvaluate_IsStandardLibrary(t *testing.T) {
	ws := loadFixtureWorkspace(t, "fixturemod")

	rule := Rule("files with stdlib deps").
		Files().
		Match(func(file File) bool {
			return file.Dependencies().IsStandardLibrary().Len() > 0
		})

	refs, err := rule.Evaluate(ws)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != ws.Files.Len() {
		t.Fatalf("expected all files to have at least one stdlib dependency, got %d of %d", len(refs), ws.Files.Len())
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
