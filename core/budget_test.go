package core

import (
	"strings"
	"testing"
)

func TestResolveBudgetAbsoluteTiers(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "token-pack", "Pack under a token budget", "# Body with some content for naive estimate.\n")
	writeSkill(t, root, "context-budget", "Estimate tokens and truncate", "# More body text for corpus.\n")
	cat := BuildCatalog([]string{root})

	cases := []struct {
		preset string
		tokens int
		canon  string
	}{
		{"super_saver", BudgetTierSuperSaver, "super_saver"},
		{"small", BudgetTierSuperSaver, "super_saver"},
		{"balanced", BudgetTierBalanced, "balanced"},
		{"medium", BudgetTierBalanced, "balanced"},
		{"max", BudgetTierMax, "max"},
		{"large", BudgetTierMax, "max"},
	}
	for _, tc := range cases {
		res, err := ResolveBudget(cat.Entries, "pack tokens", tc.preset, 0, 0)
		if err != nil {
			t.Fatalf("%s: %v", tc.preset, err)
		}
		if res.BudgetTokens != tc.tokens || res.BudgetPreset != tc.canon || res.Mode != "tier_absolute" {
			t.Fatalf("%s: %+v want tokens=%d preset=%s", tc.preset, res, tc.tokens, tc.canon)
		}
	}
}

func TestResolveBudgetAutoClamped(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "a", "alpha skill", "# a\n")
	cat := BuildCatalog([]string{root})

	short, err := ResolveBudget(cat.Entries, "hi", "auto", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if short.BudgetPreset != "auto" || short.Mode != "tier_absolute" {
		t.Fatalf("%+v", short)
	}
	if short.BudgetTokens < BudgetTierSuperSaver || short.BudgetTokens > BudgetTierMax {
		t.Fatalf("auto tokens=%d out of clamp", short.BudgetTokens)
	}

	longTask := strings.Repeat("pack context skill workflow tokens ", 80)
	// Large catalog
	for i := 0; i < 40; i++ {
		writeSkill(t, root, "skill-"+strings.Repeat("x", i+1), "desc "+strings.Repeat("y", i+1), "# body\n")
	}
	big := BuildCatalog([]string{root})
	long, err := ResolveBudget(big.Entries, longTask, "auto", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if long.BudgetTokens < short.BudgetTokens {
		t.Fatalf("expected larger auto budget for bigger task/catalog: short=%d long=%d", short.BudgetTokens, long.BudgetTokens)
	}
	if long.BudgetTokens > BudgetTierMax {
		t.Fatalf("auto exceeded max: %d", long.BudgetTokens)
	}
}

func TestResolveBudgetLegacyNaive(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "token-pack", "Pack under a token budget", "# Body with some content for naive estimate.\n")
	writeSkill(t, root, "context-budget", "Estimate tokens and truncate", "# More body text for corpus.\n")
	cat := BuildCatalog([]string{root})

	minB, err := ResolveBudget(cat.Entries, "pack tokens", "minimum", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	maxB, err := ResolveBudget(cat.Entries, "pack tokens", "maximum", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if minB.Mode != "naive_fraction" || minB.BudgetPct != 30 {
		t.Fatalf("minimum: %+v", minB)
	}
	if maxB.BudgetPct != 90 || maxB.BudgetTokens < minB.BudgetTokens {
		t.Fatalf("maximum: %+v min=%+v", maxB, minB)
	}
}

func TestResolveBudgetDefaultBalanced(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "token-pack", "Pack under a token budget", "# Body\n")
	cat := BuildCatalog([]string{root})
	res, err := ResolveBudget(cat.Entries, "pack tokens", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.BudgetPreset != "balanced" || res.BudgetTokens != BudgetTierBalanced {
		t.Fatalf("%+v", res)
	}
}

func TestResolveBudgetAbsoluteExclusive(t *testing.T) {
	_, err := ResolveBudget(nil, "t", "balanced", 0, 500)
	if err == nil {
		t.Fatal("expected mutual exclusion error")
	}
	abs, err := ResolveBudget(nil, "t", "", 0, 500)
	if err != nil || abs.BudgetTokens != 500 || abs.Mode != "absolute" {
		t.Fatalf("%+v err=%v", abs, err)
	}
}

func TestResolveBudgetEmptyCatalog(t *testing.T) {
	res, err := ResolveBudget(nil, "hello", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.BudgetTokens != EmptyCatalogBudgetFallback {
		t.Fatalf("tokens=%d", res.BudgetTokens)
	}
	if res.BudgetPreset != "balanced" || res.Mode != "empty_fallback" {
		t.Fatalf("%+v", res)
	}
}

func TestAutoBudgetTokensClamp(t *testing.T) {
	if got := AutoBudgetTokens("", 0); got != BudgetTierSuperSaver {
		t.Fatalf("empty: %d", got)
	}
	huge := AutoBudgetTokens(strings.Repeat("word ", 5000), 10_000)
	if huge != BudgetTierMax {
		t.Fatalf("huge: %d", huge)
	}
}

func TestPackContextBudgetGateTruncatesBodies(t *testing.T) {
	root := t.TempDir()
	longBody := "# Skill\n\n" + strings.Repeat("x", 4000)
	writeSkill(t, root, "bulky-skill", "bulky skill for budget gate test", longBody)

	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")

	res := PackContext(PackRequest{
		Task:          "bulky skill budget gate",
		TopN:          3,
		BudgetTokens:  200,
		IncludeBodies: true,
	}, "1.1.0")
	if res.BudgetTokens != 200 {
		t.Fatalf("budget_tokens=%d", res.BudgetTokens)
	}
	if res.EstimatedTokens <= 0 {
		t.Fatalf("estimated_tokens=%d", res.EstimatedTokens)
	}
	if res.SelectedCount < 1 {
		t.Fatal("expected selection")
	}
	if res.Items[0].Body == "" {
		t.Fatal("expected body under budget")
	}
	if !strings.Contains(res.Items[0].Body, "truncated for token budget") && len(res.Items[0].Body) > 900 {
		t.Fatalf("expected truncation marker or short body; len=%d", len(res.Items[0].Body))
	}
}

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Fatal("empty")
	}
	if EstimateTokens("abcd") != 1 {
		t.Fatalf("got %d", EstimateTokens("abcd"))
	}
	if EstimateTokens("abcdefgh") != 2 {
		t.Fatalf("got %d", EstimateTokens("abcdefgh"))
	}
}
