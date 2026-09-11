package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/codegraph/mcp"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	cmd := args[0]
	switch cmd {
	case "graph", "export":
		return runGraph(args[1:], stdout, stderr)
	case "query":
		return runQuery(args[1:], stdout, stderr)
	case "skill":
		return runSkill(args[1:], stdout, stderr)
	case "mcp":
		return runMCP(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "version", "-v", "--version":
		fmt.Fprintln(stdout, "archscout version dev")
		return nil
	default:
		return fmt.Errorf("unknown command %q (run 'archscout help' for usage)", cmd)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `archscout - Architecture exploration, validation, and graph tooling for Go

Usage:
  archscout <command> [arguments]

Commands:
  graph    Generate and export codebase graph (SQLite, Mermaid, DOT, JSON)
  query    Query codebase graph (FTS5 search, callers, dependencies, implementers, skill)
  skill    Emit or install AI agent skill definition for ArchScout
  mcp      Start Model Context Protocol (MCP) server over stdio for AI coding agents

Use "archscout <command> -h" for more information about a command.
`)
}

func runGraph(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)

	sqlitePath := fs.String("sqlite", "", "Path to export SQLite database with FTS5 and vector schema")
	format := fs.String("format", "", "Output diagram/data format: mermaid, dot, json")
	outFile := fs.String("out", "", "Write output to file instead of stdout")
	typeInfo := fs.Bool("type-info", true, "Load full Go type info for callee and interface resolution")
	nodeKindsStr := fs.String("node-kinds", "", "Comma-separated node kinds to include (module,package,file,type,function)")
	edgeKindsStr := fs.String("edge-kinds", "", "Comma-separated edge kinds to include (contains,imports,calls,implements,embeds,referencestype,dependson)")
	title := fs.String("title", "", "Diagram title (mermaid/dot)")
	direction := fs.String("direction", "LR", "Mermaid diagram direction (LR, TD, TB, RL, BT)")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: archscout graph [options] [path]\n\n")
		fmt.Fprintf(stderr, "Generate and export the codebase graph to SQLite, Mermaid markdown, Graphviz DOT, or JSON.\n\n")
		fmt.Fprintf(stderr, "Options:\n")
		fs.PrintDefaults()
	}

	valueFlags := map[string]bool{
		"sqlite": true, "format": true, "out": true, "node-kinds": true,
		"edge-kinds": true, "title": true, "direction": true,
	}
	reordered := reorderFlagsFirst(args, valueFlags)

	if err := fs.Parse(reordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}

	var opts []archscout.LoadWorkspaceOption
	if *typeInfo {
		opts = append(opts, archscout.WithTypeInfo())
	}

	ctx := context.Background()
	ws, err := archscout.LoadWorkspace(ctx, dir, opts...)
	if err != nil {
		return fmt.Errorf("loading workspace at %s: %w", dir, err)
	}

	graph := ws.CodeGraph()

	// SQLite export
	if *sqlitePath != "" {
		dirPath := filepath.Dir(*sqlitePath)
		if dirPath != "" && dirPath != "." {
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("creating directory for sqlite db: %w", err)
			}
		}
		if err := ws.ExportSQLite(ctx, *sqlitePath); err != nil {
			return fmt.Errorf("exporting sqlite database: %w", err)
		}
		fmt.Fprintf(stderr, "Exported SQLite database to %s (%d nodes, %d edges)\n", *sqlitePath, len(graph.Nodes()), len(graph.Edges()))
		if *format == "" {
			return nil
		}
	}

	activeFormat := strings.ToLower(strings.TrimSpace(*format))
	if activeFormat == "" {
		activeFormat = "mermaid"
	}

	var exportOpts []archscout.ExportOption
	if *title != "" {
		exportOpts = append(exportOpts, archscout.WithTitle(*title))
	}
	if *direction != "" {
		exportOpts = append(exportOpts, archscout.WithDirection(*direction))
	}
	if *nodeKindsStr != "" {
		kinds, err := parseNodeKinds(*nodeKindsStr)
		if err != nil {
			return err
		}
		exportOpts = append(exportOpts, archscout.WithNodeKinds(kinds...))
	}
	if *edgeKindsStr != "" {
		kinds, err := parseEdgeKinds(*edgeKindsStr)
		if err != nil {
			return err
		}
		exportOpts = append(exportOpts, archscout.WithEdgeKinds(kinds...))
	}

	var content []byte
	switch activeFormat {
	case "mermaid":
		content = []byte(graph.ToMermaid(exportOpts...))
	case "dot":
		content = []byte(graph.ToDOT(exportOpts...))
	case "json":
		jsonBytes, err := graph.ToJSON()
		if err != nil {
			return fmt.Errorf("serializing to json: %w", err)
		}
		content = jsonBytes
	default:
		return fmt.Errorf("unsupported format %q (supported: mermaid, dot, json)", activeFormat)
	}

	if *outFile != "" {
		outDir := filepath.Dir(*outFile)
		if outDir != "" && outDir != "." {
			if err := os.MkdirAll(outDir, 0755); err != nil {
				return fmt.Errorf("creating directory for output: %w", err)
			}
		}
		if err := os.WriteFile(*outFile, content, 0644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		fmt.Fprintf(stderr, "Wrote %s export to %s\n", activeFormat, *outFile)
	} else {
		if _, err := stdout.Write(content); err != nil {
			return err
		}
		if len(content) > 0 && content[len(content)-1] != '\n' {
			fmt.Fprintln(stdout)
		}
	}

	return nil
}

func runQuery(args []string, stdout, stderr io.Writer) error {
	for i, a := range args {
		if strings.ToLower(a) == "skill" {
			remaining := append([]string{}, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return runSkill(remaining, stdout, stderr)
		}
	}

	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(stderr)

	dbPath := fs.String("db", ".archscout/codegraph.db", "Path to SQLite database")
	dir := fs.String("dir", ".", "Project root directory if database needs auto-generation")
	depth := fs.Int("depth", 3, "Recursion depth for callers/deps traversal")
	limit := fs.Int("limit", 10, "Maximum results to return for search/fts")
	jsonOut := fs.Bool("json", false, "Output results in JSON format")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: archscout query [mode] <target> [flags]\n\n")
		fmt.Fprintf(stderr, "Query codebase graph (FTS5 search, callers, dependencies, implementers, symbol info).\n\n")
		fmt.Fprintf(stderr, "Modes:\n")
		fmt.Fprintf(stderr, "  fts, search <term>        Full-text search across symbols, packages, and types\n")
		fmt.Fprintf(stderr, "  callers <symbol>          Trace multi-hop callers to a function or method\n")
		fmt.Fprintf(stderr, "  deps <symbol>             Trace multi-hop dependencies from a symbol or package\n")
		fmt.Fprintf(stderr, "  info <symbol>             Show metadata and source location of a symbol\n")
		fmt.Fprintf(stderr, "  implementers <interface>  List types that implement an interface\n")
		fmt.Fprintf(stderr, "  skill                     Emit skill installation instructions and definition for AI agents\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	valueFlags := map[string]bool{"db": true, "dir": true, "depth": true, "limit": true}
	reordered := reorderFlagsFirst(args, valueFlags)

	if err := fs.Parse(reordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing query argument")
	}

	mode := fs.Arg(0)
	ctx := context.Background()
	store, err := ensureStore(ctx, *dbPath, *dir, stderr)
	if err != nil {
		return err
	}
	defer store.Close()

	target := ""
	if fs.NArg() > 1 {
		target = fs.Arg(1)
	} else {
		target = mode
		mode = "fts"
	}

	switch strings.ToLower(mode) {
	case "fts", "search":
		nodes, err := store.SearchFTS(ctx, target, *limit)
		if err != nil {
			return err
		}
		if *jsonOut {
			return outputJSON(stdout, nodes)
		}
		if len(nodes) == 0 {
			fmt.Fprintf(stdout, "No symbols matched %q\n", target)
			return nil
		}
		fmt.Fprintf(stdout, "Found %d symbol(s) matching %q:\n", len(nodes), target)
		for _, n := range nodes {
			loc := ""
			if n.Ref.Filename != "" {
				loc = fmt.Sprintf(" (%s:%d)", n.Ref.Filename, n.Ref.Line)
			}
			fmt.Fprintf(stdout, "  [%-8s] %s%s\n", n.Kind, n.ID, loc)
		}
		return nil

	case "callers":
		callers, err := store.TraceCallers(ctx, target, *depth)
		if err != nil {
			return err
		}
		if *jsonOut {
			return outputJSON(stdout, map[string]any{
				"target":  target,
				"depth":   *depth,
				"callers": callers,
			})
		}
		if len(callers) == 0 {
			fmt.Fprintf(stdout, "No callers found for %s (depth %d)\n", target, *depth)
			return nil
		}
		fmt.Fprintf(stdout, "Callers of %s (depth %d):\n", target, *depth)
		for _, c := range callers {
			fmt.Fprintf(stdout, "  ← %s\n", c)
		}
		return nil

	case "deps", "dependencies":
		deps, err := store.TraceDependencies(ctx, target, *depth)
		if err != nil {
			return err
		}
		if *jsonOut {
			return outputJSON(stdout, map[string]any{
				"source":       target,
				"depth":        *depth,
				"dependencies": deps,
			})
		}
		if len(deps) == 0 {
			fmt.Fprintf(stdout, "No dependencies found for %s (depth %d)\n", target, *depth)
			return nil
		}
		fmt.Fprintf(stdout, "Dependencies of %s (depth %d):\n", target, *depth)
		for _, d := range deps {
			fmt.Fprintf(stdout, "  → %s\n", d)
		}
		return nil

	case "info":
		node, err := store.GetNode(ctx, target)
		if err != nil {
			return err
		}
		if node == nil {
			candidates, _ := store.SearchFTS(ctx, target, 1)
			if len(candidates) > 0 {
				node = &candidates[0]
			} else {
				return fmt.Errorf("symbol %q not found", target)
			}
		}
		if *jsonOut {
			return outputJSON(stdout, node)
		}
		fmt.Fprintf(stdout, "Symbol:  %s\n", node.ID)
		fmt.Fprintf(stdout, "Kind:    %s\n", node.Kind)
		fmt.Fprintf(stdout, "Name:    %s\n", node.Name)
		if node.QName != "" {
			fmt.Fprintf(stdout, "QName:   %s\n", node.QName)
		}
		if node.PackageID != "" {
			fmt.Fprintf(stdout, "Package: %s\n", node.PackageID)
		}
		if node.Ref.Filename != "" {
			fmt.Fprintf(stdout, "Source:  %s:%d:%d\n", node.Ref.Filename, node.Ref.Line, node.Ref.Column)
		}
		return nil

	case "implementers":
		implementers, err := store.GetImplementers(ctx, target)
		if err != nil {
			return err
		}
		if *jsonOut {
			return outputJSON(stdout, map[string]any{
				"interface":    target,
				"implementers": implementers,
			})
		}
		if len(implementers) == 0 {
			fmt.Fprintf(stdout, "No implementers found for interface %s\n", target)
			return nil
		}
		fmt.Fprintf(stdout, "Implementers of %s:\n", target)
		for _, imp := range implementers {
			fmt.Fprintf(stdout, "  ✓ %s\n", imp)
		}
		return nil

	default:
		return fmt.Errorf("unknown query mode %q (valid: fts, callers, deps, info, implementers)", mode)
	}
}

func runMCP(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)

	dbPath := fs.String("db", ".archscout/codegraph.db", "Path to SQLite database")
	dir := fs.String("dir", ".", "Project root directory if database needs auto-generation")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: archscout mcp [flags]\n\n")
		fmt.Fprintf(stderr, "Start Model Context Protocol (MCP) JSON-RPC 2.0 server over stdio.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	valueFlags := map[string]bool{"db": true, "dir": true}
	reordered := reorderFlagsFirst(args, valueFlags)

	if err := fs.Parse(reordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	ctx := context.Background()
	store, err := ensureStore(ctx, *dbPath, *dir, stderr)
	if err != nil {
		return err
	}
	defer store.Close()

	server := mcp.NewServer(store)
	return server.Serve(ctx, os.Stdin, stdout)
}

func reorderFlagsFirst(args []string, valueFlags map[string]bool) []string {
	var flags []string
	var posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			name := strings.TrimLeft(arg, "-")
			if idx := strings.Index(name, "="); idx != -1 {
				name = name[:idx]
			} else if valueFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}
	return append(flags, posArgs...)
}

func ensureStore(ctx context.Context, dbPath, dir string, stderr io.Writer) (*archscout.SQLiteStore, error) {
	if dbPath == "" {
		dbPath = ".archscout/codegraph.db"
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		targetDir := dir
		if targetDir == "" {
			targetDir = "."
		}
		goMod := filepath.Join(targetDir, "go.mod")
		if _, err := os.Stat(goMod); err == nil {
			fmt.Fprintf(stderr, "Database %s not found. Exporting codebase graph from %s...\n", dbPath, targetDir)
			if dirPath := filepath.Dir(dbPath); dirPath != "" && dirPath != "." {
				if err := os.MkdirAll(dirPath, 0755); err != nil {
					return nil, fmt.Errorf("creating directory for db: %w", err)
				}
			}
			ws, err := archscout.LoadWorkspace(ctx, targetDir, archscout.WithTypeInfo())
			if err != nil {
				return nil, fmt.Errorf("loading workspace: %w", err)
			}
			if err := ws.ExportSQLite(ctx, dbPath); err != nil {
				return nil, fmt.Errorf("exporting sqlite: %w", err)
			}
			fmt.Fprintf(stderr, "Export complete.\n")
		} else {
			return nil, fmt.Errorf("database %s not found and no go.mod in %s", dbPath, targetDir)
		}
	}

	return archscout.OpenSQLite(dbPath)
}

func outputJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func runSkill(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("skill", flag.ContinueOnError)
	fs.SetOutput(stderr)

	install := fs.Bool("install", false, "Install skill to target directory (default: .agents/skills/archscout/SKILL.md)")
	outFile := fs.String("out", "", "Target file path for skill installation")
	jsonOut := fs.Bool("json", false, "Emit skill metadata and content as JSON")

	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: archscout query skill [flags]\n")
		fmt.Fprintf(stderr, "       archscout skill [flags]\n\n")
		fmt.Fprintf(stderr, "Emit agent skill definition and installation instructions for AI coding agents.\n\n")
		fmt.Fprintf(stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	valueFlags := map[string]bool{"out": true}
	reordered := reorderFlagsFirst(args, valueFlags)
	if err := fs.Parse(reordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	skillName := "archscout"
	skillDesc := "Query Go codebase architecture, symbol locations, multi-hop caller chains, dependencies, and interface implementations using ArchScout's SQLite code graph."
	skillContent := getSkillMarkdown()

	targetPath := *outFile
	if targetPath == "" && *install {
		targetPath = filepath.Join(".agents", "skills", "archscout", "SKILL.md")
	}

	if targetPath != "" {
		dir := filepath.Dir(targetPath)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("creating directory %s: %w", dir, err)
			}
		}
		if err := os.WriteFile(targetPath, []byte(skillContent), 0644); err != nil {
			return fmt.Errorf("writing skill file %s: %w", targetPath, err)
		}
		fmt.Fprintf(stdout, "ArchScout agent skill successfully installed to: %s\n", targetPath)
		return nil
	}

	if *jsonOut {
		payload := map[string]any{
			"name":        skillName,
			"description": skillDesc,
			"recommended_locations": []string{
				".agents/skills/archscout/SKILL.md",
				".claude/skills/archscout/SKILL.md",
				".gemini/skills/archscout/SKILL.md",
			},
			"install_command": "archscout query skill --install",
			"content":         skillContent,
		}
		return outputJSON(stdout, payload)
	}

	fmt.Fprintln(stdout, `ArchScout Agent Skill Installation

AI coding agents (Antigravity, Claude Code, Cursor, Windsurf) can use ArchScout as a specialized skill for architectural inspection, caller tracing, and symbol lookup.

To install automatically:
  archscout query skill --install
  # or to a custom path:
  archscout query skill --install --out .agents/skills/archscout/SKILL.md

Recommended Skill Locations:
  Project:  .agents/skills/archscout/SKILL.md
            .claude/skills/archscout/SKILL.md
            .gemini/skills/archscout/SKILL.md
  User:     ~/.gemini/antigravity-cli/skills/archscout/SKILL.md
            ~/.claude/skills/archscout/SKILL.md

================================================================================
SKILL.md Content:
================================================================================`)
	fmt.Fprintln(stdout, skillContent)
	return nil
}

func getSkillMarkdown() string {
	return `---
name: archscout
description: Query Go codebase architecture, symbol locations, multi-hop caller chains, dependencies, and interface implementations using ArchScout's SQLite code graph.
---

# ArchScout Code Graph Skill

Use this skill when you need to understand Go codebase architecture, trace callers/dependencies, locate symbol declarations, or inspect interface implementations without blowing context window limits.

## Overview

ArchScout maintains an indexed SQLite graph (` + "`.archscout/codegraph.db`" + `) with FTS5 full-text search and recursive CTE graph traversal.
If the database does not exist yet, running any query automatically analyzes the Go workspace and builds it on the fly.

## Commands

### 1. Keyword & Symbol Search (FTS5)
Search functions, types, packages, and variables:
` + "```bash\n" + `archscout query fts "<query>"
# or simply:
archscout query "<query>"
` + "```\n" + `Add ` + "`--limit <n>`" + ` to control result count (default 10).
Add ` + "`--json`" + ` for machine-readable JSON output.

### 2. Inspect Symbol Details
Retrieve exact file path, line, column, package, kind, and full qualified name:
` + "```bash\n" + `archscout query info "<qualified-symbol-or-name>"
` + "```\n\n" + `### 3. Trace Caller Chains (Who calls this?)
Find multi-hop upstream callers of a function or method:
` + "```bash\n" + `archscout query callers "<function-or-method>" --depth 3
` + "```\n\n" + `### 4. Trace Dependencies (What does this depend on?)
Find multi-hop downstream calls and package imports:
` + "```bash\n" + `archscout query deps "<package-or-symbol>" --depth 3
` + "```\n\n" + `### 5. Find Interface Implementations
Find all concrete types implementing a Go interface:
` + "```bash\n" + `archscout query implementers "<interface-name>"
` + "```\n\n" + `### 6. Regenerate or Export Graph
Rebuild the graph or export to Mermaid/DOT diagrams:
` + "```bash\n" + `# Rebuild SQLite database:
archscout graph --sqlite .archscout/codegraph.db .

# Export Mermaid diagram:
archscout graph --format mermaid --out architecture.mmd .
` + "```\n"
}

func parseNodeKinds(s string) ([]archscout.CodeNodeKind, error) {
	var kinds []archscout.CodeNodeKind
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		switch part {
		case "module":
			kinds = append(kinds, archscout.CodeNodeKindModule)
		case "package", "pkg":
			kinds = append(kinds, archscout.CodeNodeKindPackage)
		case "file":
			kinds = append(kinds, archscout.CodeNodeKindFile)
		case "type":
			kinds = append(kinds, archscout.CodeNodeKindType)
		case "function", "func":
			kinds = append(kinds, archscout.CodeNodeKindFunction)
		default:
			return nil, fmt.Errorf("unknown node kind %q (valid: module, package, file, type, function)", part)
		}
	}
	return kinds, nil
}

func parseEdgeKinds(s string) ([]archscout.CodeEdgeKind, error) {
	var kinds []archscout.CodeEdgeKind
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		part = strings.ReplaceAll(part, "-", "")
		part = strings.ReplaceAll(part, "_", "")
		if part == "" {
			continue
		}
		switch part {
		case "contains":
			kinds = append(kinds, archscout.CodeEdgeKindContains)
		case "imports":
			kinds = append(kinds, archscout.CodeEdgeKindImports)
		case "dependson":
			kinds = append(kinds, archscout.CodeEdgeKindDependsOn)
		case "calls":
			kinds = append(kinds, archscout.CodeEdgeKindCalls)
		case "implements":
			kinds = append(kinds, archscout.CodeEdgeKindImplements)
		case "embeds":
			kinds = append(kinds, archscout.CodeEdgeKindEmbeds)
		case "referencestype":
			kinds = append(kinds, archscout.CodeEdgeKindReferencesType)
		default:
			return nil, fmt.Errorf("unknown edge kind %q (valid: contains, imports, dependson, calls, implements, embeds, referencestype)", part)
		}
	}
	return kinds, nil
}
