//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/version"
)

func main() {
	_ = os.Unsetenv("SMARTCONTEXT_SKILL_ROOTS")
	res := core.PackContext(core.PackRequest{
		Task: "Windows PowerShell SmartContext development",
		TopN: 3,
	}, version.Version)
	b, _ := json.MarshalIndent(map[string]any{
		"empty_catalog":  res.EmptyCatalog,
		"selected_count": res.SelectedCount,
		"selected_names": res.SelectedNames,
		"root_count":     res.RootCount,
	}, "", "  ")
	fmt.Println(string(b))
	if res.SelectedCount < 1 {
		os.Exit(1)
	}
}
