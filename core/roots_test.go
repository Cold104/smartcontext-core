package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cold104/smartcontext-core/version"
)

func clearHostIDEWorkspaceEnv(t *testing.T) {
	t.Helper()
	for _, key := range hostIDEWorkspaceEnvKeys {
		t.Setenv(key, "")
	}
}

func TestDefaultRootsEnvOverride(t *testing.T) {
	// Semicolon-separated contract is OS-agnostic (avoids Windows drive-letter colon issues).
	a := filepath.Join("tmp", "skills-a")
	b := filepath.Join("tmp", "skills-b")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", a+"; "+b)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	roots := DefaultRoots()
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d: %v", len(roots), roots)
	}
	if roots[0] != a || roots[1] != b {
		t.Fatalf("unexpected roots: %v", roots)
	}
}

func TestDefaultRootsIncludesAgentMarkers(t *testing.T) {
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	dir := t.TempDir()
	t.Chdir(dir)

	roots := DefaultRoots()
	joined := strings.Join(roots, "\n")
	want := filepath.Join(dir, ".cursor", "skills")
	found := false
	for _, r := range roots {
		if r == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in roots:\n%s", want, joined)
	}
}

func TestWorkspaceRootsEnvFindsNestedSkills(t *testing.T) {
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	project := t.TempDir()
	skillDir := filepath.Join(project, ".cursor", "skills", "terraform-invalid-foreach")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: terraform-invalid-foreach\ndescription: Invalid for_each\n---\n\n# TF\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", project)
	// cwd elsewhere (simulate MCP home/default)
	elsewhere := t.TempDir()
	t.Chdir(elsewhere)

	cat := BuildCatalog(nil)
	if len(cat.Entries) < 1 {
		t.Fatalf("expected skill via WORKSPACE_ROOTS, roots=%v", cat.Roots)
	}
	found := false
	for _, e := range cat.Entries {
		if e.Name == "terraform-invalid-foreach" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("terraform-invalid-foreach not found: %+v", cat.Entries)
	}
}

func TestWalkUpFromNestedCwdFindsProjectSkills(t *testing.T) {
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(project, ".cursor", "skills", "terraform-state-lock")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: terraform-state-lock\ndescription: State lock\n---\n\n# Lock\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(project, "go")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	cat := BuildCatalog(nil)
	found := false
	for _, e := range cat.Entries {
		if e.Name == "terraform-state-lock" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("walk-up should find terraform-state-lock; roots=%v entries=%+v", cat.Roots, cat.Entries)
	}
}

func TestHostAutoCursorWorkspaceFindsSkills(t *testing.T) {
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	project := t.TempDir()
	skillDir := filepath.Join(project, ".cursor", "skills", "cursor-workspace-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: cursor-workspace-skill\ndescription: From CURSOR_WORKSPACE\n---\n\n# Cursor\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	elsewhere := t.TempDir()
	t.Chdir(elsewhere)
	t.Setenv("CURSOR_WORKSPACE", project)

	cat := BuildCatalog(nil)
	found := false
	for _, e := range cat.Entries {
		if e.Name == "cursor-workspace-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("CURSOR_WORKSPACE should find skill; roots=%v entries=%+v", cat.Roots, cat.Entries)
	}
}

func TestHostAutoWorkspaceFolderFindsSkills(t *testing.T) {
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	project := t.TempDir()
	skillDir := filepath.Join(project, ".agents", "skills", "workspace-folder-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: workspace-folder-skill\ndescription: From WORKSPACE_FOLDER\n---\n\n# Folder\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	elsewhere := t.TempDir()
	t.Chdir(elsewhere)
	t.Setenv("WORKSPACE_FOLDER", project)

	cat := BuildCatalog(nil)
	found := false
	for _, e := range cat.Entries {
		if e.Name == "workspace-folder-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("WORKSPACE_FOLDER should find skill; roots=%v entries=%+v", cat.Roots, cat.Entries)
	}
}

func TestExplicitWorkspaceRootsStillOverrideHostAuto(t *testing.T) {
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	wanted := t.TempDir()
	skillDir := filepath.Join(wanted, ".cursor", "skills", "explicit-ws-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: explicit-ws-skill\ndescription: Explicit WORKSPACE_ROOTS wins\n---\n\n# Explicit\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	distractor := t.TempDir()
	badSkill := filepath.Join(distractor, ".cursor", "skills", "distractor-skill")
	if err := os.MkdirAll(badSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	badBody := "---\nname: distractor-skill\ndescription: Should not be selected via host-auto\n---\n\n# No\n"
	if err := os.WriteFile(filepath.Join(badSkill, "SKILL.md"), []byte(badBody), 0o644); err != nil {
		t.Fatal(err)
	}

	elsewhere := t.TempDir()
	t.Chdir(elsewhere)
	t.Setenv("CURSOR_WORKSPACE", distractor)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", wanted)

	cat := BuildCatalog(nil)
	names := map[string]bool{}
	for _, e := range cat.Entries {
		names[e.Name] = true
	}
	if !names["explicit-ws-skill"] {
		t.Fatalf("expected explicit-ws-skill; entries=%+v", cat.Entries)
	}
	if names["distractor-skill"] {
		t.Fatalf("SMARTCONTEXT_WORKSPACE_ROOTS should skip host-auto distractor; entries=%+v", cat.Entries)
	}
}

func TestPackContextExplicitRoots(t *testing.T) {
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	project := t.TempDir()
	skillDir := filepath.Join(project, ".cursor", "skills", "demo-root-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: demo-root-skill\ndescription: Explicit roots pack demo\n---\n\n# Demo\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	elsewhere := t.TempDir()
	t.Chdir(elsewhere)

	res := PackContext(PackRequest{
		Task:  "explicit roots pack demo",
		TopN:  3,
		Roots: []string{project},
	}, version.Version)
	if res.EmptyCatalog {
		t.Fatal("expected non-empty catalog via explicit roots")
	}
	hit := false
	for _, n := range res.SelectedNames {
		if n == "demo-root-skill" {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("selected=%v want demo-root-skill", res.SelectedNames)
	}
}

func TestPackContextEmptyCatalog(t *testing.T) {
	root := filepath.Join(t.TempDir(), "empty-skills")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	res := PackContext(PackRequest{Task: "hello"}, version.Version)
	if !res.EmptyCatalog {
		t.Fatal("expected empty_catalog")
	}
	if res.SelectedCount != 0 {
		t.Fatalf("selected_count=%d", res.SelectedCount)
	}
	if res.Task != "hello" {
		t.Fatalf("task=%q", res.Task)
	}
	if res.BudgetPreset != "balanced" {
		t.Fatalf("budget_preset=%q", res.BudgetPreset)
	}
	if res.BudgetTokens != EmptyCatalogBudgetFallback {
		t.Fatalf("budget_tokens=%d want %d", res.BudgetTokens, EmptyCatalogBudgetFallback)
	}
	if res.RootCount != 1 || len(res.Roots) != 1 || res.Roots[0] != root {
		t.Fatalf("roots shape: count=%d roots=%v", res.RootCount, res.Roots)
	}
	if res.Version != version.Version {
		t.Fatalf("version=%q want %s", res.Version, version.Version)
	}
	if !strings.Contains(res.Markdown, "SmartContext pack") {
		t.Fatalf("markdown missing title: %s", res.Markdown)
	}
	if !strings.Contains(res.Note, "empty catalog") {
		t.Fatalf("note should mark empty catalog: %q", res.Note)
	}
}

func TestReportDiscovery(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "listed-skill", "Listed for discover report", "# Body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)

	rep := ReportDiscovery(nil)
	if rep.SkillCount < 1 {
		t.Fatalf("skill_count=%d", rep.SkillCount)
	}
	hit := false
	for _, n := range rep.SkillNames {
		if n == "listed-skill" {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("skill_names=%v", rep.SkillNames)
	}
	if rep.Host.OS == "" || rep.Host.Arch == "" || rep.Host.NumCPU < 1 {
		t.Fatalf("host summary incomplete: %+v", rep.Host)
	}
	if !strings.Contains(rep.Note, version.Version) {
		t.Fatalf("note should include version %s: %q", version.Version, rep.Note)
	}
	if strings.Contains(rep.Note, "1.1.0-dev") {
		t.Fatalf("note still has stale 1.1.0-dev: %q", rep.Note)
	}
	if rep.WorkspaceSeedSource != SeedSourceSkillRootsEnv {
		t.Fatalf("workspace_seed_source=%q want %s", rep.WorkspaceSeedSource, SeedSourceSkillRootsEnv)
	}
}

func TestCursorWorkspaceFromHomeCwd(t *testing.T) {
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	clearHostIDEWorkspaceEnv(t)

	project := t.TempDir()
	skillDir := filepath.Join(project, ".cursor", "skills", "home-cwd-cursor-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: home-cwd-cursor-skill\ndescription: Found via CURSOR_WORKSPACE while cwd is home\n---\n\n# Home cwd\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("UserHomeDir unavailable")
	}
	t.Chdir(home)
	t.Setenv("CURSOR_WORKSPACE", project)

	cat := BuildCatalog(nil)
	found := false
	for _, e := range cat.Entries {
		if e.Name == "home-cwd-cursor-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("CURSOR_WORKSPACE from home cwd should find skill; roots=%v entries=%+v", cat.Roots, cat.Entries)
	}

	diag := DiagnoseRootResolution(nil)
	if !diag.CwdIsHome {
		t.Fatalf("cwd_is_home=false want true (cwd=%s home=%s)", mustGetwd(t), home)
	}
	if diag.WorkspaceSeedSource != SeedSourceIDEEnv {
		t.Fatalf("workspace_seed_source=%q want %s", diag.WorkspaceSeedSource, SeedSourceIDEEnv)
	}
	if diag.Warning != "" {
		t.Fatalf("unexpected warning when IDE workspace resolves project skills: %q", diag.Warning)
	}

	rep := ReportDiscovery(nil)
	if !rep.CwdIsHome || rep.WorkspaceSeedSource != SeedSourceIDEEnv {
		t.Fatalf("discover diagnostics: cwd_is_home=%v source=%q", rep.CwdIsHome, rep.WorkspaceSeedSource)
	}

	info := CollectHostInfo()
	if !info.CwdIsHome || info.WorkspaceSeedSource != SeedSourceIDEEnv {
		t.Fatalf("host_info diagnostics: cwd_is_home=%v source=%q", info.CwdIsHome, info.WorkspaceSeedSource)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}
