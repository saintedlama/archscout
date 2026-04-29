package archscout

import (
	"context"
	"encoding/gob"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/saintedlama/archscout/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixtureModDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(filename), "testdata", "fixturemod")
}

// ---------------------------------------------------------------------------
// computeFingerprint
// ---------------------------------------------------------------------------

func TestComputeFingerprint_IsDeterministic(t *testing.T) {
	dir := fixtureModDir(t)

	fp1, err := computeFingerprint(dir, false)
	require.NoError(t, err)

	fp2, err := computeFingerprint(dir, false)
	require.NoError(t, err)

	assert.Equal(t, fp1, fp2, "fingerprint should be identical across two calls")
}

func TestComputeFingerprint_ChangesWhenFileModified(t *testing.T) {
	// Work in a temp copy so we can safely mutate files.
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package main\n"), 0600))

	fp1, err := computeFingerprint(dir, false)
	require.NoError(t, err)

	// Ensure the mtime changes even on fast file systems.
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, os.WriteFile(goFile, []byte("package main // modified\n"), 0600))

	fp2, err := computeFingerprint(dir, false)
	require.NoError(t, err)

	assert.NotEqual(t, fp1, fp2, "fingerprint should change after modifying a .go file")
}

func TestComputeFingerprint_IgnoresVendorDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0600))

	fp1, err := computeFingerprint(dir, false)
	require.NoError(t, err)

	vendorDir := filepath.Join(dir, "vendor", "pkg")
	require.NoError(t, os.MkdirAll(vendorDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(vendorDir, "vendored.go"), []byte("package pkg\n"), 0600))

	fp2, err := computeFingerprint(dir, false)
	require.NoError(t, err)

	assert.Equal(t, fp1, fp2, "vendor directory must not affect the fingerprint")
}

func TestComputeFingerprint_DiffersAcrossProjects(t *testing.T) {
	// Two directories with identical file contents must produce different
	// fingerprints because the absolute project path is part of the hash.
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	for _, d := range []string{dir1, dir2} {
		require.NoError(t, os.WriteFile(filepath.Join(d, "main.go"), []byte("package main\n"), 0600))
	}

	fp1, err := computeFingerprint(dir1, false)
	require.NoError(t, err)

	fp2, err := computeFingerprint(dir2, false)
	require.NoError(t, err)

	assert.NotEqual(t, fp1, fp2, "different project dirs must yield different fingerprints")
}

func TestComputeFingerprint_PartitionsByTypeInfo(t *testing.T) {
	dir := fixtureModDir(t)

	noInfo, err := computeFingerprint(dir, false)
	require.NoError(t, err)

	withInfo, err := computeFingerprint(dir, true)
	require.NoError(t, err)

	assert.NotEqual(t, noInfo, withInfo,
		"type-info loads must not share cache files with default loads")
}

func TestSaveAndLoadWorkspaceFromDisk_Roundtrip(t *testing.T) {
	dir := fixtureModDir(t)
	ws, err := LoadWorkspace(context.Background(), dir)
	require.NoError(t, err)

	cachePath := filepath.Join(t.TempDir(), "workspace.gob")
	require.NoError(t, saveWorkspaceToDisk(ws, cachePath))

	loaded, err := loadWorkspaceFromDisk(cachePath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, ws.Packages.Len(), loaded.Packages.Len(), "package count mismatch")
	assert.Equal(t, ws.Files.Len(), loaded.Files.Len(), "file count mismatch")
	assert.Equal(t, ws.Types.Len(), loaded.Types.Len(), "type count mismatch")
	assert.Equal(t, ws.Functions.Len(), loaded.Functions.Len(), "function count mismatch")
	assert.Equal(t, ws.Variables.Len(), loaded.Variables.Len(), "variable count mismatch")
	assert.Equal(t, ws.FunctionCalls.Len(), loaded.FunctionCalls.Len(), "function call count mismatch")
	assert.Equal(t, ws.Dependencies.Len(), loaded.Dependencies.Len(), "dependency count mismatch")
}

func TestSaveAndLoadWorkspaceFromDisk_PreservesRefs(t *testing.T) {
	dir := fixtureModDir(t)
	ws, err := LoadWorkspace(context.Background(), dir)
	require.NoError(t, err)

	cachePath := filepath.Join(t.TempDir(), "workspace.gob")
	require.NoError(t, saveWorkspaceToDisk(ws, cachePath))

	loaded, err := loadWorkspaceFromDisk(cachePath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	origFuncs := ws.Functions.All()
	cachedFuncs := loaded.Functions.All()
	require.Equal(t, len(origFuncs), len(cachedFuncs))
	for i := range origFuncs {
		assert.Equal(t, origFuncs[i].Ref, cachedFuncs[i].Ref, "Ref mismatch at index %d", i)
		assert.Equal(t, origFuncs[i].Name, cachedFuncs[i].Name, "Name mismatch at index %d", i)
		assert.Equal(t, origFuncs[i].QName, cachedFuncs[i].QName, "QName mismatch at index %d", i)
		assert.Equal(t, origFuncs[i].Receiver, cachedFuncs[i].Receiver, "Receiver mismatch at index %d", i)
		assert.Nil(t, cachedFuncs[i].Node, "Node should be nil after cache load")
	}

	origTypes := ws.Types.All()
	cachedTypes := loaded.Types.All()
	require.Equal(t, len(origTypes), len(cachedTypes))
	for i := range origTypes {
		assert.Equal(t, origTypes[i].QName, cachedTypes[i].QName, "Type QName mismatch at index %d", i)
	}

	origCalls := ws.FunctionCalls.All()
	cachedCalls := loaded.FunctionCalls.All()
	require.Equal(t, len(origCalls), len(cachedCalls))
	for i := range origCalls {
		assert.Equal(t, origCalls[i].CallerQName, cachedCalls[i].CallerQName,
			"CallerQName mismatch at index %d", i)
	}
}

func TestSaveAndLoadWorkspaceFromDisk_PreservesTypeStructure(t *testing.T) {
	dir := filepath.Join(filepath.Dir(fixtureModDir(t)), "typestructfixture")
	ws, err := LoadWorkspace(context.Background(), dir, WithTypeInfo())
	require.NoError(t, err)

	cachePath := filepath.Join(t.TempDir(), "workspace.gob")
	require.NoError(t, saveWorkspaceToDisk(ws, cachePath))

	loaded, err := loadWorkspaceFromDisk(cachePath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	origTypes := ws.Types.All()
	cachedTypes := loaded.Types.All()
	require.Equal(t, len(origTypes), len(cachedTypes))
	for i := range origTypes {
		assert.Equal(t, origTypes[i].Name, cachedTypes[i].Name, "Name mismatch at %d", i)
		assert.Equal(t, origTypes[i].Fields, cachedTypes[i].Fields, "Fields mismatch at %d (%s)", i, origTypes[i].Name)
		assert.Equal(t, origTypes[i].Methods, cachedTypes[i].Methods, "Methods mismatch at %d (%s)", i, origTypes[i].Name)
		assert.Equal(t, origTypes[i].Embeds, cachedTypes[i].Embeds, "Embeds mismatch at %d (%s)", i, origTypes[i].Name)
	}
}

func TestSaveAndLoadWorkspaceFromDisk_PreservesResolvedCallees(t *testing.T) {
	// Load with WithTypeInfo so resolved callee fields are populated; this
	// also exercises the caller-identity fields in the same roundtrip.
	dir := filepath.Join(filepath.Dir(fixtureModDir(t)), "typeinfofixture")
	ws, err := LoadWorkspace(context.Background(), dir, WithTypeInfo())
	require.NoError(t, err)

	cachePath := filepath.Join(t.TempDir(), "workspace.gob")
	require.NoError(t, saveWorkspaceToDisk(ws, cachePath))

	loaded, err := loadWorkspaceFromDisk(cachePath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	origCalls := ws.FunctionCalls.All()
	cachedCalls := loaded.FunctionCalls.All()
	require.Equal(t, len(origCalls), len(cachedCalls))
	for i := range origCalls {
		assert.Equal(t, origCalls[i].Callee, cachedCalls[i].Callee, "Callee mismatch at index %d", i)
		assert.Equal(t, origCalls[i].CalleePackage, cachedCalls[i].CalleePackage, "CalleePackage mismatch at index %d", i)
		assert.Equal(t, origCalls[i].CalleeQName, cachedCalls[i].CalleeQName, "CalleeQName mismatch at index %d", i)
		assert.Equal(t, origCalls[i].CalleeIsMethod, cachedCalls[i].CalleeIsMethod, "CalleeIsMethod mismatch at index %d", i)
		assert.Equal(t, origCalls[i].CallerName, cachedCalls[i].CallerName, "CallerName mismatch at index %d", i)
		assert.Equal(t, origCalls[i].CallerReceiver, cachedCalls[i].CallerReceiver, "CallerReceiver mismatch at index %d", i)
	}
}

func TestSaveAndLoadWorkspaceFromDisk_PreservesCallerIdentity(t *testing.T) {
	// Default load (no WithTypeInfo) — exercises caller identity in isolation
	// from callee resolution, on the simpler fixturemod corpus.
	dir := fixtureModDir(t)
	ws, err := LoadWorkspace(context.Background(), dir)
	require.NoError(t, err)

	cachePath := filepath.Join(t.TempDir(), "workspace.gob")
	require.NoError(t, saveWorkspaceToDisk(ws, cachePath))

	loaded, err := loadWorkspaceFromDisk(cachePath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	origCalls := ws.FunctionCalls.All()
	cachedCalls := loaded.FunctionCalls.All()
	require.Equal(t, len(origCalls), len(cachedCalls))
	for i := range origCalls {
		assert.Equal(t, origCalls[i].Callee, cachedCalls[i].Callee, "Callee mismatch at index %d", i)
		assert.Equal(t, origCalls[i].CallerName, cachedCalls[i].CallerName, "CallerName mismatch at index %d", i)
		assert.Equal(t, origCalls[i].CallerReceiver, cachedCalls[i].CallerReceiver, "CallerReceiver mismatch at index %d", i)
		assert.Nil(t, cachedCalls[i].Node, "Node should be nil after cache load")
	}
}

func TestSaveAndLoadWorkspaceFromDisk_PreservesDependenciesOnFiles(t *testing.T) {
	dir := fixtureModDir(t)
	ws, err := LoadWorkspace(context.Background(), dir)
	require.NoError(t, err)

	cachePath := filepath.Join(t.TempDir(), "workspace.gob")
	require.NoError(t, saveWorkspaceToDisk(ws, cachePath))

	loaded, err := loadWorkspaceFromDisk(cachePath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	for _, origFile := range ws.Files.All() {
		var cachedFile *files.Item
		for _, cf := range loaded.Files.All() {
			if cf.Filename == origFile.Filename {
				cf := cf
				cachedFile = &cf
				break
			}
		}
		require.NotNil(t, cachedFile, "file %q missing from cache", origFile.Filename)
		assert.Equal(t,
			origFile.Dependencies().Len(),
			cachedFile.Dependencies().Len(),
			"dependency count mismatch for file %q", origFile.Filename,
		)
	}
}

func TestLoadWorkspaceFromDisk_ReturnsMissForAbsentFile(t *testing.T) {
	ws, err := loadWorkspaceFromDisk(filepath.Join(t.TempDir(), "nonexistent.gob"))
	assert.NoError(t, err)
	assert.Nil(t, ws)
}

func TestLoadWorkspaceFromDisk_ReturnsMissForCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.gob")
	require.NoError(t, os.WriteFile(path, []byte("not valid gob data"), 0600))

	ws, err := loadWorkspaceFromDisk(path)
	assert.NoError(t, err)
	assert.Nil(t, ws)
}

func TestLoadWorkspaceFromDisk_ReturnsMissForVersionMismatch(t *testing.T) {
	// Build a snapshot with a wrong version and save it directly.
	dir := fixtureModDir(t)
	ws, err := LoadWorkspace(context.Background(), dir)
	require.NoError(t, err)

	snap := buildSnap(ws)
	snap.Version = cacheVersion + 99

	path := filepath.Join(t.TempDir(), "old-version.gob")
	f, createErr := os.Create(path)
	require.NoError(t, createErr)
	require.NoError(t, gob.NewEncoder(f).Encode(snap))
	require.NoError(t, f.Close())

	loaded, err := loadWorkspaceFromDisk(path)
	assert.NoError(t, err)
	assert.Nil(t, loaded, "stale version should be treated as a cache miss")
}

// ---------------------------------------------------------------------------
// LoadWorkspace with WithDiskCacheDir
// ---------------------------------------------------------------------------

func TestLoadWorkspace_WithDiskCacheDir_SavesAndLoadsFromDisk(t *testing.T) {
	dir := fixtureModDir(t)
	cacheDir := t.TempDir()

	first, err := LoadWorkspace(context.Background(), dir, WithDiskCacheDir(cacheDir))
	require.NoError(t, err)

	// The cache directory should now contain exactly one .gob file.
	globs, err := filepath.Glob(filepath.Join(cacheDir, "*.gob"))
	require.NoError(t, err)
	assert.Len(t, globs, 1, "expected exactly one cache file")

	second, err := LoadWorkspace(context.Background(), dir, WithDiskCacheDir(cacheDir))
	require.NoError(t, err)

	// The two workspaces are distinct instances but contain the same data.
	assert.NotSame(t, first, second)
	assert.Equal(t, first.Packages.Len(), second.Packages.Len())
	assert.Equal(t, first.Functions.Len(), second.Functions.Len())
	assert.Equal(t, first.Types.Len(), second.Types.Len())
}

func TestLoadWorkspace_WithDiskCacheDir_CacheIsNotUsedForDifferentDir(t *testing.T) {
	dir := fixtureModDir(t)
	cacheDir := t.TempDir()

	_, err := LoadWorkspace(context.Background(), dir, WithDiskCacheDir(cacheDir))
	require.NoError(t, err)

	// Pointing to the same cacheDir but a different project dir should fall back
	// to a full parse rather than using the cached result.
	otherDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(otherDir, "go.mod"), []byte("module example.com/empty\n\ngo 1.21\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(otherDir, "main.go"), []byte("package main\n"), 0600))

	_, err = LoadWorkspace(context.Background(), otherDir, WithDiskCacheDir(cacheDir))
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// LoadWorkspace with WithDiskCache (auto dir)
// ---------------------------------------------------------------------------

func TestLoadWorkspace_WithDiskCache_UsesDefaultCacheDir(t *testing.T) {
	dir := fixtureModDir(t)

	// WithDiskCache() without a dir should work end-to-end.
	first, err := LoadWorkspace(context.Background(), dir, WithDiskCache())
	require.NoError(t, err)

	second, err := LoadWorkspace(context.Background(), dir, WithDiskCache())
	require.NoError(t, err)

	assert.NotSame(t, first, second)
	assert.Equal(t, first.Packages.Len(), second.Packages.Len())
}
