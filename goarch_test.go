package goarch_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/saintedlama/goarch"
	"github.com/saintedlama/goarch/internaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadWorkspace_LoadsPackagesAndFiles(t *testing.T) {
	program := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	require.GreaterOrEqual(t, program.Packages.Len(), 2, "expected at least 2 packages")

	for _, pkg := range program.Packages.All() {
		assert.NotEmpty(t, pkg.ID, "package ID should not be empty")
		assert.NotNil(t, pkg.FileSet, "package %q has nil file set", pkg.ID)
		assert.NotEmpty(t, pkg.Files, "package %q should contain at least one file", pkg.ID)
		for _, file := range pkg.Files {
			assert.NotNil(t, file.Node, "package %q has file with nil AST node", pkg.ID)
		}
	}
}

func TestLoadWorkspace_BuildsTopLevelCollections(t *testing.T) {
	program := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	assert.Greater(t, program.Files.Len(), 0, "expected at least one file entry")
	assert.Greater(t, program.Types.Len(), 0, "expected at least one type entry")
	assert.Greater(t, program.Functions.Len(), 0, "expected at least one function entry")
	assert.Greater(t, program.Variables.Len(), 0, "expected at least one variable entry")
	assert.Greater(t, program.FunctionCalls.Len(), 0, "expected at least one function call entry")
}

func TestLoadWorkspace_WithInMemoryCache_ReusesWorkspaceForSamePath(t *testing.T) {
	fixtureDir := fixtureDir(t, "fixturemod")

	first, err := goarch.LoadWorkspace(context.Background(), fixtureDir, goarch.WithInMemoryCache())
	require.NoError(t, err)

	second, err := goarch.LoadWorkspace(context.Background(), fixtureDir, goarch.WithInMemoryCache())
	require.NoError(t, err)

	assert.Same(t, first, second, "expected cached workspace instance to be reused")
}

func TestLoadWorkspace_WithoutInMemoryCache_DoesNotReuseWorkspace(t *testing.T) {
	fixtureDir := fixtureDir(t, "fixturemod")

	first, err := goarch.LoadWorkspace(context.Background(), fixtureDir)
	require.NoError(t, err)

	second, err := goarch.LoadWorkspace(context.Background(), fixtureDir)
	require.NoError(t, err)

	assert.NotSame(t, first, second, "expected distinct workspace instances when cache option is disabled")
}

func fixtureDir(t *testing.T, fixtureName string) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")

	return filepath.Join(filepath.Dir(filename), "testdata", fixtureName)
}
