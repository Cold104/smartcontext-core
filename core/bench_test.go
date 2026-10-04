package core

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Cold104/smartcontext-core/version"
)

const benchCatalogSize = 20

func prepareBenchCatalog(tb testing.TB, n int) string {
	tb.Helper()
	root := tb.TempDir()
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("bench-skill-%03d", i)
		desc := fmt.Sprintf("Synthetic skill %s for hermes docker sandbox catalog packing", name)
		writeSkill(tb, root, name, desc, "# "+name+"\n\nBody text for latency benches.\n")
	}
	return root
}

func benchPackRequest(root string) PackRequest {
	return PackRequest{
		Task:  "hermes docker sandbox skill catalog index",
		TopN:  5,
		Roots: []string{root},
	}
}

func BenchmarkPackContext(b *testing.B) {
	root := prepareBenchCatalog(b, benchCatalogSize)
	req := benchPackRequest(root)
	// Warm once so first-iteration FS noise is outside the timed loop.
	_ = PackContext(req, version.Version)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = PackContext(req, version.Version)
	}
}

func BenchmarkBuildCatalog(b *testing.B) {
	root := prepareBenchCatalog(b, benchCatalogSize)
	_ = BuildCatalog([]string{root})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildCatalog([]string{root})
	}
}

func BenchmarkDiscoverSkills(b *testing.B) {
	root := prepareBenchCatalog(b, benchCatalogSize)
	roots := ResolveRoots([]string{root})
	_ = DiscoverSkills(roots)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DiscoverSkills(roots)
	}
}

func percentileNS(samples []int64, pct float64) int64 {
	if len(samples) == 0 {
		return 0
	}
	cp := append([]int64(nil), samples...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	idx := int(float64(len(cp)-1) * pct / 100.0)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

// TestLatencyBudgets gates PackContext / BuildCatalog p50 against product or CI budgets.
// See go/docs/LATENCY.md. Env CI / SMARTCONTEXT_BENCH_CI selects 2× CI ceilings.
func TestLatencyBudgets(t *testing.T) {
	budget := ResolveLatencyBudget(false)
	root := prepareBenchCatalog(t, benchCatalogSize)
	req := benchPackRequest(root)

	const samples = 25
	// Warm
	_ = PackContext(req, version.Version)
	_ = BuildCatalog([]string{root})

	packNS := make([]int64, 0, samples)
	buildNS := make([]int64, 0, samples)
	for i := 0; i < samples; i++ {
		start := time.Now()
		res := PackContext(req, version.Version)
		packNS = append(packNS, time.Since(start).Nanoseconds())
		if res.EmptyCatalog {
			t.Fatal("bench fixture produced empty catalog")
		}

		start = time.Now()
		cat := BuildCatalog([]string{root})
		buildNS = append(buildNS, time.Since(start).Nanoseconds())
		if len(cat.Entries) < benchCatalogSize {
			t.Fatalf("catalog size=%d want >= %d", len(cat.Entries), benchCatalogSize)
		}
	}

	packP50 := percentileNS(packNS, 50)
	buildP50 := percentileNS(buildNS, 50)
	t.Logf("latency_budget label=%s pack_p50_ms=%.3f ceil_ms=%.3f build_p50_ms=%.3f ceil_ms=%.3f",
		budget.Label,
		float64(packP50)/1e6, float64(budget.PackNSP50)/1e6,
		float64(buildP50)/1e6, float64(budget.BuildNSP50)/1e6,
	)

	if packP50 > budget.PackNSP50 {
		t.Fatalf("PackContext p50=%d ns (%.3f ms) exceeds %s budget %d ns (%.3f ms); see go/docs/LATENCY.md",
			packP50, float64(packP50)/1e6, budget.Label, budget.PackNSP50, float64(budget.PackNSP50)/1e6)
	}
	if buildP50 > budget.BuildNSP50 {
		t.Fatalf("BuildCatalog p50=%d ns (%.3f ms) exceeds %s budget %d ns (%.3f ms); see go/docs/LATENCY.md",
			buildP50, float64(buildP50)/1e6, budget.Label, budget.BuildNSP50, float64(budget.BuildNSP50)/1e6)
	}
}

func TestResolveLatencyBudgetCI(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("SMARTCONTEXT_BENCH_CI", "")
	p := ResolveLatencyBudget(false)
	if p.Label != "product" {
		t.Fatalf("label=%q", p.Label)
	}
	t.Setenv("SMARTCONTEXT_BENCH_CI", "1")
	c := ResolveLatencyBudget(false)
	if c.Label != "ci" {
		t.Fatalf("ci label=%q", c.Label)
	}
	if c.PackNSP50 != p.PackNSP50*2 || c.BuildNSP50 != p.BuildNSP50*2 {
		t.Fatalf("ci should be 2x product: product=%+v ci=%+v", p, c)
	}
}

func TestLatencyBudgetFixtureIsolated(t *testing.T) {
	// Ensure bench helper does not depend on ambient SMARTCONTEXT_* roots.
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", filepath.Join(t.TempDir(), "absent"))
	root := prepareBenchCatalog(t, 3)
	cat := BuildCatalog([]string{root})
	if len(cat.Entries) != 3 {
		t.Fatalf("entries=%d", len(cat.Entries))
	}
}
