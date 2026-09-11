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

### As a Library
```bash
go get github.com/saintedlama/archscout
```

### As a Go Tool (Go 1.24+)
Add `archscout` as a version-locked tool dependency to your project's `go.mod`:
```bash
go get -tool github.com/saintedlama/archscout/cmd/archscout
```
Then invoke it directly using `go tool`:
```bash
go tool archscout graph --sqlite .archscout/codegraph.db .
```

### As a Global CLI Binary
```bash
go install github.com/saintedlama/archscout/cmd/archscout@latest
archscout graph --help
```

## CLI Usage

When installed as a tool or binary, `archscout graph` lets you inspect, visualize, and persist the codebase graph directly from the terminal without writing code:

```bash
archscout graph [options] [path]
# or via go tool:
go tool archscout graph [options] [path]
```

### Options

| Option | Default | Description |
| --- | --- | --- |
| `-sqlite <file>` | `""` | Export a SQLite database with FTS5 search and vector tables for AI agents |
| `-format <fmt>` | `mermaid` | Output diagram/data format: `mermaid`, `dot`, `json` |
| `-out <file>` | `stdout` | Write output to a file instead of stdout |
| `-type-info` | `true` | Load Go type information for callee and interface resolution |
| `-node-kinds <list>` | all | Comma-separated node kinds: `module,package,file,type,function` |
| `-edge-kinds <list>` | all | Comma-separated edge kinds: `contains,imports,calls,implements,embeds,referencestype,dependson` |
| `-direction <dir>` | `LR` | Mermaid layout direction (`LR`, `TD`, `TB`, `RL`, `BT`) |
| `-title <text>` | `""` | Title header for Mermaid and DOT diagrams |

### Common Recipes

```bash
# Export SQLite database for AI coding agents:
archscout graph --sqlite .archscout/codegraph.db .

# Render full codebase architecture as a Mermaid diagram:
archscout graph --format mermaid --out architecture.mmd .

# Generate a high-level package import graph in Graphviz DOT:
archscout graph --format dot --node-kinds package --edge-kinds imports --out imports.dot .

# Export complete graph structure as JSON for custom tooling:
archscout graph --format json --out graph.json .
```

### CLI Query Tool (`archscout query`)

Query symbol locations, multi-hop caller chains, dependencies, and implementations from the terminal or scripts (auto-generates the database if missing):

```bash
# Full-text search (FTS5) for symbols, functions, types:
archscout query fts "Workspace"
# or simply:
archscout query "Workspace"

# Trace multi-hop callers to a function/method up to depth N:
archscout query callers "github.com/org/repo/pkg.ProcessOrder" --depth 3

# Trace outgoing dependencies (calls, imports) from a package or symbol:
archscout query deps "github.com/org/repo/pkg" --depth 2

# Inspect detailed metadata and exact source location of a symbol:
archscout query info "OrderService"

# Find concrete implementations of an interface:
archscout query implementers "example.com/api.Greeter"

# Output results as JSON for consumption by agent tools or scripts:
archscout query callers "ProcessOrder" --json

# Emit skill installation instructions and SKILL.md definition for AI agents:
archscout query skill

# Automatically install the skill into the project repository (.agents/skills/archscout/SKILL.md):
archscout query skill --install
```

### Model Context Protocol (MCP) Server (`archscout mcp`)

Run an MCP server over stdio to give AI coding assistants (Cursor, Windsurf, Claude Code, Cline) live access to the codebase graph:

```bash
archscout mcp --db .archscout/codegraph.db
```

#### Configuring in Cursor / Windsurf / Claude Desktop

Add to your `.cursor/mcp.json` or `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "archscout": {
      "command": "go",
      "args": ["tool", "archscout", "mcp", "--db", ".archscout/codegraph.db"]
    }
  }
}
```

Exposed MCP Tools:
- `search_symbols(query, limit)`: Fast FTS5 full-text keyword and prefix search across all symbols.
- `trace_callers(symbol, depth)`: Recursive CTE search finding all upstream callers.
- `trace_dependencies(symbol, depth)`: Recursive CTE search finding downstream calls and imports.
- `get_symbol_info(symbol)`: Retrieves kind, QName, package, file, line, and column.
- `get_implementers(interface)`: Lists concrete types satisfying a Go interface.

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

Each `FunctionCall` also carries the function declaration that lexically
encloses the call site. This is useful for asking who, exactly, is calling
something:

```go
// Every method on *Service that calls fmt.Errorf.
refs := workspace.FunctionCalls.Match(func(call archscout.FunctionCall) bool {
  return call.Callee == "fmt.Errorf" &&
    call.CallerReceiver == "*Service"
})
```

`CallerName` and `CallerReceiver` are empty for calls that appear at package
level (for example, inside a `var x = foo()` initializer). For methods,
`CallerReceiver` mirrors the raw receiver text from `Function.Receiver`
(e.g. `"*Service"` for a pointer receiver, `"Service"` for a value receiver).

`CallerQName` is the canonical fully-qualified name of the enclosing
function (or empty at package level), composed identically to
`Function.QName`. It's the join key when correlating calls back to function
declarations:

```go
// Every method on *Service, with each one's set of distinct callees.
calls := workspace.FunctionCalls.Match(func(call archscout.FunctionCall) bool {
  return call.CallerQName == "github.com/your-project/api.Service.Run"
})
```

`Function.QName` and `Type.QName` are always populated and follow the same
convention: `<importpath>.<Name>` for plain functions and types,
`<importpath>.<RecvType>.<Name>` for methods (pointer indirection on the
receiver stripped). They're the identifier you'd persist in any external
graph or store; the unqualified `Name` + `Receiver` remain available for
display purposes.

The default `Callee` field is the syntactic callee text from source — it's
useful for grep-style matches but treats `crypto.Sign` and `c.Sign` (a method
on a type aliased `crypto`) as different callees. For cross-package edges,
load with `WithTypeInfo()` to get fully-qualified resolution:

```go
ws, err := archscout.LoadWorkspace(ctx, ".", archscout.WithTypeInfo())

// Every call into the strings package, regardless of how it was written.
refs := ws.FunctionCalls.Match(func(call archscout.FunctionCall) bool {
  return call.CalleePackage == "strings"
})

// All method calls on api.Service.
refs = ws.FunctionCalls.Match(func(call archscout.FunctionCall) bool {
  return call.CalleeIsMethod &&
    call.CalleeQName == "example.com/your-project/api.Service.Run"
})
```

`CalleeQName` has the form `<importpath>.<TypeName>.<MethodName>` for methods
(pointer indirection on the receiver is stripped) and `<importpath>.<FuncName>`
for plain functions. For interface dispatch, the qname resolves to the
interface's defining method — type information alone cannot know the dynamic
implementer at runtime.

`WithTypeInfo()` is opt-in because loading full Go type information is
substantially slower and uses more memory than the default mode. The disk
cache fingerprints type-info loads separately, so toggling the option will
not return stale, partially-populated workspaces.

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
| `Types`         | `Type`         | `Name`, `QName`, `Kind`, `Fields`, `Methods`, `Embeds`                              |
| `Functions`     | `Function`     | `Name`, `QName`, `Receiver`                                                         |
| `Variables`     | `Variable`     | `Name`, `Kind`                                                                      |
| `FunctionCalls` | `FunctionCall` | `Callee`, `CalleePackage`, `CalleeQName`, `CalleeIsMethod`, `CallerName`, `CallerReceiver`, `CallerQName` |
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

### 9. Find interface implementers

When a workspace is loaded with `WithTypeInfo()`, `BuildImplementsGraph` lets
you ask which concrete types satisfy a given interface and which interfaces
a given type implements:

```go
import "github.com/saintedlama/archscout"

ws, err := archscout.LoadWorkspace(ctx, ".", archscout.WithTypeInfo())
graph := archscout.BuildImplementsGraph(ws)

// Who implements example.com/api.Greeter?
for _, qname := range graph.Implementers("example.com/api.Greeter") {
    fmt.Println(qname)
}

// Restrict to a specific package — useful when generated mocks should be
// excluded from a "who satisfies this in production code?" query.
real := graph.Implementers(
    "example.com/api.Greeter",
    "example.com/your-project/...",
)

// Which interfaces does example.com/your-project.PointerGreeter satisfy?
ifaces := graph.Interfaces("example.com/your-project.PointerGreeter")
```

Empty interfaces (`interface{}` / `any`) are intentionally not indexed —
every concrete type trivially implements them, which is rarely the answer
you want. A type that satisfies an interface only via its pointer method set
is still listed; the receiver semantics live in the underlying `*types.Named`
if you need them.

`BuildImplementsGraph` returns an empty (but safe) graph when the workspace
was loaded without `WithTypeInfo()` or restored from a disk cache, since type
information is not serialized.

`ImplementsGraph` methods:

| Method                                   | Description                                                                            |
| ---------------------------------------- | -------------------------------------------------------------------------------------- |
| `Implementers(ifaceQName, patterns...)`  | Sorted concrete-type qnames that satisfy the interface, optionally filtered by package |
| `Interfaces(typeQName)`                  | Sorted interface qnames the given type satisfies                                       |

### 10. Inspect type structure

`Type` items expose the inner shape of structs and interfaces:

```go
import "github.com/saintedlama/archscout"

ws, err := archscout.LoadWorkspace(ctx, ".", archscout.WithTypeInfo())

// Find every struct that embeds inner.Base.
for _, t := range ws.Types.All() {
  for _, embed := range t.Embeds {
    if embed == "example.com/your-project/inner.Base" {
      fmt.Println(t.Name, "embeds inner.Base")
    }
  }
}

// Find every interface with at least three directly declared methods.
for _, t := range ws.Types.All() {
  if t.Kind == "interface" && len(t.Methods) >= 3 {
    fmt.Println(t.Name)
  }
}
```

`Fields` is the declared struct fields, including embedded entries
(`Embedded == true`, `Name == ""`). Multi-name fields like
`Age, Year int` fan out to one `FieldInfo` per name. Tags are returned
without the surrounding backticks. With `WithTypeInfo()`, `TypeQName`
resolves cross-package field types; without it, only the syntactic
`TypeName` is populated.

`Methods` lists only the methods declared *directly* on an interface.
Methods contributed by embedded interfaces are not flattened in — follow
`Embeds` for those.

`Embeds` is a flat list of embedded type identifiers — fully qualified when
`WithTypeInfo()` is enabled, syntactic source text otherwise. It exists for
both struct embeds and embedded interfaces, so the question "what does this
type compose with?" is one slice access regardless of kind.

Concrete-type methods (those declared via `func (T) ...`) live in the
`Functions` collection with a non-empty `Receiver`; they are intentionally
not duplicated under `Type.Methods`.

### 11. Query and visualize the full codebase graph

`ws.CodeGraph()` (or `archscout.BuildCodeGraph(ws)`) provides a unified, multi-level directed graph connecting modules, packages, files, types, and functions with structural containment, call graphs, interface implementations, and dependencies:

```go
import "github.com/saintedlama/archscout"

ws, _ := archscout.LoadWorkspace(ctx, ".", archscout.WithTypeInfo())
graph := ws.CodeGraph()

// 1. Cross-tier dependency queries:
// Which modules does a specific function call?
calledModules := graph.DependenciesOf(
    archscout.FunctionNodeID("example.com/app/service.OrderService.Create"),
    archscout.CodeNodeKindModule,
    archscout.CodeEdgeKindCalls,
)

// Which packages does a specific file depend on?
importedPkgs := graph.DependenciesOf(
    archscout.FileNodeID("/path/to/order.go"),
    archscout.CodeNodeKindPackage,
    archscout.CodeEdgeKindImports,
)

// Which functions call into the domain package?
callers := graph.DependentsOf(
    archscout.PackageNodeID("example.com/app/domain"),
    archscout.CodeNodeKindFunction,
    archscout.CodeEdgeKindCalls,
)

// 2. Roll up fine-grained edges to architectural tiers:
// Condense all function calls and type references into a Package-to-Package call graph:
pkgCallGraph := graph.Rollup(archscout.CodeNodeKindPackage, archscout.CodeEdgeKindCalls)

// Condense to high-level Module-to-Module dependencies:
moduleGraph := graph.Rollup(archscout.CodeNodeKindModule)

// 3. Detect cycles or explain reachability paths:
cycles := graph.Cycles(archscout.CodeEdgeKindImports)
paths := graph.Paths(
    archscout.PackageNodeID("example.com/app/cmd"),
    archscout.PackageNodeID("example.com/app/db"),
    5, // max depth
)

// 4. Export diagrams:
mermaidMD := graph.ToMermaid(
    archscout.WithNodeKinds(archscout.CodeNodeKindPackage),
    archscout.WithEdgeKinds(archscout.CodeEdgeKindImports),
    archscout.WithTitle("Package Imports"),
)

dotOutput := graph.ToDOT(archscout.WithTitle("Component Dependencies"))
jsonBytes, _ := graph.ToJSON()
```

`CodeGraph` methods:

| Method                                               | Description                                                                           |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------- |
| `Nodes(kinds...)`                                    | Sorted slice of all nodes, optionally filtered by `NodeKind`                          |
| `Edges(kinds...)`                                    | Sorted slice of all edges, optionally filtered by `EdgeKind`                          |
| `DirectDependencies(nodeID, kinds...)`               | Target nodes directly connected from `nodeID` via outgoing edges                     |
| `DirectDependents(nodeID, kinds...)`                 | Source nodes directly connected to `nodeID` via incoming edges                        |
| `TransitiveDependencies(nodeID, kinds...)`           | All reachable nodes along outgoing edges via BFS                                      |
| `TransitiveDependents(nodeID, kinds...)`             | All nodes that can reach `nodeID` via incoming edges BFS                              |
| `DependenciesOf(nodeID, targetKind, kinds...)`       | Target-kind nodes depended on by `nodeID` or its contained children                  |
| `DependentsOf(nodeID, sourceKind, kinds...)`         | Source-kind nodes depending on `nodeID` or its contained children                      |
| `Rollup(targetKind, kinds...)`                       | Condenses lower-level edges to the specified architectural tier (`Package`, `Module`) |
| `InPackage(patterns...)`                             | Returns a filtered subgraph scoped to packages matching glob patterns                |
| `Paths(srcID, dstID, maxDepth, kinds...)`            | Returns all simple directed paths between two nodes                                   |
| `Cycles(kinds...)`                                   | Detects directed cycles across designated edge kinds                                  |
| `ToMermaid(opts...)` / `ToDOT(opts...)` / `ToJSON()` | Exports the graph to Mermaid markdown, Graphviz DOT, or JSON                          |

### 12. Export to SQLite & Vector DB for AI Coding Agents

AI coding agents (e.g. Claude Code, Cursor, Windsurf, Devin) consume codebases best when they can run indexed full-text searches and structural graph queries without blowing through context windows. ArchScout can serialize its entire codebase graph into a portable, single-file SQLite database with FTS5 search, vector embeddings, and recursive CTE graph traversal:

```go
import (
    "context"

    "github.com/saintedlama/archscout"
)

// 1. Export workspace graph to SQLite (with FTS5 & optional vector embeddings)
ctx := context.Background()
err := ws.ExportSQLite(ctx, ".archscout/codegraph.db",
    archscout.WithEmbeddingFunc(myEmbedder, 1536), // Optional: compute vector embeddings
)

// 2. Query in Go using the built-in SQLiteStore:
store, err := archscout.OpenSQLite(".archscout/codegraph.db")
defer store.Close()

// Full-text search across symbols, packages, and signatures:
nodes, _ := store.SearchFTS(ctx, "OrderService", 5)

// Multi-hop caller chain traversal (Recursive CTE in SQL):
callers, _ := store.TraceCallers(ctx, "github.com/org/repo/pkg.ProcessOrder", 3)

// Hybrid vector + graph search:
expansions, _ := store.HybridSearch(ctx, queryEmbedding, 3, 2)
for _, exp := range expansions {
    fmt.Printf("Seed symbol %s matched (dist: %.3f), called by: %v\n",
        exp.MatchedNodeID, exp.Distance, exp.CallerIDs)
}
```

#### GraphRAG in Pure SQL

Any agent with SQLite access (via CLI, Python, or standard MCP tools) can query the database directly. For example, combining semantic vector similarity with a 3-hop caller expansion:

```sql
WITH top_seeds AS (
    SELECT e.node_id, vec_distance_cosine(e.embedding, :query_vec) AS distance
    FROM node_embeddings e
    ORDER BY distance ASC
    LIMIT 3
),
callers AS (
    SELECT s.node_id AS seed_id, s.distance, ed.source_id AS caller_id, 1 AS depth
    FROM top_seeds s
    LEFT JOIN edges ed ON ed.target_id = s.node_id AND ed.kind = 'calls'
    UNION ALL
    SELECT c.seed_id, c.distance, ed.source_id, c.depth + 1
    FROM edges ed
    JOIN callers c ON ed.target_id = c.caller_id
    WHERE ed.kind = 'calls' AND c.depth < 3
)
SELECT seed_id, distance, caller_id FROM callers;
```

#### CLI Tool (`archscout graph`)

You can also generate and export graphs directly using `go tool archscout` (or the installed binary):

```bash
# Generate SQLite database with FTS5 for AI coding agents:
go tool archscout graph --sqlite .archscout/codegraph.db .
# or if installed globally:
archscout graph --sqlite .archscout/codegraph.db .

# Export Mermaid diagram to file:
archscout graph --format mermaid --out diagram.mmd .

# Export Graphviz DOT or JSON:
archscout graph --format dot .
archscout graph --format json --out graph.json .
```

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
- `WithTypeInfo() LoadWorkspaceOption` — load full Go type information so `FunctionCall.CalleePackage`, `CalleeQName` and `CalleeIsMethod` are populated
- `Module(path)` — helper for building fully-qualified package patterns
- `BuildPackageGraph(c dependencies.Collection) *PackageGraph` — builds a transitive package graph from a dependency collection
- `BuildImplementsGraph(ws *Workspace) *ImplementsGraph` — builds an interface-implementation graph from a `WithTypeInfo()` workspace
- `BuildCodeGraph(ws *Workspace) *CodeGraph` — builds a unified multi-level code graph (modules, packages, files, types, functions)
- `ws.CodeGraph() *CodeGraph` — returns the unified code graph for the workspace
- `ws.ExportSQLite(ctx, dbPath, opts...)` — exports the codebase graph to a SQLite database with FTS5 and optional vector embeddings
- `archscout.OpenSQLite(dbPath) (*SQLiteStore, error)` — opens an exported SQLite database for FTS, vector, and CTE queries
- `WithEmbeddingFunc(fn, dimensions)` — sets the vector embedding generator for SQLite export
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
