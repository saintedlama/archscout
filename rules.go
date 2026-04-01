package goarch

import (
	"fmt"
	"testing"

	"github.com/saintedlama/goarch/common"
	"github.com/saintedlama/goarch/dependencies"
	"github.com/saintedlama/goarch/files"
	"github.com/saintedlama/goarch/functioncalls"
	"github.com/saintedlama/goarch/functions"
	"github.com/saintedlama/goarch/packages"
	"github.com/saintedlama/goarch/types"
	"github.com/saintedlama/goarch/variables"
)

type testFilterMode int

const (
	testFilterAny testFilterMode = iota
	testFilterOnly
	testFilterExclude
)

// RuleBuilder starts construction for named architecture rules.
type RuleBuilder struct {
	name string
}

type ruleFilters struct {
	inPackages    []string
	notInPackages []string
	testFilter    testFilterMode
}

// Rule creates a new named rule that can be configured independently of a workspace.
func Rule(name string) RuleBuilder {
	return RuleBuilder{name: name}
}

// Packages configures a package-entry rule.
func (b RuleBuilder) Packages() *PackageRule {
	return &PackageRule{name: b.name}
}

// Files configures a file-entry rule.
func (b RuleBuilder) Files() *FileRule {
	return &FileRule{name: b.name}
}

// Types configures a type-entry rule.
func (b RuleBuilder) Types() *TypeRule {
	return &TypeRule{name: b.name}
}

// Functions configures a function-entry rule.
func (b RuleBuilder) Functions() *FunctionRule {
	return &FunctionRule{name: b.name}
}

// Variables configures a variable-entry rule.
func (b RuleBuilder) Variables() *VariableRule {
	return &VariableRule{name: b.name}
}

// FunctionCalls configures a function-call-entry rule.
func (b RuleBuilder) FunctionCalls() *FunctionCallRule {
	return &FunctionCallRule{name: b.name}
}

// Dependencies configures a dependency-entry rule.
func (b RuleBuilder) Dependencies() *DependencyRule {
	return &DependencyRule{name: b.name}
}

// PackageRule evaluates predicates against package entries.
type PackageRule struct {
	name    string
	filters ruleFilters
	matcher PackageMatchFunc
}

// FileRule evaluates predicates against file entries.
type FileRule struct {
	name    string
	filters ruleFilters
	matcher FileMatchFunc
}

// TypeRule evaluates predicates against type entries.
type TypeRule struct {
	name    string
	filters ruleFilters
	matcher TypeMatchFunc
}

// FunctionRule evaluates predicates against function entries.
type FunctionRule struct {
	name    string
	filters ruleFilters
	matcher FunctionMatchFunc
}

// VariableRule evaluates predicates against variable entries.
type VariableRule struct {
	name    string
	filters ruleFilters
	matcher VariableMatchFunc
}

// FunctionCallRule evaluates predicates against function call entries.
type FunctionCallRule struct {
	name    string
	filters ruleFilters
	matcher FunctionCallMatchFunc
}

// DependencyRule evaluates predicates against dependency entries.
type DependencyRule struct {
	name            string
	filters         ruleFilters
	matcher         DependencyMatchFunc
	within          *bool
	stdlibOnly      bool
	thirdPartyOnly  bool
	dependsOn       []string
	doesNotDependOn []string
}

func (r *PackageRule) InPackage(patterns ...string) *PackageRule {
	r.filters.inPackages = append(r.filters.inPackages, patterns...)
	return r
}

func (r *PackageRule) NotInPackage(patterns ...string) *PackageRule {
	r.filters.notInPackages = append(r.filters.notInPackages, patterns...)
	return r
}

func (r *PackageRule) IsTest() *PackageRule {
	r.filters.testFilter = testFilterOnly
	return r
}

func (r *PackageRule) IsNotTest() *PackageRule {
	r.filters.testFilter = testFilterExclude
	return r
}

func (r *PackageRule) Match(matcher PackageMatchFunc) *PackageRule {
	r.matcher = matcher
	return r
}

func (r *PackageRule) Test(t testing.TB, ws *Workspace) {
	t.Helper()
	refs, err := r.Evaluate(ws)
	failRuleIfNeeded(t, r.name, refs, err)
}

func (r *PackageRule) Evaluate(ws *Workspace) (Refs, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	if r.matcher == nil {
		return nil, fmt.Errorf("no matcher configured")
	}

	collection := ws.Packages
	collection = applyPackageFilters(collection, r.filters)

	return collection.Match(r.matcher), nil
}

func (r *FileRule) InPackage(patterns ...string) *FileRule {
	r.filters.inPackages = append(r.filters.inPackages, patterns...)
	return r
}

func (r *FileRule) NotInPackage(patterns ...string) *FileRule {
	r.filters.notInPackages = append(r.filters.notInPackages, patterns...)
	return r
}

func (r *FileRule) IsTest() *FileRule {
	r.filters.testFilter = testFilterOnly
	return r
}

func (r *FileRule) IsNotTest() *FileRule {
	r.filters.testFilter = testFilterExclude
	return r
}

func (r *FileRule) Match(matcher FileMatchFunc) *FileRule {
	r.matcher = matcher
	return r
}

func (r *FileRule) Test(t testing.TB, ws *Workspace) {
	t.Helper()
	refs, err := r.Evaluate(ws)
	failRuleIfNeeded(t, r.name, refs, err)
}

func (r *FileRule) Evaluate(ws *Workspace) (Refs, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	if r.matcher == nil {
		return nil, fmt.Errorf("no matcher configured")
	}

	collection := ws.Files
	collection = applyFileFilters(collection, r.filters)

	return collection.Match(r.matcher), nil
}

func (r *TypeRule) InPackage(patterns ...string) *TypeRule {
	r.filters.inPackages = append(r.filters.inPackages, patterns...)
	return r
}

func (r *TypeRule) NotInPackage(patterns ...string) *TypeRule {
	r.filters.notInPackages = append(r.filters.notInPackages, patterns...)
	return r
}

func (r *TypeRule) IsTest() *TypeRule {
	r.filters.testFilter = testFilterOnly
	return r
}

func (r *TypeRule) IsNotTest() *TypeRule {
	r.filters.testFilter = testFilterExclude
	return r
}

func (r *TypeRule) Match(matcher TypeMatchFunc) *TypeRule {
	r.matcher = matcher
	return r
}

func (r *TypeRule) Test(t testing.TB, ws *Workspace) {
	t.Helper()
	refs, err := r.Evaluate(ws)
	failRuleIfNeeded(t, r.name, refs, err)
}

func (r *TypeRule) Evaluate(ws *Workspace) (Refs, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	if r.matcher == nil {
		return nil, fmt.Errorf("no matcher configured")
	}

	collection := ws.Types
	collection = applyTypeFilters(collection, r.filters)

	return collection.Match(r.matcher), nil
}

func (r *FunctionRule) InPackage(patterns ...string) *FunctionRule {
	r.filters.inPackages = append(r.filters.inPackages, patterns...)
	return r
}

func (r *FunctionRule) NotInPackage(patterns ...string) *FunctionRule {
	r.filters.notInPackages = append(r.filters.notInPackages, patterns...)
	return r
}

func (r *FunctionRule) IsTest() *FunctionRule {
	r.filters.testFilter = testFilterOnly
	return r
}

func (r *FunctionRule) IsNotTest() *FunctionRule {
	r.filters.testFilter = testFilterExclude
	return r
}

func (r *FunctionRule) Match(matcher FunctionMatchFunc) *FunctionRule {
	r.matcher = matcher
	return r
}

func (r *FunctionRule) Test(t testing.TB, ws *Workspace) {
	t.Helper()
	refs, err := r.Evaluate(ws)
	failRuleIfNeeded(t, r.name, refs, err)
}

func (r *FunctionRule) Evaluate(ws *Workspace) (Refs, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	if r.matcher == nil {
		return nil, fmt.Errorf("no matcher configured")
	}

	collection := ws.Functions
	collection = applyFunctionFilters(collection, r.filters)

	return collection.Match(r.matcher), nil
}

func (r *VariableRule) InPackage(patterns ...string) *VariableRule {
	r.filters.inPackages = append(r.filters.inPackages, patterns...)
	return r
}

func (r *VariableRule) NotInPackage(patterns ...string) *VariableRule {
	r.filters.notInPackages = append(r.filters.notInPackages, patterns...)
	return r
}

func (r *VariableRule) IsTest() *VariableRule {
	r.filters.testFilter = testFilterOnly
	return r
}

func (r *VariableRule) IsNotTest() *VariableRule {
	r.filters.testFilter = testFilterExclude
	return r
}

func (r *VariableRule) Match(matcher VariableMatchFunc) *VariableRule {
	r.matcher = matcher
	return r
}

func (r *VariableRule) Test(t testing.TB, ws *Workspace) {
	t.Helper()
	refs, err := r.Evaluate(ws)
	failRuleIfNeeded(t, r.name, refs, err)
}

func (r *VariableRule) Evaluate(ws *Workspace) (Refs, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	if r.matcher == nil {
		return nil, fmt.Errorf("no matcher configured")
	}

	collection := ws.Variables
	collection = applyVariableFilters(collection, r.filters)

	return collection.Match(r.matcher), nil
}

func (r *FunctionCallRule) InPackage(patterns ...string) *FunctionCallRule {
	r.filters.inPackages = append(r.filters.inPackages, patterns...)
	return r
}

func (r *FunctionCallRule) NotInPackage(patterns ...string) *FunctionCallRule {
	r.filters.notInPackages = append(r.filters.notInPackages, patterns...)
	return r
}

func (r *FunctionCallRule) IsTest() *FunctionCallRule {
	r.filters.testFilter = testFilterOnly
	return r
}

func (r *FunctionCallRule) IsNotTest() *FunctionCallRule {
	r.filters.testFilter = testFilterExclude
	return r
}

func (r *FunctionCallRule) Match(matcher FunctionCallMatchFunc) *FunctionCallRule {
	r.matcher = matcher
	return r
}

func (r *FunctionCallRule) Test(t testing.TB, ws *Workspace) {
	t.Helper()
	refs, err := r.Evaluate(ws)
	failRuleIfNeeded(t, r.name, refs, err)
}

func (r *FunctionCallRule) Evaluate(ws *Workspace) (Refs, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}
	if r.matcher == nil {
		return nil, fmt.Errorf("no matcher configured")
	}

	collection := ws.FunctionCalls
	collection = applyFunctionCallFilters(collection, r.filters)

	return collection.Match(r.matcher), nil
}

func (r *DependencyRule) InPackage(patterns ...string) *DependencyRule {
	r.filters.inPackages = append(r.filters.inPackages, patterns...)
	return r
}

func (r *DependencyRule) NotInPackage(patterns ...string) *DependencyRule {
	r.filters.notInPackages = append(r.filters.notInPackages, patterns...)
	return r
}

func (r *DependencyRule) IsTest() *DependencyRule {
	r.filters.testFilter = testFilterOnly
	return r
}

func (r *DependencyRule) IsNotTest() *DependencyRule {
	r.filters.testFilter = testFilterExclude
	return r
}

func (r *DependencyRule) IsWithinWorkspace() *DependencyRule {
	value := true
	r.within = &value
	return r
}

func (r *DependencyRule) IsExternal() *DependencyRule {
	value := false
	r.within = &value
	return r
}

func (r *DependencyRule) IsStandardLibrary() *DependencyRule {
	r.stdlibOnly = true
	return r
}

func (r *DependencyRule) IsThirdParty() *DependencyRule {
	r.thirdPartyOnly = true
	return r
}

func (r *DependencyRule) DependOn(patterns ...string) *DependencyRule {
	r.dependsOn = append(r.dependsOn, patterns...)
	return r
}

func (r *DependencyRule) DoNotDependOn(patterns ...string) *DependencyRule {
	r.doesNotDependOn = append(r.doesNotDependOn, patterns...)
	return r
}

func (r *DependencyRule) Match(matcher DependencyMatchFunc) *DependencyRule {
	r.matcher = matcher
	return r
}

func (r *DependencyRule) Test(t testing.TB, ws *Workspace) {
	t.Helper()
	refs, err := r.Evaluate(ws)
	failRuleIfNeeded(t, r.name, refs, err)
}

func (r *DependencyRule) Evaluate(ws *Workspace) (Refs, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is nil")
	}

	collection := ws.Dependencies
	collection = applyDependencyFilters(collection, r.filters)

	if r.within != nil {
		if *r.within {
			collection = collection.IsWithinWorkspace()
		} else {
			collection = collection.IsExternal()
		}
	}
	if r.stdlibOnly {
		collection = collection.IsStandardLibrary()
	}
	if r.thirdPartyOnly {
		collection = collection.IsThirdParty()
	}
	if len(r.dependsOn) > 0 {
		collection = collection.DependOn(r.dependsOn...)
	}
	if len(r.doesNotDependOn) > 0 {
		collection = collection.DoNotDependOn(r.doesNotDependOn...)
	}

	if r.matcher != nil {
		return collection.Match(r.matcher), nil
	}

	// No matcher: every dependency surviving the filters is a violation.
	if collection.Len() == 0 {
		return nil, nil
	}
	var refs Refs
	for _, item := range collection.All() {
		refs = append(refs, item.Ref)
	}
	return refs, nil
}

func applyPackageFilters(collection packages.Collection, filters ruleFilters) packages.Collection {
	if len(filters.inPackages) > 0 {
		collection = collection.InPackage(filters.inPackages...)
	}
	if len(filters.notInPackages) > 0 {
		collection = collection.NotInPackage(filters.notInPackages...)
	}
	switch filters.testFilter {
	case testFilterOnly:
		collection = collection.IsTest()
	case testFilterExclude:
		collection = collection.IsNotTest()
	}

	return collection
}

func applyFileFilters(collection files.Collection, filters ruleFilters) files.Collection {
	if len(filters.inPackages) > 0 {
		collection = collection.InPackage(filters.inPackages...)
	}
	if len(filters.notInPackages) > 0 {
		collection = collection.NotInPackage(filters.notInPackages...)
	}
	switch filters.testFilter {
	case testFilterOnly:
		collection = collection.IsTest()
	case testFilterExclude:
		collection = collection.IsNotTest()
	}

	return collection
}

func applyTypeFilters(collection types.Collection, filters ruleFilters) types.Collection {
	if len(filters.inPackages) > 0 {
		collection = collection.InPackage(filters.inPackages...)
	}
	if len(filters.notInPackages) > 0 {
		collection = collection.NotInPackage(filters.notInPackages...)
	}
	switch filters.testFilter {
	case testFilterOnly:
		collection = collection.IsTest()
	case testFilterExclude:
		collection = collection.IsNotTest()
	}

	return collection
}

func applyFunctionFilters(collection functions.Collection, filters ruleFilters) functions.Collection {
	if len(filters.inPackages) > 0 {
		collection = collection.InPackage(filters.inPackages...)
	}
	if len(filters.notInPackages) > 0 {
		collection = collection.NotInPackage(filters.notInPackages...)
	}
	switch filters.testFilter {
	case testFilterOnly:
		collection = collection.IsTest()
	case testFilterExclude:
		collection = collection.IsNotTest()
	}

	return collection
}

func applyVariableFilters(collection variables.Collection, filters ruleFilters) variables.Collection {
	if len(filters.inPackages) > 0 {
		collection = collection.InPackage(filters.inPackages...)
	}
	if len(filters.notInPackages) > 0 {
		collection = collection.NotInPackage(filters.notInPackages...)
	}
	switch filters.testFilter {
	case testFilterOnly:
		collection = collection.IsTest()
	case testFilterExclude:
		collection = collection.IsNotTest()
	}

	return collection
}

func applyFunctionCallFilters(collection functioncalls.Collection, filters ruleFilters) functioncalls.Collection {
	if len(filters.inPackages) > 0 {
		collection = collection.InPackage(filters.inPackages...)
	}
	if len(filters.notInPackages) > 0 {
		collection = collection.NotInPackage(filters.notInPackages...)
	}
	switch filters.testFilter {
	case testFilterOnly:
		collection = collection.IsTest()
	case testFilterExclude:
		collection = collection.IsNotTest()
	}

	return collection
}

func applyDependencyFilters(collection dependencies.Collection, filters ruleFilters) dependencies.Collection {
	if len(filters.inPackages) > 0 {
		collection = collection.InPackage(filters.inPackages...)
	}
	if len(filters.notInPackages) > 0 {
		collection = collection.NotInPackage(filters.notInPackages...)
	}
	switch filters.testFilter {
	case testFilterOnly:
		collection = collection.IsTest()
	case testFilterExclude:
		collection = collection.IsNotTest()
	}

	return collection
}

func failRuleIfNeeded(t testing.TB, name string, refs common.Refs, err error) {
	if err != nil {
		t.Fatalf("rule %q misconfigured: %v", name, err)
	}
	if len(refs) == 0 {
		return
	}

	t.Fatalf(
		"rule %q violated: %d match(es)\n%s",
		name,
		len(refs),
		refs.Format(WithRefPackage(), WithRefKind()),
	)
}
