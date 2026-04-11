# ![archscout logo](./assets/logo.png) ArchScout

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

## Why use Archscout

Most Go architecture tools focus on a narrow slice of the problem: check a dependency, enforce a layer rule, done. Archscout is different in four important ways.

### 1. Explore first, enforce second

Understanding a codebase matters as much as policing it. Archscout ships exploration helpers — `UniqueTargets()`, `UniqueSourcePackages()`, `GroupBySourcePackage()`, `GroupByTargetPackage()` — designed for asking questions like "who imports my domain layer?" or "what does the UI layer actually reach?" Most tools give you a pass/fail assertion. Archscout also gives you the map.

### 2. Seven collections, one mental model

Every code element — packages, files, types, functions, variables, function calls, and raw import dependencies — is a filterable, chainable collection with the same API. You don't learn a separate DSL per check. You learn `InPackage`, `IsNotTest`, `Match` once and apply them everywhere. Checking for `panic` calls uses exactly the same pattern as checking dependency boundaries.

### 3. Transitive graph analysis

Archscout builds a proper directed dependency graph with `BuildPackageGraph`, letting you ask transitive questions: does the domain layer _ever_ reach infrastructure, through any number of hops? Which packages are reachable from the UI layer? Who (directly or transitively) imports the domain? Other tools check direct edges only.

### 4. No boilerplate, no configuration

Load a workspace, write a Go test function, call `.Test(t, workspace)`. No layer definitions to register upfront, no config files, no parsing phases to manage manually. Rules are plain Go values — they compose, they can be shared across test files, and they live exactly where your tests live.

### 5. The AST is yours

Archscout is a thin layer over Go's own analysis tooling — it doesn't hide the underlying code model behind opaque abstractions. Every `Match` predicate receives a real typed value (`Type`, `Function`, `Variable`, `FunctionCall`, `Dependency`) that you can inspect with plain Go code. If the built-in filters don't cover your case, you reach into the item directly:

```go
import "github.com/saintedlama/archscout"

// Find all exported functions whose name starts with "New" but have no receiver —
// a check no built-in rule needs to exist for.
refs := workspace.Functions.
  InPackage("github.com/your-project/...").
  IsNotTest().
  Match(func(f archscout.Function) bool {
    return len(f.Name) >= 3 &&
      f.Name[:3] == "New" &&
      f.Receiver == "" &&
      f.Name[0] >= 'A' && f.Name[0] <= 'Z'
  })
```

There is no "escape hatch" needed — the item **is** the data. This makes archscout equally useful for ad-hoc exploration and for hardening automation that runs in CI.

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
import "github.com/saintedlama/archscout"

refs := workspace.FunctionCalls.
  InPackage("github.com/your-project/...").
  IsNotTest().
  Match(func(call archscout.FunctionCall) bool {
    return call.Callee == "fmt.Errorf"
  })
```

### 2. Validate architecture with reusable rules

```go
import "github.com/saintedlama/archscout"

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

### 3. Assert existence

Use `ShouldExist()` on `Packages`, `Types`, and `Functions` rules to assert that at
least one entry survives the filter chain. Combine with `Match` to pin a specific item:

```go
import "github.com/saintedlama/archscout"

archscout.Rule("domain package must exist").
  Packages().
  InPackage("github.com/your-project/domain").
  ShouldExist().
  Test(t, workspace)

archscout.Rule("Repository interface must be defined in domain").
  Types().
  InPackage("github.com/your-project/domain").
  ShouldExist().
  Match(func(t archscout.Type) bool { return t.Name == "Repository" }).
  Test(t, workspace)
```

### 4. Reason about dependencies

Dependency checks can be done directly or through files/packages.

```go
import "github.com/saintedlama/archscout"

rule := archscout.Rule("files with no stdlib deps").
  Files().
  Match(func(file archscout.File) bool {
    return file.Dependencies().IsStandardLibrary().Len() == 0
  })

rule.Test(t, workspace)
```

For hierarchy-style reporting, use `workspace.Dependencies.Tree()`.

### 5. Explore dependencies in large codebases

Three aggregation helpers make it easy to answer high-level questions without
counting raw import statements:

```go
import (
  "fmt"

  "github.com/saintedlama/archscout"
)

mod := archscout.Module("github.com/your-project")

// What does the UI layer reach (workspace-internal, non-test)?
targets := workspace.Dependencies.
  InPackage(mod.Pkg("ui/...")).
  IsNotTest().
  IsWithinWorkspace().
  UniqueTargets()
// → ["github.com/your-project/audio", "github.com/your-project/domain", ...]

// Who imports the domain layer?
importers := workspace.Dependencies.
  DependOn(mod.Pkg("domain/...")).
  IsNotTest().
  UniqueSourcePackages()
// → ["github.com/your-project/application", "github.com/your-project/ui/tracker", ...]

// Full per-package breakdown
for pkg, deps := range workspace.Dependencies.IsNotTest().IsWithinWorkspace().GroupBySourcePackage() {
  fmt.Printf("%s → %v\n", pkg, deps.UniqueTargets())
}
```

### 6. Reduce repetition with Module

Use `Module` to avoid repeating the module path across patterns:

```go
import "github.com/saintedlama/archscout"

mod := archscout.Module("github.com/your-project")

archscout.Rule("ui/common must not depend on other internal packages").
  Dependencies().
  InPackage(mod.Pkg("ui/common/...")).
  IsNotTest().
  DependOn(mod.Pkgs(
    "audio/...",
    "persistence/...",
    "player/...",
  )...).
  Test(t, workspace)
```

`mod.Pkg("sub/path")` returns a single fully-qualified pattern.
`mod.Pkgs("a/...", "b/...")` returns a `[]string` of fully-qualified patterns.

## What You Can Query

`archscout` exposes seven collections on `Workspace`:

| Field           | Item type      | Notable fields                                                                      |
| --------------- | -------------- | ----------------------------------------------------------------------------------- |
| `Packages`      | `Package`      | `ID`, `Name`, `Files`, `Dependencies()`                                             |
| `Files`         | `File`         | `Filename`, `Dependencies()`                                                        |
| `Types`         | `Type`         | `Name`, `Kind`                                                                      |
| `Functions`     | `Function`     | `Name`, `Receiver`                                                                  |
| `Variables`     | `Variable`     | `Name`, `Kind`                                                                      |
| `FunctionCalls` | `FunctionCall` | `Callee`                                                                            |
| `Dependencies`  | `Dependency`   | `ImportPath`, `WithinWorkspace`, `External`, `StandardLibrary`, `TargetPackageName` |

All collections support:

| Method                      | Description                                             |
| --------------------------- | ------------------------------------------------------- |
| `All()`                     | Returns a snapshot slice of all items                   |
| `Len()`                     | Number of items                                         |
| `Match(func)`               | Applies a predicate; returns matching `Refs`            |
| `InPackage(patterns...)`    | Keeps items whose source package matches any pattern    |
| `NotInPackage(patterns...)` | Excludes items whose source package matches any pattern |
| `IsTest()`                  | Keeps items from `_test.go` files                       |
| `IsNotTest()`               | Excludes items from `_test.go` files                    |

Dependencies additionally support:

| Method                       | Description                                               |
| ---------------------------- | --------------------------------------------------------- |
| `DependOn(patterns...)`      | Keeps items whose import path matches any pattern         |
| `DependsOn(pattern)`         | Keeps items whose import path matches a single pattern    |
| `DoNotDependOn(patterns...)` | Excludes items whose import path matches any pattern      |
| `IsWithinWorkspace()`        | Keeps imports that resolve to workspace packages          |
| `IsExternal()`               | Keeps imports that resolve outside the workspace          |
| `IsStandardLibrary()`        | Keeps standard library imports                            |
| `IsThirdParty()`             | Keeps external, non-stdlib imports                        |
| `UniqueTargets()`            | Sorted, deduplicated import paths in the collection       |
| `UniqueSourcePackages()`     | Sorted, deduplicated source package IDs in the collection |
| `GroupBySourcePackage()`     | Partitions into one sub-collection per source package     |
| `GroupByTargetPackage()`     | Partitions into one sub-collection per imported package   |
| `Tree()`                     | Builds a hierarchical `TreeNode` from import paths        |

### 7. Speed up repeated loads with a disk cache

Enable disk cache to make repeated `LoadWorkspace` calls much faster on large
codebases, especially when exploring them interactively. Cache entries are
invalidated automatically when `.go` files or `go.sum` change.

Use zero-config caching:

```go
import (
  "context"

  "github.com/saintedlama/archscout"
)

workspace, err := archscout.LoadWorkspace(
    context.Background(), ".",
    archscout.WithDiskCache(),
    archscout.WithReporter(func(msg string) { fmt.Println(msg) }),
)
```

Use `WithDiskCacheDir(dir)` when you need an explicit location (for example in CI):

```go
workspace, err := archscout.LoadWorkspace(
    context.Background(), ".",
    archscout.WithDiskCacheDir("/tmp/my-project-cache"),
)
```

> **Note:** After loading from disk cache, AST `Node` fields are `nil`. Normal filters and rule checks continue to work.

### 8. Build and query the transitive package graph

`BuildPackageGraph` converts a dependency collection into a directed graph that
supports transitive reachability queries. It only considers workspace-internal
imports, so filter the collection first if needed:

```go
import "github.com/saintedlama/archscout"

mod := archscout.Module("github.com/your-project")

graph := archscout.BuildPackageGraph(
    workspace.Dependencies.IsNotTest().IsWithinWorkspace(),
)

// All workspace packages in the graph
pkgs := graph.Packages()

// Direct imports of the application layer
direct := graph.DirectDependencies(mod.Pkg("application/..."))

// Everything reachable (any number of hops) from the UI layer
all := graph.TransitiveDependencies(mod.Pkg("ui/..."))

// Does domain ever (transitively) reach infrastructure?
if graph.TransitivelyReaches(
    []string{mod.Pkg("domain/...")},
    []string{mod.Pkg("infrastructure/...")},
) {
    t.Error("domain must not depend on infrastructure")
}

// Single-hop version of the same check
if graph.DirectlyReaches(
    []string{mod.Pkg("application/...")},
    []string{mod.Pkg("infrastructure/...")},
) {
    t.Error("application must not directly import infrastructure")
}

// Who imports the domain layer?
importers := graph.Importers(mod.Pkg("domain/..."))
// → ["github.com/your-project/application", "github.com/your-project/ui/tracker"]
```

`PackageGraph` methods:

| Method                                          | Description                                                        |
| ----------------------------------------------- | ------------------------------------------------------------------ |
| `Packages()`                                    | Sorted set of all package IDs (sources and targets)                |
| `DirectDependencies(patterns...)`               | Packages directly imported by packages matching patterns           |
| `TransitiveDependencies(patterns...)`           | All packages reachable via one or more hops from matching packages |
| `TransitivelyReaches(fromPatterns, toPatterns)` | Reports whether any matching source can reach any matching target  |
| `DirectlyReaches(fromPatterns, toPatterns)`     | Same as above but only considers single-hop edges                  |
| `Importers(patterns...)`                        | Packages that directly import any package matching patterns        |

All methods support the `/...` glob convention.

## Refs and Formatting

Rule violations are returned as `Refs` — each `Ref` identifies a source location:

```go
import (
  "fmt"

  "github.com/saintedlama/archscout"
)

refs, err := rule.Evaluate(workspace)
fmt.Println(archscout.FormatRefs(refs))

// Customise output
fmt.Println(archscout.FormatRefs(refs,
  archscout.WithRefPackage(),
  archscout.WithRefKind(),
  archscout.WithoutRefColumn(),
))
```

Available format options: `WithRefPackage()`, `WithRefKind()`, `WithoutRefFile()`,
`WithoutRefLine()`, `WithoutRefColumn()`, `WithoutRefMatch()`, `WithRefSeparator(sep)`,
`WithoutSeparator()`.

## Public API

- `LoadWorkspace(ctx, dir, opts...) (*Workspace, error)`
- `WithReporter(func(string)) LoadWorkspaceOption` — progress callback
- `WithInMemoryCache() LoadWorkspaceOption` — reuse a loaded workspace within the process
- `WithDiskCache() LoadWorkspaceOption` — persist a workspace snapshot in the platform-default cache directory
- `WithDiskCacheDir(dir string) LoadWorkspaceOption` — persist a workspace snapshot in an explicit directory
- `Module(path)` — helper for building fully-qualified package patterns
- `BuildPackageGraph(c dependencies.Collection) *PackageGraph` — builds a transitive package graph from a dependency collection
- `Rule(name)` — entry point for all rule construction

Rule types expose:

- fluent filters (package/test and kind-specific filters)
- `ShouldExist()` — assert at least one match exists (`Packages`, `Types`, `Functions`)
- `Match(func)`
- `Evaluate(workspace) (Refs, error)`
- `Test(t, workspace)` — fails the test if any refs are returned (or none when `ShouldExist`)

## Development

```bash
make fmt
make vet
make lint
make build
make test-verbose
```

## Notes

- `LoadWorkspace` expects a Go module directory with `go.mod`.
- `WithReporter(...)` is optional and useful for progress output.
- `WithInMemoryCache()` is optional and reuses a loaded workspace by path.
- `WithDiskCache()` is optional; stores cache files in `os.UserCacheDir()/archscout`
  (falls back to `os.TempDir()/archscout-cache`). Different projects sharing the
  same cache directory never collide because the absolute project path is part of
  the fingerprint hash.
- `WithDiskCacheDir(dir)` is optional; identical to `WithDiskCache()` but lets
  you control exactly where cache files are written.
- On a cache hit, go/ast `Node` fields (`Function.Node`, `Type.Node`, etc.) are
  `nil`. All string-based queries, filter chains, and rule checks work normally;
  only custom predicates that dereference the raw AST pointer are affected.
- Pattern matching: a pattern ending in `/...` matches the base path and all sub-paths.

## License

This project is licensed under the MIT License. See `LICENSE` for details.
