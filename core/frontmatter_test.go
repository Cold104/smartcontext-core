package core

import (
	"reflect"
	"testing"
)

func TestParseSkillFrontmatterListsAndKindOS(t *testing.T) {
	text := `---
name: pack-operator
description: Pack and discover workflows
tags: [pack, discover, mcp]
triggers:
  - pack_context
  - discover_skills
os: windows
kind: skill
related: [budget-aware-packing, host-os-windows]
aliases: ["tf pack", pack-ops]
governed_by: [secure-pack-hygiene]
---

# Body
`
	fm := ParseSkillFrontmatter(text)
	if fm.Name != "pack-operator" {
		t.Fatalf("name=%q", fm.Name)
	}
	if fm.Description != "Pack and discover workflows" {
		t.Fatalf("description=%q", fm.Description)
	}
	if !reflect.DeepEqual(fm.Tags, []string{"pack", "discover", "mcp"}) {
		t.Fatalf("tags=%v", fm.Tags)
	}
	if !reflect.DeepEqual(fm.Triggers, []string{"pack_context", "discover_skills"}) {
		t.Fatalf("triggers=%v", fm.Triggers)
	}
	if fm.OS != "windows" {
		t.Fatalf("os=%q", fm.OS)
	}
	if fm.Kind != "skill" {
		t.Fatalf("kind=%q", fm.Kind)
	}
	if !reflect.DeepEqual(fm.Related, []string{"budget-aware-packing", "host-os-windows"}) {
		t.Fatalf("related=%v", fm.Related)
	}
	if !reflect.DeepEqual(fm.Aliases, []string{"tf pack", "pack-ops"}) {
		t.Fatalf("aliases=%v", fm.Aliases)
	}
	if !reflect.DeepEqual(fm.GovernedBy, []string{"secure-pack-hygiene"}) {
		t.Fatalf("governed_by=%v", fm.GovernedBy)
	}
}

func TestParseSkillFrontmatterRuleAppliesTo(t *testing.T) {
	text := `---
name: secure-pack-hygiene
description: Limit pack/factory secret leakage
type: rule
os: any
tags: security
applies_to: [smartcontext-pack-operator, budget-aware-packing, tag:factory]
related: []
aliases: [secure hygiene, no-secrets-in-pack]
---
`
	fm := ParseSkillFrontmatter(text)
	if fm.Kind != "rule" {
		t.Fatalf("kind from type=%q want rule", fm.Kind)
	}
	if fm.OS != "any" {
		t.Fatalf("os=%q", fm.OS)
	}
	wantApplies := []string{"smartcontext-pack-operator", "budget-aware-packing", "tag:factory"}
	if !reflect.DeepEqual(fm.AppliesTo, wantApplies) {
		t.Fatalf("applies_to=%v want %v", fm.AppliesTo, wantApplies)
	}
	if !reflect.DeepEqual(fm.Aliases, []string{"secure hygiene", "no-secrets-in-pack"}) {
		t.Fatalf("aliases=%v", fm.Aliases)
	}
	if len(fm.Related) != 0 {
		t.Fatalf("related=%v want empty", fm.Related)
	}
}

func TestParseStringListVariants(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"[a, b]", []string{"a", "b"}},
		{"- a - b", []string{"a", "b"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{`"x y", z`, []string{"x y", "z"}},
		{"", nil},
		{"[]", nil},
	}
	for _, tc := range cases {
		got := ParseStringList(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("ParseStringList(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeOSAndKind(t *testing.T) {
	if NormalizeOS("WIN32") != "windows" {
		t.Fatal(NormalizeOS("WIN32"))
	}
	if NormalizeOS("darwin") != "macos" {
		t.Fatal(NormalizeOS("darwin"))
	}
	if NormalizeOS("") != "any" {
		t.Fatal(NormalizeOS(""))
	}
	if NormalizeKind("RULE") != "rule" {
		t.Fatal(NormalizeKind("RULE"))
	}
	if NormalizeKind("workflow") != "skill" {
		t.Fatal(NormalizeKind("workflow"))
	}
}

func TestCatalogKey(t *testing.T) {
	a := CatalogKey("skill", "Pack-Operator")
	b := CatalogKey("Skill", "pack-operator")
	if a != b {
		t.Fatalf("keys differ: %q vs %q", a, b)
	}
	if CatalogKey("skill", "x") == CatalogKey("rule", "x") {
		t.Fatal("skill and rule with same name must differ")
	}
}
