package builder

import (
	"github.com/saintedlama/goarch/files"
	"github.com/saintedlama/goarch/functioncalls"
	"github.com/saintedlama/goarch/functions"
	"github.com/saintedlama/goarch/packages"
	"github.com/saintedlama/goarch/types"
	"github.com/saintedlama/goarch/variables"
)

// Snapshot is the immutable collection set produced by a Builder.
type Snapshot struct {
	Packages      packages.Collection
	Files         files.Collection
	Types         types.Collection
	Functions     functions.Collection
	Variables     variables.Collection
	FunctionCalls functioncalls.Collection
}

// Builder accumulates mutable workspace state before producing an immutable snapshot.
type Builder struct {
	packages      []packages.Item
	files         []files.Item
	types         []types.Item
	functions     []functions.Item
	variables     []variables.Item
	functionCalls []functioncalls.Item
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

// Build constructs an immutable workspace snapshot from the collected items.
func (builder *Builder) Build() Snapshot {
	return Snapshot{
		Packages:      packages.NewCollection(builder.packages),
		Files:         files.NewCollection(builder.files),
		Types:         types.NewCollection(builder.types),
		Functions:     functions.NewCollection(builder.functions),
		Variables:     variables.NewCollection(builder.variables),
		FunctionCalls: functioncalls.NewCollection(builder.functionCalls),
	}
}
