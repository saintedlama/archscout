package builder

import (
	"github.com/saintedlama/archscout/dependencies"
	"github.com/saintedlama/archscout/files"
	"github.com/saintedlama/archscout/functioncalls"
	"github.com/saintedlama/archscout/functions"
	"github.com/saintedlama/archscout/packages"
	"github.com/saintedlama/archscout/types"
	"github.com/saintedlama/archscout/variables"
)

// Snapshot is the immutable collection set produced by a Builder.
type Snapshot struct {
	Packages      packages.Collection
	Files         files.Collection
	Types         types.Collection
	Functions     functions.Collection
	Variables     variables.Collection
	FunctionCalls functioncalls.Collection
	Dependencies  dependencies.Collection
}

// Builder accumulates mutable workspace state before producing an immutable snapshot.
type Builder struct {
	packages      []packages.Item
	files         []files.Item
	types         []types.Item
	functions     []functions.Item
	variables     []variables.Item
	functionCalls []functioncalls.Item
	dependencies  []dependencies.Item
}

// New creates a mutable workspace builder.
func New() *Builder {
	return &Builder{}
}

// AddPackage appends a package item to the builder.
func (builder *Builder) AddPackage(item packages.Item) {
	builder.packages = append(builder.packages, item)
}

// AddFile appends a file item to the builder.
func (builder *Builder) AddFile(item files.Item) {
	builder.files = append(builder.files, item)
}

// AddType appends a type item to the builder.
func (builder *Builder) AddType(item types.Item) {
	builder.types = append(builder.types, item)
}

// AddFunction appends a function item to the builder.
func (builder *Builder) AddFunction(item functions.Item) {
	builder.functions = append(builder.functions, item)
}

// AddVariable appends a variable item to the builder.
func (builder *Builder) AddVariable(item variables.Item) {
	builder.variables = append(builder.variables, item)
}

// AddFunctionCall appends a function call item to the builder.
func (builder *Builder) AddFunctionCall(item functioncalls.Item) {
	builder.functionCalls = append(builder.functionCalls, item)
}

// AddDependency appends a dependency item to the builder.
func (builder *Builder) AddDependency(item dependencies.Item) {
	builder.dependencies = append(builder.dependencies, item)
}

// Build constructs an immutable workspace snapshot from the collected items.
func (builder *Builder) Build() Snapshot {
	dependenciesByFilename := make(map[string][]dependencies.Item, len(builder.files))
	dependenciesByPackage := make(map[string][]dependencies.Item, len(builder.packages))
	for _, item := range builder.dependencies {
		dependenciesByFilename[item.Ref.Filename] = append(dependenciesByFilename[item.Ref.Filename], item)
		dependenciesByPackage[item.Ref.PackageID] = append(dependenciesByPackage[item.Ref.PackageID], item)
	}

	filesWithDependencies := make([]files.Item, 0, len(builder.files))
	for _, item := range builder.files {
		filesWithDependencies = append(filesWithDependencies, item.WithDependencies(dependenciesByFilename[item.Ref.Filename]))
	}

	packagesWithDependencies := make([]packages.Item, 0, len(builder.packages))
	for _, item := range builder.packages {
		packagesWithDependencies = append(packagesWithDependencies, item.WithDependencies(dependenciesByPackage[item.ID]))
	}

	return Snapshot{
		Packages:      packages.NewCollection(packagesWithDependencies),
		Files:         files.NewCollection(filesWithDependencies),
		Types:         types.NewCollection(builder.types),
		Functions:     functions.NewCollection(builder.functions),
		Variables:     variables.NewCollection(builder.variables),
		FunctionCalls: functioncalls.NewCollection(builder.functionCalls),
		Dependencies:  dependencies.NewCollection(builder.dependencies),
	}
}
