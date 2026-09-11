package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixtureDir(t *testing.T, fixtureName string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// cmd/archscout/ -> ../../testdata/<fixtureName>
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", fixtureName)
}

func TestRun_Usage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "archscout <command>")
	assert.Contains(t, stdout.String(), "graph")
	assert.Contains(t, stdout.String(), "query")
	assert.Contains(t, stdout.String(), "mcp")
}

func TestRun_GraphMermaid(t *testing.T) {
	dir := fixtureDir(t, "fixturemod")
	var stdout, stderr bytes.Buffer

	err := run([]string{"graph", "--format", "mermaid", dir}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "graph LR")
}

func TestRun_GraphDOT(t *testing.T) {
	dir := fixtureDir(t, "fixturemod")
	var stdout, stderr bytes.Buffer

	err := run([]string{"graph", "--format", "dot", "--title", "TestGraph", dir}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "digraph")
}

func TestRun_GraphJSON(t *testing.T) {
	dir := fixtureDir(t, "fixturemod")
	var stdout, stderr bytes.Buffer

	err := run([]string{"graph", "--format", "json", dir}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"nodes"`)
	assert.Contains(t, stdout.String(), `"edges"`)
}

func TestRun_GraphSQLite(t *testing.T) {
	dir := fixtureDir(t, "fixturemod")
	dbPath := filepath.Join(t.TempDir(), "sub", "export.db")
	var stdout, stderr bytes.Buffer

	err := run([]string{"graph", "--sqlite", dbPath, dir}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "Exported SQLite database")

	// Verify database can be opened and queried with ArchScout SQLiteStore
	store, err := archscout.OpenSQLite(dbPath)
	require.NoError(t, err)
	defer store.Close()

	nodes, err := store.SearchFTS(t.Context(), "domain", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, nodes)
}

func TestRun_GraphOutFile(t *testing.T) {
	dir := fixtureDir(t, "fixturemod")
	outFile := filepath.Join(t.TempDir(), "diagram.mmd")
	var stdout, stderr bytes.Buffer

	err := run([]string{"graph", "--format", "mermaid", "--out", outFile, dir}, &stdout, &stderr)
	require.NoError(t, err)
	assert.FileExists(t, outFile)
}

func TestRun_ExportAlias(t *testing.T) {
	dir := fixtureDir(t, "fixturemod")
	var stdout, stderr bytes.Buffer

	err := run([]string{"export", "--format", "mermaid", dir}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "graph LR")
}

func TestRun_QueryCommands(t *testing.T) {
	dir := fixtureDir(t, "fixturemod")
	dbPath := filepath.Join(t.TempDir(), "query_test.db")

	// Pre-export database
	var stdout, stderr bytes.Buffer
	err := run([]string{"graph", "--sqlite", dbPath, dir}, &stdout, &stderr)
	require.NoError(t, err)

	// 1. FTS query
	stdout.Reset()
	stderr.Reset()
	err = run([]string{"query", "fts", "domain", "--db", dbPath}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Found")

	// 2. FTS JSON query
	stdout.Reset()
	stderr.Reset()
	err = run([]string{"query", "fts", "domain", "--db", dbPath, "--json"}, &stdout, &stderr)
	require.NoError(t, err)
	var nodes []archscout.CodeNode
	err = json.Unmarshal(stdout.Bytes(), &nodes)
	require.NoError(t, err)
	assert.NotEmpty(t, nodes)

	// 3. Info query
	stdout.Reset()
	stderr.Reset()
	err = run([]string{"query", "info", "example.com/fixturemod/domain", "--db", dbPath}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Symbol:")
	assert.Contains(t, stdout.String(), "domain")

	// 4. Callers query
	stdout.Reset()
	stderr.Reset()
	err = run([]string{"query", "callers", "fmt.Println", "--db", dbPath, "--depth", "2"}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Callers of fmt.Println")

	// 5. Deps query
	stdout.Reset()
	stderr.Reset()
	err = run([]string{"query", "deps", "example.com/fixturemod/application", "--db", dbPath}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Dependencies of")

	// 6. Skill query text
	stdout.Reset()
	stderr.Reset()
	err = run([]string{"query", "skill"}, &stdout, &stderr)
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "ArchScout Agent Skill Installation")
	assert.Contains(t, stdout.String(), "name: archscout")

	// 7. Skill query JSON
	stdout.Reset()
	stderr.Reset()
	err = run([]string{"query", "skill", "--json"}, &stdout, &stderr)
	require.NoError(t, err)
	var skillMap map[string]any
	err = json.Unmarshal(stdout.Bytes(), &skillMap)
	require.NoError(t, err)
	assert.Equal(t, "archscout", skillMap["name"])
	assert.NotEmpty(t, skillMap["content"])

	// 8. Skill install to file
	skillFile := filepath.Join(t.TempDir(), "SKILL.md")
	stdout.Reset()
	stderr.Reset()
	err = run([]string{"query", "skill", "--install", "--out", skillFile}, &stdout, &stderr)
	require.NoError(t, err)
	assert.FileExists(t, skillFile)
}

func TestRun_SkillTopLevel(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"skill", "--json"}, &stdout, &stderr)
	require.NoError(t, err)
	var skillMap map[string]any
	err = json.Unmarshal(stdout.Bytes(), &skillMap)
	require.NoError(t, err)
	assert.Equal(t, "archscout", skillMap["name"])
}
