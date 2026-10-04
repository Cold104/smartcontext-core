package core

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Cold104/smartcontext-core/version"
)

// POC eval fixtures mirror Python token_benchmark.STANDARD_TASKS shape
// (gold skill names + distractors) at a smaller POC scale.
// See: Repos/smartcontext/src/smartcontext/core/token_benchmark.py

type evalTask struct {
	id       string
	prompt   string
	gold     []string // expected in top-k (any hit = task hit for hit-rate)
	negative bool     // distractor: gold skills must NOT dominate; optional empty gold
}

func writeEvalCorpus(t *testing.T, root string) {
	t.Helper()
	skills := []struct{ name, desc string }{
		{"hermes-agent-team", "Launch Hermes engineering team Docker persona containers"},
		{"docker-personas", "Docker resource limits for indexer implementer critic docs"},
		{"skill-catalog", "Index Cursor SKILL.md files into a unified skill rule catalog"},
		{"frontmatter-parse", "Parse YAML frontmatter name and description from skills"},
		{"windows-dev", "Windows PowerShell development patterns on F drive for SmartContext"},
		{"powershell-paths", "Quoting and path joining for Python on Windows shells"},
		{"token-pack", "Pack only necessary skill context under a token budget for Cursor"},
		{"context-budget", "Estimate tokens and truncate bodies to cut prompt spend"},
		{"cycle-review", "Hermes cycle evaluation lint smoke review gate before DONE"},
		{"merge-gate", "Merge sandbox to main only after critic and review PASS"},
		{"unrelated-cooking", "Recipes for sourdough bread and kitchen timers"},
		{"unrelated-gardening", "Seasonal planting calendar for tomatoes and herbs"},
		{"unrelated-travel", "Packing lists for international flights and hotels"},
		{"unrelated-finance", "Personal budgeting spreadsheets and tax filing tips"},
		{"unrelated-music", "Guitar chord progressions and practice metronomes"},
	}
	for _, s := range skills {
		writeSkill(t, root, s.name, s.desc, "# "+s.name+"\n\nGuidance body.\n")
	}
}

func evalTasks() []evalTask {
	return []evalTask{
		{
			id:     "hermes-team",
			prompt: "launch Hermes engineering team containers docker persona indexer",
			gold:   []string{"hermes-agent-team", "docker-personas"},
		},
		{
			id:     "skill-index",
			prompt: "index Cursor skills and rules into a catalog with frontmatter",
			gold:   []string{"skill-catalog", "frontmatter-parse"},
		},
		{
			id:     "windows-smartcontext",
			prompt: "Windows PowerShell SmartContext development on F drive paths",
			gold:   []string{"windows-dev", "powershell-paths"},
		},
		{
			id:     "token-ingest",
			prompt: "reduce token usage when ingesting code into Cursor chat context",
			gold:   []string{"token-pack", "context-budget"},
		},
		{
			id:     "cycle-review",
			prompt: "evaluate Hermes cycle with lint smoke review before DONE merge",
			gold:   []string{"cycle-review", "merge-gate"},
		},
		{
			id:       "distractor-cooking",
			prompt:   "sourdough bread recipes and kitchen timer techniques",
			gold:     []string{"unrelated-cooking"},
			negative: false, // positive for cooking skill; also checks non-dev skills can win
		},
		{
			id:       "negative-astronomy",
			prompt:   "orbital mechanics of exoplanet transit photometry",
			gold:     nil, // no gold — selection may be non-empty (score-0 discovery order) but must not hit windows-dev as #1 with score>0
			negative: true,
		},
	}
}

func hitAny(selected []string, gold []string) bool {
	if len(gold) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(selected))
	for _, n := range selected {
		set[strings.ToLower(n)] = struct{}{}
	}
	for _, g := range gold {
		if _, ok := set[strings.ToLower(g)]; ok {
			return true
		}
	}
	return false
}

func recallAtK(selected []string, gold []string, k int) float64 {
	if len(gold) == 0 {
		return 0
	}
	top := selected
	if len(top) > k {
		top = top[:k]
	}
	set := make(map[string]struct{}, len(top))
	for _, n := range top {
		set[strings.ToLower(n)] = struct{}{}
	}
	hits := 0
	for _, g := range gold {
		if _, ok := set[strings.ToLower(g)]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(gold))
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func TestPocEvalDiscoverFixture(t *testing.T) {
	root := t.TempDir()
	writeEvalCorpus(t, root)

	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")

	const iters = 8
	durs := make([]float64, 0, iters)
	var last Catalog
	for i := 0; i < iters; i++ {
		start := time.Now()
		last = BuildCatalog(nil)
		durs = append(durs, float64(time.Since(start).Microseconds())/1000.0)
	}
	sort.Float64s(durs)

	if len(last.Entries) != 15 {
		t.Fatalf("discover count=%d want 15", len(last.Entries))
	}
	names := make([]string, 0, len(last.Entries))
	for _, e := range last.Entries {
		names = append(names, e.Name)
	}
	for _, want := range []string{"windows-dev", "hermes-agent-team", "unrelated-cooking"} {
		if !slices.Contains(names, want) {
			t.Fatalf("missing expected skill %q in %v", want, names)
		}
	}
	t.Logf("discover_fixture_ms p50=%.3f avg=%.3f n=%d catalog=%d", percentile(durs, 50), mean(durs), iters, len(last.Entries))
}

func TestPocEvalPackAccuracyAndLatency(t *testing.T) {
	root := t.TempDir()
	writeEvalCorpus(t, root)

	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")

	const packIters = 8
	const topN = 5
	tasks := evalTasks()

	var hits, scoredTasks int
	var packMS []float64

	for _, task := range tasks {
		taskMS := make([]float64, 0, packIters)
		var last PackResult
		for i := 0; i < packIters; i++ {
			start := time.Now()
			last = PackContext(PackRequest{Task: task.prompt, TopN: topN}, version.Version)
			taskMS = append(taskMS, float64(time.Since(start).Microseconds())/1000.0)
		}
		packMS = append(packMS, taskMS...)
		sort.Float64s(taskMS)

		if last.EmptyCatalog {
			t.Fatalf("task %s: empty catalog", task.id)
		}
		if last.SelectedCount < 1 {
			t.Fatalf("task %s: selected_count=%d", task.id, last.SelectedCount)
		}

		if task.negative {
			// Negative: no gold. Fail if a strongly scored windows-dev/token-pack leads.
			if len(last.Items) > 0 && last.Items[0].Score > 0 {
				lead := strings.ToLower(last.Items[0].Name)
				if lead == "windows-dev" || lead == "token-pack" || lead == "hermes-agent-team" {
					t.Fatalf("negative task %s unexpectedly led with %s score=%.0f", task.id, last.Items[0].Name, last.Items[0].Score)
				}
			}
			t.Logf("task=%s negative ok selected=%v p50_ms=%.3f", task.id, last.SelectedNames, percentile(taskMS, 50))
			continue
		}

		scoredTasks++
		hit := hitAny(last.SelectedNames, task.gold)
		if hit {
			hits++
		}
		rec := recallAtK(last.SelectedNames, task.gold, topN)
		t.Logf("task=%s hit=%v recall@%d=%.2f selected=%v gold=%v p50_ms=%.3f",
			task.id, hit, topN, rec, last.SelectedNames, task.gold, percentile(taskMS, 50))
		if !hit {
			t.Errorf("task %s: expected one of %v in selected %v", task.id, task.gold, last.SelectedNames)
		}
	}

	sort.Float64s(packMS)
	hitRate := 0.0
	if scoredTasks > 0 {
		hitRate = float64(hits) / float64(scoredTasks)
	}
	t.Logf("pack_ms p50=%.3f avg=%.3f n=%d hit_rate=%.2f (%d/%d)",
		percentile(packMS, 50), mean(packMS), len(packMS), hitRate, hits, scoredTasks)
	if hitRate < 0.8 {
		t.Fatalf("hit_rate=%.2f below POC floor 0.80", hitRate)
	}
}

func TestPocEvalRepackAfterCatalogChange(t *testing.T) {
	// Stand-in for "skill refinement": Go has no refine loop; measure re-pack
	// after a catalog mutation (new SKILL.md appears).
	root := t.TempDir()
	writeSkill(t, root, "alpha-skill", "Alpha patterns for packing", "# Alpha")

	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")

	before := PackContext(PackRequest{Task: "brand new omega workflow", TopN: 3}, version.Version)
	if before.SelectedCount < 1 {
		t.Fatal("expected non-empty pack before mutation")
	}
	if hitAny(before.SelectedNames, []string{"omega-skill"}) {
		t.Fatal("omega-skill should not exist yet")
	}

	writeSkill(t, root, "omega-skill", "Brand new omega workflow patterns for packing", "# Omega")

	start := time.Now()
	after := PackContext(PackRequest{Task: "brand new omega workflow", TopN: 3}, version.Version)
	ms := float64(time.Since(start).Microseconds()) / 1000.0

	if !hitAny(after.SelectedNames, []string{"omega-skill"}) {
		t.Fatalf("after catalog change expected omega-skill in %v", after.SelectedNames)
	}
	t.Logf("repack_after_catalog_change_ms=%.3f selected=%v (refine loop N/A in Go POC)", ms, after.SelectedNames)
}

func TestPocEvalDefaultRootsDiscoverSmoke(t *testing.T) {
	// Informational: default roots on this machine (may be empty or large).
	// Does not fail on empty — documents live root_count / catalog size.
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	_ = os.Unsetenv("SMARTCONTEXT_SKILL_ROOTS")

	start := time.Now()
	cat := BuildCatalog(nil)
	ms := float64(time.Since(start).Microseconds()) / 1000.0
	t.Logf("default_roots discover_ms=%.3f root_count=%d catalog_size=%d", ms, len(cat.Roots), len(cat.Entries))

	// If cwd nested skills exist under go/, they may appear; just ensure no panic.
	_ = filepath.Base(t.TempDir())
}
