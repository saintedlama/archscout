package functioncalls_test

import (
	"strings"
	"testing"

	"github.com/saintedlama/archscout/common"
	"github.com/saintedlama/archscout/functioncalls"
	"github.com/saintedlama/archscout/internaltest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionCalls_FindsExpectedFmtErrorfCalls(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	refs := workspace.FunctionCalls.Match(func(call functioncalls.Item) bool {
		return call.Callee == "fmt.Errorf"
	})
	require.Len(t, refs, 3, "expected 3 fmt.Errorf refs")

	var sawApp, sawInfra, sawSub bool
	for _, f := range refs {
		normalized := strings.ReplaceAll(f.Filename, "\\", "/")
		if strings.HasSuffix(normalized, "/application/service.go") {
			sawApp = true
		}
		if strings.HasSuffix(normalized, "/infrastructure/repo.go") {
			sawInfra = true
		}
		if strings.HasSuffix(normalized, "/subpkg/sub.go") {
			sawSub = true
		}
	}

	assert.True(t, sawApp, "did not find fmt.Errorf in application/service.go")
	assert.True(t, sawInfra, "did not find fmt.Errorf in infrastructure/repo.go")
	assert.True(t, sawSub, "did not find fmt.Errorf in subpkg/sub.go")
}

func TestFunctionCalls_InPackageAndNotInPackage_CanBeChained(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	// Only application-layer fmt.Errorf calls (excluding infrastructure and subpkg).
	refs := workspace.FunctionCalls.
		InPackage("example.com/fixturemod/application").
		Match(func(call functioncalls.Item) bool {
			return call.Callee == "fmt.Errorf"
		})

	require.Len(t, refs, 1, "expected 1 fmt.Errorf call in application layer")

	normalized := strings.ReplaceAll(refs[0].Filename, "\\", "/")
	assert.True(t, strings.HasSuffix(normalized, "/application/service.go"), "expected ref in application/service.go")
}

func TestFunctionCalls_InTestAndNotInTest_FilterByTestFilenames(t *testing.T) {
	calls := functioncalls.NewCollection([]functioncalls.Item{
		{Ref: common.Ref{Filename: "pkg/file.go"}, Callee: "fmt.Println"},
		{Ref: common.Ref{Filename: "pkg/file_test.go"}, Callee: "fmt.Println"},
	})

	assert.Len(t, calls.IsTest().All(), 1, "expected one test call entry")
	assert.Len(t, calls.IsNotTest().All(), 1, "expected one non-test call entry")
}

func TestFunctionCalls_CallerIdentity_ResolvedForFunctionsAndMethods(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	type sighting struct {
		caller   string
		receiver string
	}

	got := map[string]sighting{}
	for _, item := range workspace.FunctionCalls.All() {
		got[item.Callee] = sighting{
			caller:   item.CallerName,
			receiver: item.CallerReceiver,
		}
	}

	// fmt.Errorf in subpkg/sub.go is called from func SubErr (no receiver).
	// fmt.Errorf in application/service.go is called from method (s *OrderService) PlaceOrder.
	// fmt.Errorf in infrastructure/repo.go is called from method (r *OrderRepository) Find.
	// We don't know which Callee the map kept (last wins), assert the caller information is present for any of them.
	for callee, s := range got {
		if callee != "fmt.Errorf" {
			continue
		}
		assert.NotEmpty(t, s.caller, "fmt.Errorf call should have a non-empty CallerName")
	}

	var sawPlaceOrderCaller bool
	for _, item := range workspace.FunctionCalls.All() {
		if item.Callee != "domain.NewOrder" {
			continue
		}
		sawPlaceOrderCaller = true
		assert.Equal(t, "PlaceOrder", item.CallerName, "expected enclosing function to be PlaceOrder")
		assert.Equal(t, "*OrderService", item.CallerReceiver, "expected pointer receiver on caller")
	}
	assert.True(t, sawPlaceOrderCaller, "expected to see at least one domain.NewOrder call")

	var sawMainCall bool
	for _, item := range workspace.FunctionCalls.All() {
		if item.Callee != "repo.Save" {
			continue
		}
		sawMainCall = true
		assert.Equal(t, "main", item.CallerName)
		assert.Empty(t, item.CallerReceiver, "main has no receiver")
	}
	assert.True(t, sawMainCall, "expected to see repo.Save call from main")
}

func TestFunctionCalls_CallerIdentity_EmptyForPackageLevelInitializers(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "callerfixture")

	// errors.New in `var ErrSentinel = errors.New(...)` is at package level
	var sawSentinel bool
	for _, item := range workspace.FunctionCalls.All() {
		if item.Callee != "errors.New" {
			continue
		}
		if item.CallerName == "" {
			sawSentinel = true
			assert.Empty(t, item.CallerReceiver)
		}
	}
	assert.True(t, sawSentinel, "expected to see errors.New at package level with empty caller")
}

func TestFunctionCalls_CallerIdentity_PreservedAcrossClosures(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "callerfixture")

	var sawClosureCall bool
	for _, item := range workspace.FunctionCalls.All() {
		if item.Callee != "strings.ToUpper" {
			continue
		}
		sawClosureCall = true
		assert.Equal(t, "Validate", item.CallerName, "closure should not shadow the enclosing FuncDecl")
		assert.Equal(t, "*Service", item.CallerReceiver)
	}
	assert.True(t, sawClosureCall, "expected to see strings.ToUpper call inside closure")
}

func TestFunctionCalls_CallerQName_ComposedFromEnclosingFuncDecl(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "fixturemod")

	// domain.NewOrder is called from (s *OrderService).PlaceOrder. CallerQName
	// should be the canonical qname of that method, with pointer indirection
	// stripped from the receiver.
	var sawMethod, sawTopLevel bool
	for _, item := range workspace.FunctionCalls.All() {
		if item.Callee == "domain.NewOrder" {
			sawMethod = true
			assert.Equal(t,
				"example.com/fixturemod/application.OrderService.PlaceOrder",
				item.CallerQName,
				"CallerQName must compose package + receiver type + method name")
		}
		// repo.Save is called from main (a top-level FuncDecl with no receiver).
		if item.Callee == "repo.Save" {
			sawTopLevel = true
			assert.Equal(t, "example.com/fixturemod.main", item.CallerQName)
		}
	}
	assert.True(t, sawMethod, "expected to see domain.NewOrder call")
	assert.True(t, sawTopLevel, "expected to see repo.Save call")
}

func TestFunctionCalls_CallerQName_EmptyForPackageLevelInitializers(t *testing.T) {
	workspace := internaltest.LoadFixtureWorkspace(t, "callerfixture")

	// errors.New in `var ErrSentinel = errors.New(...)` has no enclosing
	// FuncDecl, so CallerQName must be empty.
	var sawSentinel bool
	for _, item := range workspace.FunctionCalls.All() {
		if item.Callee == "errors.New" && item.CallerName == "" {
			sawSentinel = true
			assert.Empty(t, item.CallerQName)
		}
	}
	assert.True(t, sawSentinel, "expected to see errors.New at package level")
}
