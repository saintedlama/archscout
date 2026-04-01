# archscout

`archscout` is a small Go library for loading source code with `go/packages` and exposing simple AST-backed collections for analysis.

This project is an experiment to explore low-fidelity code exploration and testing architectures: fast, coarse-grained structural indexing first, then matcher-based checks layered on top.

## Status

Experimental. APIs and behavior may change as ideas are tested.

## What it indexes

After loading a module, `archscout` builds top-level collections for:

- Packages
- Files
- Types
- Functions
- Variables
- Function calls
- Dependencies

Each collection supports a fluent `Match(...)` API that returns code refs with source references.
Refs also carry a kind-specific match label and can be rendered with `FormatRef` or `FormatRefs`.
Collections also support chainable package filters via `InPackage(...)` and `NotInPackage(...)`.
Use the `"/..."` suffix to match a package and all of its sub-packages.
Collections also support test-file filters via `IsTest()` and `IsNotTest()`.

## Install

```bash
go get github.com/saintedlama/archscout
```

## Public API

- `LoadWorkspace(ctx, dir, opts...) (*Workspace, error)`
- `WithReporter(func(string)) LoadWorkspaceOption`
- `WithInMemoryCache() LoadWorkspaceOption`
- `Rule(name)` to define workspace-independent reusable rules with `rule.Test(t, workspace)`
- Workspace matcher methods:
  - `workspace.MatchPackages(...)`
  - `workspace.MatchFiles(...)`
  - `workspace.MatchTypes(...)`
  - `workspace.MatchFunctions(...)`
  - `workspace.MatchVariables(...)`
  - `workspace.MatchFunctionCalls(...)`
  - `workspace.MatchDependencies(...)`

## Rules

Rules are configured independently of a workspace and can be reused across tests:

```go
forbidden := []string{"panic", "os.Exit"}

rule := archscout.Rule("panic and os.Exit forbidden in library code").
  FunctionCalls().
  InPackage("github.com/your-project/...").
  NotInPackage("github.com/your-project/internal/...").
  IsNotTest().
  Match(func(fc archscout.FunctionCall) bool {
    return slices.Contains(forbidden, fc.Callee)
  })

rule.Test(t, workspace)
```

## Quick start

```go
package architecture_test

import (
  "context"
  "fmt"
  "testing"

  "github.com/saintedlama/archscout"
)

func TestNoFmtErrorfCalls(t *testing.T) {
  workspace, err := archscout.LoadWorkspace(
    context.Background(),
    ".",
    archscout.WithReporter(func(msg string) {
      fmt.Println(msg)
    }),
  )
  if err != nil {
    t.Fatalf("LoadWorkspace failed: %v", err)
  }

  refs := workspace.FunctionCalls.
    InPackage("github.com/your-project/...").
    NotInPackage("github.com/your-project/internal/...").
    IsNotTest().
    Match(func(c archscout.FunctionCall) bool {
      if c.Callee == "fmt.Errorf" {
        return true
      }
      return false
    })

  if len(refs) == 0 {
    return
  }

  t.Fatalf("fmt.Errorf is forbidden in %s", refs.Format(archscout.WithRefPackage()))
}
```

If you do not want progress output:

```go
workspace, err := archscout.LoadWorkspace(context.Background(), ".")
```

Run it with:

```bash
go test ./...
```

## Development

Available `make` targets:

- `make fmt`
- `make vet`
- `make build`
- `make test-verbose`

CI runs these checks on pushes and pull requests.

## Notes

- `LoadWorkspace` expects a Go module directory (with `go.mod`).
- Progress reporting is optional via `archscout.WithReporter(func(string) { ... })`.
- In-memory caching is optional via `archscout.WithInMemoryCache()` and reuses an already loaded workspace by path.
- Package loading is based on `golang.org/x/tools/go/packages` for more precise module-aware parsing than ad-hoc file parsing.
