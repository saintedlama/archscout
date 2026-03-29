package goarch

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sync"

	"github.com/saintedlama/goarch/common"
	"github.com/saintedlama/goarch/files"
	"github.com/saintedlama/goarch/functioncalls"
	"github.com/saintedlama/goarch/functions"
	"github.com/saintedlama/goarch/packages"
	"github.com/saintedlama/goarch/types"
	"github.com/saintedlama/goarch/variables"

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
}

// Top-level aliases for convenient consumption from goarch package.
type Ref = common.Ref

type Package = packages.Item
type PackageFile = packages.File
type File = files.Item

type Type = types.Item
type Function = functions.Item
type Variable = variables.Item
type FunctionCall = functioncalls.Item

type PackageMatchFunc = packages.MatchFunc
type FileMatchFunc = files.MatchFunc
type TypeMatchFunc = types.MatchFunc
type FunctionMatchFunc = functions.MatchFunc
type VariableMatchFunc = variables.MatchFunc
type FunctionCallMatchFunc = functioncalls.MatchFunc

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
		Mode: toolspackages.NeedName | toolspackages.NeedFiles | toolspackages.NeedSyntax |
			toolspackages.NeedCompiledGoFiles | toolspackages.NeedImports,
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

	workspace := &Workspace{}
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

			workspace.Files.Add(files.Item{
				Ref:      newRef(p, filename, file),
				Filename: filename,
				Node:     file,
			})

			indexFileEntries(&workspace.Types, &workspace.Functions, &workspace.Variables, &workspace.FunctionCalls, p, filename, file)
		}

		workspace.Packages.Add(p)
	}

	return workspace, nil
}

// MatchPackages runs a matcher over all packages and returns generated code refs.
func (workspace *Workspace) MatchPackages(matcher PackageMatchFunc) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Packages.Match(matcher)
}

// MatchFiles runs a matcher over all file entries and returns generated code refs.
func (workspace *Workspace) MatchFiles(matcher FileMatchFunc) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Files.Match(matcher)
}

// MatchTypes runs a matcher over all type entries and returns generated code refs.
func (workspace *Workspace) MatchTypes(matcher TypeMatchFunc) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Types.Match(matcher)
}

// MatchFunctions runs a matcher over all function entries and returns generated code refs.
func (workspace *Workspace) MatchFunctions(matcher FunctionMatchFunc) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Functions.Match(matcher)
}

// MatchVariables runs a matcher over all variable entries and returns generated code refs.
func (workspace *Workspace) MatchVariables(matcher VariableMatchFunc) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Variables.Match(matcher)
}

// MatchFunctionCalls runs a matcher over all call entries and returns generated code refs.
func (workspace *Workspace) MatchFunctionCalls(matcher FunctionCallMatchFunc) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.FunctionCalls.Match(matcher)
}

func indexFileEntries(
	typeEntries *types.Collection,
	functionEntries *functions.Collection,
	variableEntries *variables.Collection,
	callEntries *functioncalls.Collection,
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
			typeEntries.Add(types.Item{
				Ref:  newRef(pkg, filename, node),
				Name: node.Name.Name,
				Kind: exprKind(node.Type),
				Node: node,
			})

		case *ast.FuncDecl:
			receiver := ""
			if node.Recv != nil && len(node.Recv.List) > 0 {
				receiver = exprText(node.Recv.List[0].Type)
			}
			functionEntries.Add(functions.Item{
				Ref:      newRef(pkg, filename, node),
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
				variableEntries.Add(variables.Item{
					Ref:  newRef(pkg, filename, name),
					Name: name.Name,
					Kind: kind,
					Node: name,
				})
			}

		case *ast.CallExpr:
			callEntries.Add(functioncalls.Item{
				Ref:    newRef(pkg, filename, node),
				Callee: calleeName(node.Fun),
				Node:   node,
			})
		}

		return true
	})
}

func newRef(pkg packages.Item, fallbackFilename string, n ast.Node) common.Ref {
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
	}
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
