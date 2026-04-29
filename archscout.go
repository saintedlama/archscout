package archscout

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	gotypes "go/types"
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
	"github.com/saintedlama/archscout/implementsgraph"
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
	// TODO: this feels awkward here, should we move someplace else or rename?
	// typed holds resolved go/types packages for every workspace-internal
	// package. Populated only when LoadWorkspace was called with
	// WithTypeInfo(); nil otherwise (and after a disk-cache hit, since type
	// information is not serialized). Consumers reach this through
	// TypedPackages().
	typed []*gotypes.Package
}

// TypedPackages returns the slice of resolved go/types packages indexed by
// the workspace. Returns nil when the workspace was loaded without
// WithTypeInfo() or restored from a disk cache.
// The slice is shared with the workspace; callers must not mutate it.
func (ws *Workspace) TypedPackages() []*gotypes.Package {
	if ws == nil {
		return nil
	}
	return ws.typed
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

// ImplementsGraph stores interface-implementation edges over the workspace's resolved go/types packages.
type ImplementsGraph = implementsgraph.Graph

// BuildImplementsGraph constructs an ImplementsGraph from the workspace's
// resolved type information. Returns an empty graph when the workspace was
// loaded without WithTypeInfo() (or restored from a disk cache, which does
// not preserve type information):
//
//	ws, _ := archscout.LoadWorkspace(ctx, ".", archscout.WithTypeInfo())
//	graph := archscout.BuildImplementsGraph(ws)
//	for _, qname := range graph.Implementers("example.com/api.Greeter") {
//	    fmt.Println(qname)
//	}
func BuildImplementsGraph(ws *Workspace) *ImplementsGraph {
	if ws == nil {
		return implementsgraph.Build(nil)
	}
	return implementsgraph.Build(ws.typed)
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

	// Only trim back to the last '/' when the common prefix cuts mid-segment.
	// When i == len(a), 'a' itself is a complete import path and is already
	// a clean path boundary (e.g. "code.gitea.io/gitea" is a prefix of all
	// sub-packages). Trimming in that case would incorrectly drop the last
	// meaningful segment.
	if i < len(a) {
		if j := strings.LastIndex(prefix, "/"); j >= 0 {
			prefix = prefix[:j]
		}
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
	diskCache     bool   // true when WithDiskCache() (auto dir) is used
	diskCacheDir  string // explicit dir from WithDiskCacheDir
	typeInfo      bool   // true when WithTypeInfo() is used
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

// WithDiskCache enables a file-system cache stored in the platform-default
// cache directory (os.UserCacheDir()/archscout, or os.TempDir()/archscout-cache
// as a fallback). The project's absolute path is mixed into the fingerprint
// hash so two different projects sharing the same cache directory never collide.
//
// On the first load the workspace is serialized to a gob file whose name is a
// SHA256 fingerprint of all .go source files and go.sum under the project
// directory. Subsequent loads that find a matching fingerprint skip the
// expensive go/packages parse and reconstruction entirely.
//
// Note: go/ast Node fields (Type.Node, Function.Node, etc.) are nil when a
// workspace is loaded from the disk cache because AST pointers cannot be
// serialized. All string-based queries and rule checks work as normal.
func WithDiskCache() LoadWorkspaceOption {
	return func(opts *loadWorkspaceOptions) {
		opts.diskCache = true
	}
}

// WithDiskCacheDir enables a file-system cache stored in the given directory.
// Behaviour is identical to WithDiskCache() except the caller controls where
// cache files are written.
func WithDiskCacheDir(dir string) LoadWorkspaceOption {
	return func(opts *loadWorkspaceOptions) {
		opts.diskCacheDir = dir
	}
}

// WithTypeInfo enables loading of full Go type information by extending the
// underlying go/packages load mode with NeedTypes and NeedTypesInfo. When
// enabled, the workspace populates resolved-callee fields on every
// FunctionCall:
//
//   - CalleePackage — import path of the package that defines the callee
//   - CalleeQName   — fully-qualified name (e.g. "example.com/pkg.Service.Run"
//     for methods or "example.com/pkg.New" for functions)
//   - CalleeIsMethod — true for method calls
//
// Without this option the same fields remain empty; only the syntactic
// Callee string is available.
//
// Type-info loading is significantly more expensive than the default mode
// (typically 2-3x slower and substantially more memory hungry on large
// workspaces). Enable it only when the resolved data is needed.
//
// The disk cache fingerprints type-info loads separately from default loads,
// so toggling this option will not return stale, partially-populated
// workspaces.
func WithTypeInfo() LoadWorkspaceOption {
	return func(opts *loadWorkspaceOptions) {
		opts.typeInfo = true
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

	// Resolve effective disk cache directory.
	// WithDiskCacheDir takes precedence; WithDiskCache() falls back to the
	// platform default directory.
	effectiveCacheDir := options.diskCacheDir
	if effectiveCacheDir == "" && options.diskCache {
		effectiveCacheDir = defaultCacheDir()
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

		workspace, err := loadWithDiskCache(ctx, dir, effectiveCacheDir, options.typeInfo, report)
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

	return loadWithDiskCache(ctx, dir, effectiveCacheDir, options.typeInfo, report)
}

func parseWorkspace(ctx context.Context, dir string, withTypeInfo bool, report func(string)) (*Workspace, error) {
	mode := toolspackages.NeedName | toolspackages.NeedFiles |
		toolspackages.NeedSyntax |
		toolspackages.NeedCompiledGoFiles |
		toolspackages.NeedImports
	if withTypeInfo {
		mode |= toolspackages.NeedTypes | toolspackages.NeedTypesInfo
	}

	cfg := &toolspackages.Config{
		Dir:     dir,
		Mode:    mode,
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
	var typedPkgs []*gotypes.Package
	for _, pkg := range pkgs {
		report(fmt.Sprintf("Analyzing %s...", pkg.ID))
		if withTypeInfo && pkg.Types != nil {
			typedPkgs = append(typedPkgs, pkg.Types)
		}

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

			indexFileEntries(workspace, p, filename, file, pkg.TypesInfo)
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
		typed:         typedPkgs,
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
	typesInfo *gotypes.Info,
) {
	if file == nil {
		return
	}

	ast.Walk(&entryVisitor{
		workspace: workspace,
		pkg:       pkg,
		filename:  filename,
		file:      file,
		typesInfo: typesInfo,
	}, file)
}

// entryVisitor populates the workspace builder while walking a single file.
// It tracks the enclosing *ast.FuncDecl so that call entries can be tagged
// with the lexical caller they live inside, and threads the package's
// *types.Info through so callee qnames and field types resolve.
//
// Function literals (*ast.FuncLit) are intentionally not pushed onto the
// stack: a call inside a closure inside a method should still report the
// enclosing FuncDecl as its caller.
type entryVisitor struct {
	workspace *workspacebuilder.Builder
	pkg       packages.Item
	filename  string
	file      *ast.File
	typesInfo *gotypes.Info
	enclosing *ast.FuncDecl
}

func (v *entryVisitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}

	switch node := n.(type) {
	case *ast.TypeSpec:
		fields, fieldEmbeds := extractStructFields(v.pkg.FileSet, node, v.typesInfo)
		methods, ifaceEmbeds := extractInterfaceMethods(v.pkg.ID, node, v.typesInfo)
		embeds := fieldEmbeds
		if len(ifaceEmbeds) > 0 {
			embeds = ifaceEmbeds
		}
		v.workspace.AddType(types.Item{
			Ref:     newRef(v.pkg, v.filename, node, common.RefKindType, typeMatchText(node.Name.Name, exprKind(node.Type))),
			Name:    node.Name.Name,
			QName:   typeQName(v.pkg.ID, node.Name.Name),
			Kind:    exprKind(node.Type),
			Fields:  fields,
			Methods: methods,
			Embeds:  embeds,
			Node:    node,
		})

	case *ast.FuncDecl:
		receiver := ""
		if node.Recv != nil && len(node.Recv.List) > 0 {
			receiver = exprText(node.Recv.List[0].Type)
		}
		v.workspace.AddFunction(functions.Item{
			Ref:      newRef(v.pkg, v.filename, node, common.RefKindFunction, functionMatchText(node.Name.Name, receiver)),
			Name:     node.Name.Name,
			QName:    funcQName(v.pkg.ID, receiver, node.Name.Name),
			Receiver: receiver,
			Node:     node,
		})
		// Children of this FuncDecl are visited with a child visitor that
		// records this declaration as the enclosing caller.
		return &entryVisitor{
			workspace: v.workspace,
			pkg:       v.pkg,
			filename:  v.filename,
			file:      v.file,
			typesInfo: v.typesInfo,
			enclosing: node,
		}

	case *ast.ValueSpec:
		kind := "var"
		if genDecl, ok := enclosingGenDecl(v.file, node); ok && genDecl.Tok == token.CONST {
			kind = "const"
		}
		for _, name := range node.Names {
			v.workspace.AddVariable(variables.Item{
				Ref:  newRef(v.pkg, v.filename, name, common.RefKindVariable, variableMatchText(name.Name, kind)),
				Name: name.Name,
				Kind: kind,
				Node: name,
			})
		}

	case *ast.CallExpr:
		var (
			callerName, callerReceiver, callerQName string
		)
		if v.enclosing != nil {
			callerName = v.enclosing.Name.Name
			if v.enclosing.Recv != nil && len(v.enclosing.Recv.List) > 0 {
				callerReceiver = exprText(v.enclosing.Recv.List[0].Type)
			}
			callerQName = funcQName(v.pkg.ID, callerReceiver, callerName)
		}
		calleePkg, calleeQName, isMethod := resolveCallee(v.typesInfo, node.Fun)
		v.workspace.AddFunctionCall(functioncalls.Item{
			Ref:            newRef(v.pkg, v.filename, node, common.RefKindFunctionCall, callMatchText(v.pkg.FileSet, node)),
			Callee:         calleeName(node.Fun),
			CalleePackage:  calleePkg,
			CalleeQName:    calleeQName,
			CalleeIsMethod: isMethod,
			CallerName:     callerName,
			CallerReceiver: callerReceiver,
			CallerQName:    callerQName,
			Node:           node,
		})
	}

	return v
}

// typeQName composes a type's canonical fully-qualified name as
// "<importpath>.<TypeName>".
func typeQName(pkgID, typeName string) string {
	return pkgID + "." + typeName
}

// funcQName composes a function or method's canonical fully-qualified
// name. For methods, pointer indirection on the receiver is stripped so
// the qname is stable across pointer/value declarations of the same
// method set.
//
//	plain function: "<importpath>.<Name>"
//	method:         "<importpath>.<RecvType>.<Name>"
func funcQName(pkgID, receiver, name string) string {
	if receiver == "" {
		return pkgID + "." + name
	}
	recv := strings.TrimPrefix(receiver, "*")
	return pkgID + "." + recv + "." + name
}

// resolveCallee inspects type information to derive the import path and
// fully-qualified name of a CallExpr's callee.
//
// Returns empty values when typesInfo is nil (i.e. the workspace was loaded
// without WithTypeInfo), when the callee is not a Go function (e.g. type
// conversions, builtins, calls through interface values that can't be
// resolved), or when the resolution otherwise fails.
//
// For methods CalleeQName has the form "<importpath>.<TypeName>.<MethodName>"
// with any pointer indirection on the receiver stripped. For plain functions
// it is "<importpath>.<FuncName>". CalleePackage is the receiver type's
// defining package for methods and the function's defining package for
// plain functions; it is empty for callees in the universe scope.
func resolveCallee(typesInfo *gotypes.Info, fun ast.Expr) (calleePkg, calleeQName string, isMethod bool) {
	if typesInfo == nil {
		return "", "", false
	}

	// Strip parentheses so e.g. (foo)() resolves the same as foo().
	for {
		paren, ok := fun.(*ast.ParenExpr)
		if !ok {
			break
		}
		fun = paren.X
	}

	var obj gotypes.Object
	switch e := fun.(type) {
	case *ast.Ident:
		obj = typesInfo.Uses[e]
	case *ast.SelectorExpr:
		// Method or field selection (receiver.method or receiver.field()).
		if sel, ok := typesInfo.Selections[e]; ok {
			obj = sel.Obj()
			isMethod = sel.Kind() == gotypes.MethodVal || sel.Kind() == gotypes.MethodExpr
		} else {
			// Qualified identifier (pkg.Func).
			obj = typesInfo.Uses[e.Sel]
		}
	default:
		return "", "", false
	}

	fn, ok := obj.(*gotypes.Func)
	if !ok || fn == nil {
		return "", "", false
	}

	if sig, ok := fn.Type().(*gotypes.Signature); ok && sig.Recv() != nil {
		isMethod = true
		recvType := sig.Recv().Type()
		if ptr, ok := recvType.(*gotypes.Pointer); ok {
			recvType = ptr.Elem()
		}
		if named, ok := recvType.(*gotypes.Named); ok {
			obj := named.Obj()
			pkgPath := ""
			if obj.Pkg() != nil {
				pkgPath = obj.Pkg().Path()
			}
			qname := obj.Name() + "." + fn.Name()
			if pkgPath != "" {
				qname = pkgPath + "." + qname
			}
			return pkgPath, qname, true
		}
	}

	if fn.Pkg() != nil {
		return fn.Pkg().Path(), fn.Pkg().Path() + "." + fn.Name(), isMethod
	}
	return "", fn.Name(), isMethod
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

// typeText renders a type expression as source-equivalent text, handling
// every AST shape (named types, pointers, maps, slices, arrays, channels,
// function types, generic instantiations) by delegating to go/printer.
//
// Falls back to exprText when the FileSet is unavailable (e.g. on a test
// item synthesized without a parser run). Multi-line printer output is
// collapsed to a single line so the result is safe to use as a single
// attribute value.
func typeText(fset *token.FileSet, expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if fset != nil {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, expr); err == nil {
			return strings.Join(strings.Fields(buf.String()), " ")
		}
	}
	return exprText(expr)
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

// extractStructFields walks a TypeSpec for a struct type and returns the
// declared fields plus the syntactic identifiers of any embedded fields.
//
// Returns nil slices for non-struct types. Field type names are rendered via
// go/printer so composite types (maps, slices, function types, etc.) appear
// in full source-equivalent form rather than being dropped.
func extractStructFields(fset *token.FileSet, node *ast.TypeSpec, typesInfo *gotypes.Info) ([]types.FieldInfo, []string) {
	st, ok := node.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return nil, nil
	}

	var fields []types.FieldInfo
	var embeds []string
	for _, f := range st.Fields.List {
		typeName := typeText(fset, f.Type)
		typeQName := resolveTypeQName(typesInfo, f.Type)
		tag := unquoteTag(f.Tag)

		if len(f.Names) == 0 {
			// Embedded field — the type itself is the field name.
			fields = append(fields, types.FieldInfo{
				TypeName:  typeName,
				TypeQName: typeQName,
				Tag:       tag,
				Embedded:  true,
			})
			if name := embedIdentifier(typeQName, typeName); name != "" {
				embeds = append(embeds, name)
			}
			continue
		}
		for _, ident := range f.Names {
			fields = append(fields, types.FieldInfo{
				Name:      ident.Name,
				TypeName:  typeName,
				TypeQName: typeQName,
				Tag:       tag,
				Embedded:  false,
			})
		}
	}
	return fields, embeds
}

// extractInterfaceMethods walks a TypeSpec for an interface type and returns
// the methods directly declared on it, plus the syntactic identifiers of any
// embedded interfaces. Methods contributed by embedded interfaces are not
// flattened into the methods list — callers should follow Embeds for that.
//
// Returns nil slices for non-interface types.
func extractInterfaceMethods(pkgID string, node *ast.TypeSpec, typesInfo *gotypes.Info) ([]types.MethodInfo, []string) {
	it, ok := node.Type.(*ast.InterfaceType)
	if !ok || it.Methods == nil {
		return nil, nil
	}

	ifaceName := node.Name.Name
	var methods []types.MethodInfo
	var embeds []string
	for _, m := range it.Methods.List {
		if len(m.Names) == 0 {
			// Embedded interface — record its identifier.
			typeName := exprText(m.Type)
			typeQName := resolveTypeQName(typesInfo, m.Type)
			if name := embedIdentifier(typeQName, typeName); name != "" {
				embeds = append(embeds, name)
			}
			continue
		}
		for _, ident := range m.Names {
			info := types.MethodInfo{Name: ident.Name}
			if typesInfo != nil && pkgID != "" {
				info.QName = pkgID + "." + ifaceName + "." + ident.Name
			}
			methods = append(methods, info)
		}
	}
	return methods, embeds
}

// resolveTypeQName looks up the resolved fully-qualified name of an
// expression's type. Returns an empty string when type info is unavailable
// or when the expression's type is not a named type (e.g. a literal map,
// channel, or function type).
func resolveTypeQName(typesInfo *gotypes.Info, expr ast.Expr) string {
	if typesInfo == nil {
		return ""
	}
	t := typesInfo.TypeOf(expr)
	if t == nil {
		return ""
	}
	return namedTypeQName(t)
}

func namedTypeQName(t gotypes.Type) string {
	switch v := t.(type) {
	case *gotypes.Named:
		obj := v.Obj()
		if obj.Pkg() == nil {
			return obj.Name()
		}
		return obj.Pkg().Path() + "." + obj.Name()
	case *gotypes.Pointer:
		return namedTypeQName(v.Elem())
	}
	return ""
}

// embedIdentifier prefers the fully-qualified type name when available,
// falling back to the syntactic source text. Returns an empty string when
// neither is informative (e.g. an embedded type literal, which is not legal
// Go anyway).
func embedIdentifier(qname, syntactic string) string {
	if qname != "" {
		return qname
	}
	return syntactic
}

func unquoteTag(lit *ast.BasicLit) string {
	if lit == nil || lit.Value == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(lit.Value); err == nil {
		return unquoted
	}
	return lit.Value
}
