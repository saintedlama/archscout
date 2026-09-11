package codegraph

import (
	"path/filepath"
	"strings"

	"github.com/saintedlama/archscout/common"
	"github.com/saintedlama/archscout/dependencies"
	"github.com/saintedlama/archscout/files"
	"github.com/saintedlama/archscout/functioncalls"
	"github.com/saintedlama/archscout/functions"
	"github.com/saintedlama/archscout/implementsgraph"
	"github.com/saintedlama/archscout/packages"
	"github.com/saintedlama/archscout/types"
)

// Input carries all collections and metadata required to construct a CodeGraph.
type Input struct {
	ModuleRoot    string
	Packages      packages.Collection
	Files         files.Collection
	Types         types.Collection
	Functions     functions.Collection
	FunctionCalls functioncalls.Collection
	Dependencies  dependencies.Collection
	Implements    *implementsgraph.Graph
}

// Build constructs a fully-indexed CodeGraph from the provided collections.
func Build(in Input) *Graph {
	g := newGraph()
	seenEdges := make(map[string]struct{})

	addEdge := func(src, dst string, kind EdgeKind, ref common.Ref) {
		if src == "" || dst == "" {
			return
		}
		key := src + "|" + dst + "|" + string(kind)
		if _, exists := seenEdges[key]; exists {
			return
		}
		seenEdges[key] = struct{}{}
		g.addEdge(Edge{
			Source: src,
			Target: dst,
			Kind:   kind,
			Ref:    ref,
		})
	}

	// 1. Module node
	var modID string
	if in.ModuleRoot != "" {
		modID = ModuleNodeID(in.ModuleRoot)
		g.addNode(Node{
			ID:              modID,
			Kind:            NodeKindModule,
			Name:            in.ModuleRoot,
			WithinWorkspace: true,
		})
	}

	// 2. Packages
	for _, pkg := range in.Packages.All() {
		pkgID := PackageNodeID(pkg.ID)
		g.addNode(Node{
			ID:              pkgID,
			Kind:            NodeKindPackage,
			Name:            pkg.Name,
			PackageID:       pkg.ID,
			ParentID:        modID,
			WithinWorkspace: true,
		})
		if modID != "" {
			addEdge(modID, pkgID, EdgeKindContains, common.Ref{})
		}
	}

	// 3. Files
	for _, file := range in.Files.All() {
		fileID := FileNodeID(file.Filename)
		pkgID := PackageNodeID(file.Ref.PackageID)
		g.addNode(Node{
			ID:              fileID,
			Kind:            NodeKindFile,
			Name:            filepath.Base(file.Filename),
			PackageID:       file.Ref.PackageID,
			ParentID:        pkgID,
			Ref:             file.Ref,
			WithinWorkspace: true,
		})
		addEdge(pkgID, fileID, EdgeKindContains, file.Ref)
	}

	// 4. Dependencies (Imports)
	for _, dep := range in.Dependencies.All() {
		if dep.ImportPath == "" {
			continue
		}
		dstPkgID := PackageNodeID(dep.ImportPath)
		if !g.HasNode(dstPkgID) {
			g.addNode(Node{
				ID:              dstPkgID,
				Kind:            NodeKindPackage,
				Name:            dep.TargetPackageName,
				PackageID:       dep.ImportPath,
				WithinWorkspace: dep.WithinWorkspace,
			})
		}

		// File -> Package import
		if dep.Ref.Filename != "" {
			srcFileID := FileNodeID(dep.Ref.Filename)
			addEdge(srcFileID, dstPkgID, EdgeKindImports, dep.Ref)
		}

		// Package -> Package import
		if dep.Ref.PackageID != "" {
			srcPkgID := PackageNodeID(dep.Ref.PackageID)
			addEdge(srcPkgID, dstPkgID, EdgeKindImports, dep.Ref)
		}
	}

	// 5. Types
	for _, t := range in.Types.All() {
		typeID := TypeNodeID(t.QName)
		fileID := FileNodeID(t.Ref.Filename)
		g.addNode(Node{
			ID:              typeID,
			Kind:            NodeKindType,
			Name:            t.Name,
			QName:           t.QName,
			PackageID:       t.Ref.PackageID,
			ParentID:        fileID,
			Ref:             t.Ref,
			WithinWorkspace: true,
		})
		if fileID != "" {
			addEdge(fileID, typeID, EdgeKindContains, t.Ref)
		}

		// Embedded types
		for _, embed := range t.Embeds {
			if embed == "" {
				continue
			}
			targetTypeID := TypeNodeID(embed)
			if !g.HasNode(targetTypeID) {
				g.addNode(Node{
					ID:              targetTypeID,
					Kind:            NodeKindType,
					Name:            identifierName(embed),
					QName:           embed,
					PackageID:       qualifierPackage(embed),
					WithinWorkspace: false,
				})
			}
			addEdge(typeID, targetTypeID, EdgeKindEmbeds, t.Ref)
		}

		// Field type references
		for _, f := range t.Fields {
			if f.TypeQName == "" || f.Embedded {
				continue
			}
			targetTypeID := TypeNodeID(f.TypeQName)
			if !g.HasNode(targetTypeID) {
				g.addNode(Node{
					ID:              targetTypeID,
					Kind:            NodeKindType,
					Name:            identifierName(f.TypeQName),
					QName:           f.TypeQName,
					PackageID:       qualifierPackage(f.TypeQName),
					WithinWorkspace: false,
				})
			}
			addEdge(typeID, targetTypeID, EdgeKindReferencesType, t.Ref)
		}

		// Interface satisfaction
		if in.Implements != nil {
			for _, ifaceQName := range in.Implements.Interfaces(t.QName) {
				if ifaceQName == "" {
					continue
				}
				ifaceID := TypeNodeID(ifaceQName)
				if !g.HasNode(ifaceID) {
					g.addNode(Node{
						ID:              ifaceID,
						Kind:            NodeKindType,
						Name:            identifierName(ifaceQName),
						QName:           ifaceQName,
						PackageID:       qualifierPackage(ifaceQName),
						WithinWorkspace: false,
					})
				}
				addEdge(typeID, ifaceID, EdgeKindImplements, t.Ref)
			}
		}
	}

	// 6. Functions & Methods
	for _, fn := range in.Functions.All() {
		funcID := FunctionNodeID(fn.QName)
		fileID := FileNodeID(fn.Ref.Filename)
		parentID := fileID

		if fn.Receiver != "" {
			recvType := strings.TrimPrefix(fn.Receiver, "*")
			recvQName := fn.Ref.PackageID + "." + recvType
			parentID = TypeNodeID(recvQName)
		}

		g.addNode(Node{
			ID:              funcID,
			Kind:            NodeKindFunction,
			Name:            fn.Name,
			QName:           fn.QName,
			PackageID:       fn.Ref.PackageID,
			ParentID:        parentID,
			Ref:             fn.Ref,
			WithinWorkspace: true,
		})

		if fileID != "" {
			addEdge(fileID, funcID, EdgeKindContains, fn.Ref)
		}
		if fn.Receiver != "" && parentID != "" {
			addEdge(parentID, funcID, EdgeKindContains, fn.Ref)
		}
	}

	// 7. Function Calls
	for _, call := range in.FunctionCalls.All() {
		var callerID string
		if call.CallerQName != "" {
			callerID = FunctionNodeID(call.CallerQName)
		} else if call.Ref.Filename != "" {
			callerID = FileNodeID(call.Ref.Filename)
		}

		if callerID == "" {
			continue
		}

		if call.CalleeQName != "" {
			calleeID := FunctionNodeID(call.CalleeQName)
			if !g.HasNode(calleeID) {
				g.addNode(Node{
					ID:              calleeID,
					Kind:            NodeKindFunction,
					Name:            identifierName(call.CalleeQName),
					QName:           call.CalleeQName,
					PackageID:       call.CalleePackage,
					WithinWorkspace: false,
				})
			}
			addEdge(callerID, calleeID, EdgeKindCalls, call.Ref)
		} else if call.CalleePackage != "" {
			calleePkgID := PackageNodeID(call.CalleePackage)
			if !g.HasNode(calleePkgID) {
				g.addNode(Node{
					ID:              calleePkgID,
					Kind:            NodeKindPackage,
					Name:            identifierName(call.CalleePackage),
					PackageID:       call.CalleePackage,
					WithinWorkspace: false,
				})
			}
			addEdge(callerID, calleePkgID, EdgeKindCalls, call.Ref)
		}
	}

	g.finalize()
	return g
}

func identifierName(qname string) string {
	idx := strings.LastIndex(qname, ".")
	if idx >= 0 && idx+1 < len(qname) {
		return qname[idx+1:]
	}
	return qname
}

func qualifierPackage(qname string) string {
	idx := strings.LastIndex(qname, ".")
	if idx >= 0 {
		return qname[:idx]
	}
	return ""
}
