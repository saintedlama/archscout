# archscout

`archscout` helps you keep architecture visible and enforceable in Go codebases.

Use it to:

- Explore code structure quickly (packages, files, types, calls, dependencies)
- Write architecture tests as code
- Validate dependency boundaries continuously in CI

## Why archscout

Architecture often lives in docs, not in tests. `archscout` lets you move those rules into executable checks.

Examples:

- "domain must not depend on infrastructure"
- "library code must not call panic or os.Exit"
- "application layer may only depend on domain"

When a rule is violated, you get source refs you can print in test failures.

## Install

```bash
go get github.com/saintedlama/archscout
```

## Quick Start

```go
package architecture_test

import (
  "context"
  "testing"

  "github.com/saintedlama/archscout"
)

func TestDomainDoesNotDependOnInfrastructure(t *testing.T) {
  workspace, err := archscout.LoadWorkspace(context.Background(), ".")
  if err != nil {
    t.Fatalf("LoadWorkspace failed: %v", err)
  }

  rule := archscout.Rule("domain must not depend on infrastructure").
    Dependencies().
    InPackage("github.com/your-project/domain/...").
    DependOn("github.com/your-project/infrastructure/...")

  rule.Test(t, workspace)
}
```

## Core Workflows

### 1. Explore a codebase

```go
refs := workspace.FunctionCalls.
  InPackage("github.com/your-project/...").
  IsNotTest().
  Match(func(call archscout.FunctionCall) bool {
    return call.Callee == "fmt.Errorf"
  })
```

### 2. Validate architecture with reusable rules

```go
forbidden := map[string]bool{"panic": true, "os.Exit": true}

rule := archscout.Rule("panic and os.Exit forbidden in library code").
  FunctionCalls().
  InPackage("github.com/your-project/...").
  NotInPackage("github.com/your-project/internal/...").
  IsNotTest().
  Match(func(fc archscout.FunctionCall) bool {
    return forbidden[fc.Callee]
  })

rule.Test(t, workspace)
```

### 3. Reason about dependencies

Dependency checks can be done directly or through files/packages.

```go
rule := archscout.Rule("files with stdlib deps").
  Files().
  Match(func(file archscout.File) bool {
    return file.Dependencies().IsStandardLibrary().Len() == 0
  })

rule.Test(t, workspace)
```

For hierarchy-style reporting, use `workspace.Dependencies.Tree()`.

## What You Can Query

`archscout` exposes collections for:

- Packages
- Files
- Types
- Functions
- Variables
- Function calls
- Dependencies

Each collection supports:

- `Match(...)` for custom predicates
- package filtering via `InPackage(...)` / `NotInPackage(...)`
- test-file filtering via `IsTest()` / `IsNotTest()`

Dependencies additionally support:

- `IsWithinWorkspace()` / `IsExternal()`
- `IsStandardLibrary()` / `IsThirdParty()`
- `DependOn(...)` / `DoNotDependOn(...)`
- `Tree()`

## Public API

- `LoadWorkspace(ctx, dir, opts...) (*Workspace, error)`
- `WithReporter(func(string)) LoadWorkspaceOption`
- `WithInMemoryCache() LoadWorkspaceOption`
- `Rule(name)`

Rule types expose:

- fluent filters (package/test and kind-specific filters)
- `Match(...)`
- `Evaluate(workspace)`
- `Test(t, workspace)`

## Development

```bash
make fmt
make vet
make build
make test-verbose
```

## Notes

- `LoadWorkspace` expects a Go module directory with `go.mod`.
- `WithReporter(...)` is optional and useful for progress output.
- `WithInMemoryCache()` is optional and reuses a loaded workspace by path.

## License

This project is licensed under the MIT License. See `LICENSE` for details.
