package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/eval"
	"github.com/Cold104/smartcontext-core/lifecycle"
)

func TestMetricsRecallAndReduction(t *testing.T) {
	if got := eval.RecallAtK([]string{"a", "b", "c"}, []string{"b", "d"}, 5); got != 0.5 {
		t.Fatalf("recall=%v", got)
	}
	if got := eval.Reduction(1000, 300); got != 0.7 {
		t.Fatalf("reduction=%v", got)
	}
	if !eval.HitAny([]string{"X"}, []string{"x"}) {
		t.Fatal("hitAny case fold")
	}
}

func TestWriteTokenAndRoleFixturesDiscoverable(t *testing.T) {
	root := t.TempDir()
	n, err := eval.WriteTokenFixture(root, 4)
	if err != nil {
		t.Fatal(err)
	}
	if n != 15 {
		t.Fatalf("token skills=%d", n)
	}
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	cat := core.BuildCatalog(nil)
	if len(cat.Entries) != 15 {
		t.Fatalf("discover token=%d", len(cat.Entries))
	}

	roleRoot := t.TempDir()
	rn, err := eval.WriteRoleFixture(roleRoot, 2)
	if err != nil {
		t.Fatal(err)
	}
	if rn != 48 {
		t.Fatalf("role skills=%d want 48", rn)
	}
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", roleRoot)
	cat = core.BuildCatalog(nil)
	if len(cat.Entries) != 48 {
		t.Fatalf("discover role=%d", len(cat.Entries))
	}
}

func TestCorpusTelemetryRunTokenGate(t *testing.T) {
	if testing.Short() {
		t.Skip("full corpus run")
	}
	goRoot := filepath.Join("..", "..")
	rep, err := eval.Run(eval.Options{
		GoRoot:       goRoot,
		WriteDocs:    false,
		PackIters:    3, // faster than report default
		BudgetTokens: eval.DefaultBudgetTokens,
	})
	if err != nil {
		t.Fatal(err)
	}

	if rep.TokenSuite.SkillCount != 15 {
		t.Fatalf("token skills=%d", rep.TokenSuite.SkillCount)
	}
	if rep.TokenSuite.HitRate < 0.8 {
		t.Fatalf("token hit_rate=%.2f below 0.80 tasks=%+v", rep.TokenSuite.HitRate, rep.TokenSuite.Tasks)
	}
	if rep.TokenSuite.MeanRecallAtK < 0.8 {
		t.Fatalf("token mean_recall=%.2f below 0.80", rep.TokenSuite.MeanRecallAtK)
	}
	// Reduction gate is informational if scrub overlay shrinks measured gain;
	// still require positive reduction vs naive baseline.
	if rep.TokenSuite.MedianReduction <= 0 {
		t.Fatalf("token median_reduction=%.3f expected > 0", rep.TokenSuite.MedianReduction)
	}

	if rep.RoleSuite.SkillCount != 48 {
		t.Fatalf("role skills=%d", rep.RoleSuite.SkillCount)
	}
	// Soft floor: multi-role noise is harder; require majority hit-rate.
	if rep.RoleSuite.HitRate < 0.5 {
		t.Fatalf("role hit_rate=%.2f below soft floor 0.50", rep.RoleSuite.HitRate)
	}
	t.Logf("token hit=%.2f recall=%.2f red=%.1f%% p50=%.3fms",
		rep.TokenSuite.HitRate, rep.TokenSuite.MeanRecallAtK, rep.TokenSuite.MedianReduction*100, rep.TokenSuite.PackMSP50)
	t.Logf("role  hit=%.2f recall=%.2f red=%.1f%% p50=%.3fms gate=%v",
		rep.RoleSuite.HitRate, rep.RoleSuite.MeanRecallAtK, rep.RoleSuite.MedianReduction*100, rep.RoleSuite.PackMSP50, rep.RoleSuite.GateOK)

	if !rep.CaseStudy.ColdEmptyCatalog {
		t.Fatal("case study cold should be empty")
	}
	if !rep.CaseStudy.SeededHit {
		t.Fatalf("case study seeded miss: %v", rep.CaseStudy.SeededSelected)
	}
	if rep.Telemetry.RawTaskLeaked {
		t.Fatal("raw task leaked into telemetry JSONL")
	}
	if rep.Telemetry.PackEventsWritten < 1 {
		t.Fatal("expected pack telemetry events")
	}
	if len(rep.MCPToolNames) < 2 {
		t.Fatalf("mcp tools=%v", rep.MCPToolNames)
	}
}

func TestTelemetryHashNoRawTask(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	lifecycle.ResetMemoryEvents()
	lifecycle.EnsureLayout(home)

	secret := "do not store this exact task phrase"
	lifecycle.RecordEvent("pack", map[string]any{
		"task_hash":      lifecycle.HashTask(secret),
		"selected_count": 1,
		"selected_names": []string{"alpha"},
	}, "test", home)

	raw, err := os.ReadFile(filepath.Join(home, "telemetry", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatal("secret leaked")
	}
	if !strings.Contains(string(raw), `"type":"pack"`) {
		t.Fatalf("missing pack event: %s", raw)
	}
}
