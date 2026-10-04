package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/Cold104/smartcontext-core/version"
)

// Catalog is the discover → catalog stage result.
type Catalog struct {
	Entries []Entry  `json:"entries"`
	Roots   []string `json:"roots"`
}

// Entry is a catalog row (skill = workflow, rule = limitation).
// Catalog identity is CatalogKey(Kind, Name) — see docs/SKILL_RULE_FORMAT.md.
type Entry struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"` // skill|rule
	Path        string   `json:"path"`
	Root        string   `json:"root,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Triggers    []string `json:"triggers,omitempty"`
	OS          string   `json:"os,omitempty"` // windows|linux|macos|any
	Related     []string `json:"related,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	AppliesTo   []string `json:"applies_to,omitempty"`  // rules
	GovernedBy  []string `json:"governed_by,omitempty"` // skills
}

// CatalogKey is the deterministic catalog identity: (kind, name), case-insensitive name.
func CatalogKey(kind, name string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	n := strings.ToLower(strings.TrimSpace(name))
	return k + "\x00" + n
}

// BuildCatalog discovers skills/rules under roots.
// When roots is nil/empty, ResolveRoots(nil) applies env / walk-up discovery.
// When roots is non-empty, ResolveRoots expands them as explicit host roots.
func BuildCatalog(roots []string) Catalog {
	resolved := ResolveRoots(roots)
	return Catalog{
		Entries: DiscoverSkills(resolved),
		Roots:   resolved,
	}
}

// EmptyCatalog is kept for tests that force an empty skill tree via env roots.
func EmptyCatalog() Catalog {
	return BuildCatalog(nil)
}

// DiscoverReport is the read-only discover_skills / list_roots POC payload.
type DiscoverReport struct {
	Host                HostSummary `json:"host"`
	Roots               []string    `json:"roots"`
	RootCount           int         `json:"root_count"`
	SkillCount          int         `json:"skill_count"`
	Skills              []Entry     `json:"skills"`
	SkillNames          []string    `json:"skill_names"`
	WorkspaceEnv        string      `json:"workspace_env,omitempty"`
	SkillRootsEnv       string      `json:"skill_roots_env,omitempty"`
	CwdIsHome           bool        `json:"cwd_is_home"`
	WorkspaceSeedSource string      `json:"workspace_seed_source,omitempty"`
	Warning             string      `json:"warning,omitempty"`
	Note                string      `json:"note"`
}

// ReportDiscovery resolves roots and lists discovered skills/rules (no ranking).
func ReportDiscovery(explicitRoots []string) DiscoverReport {
	cat := BuildCatalog(explicitRoots)
	names := make([]string, 0, len(cat.Entries))
	for _, e := range cat.Entries {
		names = append(names, e.Name)
	}
	diag := DiagnoseRootResolution(explicitRoots)
	return DiscoverReport{
		Host:                SummarizeHost(),
		Roots:               cat.Roots,
		RootCount:           len(cat.Roots),
		SkillCount:          len(cat.Entries),
		Skills:              cat.Entries,
		SkillNames:          names,
		WorkspaceEnv:        strings.TrimSpace(os.Getenv("SMARTCONTEXT_WORKSPACE_ROOTS")),
		SkillRootsEnv:       strings.TrimSpace(os.Getenv("SMARTCONTEXT_SKILL_ROOTS")),
		CwdIsHome:           diag.CwdIsHome,
		WorkspaceSeedSource: diag.WorkspaceSeedSource,
		Warning:             diag.Warning,
		Note: fmt.Sprintf(
			"v%s: resolved roots + skill/rule names (read-only); see host_info for full resources",
			version.Version,
		),
	}
}
