// Discover default roots from a chosen cwd (terraform skill POC).
// Usage: go run ./scripts/discover_default_roots.go [cwd]
//
//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Cold104/smartcontext-core/core"
)

func main() {
	_ = os.Unsetenv("SMARTCONTEXT_SKILL_ROOTS")
	_ = os.Unsetenv("SMARTCONTEXT_WORKSPACE_ROOTS")
	if len(os.Args) > 1 {
		if err := os.Chdir(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "chdir: %v\n", err)
			os.Exit(1)
		}
	}
	cwd, _ := os.Getwd()
	cat := core.BuildCatalog(nil)
	fmt.Printf("cwd=%s\nroots=%d catalog=%d\n", cwd, len(cat.Roots), len(cat.Entries))
	for _, e := range cat.Entries {
		if strings.HasPrefix(e.Name, "terraform-") {
			fmt.Printf("  HIT %s -> %s\n", e.Name, e.Path)
		}
	}
}
