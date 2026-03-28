package goarch

import (
	"context"

	"github.com/saintedlama/goarch/analysis"
)

// Re-export the analysis API from the module root for simpler imports.
type Workspace struct {
	*analysis.Workspace
}

type Ref = analysis.Ref

type Package = analysis.Package
type File = analysis.File

type Type = analysis.Type
type Function = analysis.Function
type Variable = analysis.Variable
type FunctionCall = analysis.FunctionCall
type Condition = analysis.Condition

type PackageMatcher = analysis.PackageMatcher
type PackageMatchFunc = analysis.PackageMatchFunc

type TypeMatcher = analysis.TypeMatcher
type TypeMatchFunc = analysis.TypeMatchFunc

type FunctionMatcher = analysis.FunctionMatcher
type FunctionMatchFunc = analysis.FunctionMatchFunc

type VariableMatcher = analysis.VariableMatcher
type VariableMatchFunc = analysis.VariableMatchFunc

type FunctionCallMatcher = analysis.FunctionCallMatcher
type FunctionCallMatchFunc = analysis.FunctionCallMatchFunc

type ConditionMatcher = analysis.ConditionMatcher
type ConditionMatchFunc = analysis.ConditionMatchFunc

type LoadWorkspaceOption = analysis.LoadWorkspaceOption

func WithReporter(reporter func(string)) LoadWorkspaceOption {
	return analysis.WithReporter(reporter)
}

func LoadWorkspace(ctx context.Context, dir string, opts ...LoadWorkspaceOption) (*Workspace, error) {
	workspace, err := analysis.LoadWorkspace(ctx, dir, opts...)
	if err != nil {
		return nil, err
	}
	return &Workspace{Workspace: workspace}, nil
}

// MatchPackages runs a matcher over all packages and returns generated code refs.
func (workspace *Workspace) MatchPackages(matcher PackageMatcher) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Packages.Match(matcher)
}

// MatchTypes runs a matcher over all type entries and returns generated code refs.
func (workspace *Workspace) MatchTypes(matcher TypeMatcher) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Types.Match(matcher)
}

// MatchFunctions runs a matcher over all function entries and returns generated code refs.
func (workspace *Workspace) MatchFunctions(matcher FunctionMatcher) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Functions.Match(matcher)
}

// MatchVariables runs a matcher over all variable entries and returns generated code refs.
func (workspace *Workspace) MatchVariables(matcher VariableMatcher) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Variables.Match(matcher)
}

// MatchFunctionCalls runs a matcher over all call entries and returns generated code refs.
func (workspace *Workspace) MatchFunctionCalls(matcher FunctionCallMatcher) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.FunctionCalls.Match(matcher)
}

// MatchConditions runs a matcher over all condition entries and returns generated code refs.
func (workspace *Workspace) MatchConditions(matcher ConditionMatcher) []Ref {
	if workspace == nil || matcher == nil {
		return nil
	}
	return workspace.Conditions.Match(matcher)
}
