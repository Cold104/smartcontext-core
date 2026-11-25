package lifecycle

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Cold104/smartcontext-core/core"
)

// stickyExpiryFixture builds a 3-skill catalog where "goal" and "noise" tasks
// select disjoint skills at TopN=1, so any carry-over is observable.
func stickyExpiryFixture(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "1")
	t.Setenv("SMARTCONTEXT_STICKY_TTL", "")
	root := t.TempDir()
	writeLCSkill(t, root, "goal-skill", "goal objective workflow", "goal body")
	writeLCSkill(t, root, "noise-skill", "docker kubernetes noise", "noise body")
	writeLCSkill(t, root, "other-skill", "terraform aws lambda", "other body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	core.ResetMarkdownStableCache()
	return home
}

func writeStickyState(t *testing.T, home string, names []string, taskHash string, updated time.Time) {
	t.Helper()
	st := StickyPackState{
		SelectedNames: names,
		TaskHash:      taskHash,
		UpdatedAt:     updated.UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(st)
	if err := os.WriteFile(stickyPackPath(home), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

const noiseTask = "docker kubernetes noise only"

// Regression: sticky names from an unrelated, stale task leaked into every
// later pack (LoadStickyNames ignored task_hash/updated_at).
func TestStickyExpiresOnTaskChangeAfterTTL(t *testing.T) {
	home := stickyExpiryFixture(t)
	writeStickyState(t, home, []string{"goal-skill"}, HashTask("goal objective workflow"),
		time.Now().Add(-2*time.Hour))

	res := PackContext(core.PackRequest{Task: noiseTask, TopN: 1, BudgetTokens: 4000})
	if containsName(res.SelectedNames, "goal-skill") {
		t.Fatalf("stale sticky from a different task leaked into pack: %v", res.SelectedNames)
	}
	if !containsName(res.SelectedNames, "noise-skill") {
		t.Fatalf("expected fresh rank hit noise-skill: %v", res.SelectedNames)
	}
}

func TestStickyMissingTimestampDifferentTaskExpires(t *testing.T) {
	home := stickyExpiryFixture(t)
	data, _ := json.Marshal(StickyPackState{SelectedNames: []string{"goal-skill"}})
	if err := os.WriteFile(stickyPackPath(home), data, 0o644); err != nil {
		t.Fatal(err)
	}
	res := PackContext(core.PackRequest{Task: noiseTask, TopN: 1, BudgetTokens: 4000})
	if containsName(res.SelectedNames, "goal-skill") {
		t.Fatalf("legacy sticky state (no task_hash/updated_at) must not carry over: %v", res.SelectedNames)
	}
}

func TestStickySameTaskHashReusedEvenWhenOld(t *testing.T) {
	home := stickyExpiryFixture(t)
	// Load/save hash the alias-expanded task text (as PackContext does).
	writeStickyState(t, home, []string{"goal-skill"}, HashTask(ExpandTaskTokens(noiseTask, LoadAliasMap(home))),
		time.Now().Add(-48*time.Hour))
	res := PackContext(core.PackRequest{Task: noiseTask, TopN: 1, BudgetTokens: 4000})
	if !containsName(res.SelectedNames, "goal-skill") {
		t.Fatalf("same task_hash should reuse sticky: %v", res.SelectedNames)
	}
}

func TestStickyWithinTTLSurvivesPhrasingDrift(t *testing.T) {
	home := stickyExpiryFixture(t)
	writeStickyState(t, home, []string{"goal-skill"}, HashTask("goal objective workflow"),
		time.Now().Add(-5*time.Minute))
	res := PackContext(core.PackRequest{Task: noiseTask, TopN: 1, BudgetTokens: 4000})
	if !containsName(res.SelectedNames, "goal-skill") {
		t.Fatalf("mid-task drift within TTL should keep sticky: %v", res.SelectedNames)
	}
}

func TestStickyTTLEnvOverride(t *testing.T) {
	home := stickyExpiryFixture(t)
	t.Setenv("SMARTCONTEXT_STICKY_TTL", "1m")
	writeStickyState(t, home, []string{"goal-skill"}, HashTask("goal objective workflow"),
		time.Now().Add(-5*time.Minute))
	res := PackContext(core.PackRequest{Task: noiseTask, TopN: 1, BudgetTokens: 4000})
	if containsName(res.SelectedNames, "goal-skill") {
		t.Fatalf("TTL=1m should expire a 5m-old sticky from another task: %v", res.SelectedNames)
	}
}

func TestStickyExplicitNamesIgnoreStoredState(t *testing.T) {
	home := stickyExpiryFixture(t)
	writeStickyState(t, home, []string{"goal-skill"}, HashTask(noiseTask), time.Now())
	res := PackContext(core.PackRequest{
		Task: noiseTask, TopN: 1, BudgetTokens: 4000, StickyNames: []string{"other-skill"},
	})
	if !containsName(res.SelectedNames, "other-skill") {
		t.Fatalf("explicit sticky_names must be honored: %v", res.SelectedNames)
	}
	if containsName(res.SelectedNames, "goal-skill") {
		t.Fatalf("explicit sticky_names must replace stored sticky: %v", res.SelectedNames)
	}
}

func TestStickyClearedStateNotReused(t *testing.T) {
	home := stickyExpiryFixture(t)
	writeStickyState(t, home, []string{"goal-skill"}, HashTask(noiseTask), time.Now())
	if err := ClearStickyNames(home); err != nil {
		t.Fatal(err)
	}
	res := PackContext(core.PackRequest{Task: noiseTask, TopN: 1, BudgetTokens: 4000})
	if containsName(res.SelectedNames, "goal-skill") {
		t.Fatalf("cleared sticky must not be reused: %v", res.SelectedNames)
	}
}

func TestTriageReorderRenumbersRank(t *testing.T) {
	res := core.PackResult{Items: []core.PackItem{
		{Name: "a", Rank: 1}, {Name: "b", Rank: 2}, {Name: "c", Rank: 3},
	}}
	prefs := TriagePrefs{Skills: map[string]TriageEntry{
		"c": {Status: "pin"}, "a": {Status: "deprecate"},
	}}
	out := applyTriageToPack(res, prefs)
	if len(out.Items) != 2 || out.Items[0].Name != "c" || out.Items[0].Rank != 1 || out.Items[1].Rank != 2 {
		t.Fatalf("triage must renumber ranks: %+v", out.Items)
	}
}
