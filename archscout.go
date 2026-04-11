package archscout

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/saintedlama/archscout/common"
	"github.com/saintedlama/archscout/dependencies"
	"github.com/saintedlama/archscout/files"
	"github.com/saintedlama/archscout/functioncalls"
	"github.com/saintedlama/archscout/functions"
	"github.com/saintedlama/archscout/packagegraph"
	"github.com/saintedlama/archscout/packages"
	"github.com/saintedlama/archscout/types"
	"github.com/saintedlama/archscout/variables"
	workspacebuilder "github.com/saintedlama/archscout/workspace/builder"

	toolspackages "golang.org/x/tools/go/packages"
)

// Workspace is the loaded code workspace for all discovered packages.
type Workspace struct {
	Packages      packages.Collection
	Files         files.Collection
	Types         types.Collection
	Functions     functions.Collection
	Variables     variables.Collection
	FunctionCalls functioncalls.Collection
	Dependencies  dependencies.Collection
}

// Top-level aliases for convenient consumption from archscout package.
type Ref = common.Ref
type Refs = common.Refs
type RefKind = common.RefKind
type RefFormatOption = common.RefFormatOption

const (
	RefKindPackage      = common.RefKindPackage
	RefKindFile         = common.RefKindFile
	RefKindType         = common.RefKindType
	RefKindFunction     = common.RefKindFunction
	RefKindVariable     = common.RefKindVariable
	RefKindFunctionCall = common.RefKindFunctionCall
	RefKindDependency   = common.RefKindDependency
)

type Package = packages.Item
type PackageFile = packages.File
type File = files.Item

type Type = types.Item
type Function = functions.Item
type Variable = variables.Item
type FunctionCall = functioncalls.Item
type Dependency = dependencies.Item

type PackageMatchFunc = packages.MatchFunc
type FileMatchFunc = files.MatchFunc
type TypeMatchFunc = types.MatchFunc
type FunctionMatchFunc = functions.MatchFunc
type VariableMatchFunc = variables.MatchFunc
type FunctionCallMatchFunc = functioncalls.MatchFunc
type DependencyMatchFunc = dependencies.MatchFunc

// PackageGraph is a directed graph of workspace-internal package dependencies.
// See packagegraph.PackageGraph for the full API.
type PackageGraph = packagegraph.PackageGraph

// BuildPackageGraph constructs a PackageGraph from the workspace's dependency
// collection. Only workspace-internal edges are included; filter the collection
// before calling if you want to exclude test files or other dependencies:
//
//	graph := archscout.BuildPackageGraph(ws.Dependencies.IsNotTest())
func BuildPackageGraph(c dependencies.Collection) *PackageGraph {
	return packagegraph.BuildGraph(c)
}

// ModuleRoot derives the module root (e.g. "github.com/myorg/myapp") from the
// longest common import-path prefix shared by all packages in the workspace.
// It returns an empty string if the workspace contains no packages.
func (ws *Workspace) ModuleRoot() string {
	pkgs := ws.Packages.All()
	if len(pkgs) == 0 {
		return ""
	}

	ids := make([]string, len(pkgs))
	for i, p := range pkgs {
		ids[i] = p.ID
	}
	sort.Strings(ids)

	a, b := ids[0], ids[len(ids)-1]
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	prefix := a[:i]
	if j := strings.LastIndex(prefix, "/"); j >= 0 {
		prefix = prefix[:j]
	}
	return prefix
}

// Module is a Go module path that can generate fully-qualified package patterns
// without repeated string concatenation.
//
//	mod := archscout.Module("github.com/myapp/myapp")
//	mod.Pkg("ui/common/...")           // "github.com/myapp/myapp/ui/common/..."
//	mod.Pkgs("audio/...", "player/...") // []string{"github.com/myapp/myapp/audio/...", ...}
type Module string

// Pkg returns the fully-qualified package pattern for the given sub-path.
func (m Module) Pkg(subpath string) string {
	return string(m) + "/" + subpath
}

// Pkgs returns fully-qualified package patterns for each supplied sub-path.
func (m Module) Pkgs(subpaths ...string) []string {
	result := make([]string, len(subpaths))
	for i, p := range subpaths {
		result[i] = string(m) + "/" + p
	}
	return result
}

// DefaultRefFormatOptions returns the default ref formatting configuration.
func DefaultRefFormatOptions() common.RefFormatOptions {
	return common.DefaultRefFormatOptions()
}

// WithRefPackage includes package information in formatted refs.
func WithRefPackage() RefFormatOption {
	return common.WithRefPackage()
}

// WithRefKind includes the ref kind in formatted refs.
func WithRefKind() RefFormatOption {
	return common.WithRefKind()
}

// WithoutRefFile omits the filename from formatted refs.
func WithoutRefFile() RefFormatOption {
	return common.WithoutRefFile()
}

// WithoutRefLine omits the line number from formatted refs.
func WithoutRefLine() RefFormatOption {
	return common.WithoutRefLine()
}

// WithoutRefColumn omits the column number from formatted refs.
func WithoutRefColumn() RefFormatOption {
	return common.WithoutRefColumn()
}

// WithoutRefMatch omits the matched-node representation from formatted refs.
func WithoutRefMatch() RefFormatOption {
	return common.WithoutRefMatch()
}

// WithRefSeparator configures the separator used by FormatRefs.
func WithRefSeparator(separator string) RefFormatOption {
	return common.WithRefSeparator(separator)
}

// WithoutSeparator concatenates formatted refs without any separator.
func WithoutSeparator() RefFormatOption {
	return common.WithoutSeparator()
}

// FormatRef renders a single ref using the provided options.
func FormatRef(ref Ref, opts ...RefFormatOption) string {
	return common.FormatRef(ref, opts...)
}

// FormatRefs renders a slice of refs using the provided options.
func FormatRefs(refs Refs, opts ...RefFormatOption) string {
	return common.FormatRefs(refs, opts...)
}

type loadWorkspaceOptions struct {
	reporter      func(string)
	inMemoryCache bool
}

type workspaceCacheState struct {
	mu      sync.Mutex
	entries map[string]*workspaceCacheEntry
}

type workspaceCacheEntry struct {
	workspace *Workspace
	ready     chan struct{}
}

var workspaceCache = workspaceCacheState{
	entries: make(map[string]*workspaceCacheEntry),
}

// LoadWorkspaceOption configures workspace loading behavior.
type LoadWorkspaceOption func(*loadWorkspaceOptions)

// WithReporter configures a progress reporter callback.
func WithReporter(reporter func(string)) LoadWorkspaceOption {
	return func(opts *loadWorkspaceOptions) {
		opts.reporter = reporter
	}
}

// WithInMemoryCache enables process-local workspace caching.
// When enabled, repeated loads of the same path return the same workspace instance.
func WithInMemoryCache() LoadWorkspaceOption {
	return func(opts *loadWorkspaceOptions) {
		opts.inMemoryCache = true
	}
}

// LoadWorkspace loads all packages in dir and returns a workspace.
func LoadWorkspace(ctx context.Context, dir string, opts ...LoadWorkspaceOption) (*Workspace, error) {
	options := &loadWorkspaceOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}

	report := func(msg string) {
		if options.reporter != nil {
			options.reporter(msg)
		}
	}

	if options.inMemoryCache {
		cacheKey, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("resolving cache key for %q: %w", dir, err)
		}

		workspaceCache.mu.Lock()
		if existing, ok := workspaceCache.entries[cacheKey]; ok {
			workspaceCache.mu.Unlock()
			<-existing.ready
			report(fmt.Sprintf("Using cached workspace for %s", cacheKey))
			return existing.workspace, nil
		}

		entry := &workspaceCacheEntry{ready: make(chan struct{})}
		workspaceCache.entries[cacheKey] = entry
		workspaceCache.mu.Unlock()

		workspace, err := loadWorkspace(ctx, dir, report)
		if err != nil {
			workspaceCache.mu.Lock()
			delete(workspaceCache.entries, cacheKey)
			close(entry.ready)
			workspaceCache.mu.Unlock()
			return nil, err
		}

		workspaceCache.mu.Lock()
		entry.workspace = workspace
		close(entry.ready)
		workspaceCache.mu.Unlock()

		return workspace, nil
	}

	return loadWorkspace(ctx, dir, report)
}

func loadWorkspace(ctx context.Context, dir string, report func(string)) (*Workspace, error) {
	cfg := &toolspackages.Config{
		Dir: dir,
		Mode: toolspackages.NeedName | toolspackages.NeedFiles |
			toolspackages.NeedSyntax |
			toolspackages.NeedCompiledGoFiles |
			toolspackages.NeedImports,
		Context: ctx,
	}

	report("Loading packages (./...)")
	pkgs, err := toolspackages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages found in %q", dir)
	}
	report(fmt.Sprintf("Loaded %d package(s)", len(pkgs)))

	workspacePackageIDs := make(map[string]struct{}, len(pkgs))
	for _, pkg := range pkgs {
		workspacePackageIDs[pkg.ID] = struct{}{}
	}

	workspace := workspacebuilder.New()
	for _, pkg := range pkgs {
		report(fmt.Sprintf("Analyzing %s...", pkg.ID))

		p := packages.Item{
			ID:      pkg.ID,
			Name:    pkg.Name,
			FileSet: pkg.Fset,
			Errors:  pkg.Errors,
		}

		for i, file := range pkg.Syntax {
			if file == nil {
				continue
			}

			filename := ""
			if i < len(pkg.CompiledGoFiles) {
				filename = pkg.CompiledGoFiles[i]
			} else if i < len(pkg.GoFiles) {
				filename = pkg.GoFiles[i]
			}

			p.Files = append(p.Files, packages.File{
				Filename: filename,
				Node:     file,
			})

			workspace.AddFile(files.Item{
				Ref:      newRef(p, filename, file, common.RefKindFile, fileMatchText(file)),
				Filename: filename,
				Node:     file,
			})

			indexFileDependencies(workspace, p, filename, file, workspacePackageIDs)

			indexFileEntries(workspace, p, filename, file)
		}

		workspace.AddPackage(p)
	}

	snapshot := workspace.Build()
	return &Workspace{
		Packages:      snapshot.Packages,
		Files:         snapshot.Files,
		Types:         snapshot.Types,
		Functions:     snapshot.Functions,
		Variables:     snapshot.Variables,
		FunctionCalls: snapshot.FunctionCalls,
		Dependencies:  snapshot.Dependencies,
	}, nil
}

// MatchPackages runs a matcher over all packages and returns generated code refs.
func (workspace *Workspace) MatchPackages(matcher PackageMatchFunc) Refs {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Packages.Match(matcher)
}

// MatchFiles runs a matcher over all file entries and returns generated code refs.
func (workspace *Workspace) MatchFiles(matcher FileMatchFunc) Refs {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Files.Match(matcher)
}

// MatchTypes runs a matcher over all type entries and returns generated code refs.
func (workspace *Workspace) MatchTypes(matcher TypeMatchFunc) Refs {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Types.Match(matcher)
}

// MatchFunctions runs a matcher over all function entries and returns generated code refs.
func (workspace *Workspace) MatchFunctions(matcher FunctionMatchFunc) Refs {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Functions.Match(matcher)
}

// MatchVariables runs a matcher over all variable entries and returns generated code refs.
func (workspace *Workspace) MatchVariables(matcher VariableMatchFunc) Refs {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Variables.Match(matcher)
}

// MatchFunctionCalls runs a matcher over all call entries and returns generated code refs.
func (workspace *Workspace) MatchFunctionCalls(matcher FunctionCallMatchFunc) Refs {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.FunctionCalls.Match(matcher)
}

// MatchDependencies runs a matcher over all dependency entries and returns generated code refs.
func (workspace *Workspace) MatchDependencies(matcher DependencyMatchFunc) Refs {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Dependencies.Match(matcher)
}

func indexFileEntries(
	workspace *workspacebuilder.Builder,
	pkg packages.Item,
	filename string,
	file *ast.File,
) {
	if file == nil {
		return
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeSpec:
			workspace.AddType(types.Item{
				Ref:  newRef(pkg, filename, node, common.RefKindType, typeMatchText(node.Name.Name, exprKind(node.Type))),
				Name: node.Name.Name,
				Kind: exprKind(node.Type),
				Node: node,
			})

		case *ast.FuncDecl:
			receiver := ""
			if node.Recv != nil && len(node.Recv.List) > 0 {
				receiver = exprText(node.Recv.List[0].Type)
			}
			workspace.AddFunction(functions.Item{
				Ref:      newRef(pkg, filename, node, common.RefKindFunction, functionMatchText(node.Name.Name, receiver)),
				Name:     node.Name.Name,
				Receiver: receiver,
				Node:     node,
			})

		case *ast.ValueSpec:
			kind := "var"
			if genDecl, ok := enclosingGenDecl(file, node); ok && genDecl.Tok == token.CONST {
				kind = "const"
			}
			for _, name := range node.Names {
				workspace.AddVariable(variables.Item{
					Ref:  newRef(pkg, filename, name, common.RefKindVariable, variableMatchText(name.Name, kind)),
					Name: name.Name,
					Kind: kind,
					Node: name,
				})
			}

		case *ast.CallExpr:
			workspace.AddFunctionCall(functioncalls.Item{
				Ref:    newRef(pkg, filename, node, common.RefKindFunctionCall, callMatchText(pkg.FileSet, node)),
				Callee: calleeName(node.Fun),
				Node:   node,
			})
		}

		return true
	})
}

func indexFileDependencies(
	workspace *workspacebuilder.Builder,
	pkg packages.Item,
	filename string,
	file *ast.File,
	workspacePackageIDs map[string]struct{},
) {
	if file == nil {
		return
	}

	for _, importSpec := range file.Imports {
		if importSpec == nil || importSpec.Path == nil {
			continue
		}

		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil || importPath == "" {
			continue
		}

		_, withinWorkspace := workspacePackageIDs[importPath]

		workspace.AddDependency(dependencies.Item{
			Ref:               newRef(pkg, filename, importSpec, common.RefKindDependency, dependencyMatchText(importPath, withinWorkspace)),
			ImportPath:        importPath,
			WithinWorkspace:   withinWorkspace,
			External:          !withinWorkspace,
			StandardLibrary:   !strings.Contains(importPath, "."),
			TargetPackageName: importPackageName(importSpec),
		})
	}
}

func dependencyMatchText(importPath string, withinWorkspace bool) string {
	target := "external"
	if withinWorkspace {
		target = "workspace"
	}

	return "dependency " + importPath + " (" + target + ")"
}

func importPackageName(importSpec *ast.ImportSpec) string {
	if importSpec == nil || importSpec.Name == nil {
		return ""
	}

	return importSpec.Name.Name
}

func newRef(pkg packages.Item, fallbackFilename string, n ast.Node, kind common.RefKind, match string) common.Ref {
	pos := pkg.FileSet.PositionFor(n.Pos(), true)

	filename := fallbackFilename
	if pos.Filename != "" {
		filename = pos.Filename
	}

	return common.Ref{
		PackageID:   pkg.ID,
		PackageName: pkg.Name,
		Filename:    filename,
		Line:        pos.Line,
		Column:      pos.Column,
		Kind:        kind,
		Match:       match,
	}
}

func fileMatchText(file *ast.File) string {
	if file != nil && file.Name != nil && file.Name.Name != "" {
		return "file package " + file.Name.Name
	}
	return "file"
}

func typeMatchText(name, kind string) string {
	if name == "" {
		return "type"
	}
	if kind != "" && kind != "type" {
		return "type " + name + " " + kind
	}
	return "type " + name
}

func functionMatchText(name, receiver string) string {
	if receiver != "" {
		return "func (" + receiver + ") " + name
	}
	if name == "" {
		return "func"
	}
	return "func " + name
}

func variableMatchText(name, kind string) string {
	if kind == "" {
		kind = "var"
	}
	if name == "" {
		return kind
	}
	return kind + " " + name
}

func callMatchText(fileSet *token.FileSet, node *ast.CallExpr) string {
	if node == nil {
		return "call"
	}

	var buf bytes.Buffer
	if fileSet != nil && printer.Fprint(&buf, fileSet, node) == nil {
		return buf.String()
	}

	callee := calleeName(node.Fun)
	if callee == "" {
		return "call"
	}
	return callee + "(...)"
}

func calleeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		prefix := exprText(e.X)
		if prefix == "" {
			return e.Sel.Name
		}
		return prefix + "." + e.Sel.Name
	default:
		return exprText(expr)
	}
}

func exprKind(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	case *ast.ArrayType:
		return "array"
	case *ast.MapType:
		return "map"
	case *ast.FuncType:
		return "func"
	case *ast.ChanType:
		return "chan"
	default:
		return "type"
	}
}

func exprText(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		inner := exprText(e.X)
		if inner == "" {
			return "*"
		}
		return "*" + inner
	case *ast.SelectorExpr:
		left := exprText(e.X)
		if left == "" {
			return e.Sel.Name
		}
		return left + "." + e.Sel.Name
	default:
		return ""
	}
}

func enclosingGenDecl(file *ast.File, target *ast.ValueSpec) (*ast.GenDecl, bool) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			if spec == target {
				return genDecl, true
			}
		}
	}
	return nil, false
}
