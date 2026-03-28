package analysis

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"

	"github.com/saintedlama/goarch/analysis/common"
	"github.com/saintedlama/goarch/analysis/conditions"
	"github.com/saintedlama/goarch/analysis/functioncalls"
	"github.com/saintedlama/goarch/analysis/functions"
	"github.com/saintedlama/goarch/analysis/packages"
	"github.com/saintedlama/goarch/analysis/types"
	"github.com/saintedlama/goarch/analysis/variables"

	toolspackages "golang.org/x/tools/go/packages"
)

// Workspace is the loaded code workspace for all discovered packages.
type Workspace struct {
	Packages      packages.Collection
	Types         types.Collection
	Functions     functions.Collection
	Variables     variables.Collection
	FunctionCalls functioncalls.Collection
	Conditions    conditions.Collection
}

// Top-level aliases for convenient consumption from analysis package.
type Finding = common.Finding
type Ref = common.Ref

type Package = packages.Item
type File = packages.File

type Type = types.Item
type Function = functions.Item
type Variable = variables.Item
type FunctionCall = functioncalls.Item
type Condition = conditions.Item

type PackageMatcher = packages.Matcher
type PackageMatchFunc = packages.MatchFunc

type TypeMatcher = types.Matcher
type TypeMatchFunc = types.MatchFunc

type FunctionMatcher = functions.Matcher
type FunctionMatchFunc = functions.MatchFunc

type VariableMatcher = variables.Matcher
type VariableMatchFunc = variables.MatchFunc

type FunctionCallMatcher = functioncalls.Matcher
type FunctionCallMatchFunc = functioncalls.MatchFunc

type ConditionMatcher = conditions.Matcher
type ConditionMatchFunc = conditions.MatchFunc

type loadWorkspaceOptions struct {
	reporter func(string)
}

// LoadWorkspaceOption configures workspace loading behavior.
type LoadWorkspaceOption func(*loadWorkspaceOptions)

// WithReporter configures a progress reporter callback.
func WithReporter(reporter func(string)) LoadWorkspaceOption {
	return func(opts *loadWorkspaceOptions) {
		opts.reporter = reporter
	}
}

// LoadWorkspace loads all packages in dir and returns a workspace.
func LoadWorkspace(ctx context.Context, dir string, opts ...LoadWorkspaceOption) (*Workspace, error) {
	cfg := &toolspackages.Config{
		Dir: dir,
		Mode: toolspackages.NeedName | toolspackages.NeedFiles | toolspackages.NeedSyntax |
			toolspackages.NeedCompiledGoFiles | toolspackages.NeedImports,
		Context: ctx,
	}

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

	report("Loading packages (./...)...")
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
			ID:     pkg.ID,
			Name:   pkg.Name,
			Fset:   pkg.Fset,
			Errors: pkg.Errors,
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

			indexFileEntries(&workspace.Types, &workspace.Functions, &workspace.Variables, &workspace.FunctionCalls, &workspace.Conditions, p, filename, file)
		}

		workspace.Packages.Add(p)
	}

	return workspace, nil
}

func indexFileEntries(
	typeEntries *types.Collection,
	functionEntries *functions.Collection,
	variableEntries *variables.Collection,
	callEntries *functioncalls.Collection,
	conditionEntries *conditions.Collection,
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

		case *ast.IfStmt:
			conditionEntries.Add(conditions.Item{Ref: newRef(pkg, filename, node), Kind: "if", Node: node})
		case *ast.SwitchStmt:
			conditionEntries.Add(conditions.Item{Ref: newRef(pkg, filename, node), Kind: "switch", Node: node})
		case *ast.TypeSwitchStmt:
			conditionEntries.Add(conditions.Item{Ref: newRef(pkg, filename, node), Kind: "type-switch", Node: node})
		case *ast.CaseClause:
			conditionEntries.Add(conditions.Item{Ref: newRef(pkg, filename, node), Kind: "case", Node: node})
		case *ast.SelectStmt:
			conditionEntries.Add(conditions.Item{Ref: newRef(pkg, filename, node), Kind: "select", Node: node})
		case *ast.CommClause:
			conditionEntries.Add(conditions.Item{Ref: newRef(pkg, filename, node), Kind: "comm", Node: node})
		}

		return true
	})
}

func newRef(pkg packages.Item, fallbackFilename string, n ast.Node) common.Ref {
	pos := pkg.Fset.PositionFor(n.Pos(), true)
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
