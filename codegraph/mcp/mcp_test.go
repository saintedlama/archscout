package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/codegraph/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	fixtureDir := filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "fixturemod")

	ws, err := archscout.LoadWorkspace(t.Context(), fixtureDir, archscout.WithTypeInfo())
	require.NoError(t, err)

	dbPath := filepath.Join(t.TempDir(), "mcp_test.db")
	err = ws.ExportSQLite(t.Context(), dbPath)
	require.NoError(t, err)

	store, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestMCP_ServerWorkflow(t *testing.T) {
	store := prepareTestStore(t)
	server := NewServer(store)

	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search_symbols","arguments":{"query":"domain"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_symbol_info","arguments":{"symbol":"example.com/fixturemod/domain"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"trace_callers","arguments":{"symbol":"fmt.Println","depth":2}}}`,
	}

	var inBuf bytes.Buffer
	for _, req := range requests {
		inBuf.WriteString(req + "\n")
	}

	var outBuf bytes.Buffer
	err := server.Serve(context.Background(), &inBuf, &outBuf)
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(outBuf.Bytes()), []byte("\n"))
	require.Equal(t, len(requests), len(lines))

	// Validate initialize response
	var initResp response
	err = json.Unmarshal(lines[0], &initResp)
	require.NoError(t, err)
	assert.Equal(t, float64(1), initResp.ID)
	assert.Nil(t, initResp.Error)

	// Validate tools/list response
	var listResp response
	err = json.Unmarshal(lines[2], &listResp)
	require.NoError(t, err)
	resMap := listResp.Result.(map[string]any)
	tools := resMap["tools"].([]any)
	assert.GreaterOrEqual(t, len(tools), 4)

	// Validate search_symbols tool call response
	var searchResp response
	err = json.Unmarshal(lines[3], &searchResp)
	require.NoError(t, err)
	toolRes := searchResp.Result.(map[string]any)
	assert.False(t, toolRes["isError"].(bool))
	content := toolRes["content"].([]any)
	first := content[0].(map[string]any)
	assert.Contains(t, first["text"].(string), "domain")
}
