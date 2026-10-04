package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Cold104/smartcontext-core/version"
)

func writeSkill(tb testing.TB, dir, name, description, body string) string {
	tb.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		tb.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		tb.Fatal(err)
	}
	return path
}

func writeCatalogDoc(t *testing.T, dir, filename, frontmatter, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, filename)
	content := "---\n" + frontmatter + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoverSkillsAndPackMatch(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "windows-dev", "Windows and PowerShell development patterns for SmartContext", "# Windows")
	writeSkill(t, root, "other-skill", "Unrelated cooking recipes", "# Food")

	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")

	cat := BuildCatalog(nil)
	if len(cat.Entries) != 2 {
		t.Fatalf("entries=%d want 2: %+v", len(cat.Entries), cat.Entries)
	}

	res := PackContext(PackRequest{Task: "PowerShell Windows SmartContext", TopN: 1}, version.Version)
	if res.EmptyCatalog {
		t.Fatal("expected non-empty catalog")
	}
	if res.SelectedCount != 1 {
		t.Fatalf("selected_count=%d", res.SelectedCount)
	}
	if res.SelectedNames[0] != "windows-dev" {
		t.Fatalf("selected=%v want windows-dev", res.SelectedNames)
	}
	if res.Items[0].Score <= 0 {
		t.Fatalf("expected positive score, got %v", res.Items[0].Score)
	}
	if !strings.Contains(res.Markdown, "windows-dev") {
		t.Fatalf("markdown missing skill card:\n%s", res.Markdown)
	}
}

func TestParseFrontmatter(t *testing.T) {
	fields := ParseFrontmatter("---\nname: demo\ndescription: hello world\n---\n\nBody\n")
	if fields["name"] != "demo" || fields["description"] != "hello world" {
		t.Fatalf("fields=%v", fields)
	}
}

func TestDiscoverParsesRelatedAliasesKindOSAppliesTo(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "demo-skill")
	writeCatalogDoc(t, skillDir, "SKILL.md",
		`name: demo-skill
description: Demo workflow
tags: [demo, terraform]
triggers: [Invalid for_each]
os: linux
kind: skill
related: [error-class-playbook]
aliases: [demo alias]
governed_by: [secure-pack-hygiene]`,
		"# Skill body")

	ruleDir := filepath.Join(root, "rules", "secure-pack-hygiene")
	writeCatalogDoc(t, ruleDir, "RULE.md",
		`name: secure-pack-hygiene
description: Guardrail for pack
os: any
type: rule
applies_to: [demo-skill, tag:demo]
aliases: [hygiene]`,
		"# Rule body")

	entries := DiscoverSkills([]string{root})
	if len(entries) != 2 {
		t.Fatalf("entries=%d want 2: %+v", len(entries), entries)
	}

	byName := map[string]Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	skill := byName["demo-skill"]
	if skill.Kind != "skill" || skill.OS != "linux" {
		t.Fatalf("skill=%+v", skill)
	}
	if !reflect.DeepEqual(skill.Related, []string{"error-class-playbook"}) {
		t.Fatalf("related=%v", skill.Related)
	}
	if !reflect.DeepEqual(skill.Aliases, []string{"demo alias"}) {
		t.Fatalf("aliases=%v", skill.Aliases)
	}
	if !reflect.DeepEqual(skill.GovernedBy, []string{"secure-pack-hygiene"}) {
		t.Fatalf("governed_by=%v", skill.GovernedBy)
	}
	if !reflect.DeepEqual(skill.Tags, []string{"demo", "terraform"}) {
		t.Fatalf("tags=%v", skill.Tags)
	}
	if !reflect.DeepEqual(skill.Triggers, []string{"Invalid for_each"}) {
		t.Fatalf("triggers=%v", skill.Triggers)
	}

	rule := byName["secure-pack-hygiene"]
	if rule.Kind != "rule" || rule.OS != "any" {
		t.Fatalf("rule=%+v", rule)
	}
	if !reflect.DeepEqual(rule.AppliesTo, []string{"demo-skill", "tag:demo"}) {
		t.Fatalf("applies_to=%v", rule.AppliesTo)
	}
}

func TestDedupeCatalogKeyFirstWins(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeCatalogDoc(t, filepath.Join(rootA, "dup"), "SKILL.md",
		"name: dup\ndescription: first\nkind: skill\nos: any", "A")
	writeCatalogDoc(t, filepath.Join(rootB, "dup"), "SKILL.md",
		"name: dup\ndescription: second\nkind: skill\nos: any", "B")

	entries := DiscoverSkills([]string{rootA, rootB})
	if len(entries) != 1 {
		t.Fatalf("entries=%d want 1", len(entries))
	}
	if entries[0].Description != "first" {
		t.Fatalf("tie-break want first root, got description=%q path=%s", entries[0].Description, entries[0].Path)
	}
	// Same name, different kind — both kept.
	writeCatalogDoc(t, filepath.Join(rootA, "dup-rule"), "RULE.md",
		"name: dup\ndescription: as-rule\ntype: rule\nos: any", "R")
	entries = DiscoverSkills([]string{rootA})
	kinds := map[string]bool{}
	for _, e := range entries {
		if e.Name == "dup" {
			kinds[e.Kind] = true
		}
	}
	if !kinds["skill"] || !kinds["rule"] {
		t.Fatalf("want skill+rule same name, got %+v", entries)
	}
}
