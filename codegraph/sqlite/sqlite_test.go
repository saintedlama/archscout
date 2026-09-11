package sqlite_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/codegraph"
	"github.com/saintedlama/archscout/codegraph/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", name)
}

func TestExport_FTSAndEdges(t *testing.T) {
	ctx := context.Background()
	ws, err := archscout.LoadWorkspace(ctx, fixtureDir(t, "fixturemod"))
	require.NoError(t, err)

	graph := ws.CodeGraph()
	require.NotNil(t, graph)

	dbPath := filepath.Join(t.TempDir(), "test_codegraph.db")
	err = sqlite.Export(ctx, graph, dbPath)
	require.NoError(t, err)

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Full-Text Search
	ftsNodes, err := store.SearchFTS(ctx, "domain", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, ftsNodes)

	foundDomain := false
	for _, n := range ftsNodes {
		if n.Kind == codegraph.NodeKindPackage && n.Name == "domain" {
			foundDomain = true
			break
		}
	}
	assert.True(t, foundDomain, "expected domain package in FTS results")
}

func TestExport_SqliteVecKNN(t *testing.T) {
	ctx := context.Background()
	ws, err := archscout.LoadWorkspace(ctx, fixtureDir(t, "typeinfofixture"), archscout.WithTypeInfo())
	require.NoError(t, err)

	graph := ws.CodeGraph()
	require.NotNil(t, graph)

	dim := 4
	mockEmbedding := func(ctx context.Context, texts []string) ([][]float32, error) {
		results := make([][]float32, len(texts))
		for i, txt := range texts {
			if txt == "[function] example.com/typeinfofixture.main (package: example.com/typeinfofixture)" {
				results[i] = []float32{1.0, 0.0, 0.0, 0.0}
			} else {
				results[i] = []float32{0.0, 0.0, 0.0, 1.0}
			}
		}
		return results, nil
	}

	dbPath := filepath.Join(t.TempDir(), "test_vec.db")
	err = sqlite.Export(ctx, graph, dbPath, sqlite.WithEmbeddingFunc(mockEmbedding, dim))
	require.NoError(t, err)

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Query for [1.0, 0.0, 0.0, 0.0]
	matches, err := store.SearchVector(ctx, []float32{1.0, 0.0, 0.0, 0.0}, 1)
	require.NoError(t, err)
	require.NotEmpty(t, matches)

	assert.Equal(t, codegraph.FunctionNodeID("example.com/typeinfofixture.main"), matches[0].Node.ID)
	assert.InDelta(t, 0.0, matches[0].Distance, 0.0001)
}

func TestStore_TraceCallersCTE(t *testing.T) {
	ctx := context.Background()
	ws, err := archscout.LoadWorkspace(ctx, fixtureDir(t, "typeinfofixture"), archscout.WithTypeInfo())
	require.NoError(t, err)

	graph := ws.CodeGraph()
	require.NotNil(t, graph)

	dbPath := filepath.Join(t.TempDir(), "test_cte.db")
	err = sqlite.Export(ctx, graph, dbPath)
	require.NoError(t, err)

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Trace callers of fmt.Println
	callers, err := store.TraceCallers(ctx, codegraph.FunctionNodeID("fmt.Println"), 3)
	require.NoError(t, err)
	require.NotEmpty(t, callers)
	assert.Contains(t, callers, codegraph.FunctionNodeID("example.com/typeinfofixture.main"))
}

func TestStore_TraceDependenciesCTE(t *testing.T) {
	ctx := context.Background()
	ws, err := archscout.LoadWorkspace(ctx, fixtureDir(t, "typeinfofixture"), archscout.WithTypeInfo())
	require.NoError(t, err)

	graph := ws.CodeGraph()
	require.NotNil(t, graph)

	dbPath := filepath.Join(t.TempDir(), "test_deps.db")
	err = sqlite.Export(ctx, graph, dbPath)
	require.NoError(t, err)

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer store.Close()

	deps, err := store.TraceDependencies(ctx, codegraph.FunctionNodeID("example.com/typeinfofixture.main"), 3)
	require.NoError(t, err)
	require.NotEmpty(t, deps)
	assert.Contains(t, deps, codegraph.FunctionNodeID("fmt.Println"))
}

func TestStore_HybridSearch(t *testing.T) {
	ctx := context.Background()
	ws, err := archscout.LoadWorkspace(ctx, fixtureDir(t, "typeinfofixture"), archscout.WithTypeInfo())
	require.NoError(t, err)

	graph := ws.CodeGraph()
	require.NotNil(t, graph)

	dim := 4
	mockEmbedding := func(ctx context.Context, texts []string) ([][]float32, error) {
		results := make([][]float32, len(texts))
		for i, txt := range texts {
			if txt == "[function] fmt.Println (package: fmt)" {
				results[i] = []float32{1.0, 0.0, 0.0, 0.0}
			} else {
				results[i] = []float32{0.0, 0.0, 0.0, 1.0}
			}
		}
		return results, nil
	}

	dbPath := filepath.Join(t.TempDir(), "test_hybrid.db")
	err = sqlite.Export(ctx, graph, dbPath, sqlite.WithEmbeddingFunc(mockEmbedding, dim))
	require.NoError(t, err)

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Hybrid search: find closest to [1.0, 0.0, 0.0, 0.0] and expand callers up to 2 hops
	expansions, err := store.HybridSearch(ctx, []float32{1.0, 0.0, 0.0, 0.0}, 1, 2)
	require.NoError(t, err)
	require.NotEmpty(t, expansions)

	assert.Equal(t, codegraph.FunctionNodeID("fmt.Println"), expansions[0].MatchedNodeID)
	assert.Contains(t, expansions[0].CallerIDs, codegraph.FunctionNodeID("example.com/typeinfofixture.main"))
}
