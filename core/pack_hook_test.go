package core

import (
	"strings"
	"testing"
)

// Pro EnrichPack hook contract: every selected item carries rank, sticky flag,
// a real (non-pinned) score and per-item estimated tokens.
func TestPackItemsExposeRankStickyAndTokens(t *testing.T) {
	ResetMarkdownStableCache()
	root := t.TempDir()
	writePackCacheSkill(t, root, "goal-skill", "goal objective workflow", "goal body text")
	writePackCacheSkill(t, root, "noise-skill", "docker kubernetes noise", "noise body text")
	writePackCacheSkill(t, root, "other-skill", "terraform aws lambda", "other body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	res := PackContext(PackRequest{
		Task: "docker kubernetes noise", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: []string{"goal-skill"},
	}, "test")
	if len(res.Items) < 2 {
		t.Fatalf("items=%v", res.SelectedNames)
	}
	sum := 0
	for i, it := range res.Items {
		if it.Rank != i+1 {
			t.Fatalf("item %s rank=%d want %d", it.Name, it.Rank, i+1)
		}
		if it.EstimatedTokens <= 0 {
			t.Fatalf("item %s estimated_tokens=%d", it.Name, it.EstimatedTokens)
		}
		if it.Score >= 1e6 {
			t.Fatalf("item %s leaks sticky pin score %v", it.Name, it.Score)
		}
		sum += it.EstimatedTokens
		if it.Name == "goal-skill" && !it.Sticky {
			t.Fatalf("goal-skill should be flagged sticky: %+v", it)
		}
		if it.Name == "noise-skill" && it.Sticky {
			t.Fatalf("noise-skill is a rank hit, not sticky: %+v", it)
		}
	}
	if res.Items[0].Name != "goal-skill" {
		t.Fatalf("sticky item must stay first: %v", res.SelectedNames)
	}
	if strings.Contains(res.MarkdownDynamic, "1000000000") {
		t.Fatalf("dynamic markdown leaks 1e9 pin score:\n%s", res.MarkdownDynamic)
	}
	if sum <= 0 || sum > res.BudgetTokens {
		t.Fatalf("per-item token sum %d outside budget %d", sum, res.BudgetTokens)
	}
}

func TestReplaceMarkdownStableRecomposes(t *testing.T) {
	r := PackResult{MarkdownStable: "old\n", MarkdownDynamic: "dyn\n", Markdown: "old\ndyn\n"}
	r.EstimatedTokens = EstimateTokens(r.Markdown)
	out := ReplaceMarkdownStable(r, "a much longer replacement stable block\n")
	if out.Markdown != "a much longer replacement stable block\ndyn\n" {
		t.Fatalf("markdown=%q", out.Markdown)
	}
	if out.EstimatedTokens != EstimateTokens(out.Markdown) {
		t.Fatalf("estimated_tokens not recomputed: %d", out.EstimatedTokens)
	}
	if r.MarkdownStable != "old\n" {
		t.Fatal("input mutated")
	}
}
