package archscout

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/saintedlama/archscout/common"
	"github.com/saintedlama/archscout/dependencies"
	"github.com/saintedlama/archscout/files"
	"github.com/saintedlama/archscout/functioncalls"
	"github.com/saintedlama/archscout/functions"
	"github.com/saintedlama/archscout/packages"
	"github.com/saintedlama/archscout/types"
	"github.com/saintedlama/archscout/variables"
	workspacebuilder "github.com/saintedlama/archscout/workspace/builder"

	toolspackages "golang.org/x/tools/go/packages"
)

// cacheVersion must be incremented whenever the snapshot layout changes to
// prevent stale cache files from being decoded.
const cacheVersion = 1

// workspaceSnap is the gob-serializable snapshot of a Workspace.
//
// go/ast Node pointers are intentionally omitted; they cannot be serialized.
// After a successful cache load those fields in every Item will be nil.
type workspaceSnap struct {
	Version       int
	Packages      []packageSnap
	Files         []fileSnap
	Types         []typeSnap
	Functions     []functionSnap
	Variables     []variableSnap
	FunctionCalls []functionCallSnap
	Dependencies  []dependencySnap
}

type packageSnap struct {
	ID     string
	Name   string
	Files  []string
	Errors []errorSnap
}

type errorSnap struct {
	Pos  string
	Msg  string
	Kind int
}

type fileSnap struct {
	Ref      common.Ref
	Filename string
}

type typeSnap struct {
	Ref  common.Ref
	Name string
	Kind string
}

type functionSnap struct {
	Ref      common.Ref
	Name     string
	Receiver string
}

type variableSnap struct {
	Ref  common.Ref
	Name string
	Kind string
}

type functionCallSnap struct {
	Ref    common.Ref
	Callee string
}

type dependencySnap struct {
	Ref               common.Ref
	ImportPath        string
	WithinWorkspace   bool
	External          bool
	StandardLibrary   bool
	TargetPackageName string
}

// defaultCacheDir returns the platform cache directory used by WithDiskCache().
// It prefers os.UserCacheDir()/archscout-cache and falls back to os.TempDir()/archscout-cache.
func defaultCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "archscout-cache")
	}
	return filepath.Join(os.TempDir(), "archscout-cache")
}

// loadWithDiskCache checks the disk cache before delegating to parseWorkspace.
// When diskCacheDir is empty the function behaves identically to parseWorkspace.
func loadWithDiskCache(ctx context.Context, dir string, diskCacheDir string, report func(string)) (*Workspace, error) {
	if diskCacheDir == "" {
		return parseWorkspace(ctx, dir, report)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return parseWorkspace(ctx, dir, report)
	}

	fp, err := computeFingerprint(absDir)
	if err != nil {
		report(fmt.Sprintf("Warning: cache fingerprint failed (%v), falling back to full parse", err))
		return parseWorkspace(ctx, dir, report)
	}

	cachePath := filepath.Join(diskCacheDir, fp+".gob")

	if ws, _ := loadWorkspaceFromDisk(cachePath); ws != nil {
		report("Using disk-cached workspace")
		return ws, nil
	}

	ws, err := parseWorkspace(ctx, dir, report)
	if err != nil {
		return nil, err
	}

	if saveErr := saveWorkspaceToDisk(ws, cachePath); saveErr != nil {
		report(fmt.Sprintf("Warning: failed to save workspace cache: %v", saveErr))
	}

	return ws, nil
}

// computeFingerprint returns a SHA256 hex string derived from:
//   - the absolute project directory (so two projects in the same cache dir never collide),
//   - the relative path, modification time and size of every .go source file and go.sum under dir.
//
// The vendor directory and hidden directories (name starting with '.') are skipped.
func computeFingerprint(dir string) (string, error) {
	type entry struct {
		path  string
		mtime int64
		size  int64
	}

	var entries []entry

	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || (len(name) > 0 && name[0] == '.') {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".go") || name == "go.sum" {
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				return relErr
			}
			entries = append(entries, entry{
				path:  rel,
				mtime: info.ModTime().UnixNano(),
				size:  info.Size(),
			})
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("computing fingerprint for %q: %w", dir, err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	h := sha256.New()
	// Include the project dir so two different projects never produce the same fingerprint.
	fmt.Fprintf(h, "dir:%s\n", dir)
	for _, e := range entries {
		fmt.Fprintf(h, "%s\t%d\t%d\n", e.path, e.mtime, e.size)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// saveWorkspaceToDisk encodes ws as a gob snapshot and writes it atomically to path.
func saveWorkspaceToDisk(ws *Workspace, path string) error {
	snap := buildSnap(ws)

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("creating cache file: %w", err)
	}

	encErr := gob.NewEncoder(f).Encode(snap)
	closeErr := f.Close()
	if encErr != nil {
		os.Remove(tmp)
		return fmt.Errorf("encoding workspace snapshot: %w", encErr)
	}
	if closeErr != nil {
		os.Remove(tmp)
		return fmt.Errorf("flushing cache file: %w", closeErr)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming cache file into place: %w", err)
	}

	return nil
}

// loadWorkspaceFromDisk decodes a gob snapshot from path and reconstructs a Workspace.
// Returns nil, nil on cache miss (file absent, corrupt, or version mismatch).
func loadWorkspaceFromDisk(path string) (*Workspace, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, nil // treat as miss
	}
	defer f.Close()

	var snap workspaceSnap
	if err := gob.NewDecoder(f).Decode(&snap); err != nil {
		return nil, nil // corrupt cache → miss
	}
	if snap.Version != cacheVersion {
		return nil, nil // stale format → miss
	}

	return snapToWorkspace(snap), nil
}

// buildSnap converts a Workspace into its serializable snapshot form.
func buildSnap(ws *Workspace) workspaceSnap {
	snap := workspaceSnap{Version: cacheVersion}

	for _, p := range ws.Packages.All() {
		ps := packageSnap{ID: p.ID, Name: p.Name}
		for _, pf := range p.Files {
			ps.Files = append(ps.Files, pf.Filename)
		}
		for _, e := range p.Errors {
			ps.Errors = append(ps.Errors, errorSnap{Pos: e.Pos, Msg: e.Msg, Kind: int(e.Kind)})
		}
		snap.Packages = append(snap.Packages, ps)
	}

	for _, f := range ws.Files.All() {
		snap.Files = append(snap.Files, fileSnap{Ref: f.Ref, Filename: f.Filename})
	}

	for _, t := range ws.Types.All() {
		snap.Types = append(snap.Types, typeSnap{Ref: t.Ref, Name: t.Name, Kind: t.Kind})
	}

	for _, fn := range ws.Functions.All() {
		snap.Functions = append(snap.Functions, functionSnap{Ref: fn.Ref, Name: fn.Name, Receiver: fn.Receiver})
	}

	for _, v := range ws.Variables.All() {
		snap.Variables = append(snap.Variables, variableSnap{Ref: v.Ref, Name: v.Name, Kind: v.Kind})
	}

	for _, fc := range ws.FunctionCalls.All() {
		snap.FunctionCalls = append(snap.FunctionCalls, functionCallSnap{Ref: fc.Ref, Callee: fc.Callee})
	}

	for _, d := range ws.Dependencies.All() {
		snap.Dependencies = append(snap.Dependencies, dependencySnap{
			Ref:               d.Ref,
			ImportPath:        d.ImportPath,
			WithinWorkspace:   d.WithinWorkspace,
			External:          d.External,
			StandardLibrary:   d.StandardLibrary,
			TargetPackageName: d.TargetPackageName,
		})
	}

	return snap
}

// snapToWorkspace reconstructs a Workspace from a snapshot using the builder.
// Node fields (go/ast pointers) will be nil in the returned workspace.
func snapToWorkspace(snap workspaceSnap) *Workspace {
	wb := workspacebuilder.New()

	for _, ps := range snap.Packages {
		item := packages.Item{ID: ps.ID, Name: ps.Name}
		for _, fn := range ps.Files {
			item.Files = append(item.Files, packages.File{Filename: fn})
		}
		for _, e := range ps.Errors {
			item.Errors = append(item.Errors, toolspackages.Error{
				Pos:  e.Pos,
				Msg:  e.Msg,
				Kind: toolspackages.ErrorKind(e.Kind),
			})
		}
		wb.AddPackage(item)
	}

	for _, fs := range snap.Files {
		wb.AddFile(files.Item{Ref: fs.Ref, Filename: fs.Filename})
	}

	for _, ts := range snap.Types {
		wb.AddType(types.Item{Ref: ts.Ref, Name: ts.Name, Kind: ts.Kind})
	}

	for _, fs := range snap.Functions {
		wb.AddFunction(functions.Item{Ref: fs.Ref, Name: fs.Name, Receiver: fs.Receiver})
	}

	for _, vs := range snap.Variables {
		wb.AddVariable(variables.Item{Ref: vs.Ref, Name: vs.Name, Kind: vs.Kind})
	}

	for _, fcs := range snap.FunctionCalls {
		wb.AddFunctionCall(functioncalls.Item{Ref: fcs.Ref, Callee: fcs.Callee})
	}

	for _, ds := range snap.Dependencies {
		wb.AddDependency(dependencies.Item{
			Ref:               ds.Ref,
			ImportPath:        ds.ImportPath,
			WithinWorkspace:   ds.WithinWorkspace,
			External:          ds.External,
			StandardLibrary:   ds.StandardLibrary,
			TargetPackageName: ds.TargetPackageName,
		})
	}

	s := wb.Build()
	return &Workspace{
		Packages:      s.Packages,
		Files:         s.Files,
		Types:         s.Types,
		Functions:     s.Functions,
		Variables:     s.Variables,
		FunctionCalls: s.FunctionCalls,
		Dependencies:  s.Dependencies,
	}
}
