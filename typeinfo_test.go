package archscout_test

import (
	"testing"

	"github.com/saintedlama/archscout"
	"github.com/saintedlama/archscout/functioncalls"
	"github.com/saintedlama/archscout/internaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findCallByCallee returns the first FunctionCall whose syntactic Callee
// matches the given text, or fails the test if none is found.
func findCallByCallee(t *testing.T, ws *archscout.Workspace, syntactic string) functioncalls.Item {
	t.Helper()
	for _, item := range ws.FunctionCalls.All() {
		if item.Callee == syntactic {
			return item
		}
	}
	t.Fatalf("no call with syntactic Callee %q found", syntactic)
	return functioncalls.Item{}
}

func TestWithTypeInfo_ResolvesCrossPackageFunction(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())

	call := findCallByCallee(t, ws, "fmt.Println")
	assert.Equal(t, "fmt", call.CalleePackage)
	assert.Equal(t, "fmt.Println", call.CalleeQName)
	assert.False(t, call.CalleeIsMethod, "fmt.Println is a function, not a method")

	upper := findCallByCallee(t, ws, "strings.ToUpper")
	assert.Equal(t, "strings", upper.CalleePackage)
	assert.Equal(t, "strings.ToUpper", upper.CalleeQName)
}

func TestWithTypeInfo_ResolvesPointerReceiverMethod(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())

	call := findCallByCallee(t, ws, "svc.Run")
	assert.True(t, call.CalleeIsMethod)
	assert.Equal(t, "example.com/typeinfofixture/api", call.CalleePackage)
	assert.Equal(t, "example.com/typeinfofixture/api.Service.Run", call.CalleeQName,
		"pointer indirection should be stripped from method qname")
}

func TestWithTypeInfo_ResolvesValueReceiverMethod(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())

	call := findCallByCallee(t, ws, "val.Describe")
	assert.True(t, call.CalleeIsMethod)
	assert.Equal(t, "example.com/typeinfofixture/api", call.CalleePackage)
	assert.Equal(t, "example.com/typeinfofixture/api.Service.Describe", call.CalleeQName)
}

func TestWithTypeInfo_ResolvesInterfaceMethodDispatch(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())

	call := findCallByCallee(t, ws, "g.Greet")
	assert.True(t, call.CalleeIsMethod)
	assert.Equal(t, "example.com/typeinfofixture/api", call.CalleePackage)
	assert.Equal(t, "example.com/typeinfofixture/api.Greeter.Greet", call.CalleeQName)
}

func TestWithTypeInfo_LeavesBuiltinsUnresolved(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture", archscout.WithTypeInfo())

	call := findCallByCallee(t, ws, "len")
	assert.Empty(t, call.CalleeQName, "builtins should not resolve")
	assert.Empty(t, call.CalleePackage)
	assert.False(t, call.CalleeIsMethod)
}

func TestWithoutTypeInfo_LeavesResolvedFieldsEmpty(t *testing.T) {
	ws := internaltest.LoadFixtureWorkspace(t, "typeinfofixture")

	require.Greater(t, ws.FunctionCalls.Len(), 0)
	for _, call := range ws.FunctionCalls.All() {
		assert.Empty(t, call.CalleePackage, "CalleePackage should be empty without WithTypeInfo: %s", call.Callee)
		assert.Empty(t, call.CalleeQName, "CalleeQName should be empty without WithTypeInfo: %s", call.Callee)
		assert.False(t, call.CalleeIsMethod, "CalleeIsMethod should be false without WithTypeInfo: %s", call.Callee)
	}
}
