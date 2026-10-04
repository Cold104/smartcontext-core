package lifecycle_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/lifecycle"
)

func TestTriageAndAliases(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	lifecycle.ResetMemoryEvents()

	if _, err := lifecycle.SetTriageStatus("demo", "pin", "test", home, "test"); err != nil {
		t.Fatal(err)
	}
	prefs := lifecycle.ListTriagePrefs(home)
	if prefs["demo"].Status != "pin" {
		t.Fatalf("status=%v", prefs["demo"])
	}

	if _, err := lifecycle.SetAlias("tf", []string{"terraform", "iac"}, home); err != nil {
		t.Fatal(err)
	}
	expanded := lifecycle.ExpandTaskTokens("fix tf plan", lifecycle.LoadAliasMap(home))
	if !strings.Contains(expanded, "terraform") {
		t.Fatalf("expected terraform expansion: %q", expanded)
	}
}

func TestSessionStartAndRefineList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")

	root := t.TempDir()
	skillDir := filepath.Join(root, "thin-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: thin-skill\ndescription: short\n---\n\n# Thin\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	res := lifecycle.SessionStart(lifecycle.SessionStartRequest{
		Roots:       []string{root},
		Home:        home,
		MarkSession: true,
		Source:      "test",
	})
	if res.Status != "ok" {
		t.Fatalf("status=%s note=%s", res.Status, res.Note)
	}
	if res.SkillCount != 1 {
		t.Fatalf("skill_count=%d", res.SkillCount)
	}
	if !res.MarkedSession || res.Watermark == nil {
		t.Fatal("expected watermark")
	}
	if res.Refine.Count < 1 {
		t.Fatalf("expected thin_description proposal, got %+v", res.Refine)
	}

	listed := lifecycle.ListProposals(home, "pending")
	if listed.Count < 1 {
		t.Fatal("refine list empty")
	}
}

func TestApplyTriagePrefs(t *testing.T) {
	cat := core.Catalog{Entries: []core.Entry{
		{Name: "a", Kind: "skill"},
		{Name: "b", Kind: "skill"},
		{Name: "c", Kind: "skill"},
	}}
	prefs := lifecycle.TriagePrefs{Skills: map[string]lifecycle.TriageEntry{
		"b": {Status: "pin"},
		"c": {Status: "deprecate"},
	}}
	out := lifecycle.ApplyTriagePrefs(cat, prefs)
	if len(out.Entries) != 2 {
		t.Fatalf("entries=%d", len(out.Entries))
	}
	if out.Entries[0].Name != "b" {
		t.Fatalf("pin first got %s", out.Entries[0].Name)
	}
}
