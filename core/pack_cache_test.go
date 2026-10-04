package core

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writePackCacheSkill(t *testing.T, root, name, desc, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + desc + "\nkind: skill\nos: any\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackMarkdownStablePrefixOmitsTask(t *testing.T) {
	root := t.TempDir()
	writePackCacheSkill(t, root, "alpha-skill", "alpha workflow helper", "ALPHA_BODY")
	writePackCacheSkill(t, root, "beta-skill", "beta workflow helper", "BETA_BODY")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	r1 := PackContext(PackRequest{
		Task: "work on alpha workflow", TopN: 5, BudgetTokens: 4000, IncludeBodies: true,
	}, "test")
	if strings.Contains(r1.MarkdownStable, "work on alpha") {
		t.Fatalf("stable prefix must not include task: %q", r1.MarkdownStable)
	}
	if !strings.Contains(r1.MarkdownDynamic, "work on alpha") {
		t.Fatalf("dynamic suffix must include task")
	}
	if !strings.HasPrefix(r1.Markdown, r1.MarkdownStable) {
		t.Fatalf("markdown should start with stable prefix")
	}
	if !strings.Contains(r1.MarkdownStable, "## Skills") {
		t.Fatalf("stable missing Skills section")
	}
	// Same skill set + bodies → identical stable prefix when only task changes.
	r2 := PackContext(PackRequest{
		Task: "totally different phrasing about alpha workflow", TopN: 5, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: r1.SelectedNames,
	}, "test")
	if r1.MarkdownStable != r2.MarkdownStable {
		t.Fatalf("stable prefix should be byte-identical when skill set unchanged\n---a---\n%s\n---b---\n%s", r1.MarkdownStable, r2.MarkdownStable)
	}
	if r1.MarkdownDynamic == r2.MarkdownDynamic {
		t.Fatalf("dynamic suffix should change with task")
	}
}

// TestMarkdownStableByteIdentityIdenticalSelection is the industry-demand
// regression: identical selected skill sets must yield identical markdown_stable
// bytes (including whitespace), so hosts can pin the procedural prefix for KV-cache.
func TestMarkdownStableByteIdentityIdenticalSelection(t *testing.T) {
	root := t.TempDir()
	writePackCacheSkill(t, root, "zeta-skill", "zeta workflow helper", "ZETA_BODY\nline-two")
	writePackCacheSkill(t, root, "alpha-skill", "alpha workflow helper", "ALPHA_BODY\n  indented")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	sticky := []string{"alpha-skill", "zeta-skill"}
	a := PackContext(PackRequest{
		Task: "first task phrasing", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: sticky,
	}, "test")
	b := PackContext(PackRequest{
		Task: "second task phrasing", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: sticky,
	}, "test")

	if len(a.SelectedNames) == 0 {
		t.Fatal("expected non-empty selection")
	}
	sa := append([]string(nil), a.SelectedNames...)
	sb := append([]string(nil), b.SelectedNames...)
	slices.Sort(sa)
	slices.Sort(sb)
	if !slices.Equal(sa, sb) {
		t.Fatalf("selected skill sets differ: %v vs %v", a.SelectedNames, b.SelectedNames)
	}
	if !bytes.Equal([]byte(a.MarkdownStable), []byte(b.MarkdownStable)) {
		t.Fatalf("markdown_stable bytes must be identical for identical selected skill set\n---a---\n%q\n---b---\n%q",
			a.MarkdownStable, b.MarkdownStable)
	}
	if strings.Contains(a.MarkdownStable, "first task") || strings.Contains(a.MarkdownStable, "second task") {
		t.Fatalf("stable prefix must not embed task text")
	}
}

// TestCacheHitProxyMarkdownStableOverlap reports % identical prefix length for
// sequential packs with identical selection (cache-hit proxy; host measures KV %).
func TestCacheHitProxyMarkdownStableOverlap(t *testing.T) {
	root := t.TempDir()
	writePackCacheSkill(t, root, "zeta-skill", "zeta workflow helper", "ZETA_BODY")
	writePackCacheSkill(t, root, "alpha-skill", "alpha workflow helper", "ALPHA_BODY")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	sticky := []string{"alpha-skill", "zeta-skill"}
	a := PackContext(PackRequest{
		Task: "first phrasing", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: sticky,
	}, "test")
	b := PackContext(PackRequest{
		Task: "second phrasing", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: sticky,
	}, "test")

	if !bytes.Equal([]byte(a.MarkdownStable), []byte(b.MarkdownStable)) {
		t.Fatalf("markdown_stable equality required for identical selection")
	}
	if pct := IdenticalPrefixOverlapPct(a.MarkdownStable, b.MarkdownStable); pct != 100 {
		t.Fatalf("overlap%%=%v want 100", pct)
	}
	// Partial divergence: truncate one side and expect < 100.
	truncated := a.MarkdownStable
	if len(truncated) > 20 {
		truncated = truncated[:len(truncated)/2]
	}
	partial := IdenticalPrefixOverlapPct(a.MarkdownStable, truncated+"X")
	if partial >= 100 {
		t.Fatalf("divergent suffix should lower overlap%%; got %v", partial)
	}
}

func TestStickyNamesSurviveMidTaskRepack(t *testing.T) {
	root := t.TempDir()
	const needle = "NEEDLE_TOKEN_9f3c2a1b"
	writePackCacheSkill(t, root, "goal-skill", "goal objective workflow needle carrier",
		"Procedure retaining "+needle+" for recoverability tests.")
	writePackCacheSkill(t, root, "noise-skill", "unrelated docker kubernetes noise", "noise body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	first := PackContext(PackRequest{
		Task: "goal objective workflow", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
	}, "test")
	found := false
	for _, n := range first.SelectedNames {
		if n == "goal-skill" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected goal-skill in first pack, got %v", first.SelectedNames)
	}
	if !strings.Contains(first.Markdown, needle) {
		t.Fatalf("needle missing from first pack")
	}

	// Shift task toward noise; sticky must keep goal-skill + needle.
	second := PackContext(PackRequest{
		Task: "docker kubernetes noise only please", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: []string{"goal-skill"},
	}, "test")
	found = false
	for _, n := range second.SelectedNames {
		if n == "goal-skill" {
			found = true
		}
	}
	if !found {
		t.Fatalf("sticky goal-skill dropped on mid-task re-pack: %v", second.SelectedNames)
	}
	if !strings.Contains(second.Markdown, needle) {
		t.Fatalf("needle missing after mid-task re-pack")
	}
}
