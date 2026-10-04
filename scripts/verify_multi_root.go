// Verify multi-app skill-root discovery (terraform skills from non-project cwd).
// Usage (from go/): go run ./scripts/verify_multi_root.go
//
//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/version"
)

func countTerraform(cat core.Catalog) int {
	n := 0
	for _, e := range cat.Entries {
		if strings.HasPrefix(e.Name, "terraform-") {
			n++
			fmt.Printf("  HIT %s\n", e.Name)
		}
	}
	return n
}

func main() {
	origWD, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getcwd: %v\n", err)
		os.Exit(1)
	}
	project, err := filepath.Abs(filepath.Join(origWD, ".."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "abs project: %v\n", err)
		os.Exit(1)
	}
	skills := filepath.Join(project, ".cursor", "skills")
	tmp := filepath.Join(os.TempDir(), "sc-verify-multi-root")
	_ = os.MkdirAll(tmp, 0o755)
	fmt.Printf("project=%s\n", project)

	cases := []struct {
		name string
		cwd  string
		env  map[string]string
	}{
		{
			name: "walk-up from go/ (no env)",
			cwd:  ".",
			env:  map[string]string{"SMARTCONTEXT_SKILL_ROOTS": "", "SMARTCONTEXT_WORKSPACE_ROOTS": ""},
		},
		{
			name: "WORKSPACE_ROOTS from TEMP cwd",
			cwd:  tmp,
			env: map[string]string{
				"SMARTCONTEXT_SKILL_ROOTS":     "",
				"SMARTCONTEXT_WORKSPACE_ROOTS": project,
			},
		},
		{
			name: "SKILL_ROOTS from TEMP cwd",
			cwd:  tmp,
			env: map[string]string{
				"SMARTCONTEXT_SKILL_ROOTS":     skills,
				"SMARTCONTEXT_WORKSPACE_ROOTS": "",
			},
		},
	}

	fail := false
	for _, c := range cases {
		_ = os.Unsetenv("SMARTCONTEXT_SKILL_ROOTS")
		_ = os.Unsetenv("SMARTCONTEXT_WORKSPACE_ROOTS")
		for k, v := range c.env {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
		if err := os.Chdir(c.cwd); err != nil {
			fmt.Fprintf(os.Stderr, "%s: chdir: %v\n", c.name, err)
			fail = true
			continue
		}
		cwd, _ := os.Getwd()
		cat := core.BuildCatalog(nil)
		fmt.Printf("=== %s ===\ncwd=%s roots=%d catalog=%d\n",
			c.name, cwd, len(cat.Roots), len(cat.Entries))
		n := countTerraform(cat)
		fmt.Printf("terraform=%d\n", n)
		if n < 5 {
			fmt.Fprintf(os.Stderr, "FAIL: want >=5 terraform skills, got %d\n", n)
			fail = true
		}
		_ = os.Chdir(origWD)
	}

	// Explicit roots arg (pack path)
	_ = os.Unsetenv("SMARTCONTEXT_SKILL_ROOTS")
	_ = os.Unsetenv("SMARTCONTEXT_WORKSPACE_ROOTS")
	_ = os.Chdir(tmp)
	res := core.PackContext(core.PackRequest{
		Task:  "terraform invalid for_each",
		TopN:  3,
		Roots: []string{project},
	}, version.Version)
	fmt.Printf("=== pack roots arg from TEMP ===\nselected=%v empty=%v\n", res.SelectedNames, res.EmptyCatalog)
	hit := false
	for _, n := range res.SelectedNames {
		if strings.HasPrefix(n, "terraform-") {
			hit = true
		}
	}
	if !hit || res.EmptyCatalog {
		fmt.Fprintf(os.Stderr, "FAIL: pack roots arg missed terraform skills\n")
		fail = true
	}

	_ = os.Chdir(origWD)
	if fail {
		os.Exit(1)
	}
	fmt.Println("VERIFY_OK")
}
