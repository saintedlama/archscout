// cachebench times two consecutive LoadWorkspace calls with WithDiskCache
// enabled. Run with:
//
//	go run ./cmd/cachebench <project-dir> [cache-dir]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/saintedlama/archscout"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: cachebench <project-dir> [cache-dir]")
		os.Exit(1)
	}

	projectDir := os.Args[1]
	cacheDir := filepath.Join(os.TempDir(), "archscout-bench-cache")
	if len(os.Args) >= 3 {
		cacheDir = os.Args[2]
	}

	fmt.Printf("Project : %s\n", projectDir)
	fmt.Printf("Cache   : %s\n\n", cacheDir)

	run := func(label string) *archscout.Workspace {
		start := time.Now()
		ws, err := archscout.LoadWorkspace(
			context.Background(),
			projectDir,
			archscout.WithDiskCacheDir(cacheDir),
			archscout.WithReporter(func(msg string) {
				fmt.Printf("  [%s] %s\n", label, msg)
			}),
		)
		elapsed := time.Since(start)
		if err != nil {
			fmt.Fprintf(os.Stderr, "LoadWorkspace failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  => %s done in %v\n", label, elapsed)
		fmt.Printf("     packages=%d  files=%d  types=%d  functions=%d  calls=%d  deps=%d\n\n",
			ws.Packages.Len(),
			ws.Files.Len(),
			ws.Types.Len(),
			ws.Functions.Len(),
			ws.FunctionCalls.Len(),
			ws.Dependencies.Len(),
		)
		return ws
	}

	fmt.Println("=== run 1 (cold — should parse everything) ===")
	ws1 := run("cold")

	fmt.Println("=== run 2 (warm — should load from disk cache) ===")
	ws2 := run("warm")

	// Basic sanity: both runs must agree on all counts.
	ok := true
	check := func(name string, a, b int) {
		if a != b {
			fmt.Printf("MISMATCH %s: cold=%d warm=%d\n", name, a, b)
			ok = false
		}
	}
	check("packages", ws1.Packages.Len(), ws2.Packages.Len())
	check("files", ws1.Files.Len(), ws2.Files.Len())
	check("types", ws1.Types.Len(), ws2.Types.Len())
	check("functions", ws1.Functions.Len(), ws2.Functions.Len())
	check("calls", ws1.FunctionCalls.Len(), ws2.FunctionCalls.Len())
	check("deps", ws1.Dependencies.Len(), ws2.Dependencies.Len())

	if ok {
		fmt.Println("✓ cold and warm workspaces agree on all counts")
	} else {
		os.Exit(1)
	}
}
