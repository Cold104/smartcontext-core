// Pack driver for terraform skill POC (ignore build tag).
// Usage from go/: go run ./scripts/pack_terraform_skills.go
//
//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/version"
)

func main() {
	wd, _ := os.Getwd()
	// Prefer sibling ../.cursor/skills when run from go/
	skillRoot := filepath.Clean(filepath.Join(wd, "..", ".cursor", "skills"))
	if st, err := os.Stat(skillRoot); err != nil || !st.IsDir() {
		skillRoot = filepath.Join(wd, ".cursor", "skills")
	}
	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", skillRoot)
	fmt.Printf("SMARTCONTEXT_SKILL_ROOTS=%s\n", skillRoot)

	cat := core.BuildCatalog(nil)
	fmt.Printf("catalog_size=%d roots=%d\n", len(cat.Entries), len(cat.Roots))
	for _, e := range cat.Entries {
		fmt.Printf("  - %s (%s)\n", e.Name, e.Path)
	}

	tasks := []string{
		"terraform plan fails with Invalid for_each argument list of string",
		"terraform Cycle null_resource depends_on circular dependency",
		"terraform init Failed to query available provider packages no releases match constraints",
		"Error acquiring the state lock terraform force-unlock",
		"Backend configuration changed terraform init migrate-state reconfigure",
	}
	type row struct {
		Task          string   `json:"task"`
		SelectedNames []string `json:"selected_names"`
		HitTerraform  bool     `json:"hit_terraform_skill"`
	}
	var rows []row
	for _, task := range tasks {
		res := core.PackContext(core.PackRequest{Task: task, Kind: "skill", TopN: 5}, version.Version)
		hit := false
		for _, n := range res.SelectedNames {
			if len(n) >= 10 && n[:10] == "terraform-" {
				hit = true
				break
			}
		}
		rows = append(rows, row{Task: task, SelectedNames: res.SelectedNames, HitTerraform: hit})
		fmt.Printf("\ntask: %s\n  selected: %v\n  hit_terraform_*: %v\n", task, res.SelectedNames, hit)
	}
	out := filepath.Join(wd, "testdata", "terraform-poc", "_pack_results.json")
	b, _ := json.MarshalIndent(rows, "", "  ")
	_ = os.WriteFile(out, b, 0o644)
	fmt.Printf("\nwrote %s\n", out)
}
