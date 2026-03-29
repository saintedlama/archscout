# goarch

`goarch` is a small Go library for loading source code with `go/packages` and exposing simple AST-backed collections for analysis.

This project is an experiment to explore low-fidelity code exploration and testing architectures: fast, coarse-grained structural indexing first, then matcher-based checks layered on top.

## Status

Experimental. APIs and behavior may change as ideas are tested.

## What it indexes

After loading a module, `goarch` builds top-level collections for:

- Packages
- Files
- Types
- Functions
- Variables
- Function calls

Each collection supports a fluent `Match(...)` API that returns code refs with source references.

## Install

```bash
go get github.com/saintedlama/goarch
```

## Public API

- `LoadWorkspace(ctx, dir, opts...) (*Workspace, error)`
- `WithReporter(func(string)) LoadWorkspaceOption`
- `WithInMemoryCache() LoadWorkspaceOption`
- Workspace matcher methods:
  - `workspace.MatchPackages(...)`
  - `workspace.MatchFiles(...)`
  - `workspace.MatchTypes(...)`
  - `workspace.MatchFunctions(...)`
  - `workspace.MatchVariables(...)`
  - `workspace.MatchFunctionCalls(...)`

## Quick start

```go
package architecture_test

import (
  "context"
  "fmt"
  "testing"

  "github.com/saintedlama/goarch"
)

func TestNoFmtErrorfCalls(t *testing.T) {
  workspace, err := goarch.LoadWorkspace(
    context.Background(),
    ".",
    goarch.WithReporter(func(msg string) {
      fmt.Println(msg)
    }),
  )
  if err != nil {
    t.Fatalf("LoadWorkspace failed: %v", err)
  }

  refs := workspace.MatchFunctionCalls(
    goarch.FunctionCallMatchFunc(func(c goarch.FunctionCall) bool {
      if c.Callee == "fmt.Errorf" {
        return true
      }
      return false
    }),
  )

  if len(refs) == 0 {
    return
  }

  for _, f := range refs {
    t.Errorf("%s:%d:%d package=%s", f.Filename, f.Line, f.Column, f.PackageName)
  }
}
```

If you do not want progress output:

```go
workspace, err := goarch.LoadWorkspace(context.Background(), ".")
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
- Progress reporting is optional via `goarch.WithReporter(func(string) { ... })`.
- In-memory caching is optional via `goarch.WithInMemoryCache()` and reuses an already loaded workspace by path.
- Package loading is based on `golang.org/x/tools/go/packages` for more precise module-aware parsing than ad-hoc file parsing.
