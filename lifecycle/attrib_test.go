package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/version"
)

// TestPackLatencyAttribution measures pack-only vs pack+hash vs pack+offload-load
// p50 deltas (lifecycle hot-path costs). Numbers are logged for LATENCY.md;
// this test does not gate absolute ceilings (see core.TestLatencyBudgets).
func TestPackLatencyAttribution(t *testing.T) {
	ResetStablePrefixHashCache()
	ResetOffloadPointerCache()

	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "0")

	root := prepareAttribCatalog(t, 20)
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	req := core.PackRequest{
		Task: "hermes docker sandbox skill catalog index",
		TopN: 5, Roots: []string{root},
	}

	const samples = 40
	// Warm core pack + caches
	warm := core.PackContext(req, version.Version)
	if warm.EmptyCatalog || warm.MarkdownStable == "" {
		t.Fatalf("warm pack empty: catalog=%v stable_len=%d", warm.EmptyCatalog, len(warm.MarkdownStable))
	}
	_ = HashStablePrefix(warm.MarkdownStable)
	_ = LoadOffloadPointers(home) // miss → cache empty

	packOnly := make([]int64, 0, samples)
	packHashCold := make([]int64, 0, samples)
	packHashMemo := make([]int64, 0, samples)
	packOffloadCold := make([]int64, 0, samples)
	packOffloadCached := make([]int64, 0, samples)

	stable := warm.MarkdownStable
	const microIters = 2000 // amplify sub-ms ops past Windows timer quantum

	hashColdNS := make([]int64, 0, samples)
	hashMemoNS := make([]int64, 0, samples)
	offloadColdNS := make([]int64, 0, samples)
	offloadCachedNS := make([]int64, 0, samples)
	ensureLayoutNS := make([]int64, 0, samples)

	for i := 0; i < samples; i++ {
		start := time.Now()
		_ = core.PackContext(req, version.Version)
		packOnly = append(packOnly, time.Since(start).Nanoseconds())

		// pack + cold hash (full SHA each time via HashTask)
		start = time.Now()
		_ = core.PackContext(req, version.Version)
		_ = HashTask(stable)
		packHashCold = append(packHashCold, time.Since(start).Nanoseconds())

		_ = HashStablePrefix(stable) // prime memo
		start = time.Now()
		_ = core.PackContext(req, version.Version)
		_ = HashStablePrefix(stable)
		packHashMemo = append(packHashMemo, time.Since(start).Nanoseconds())

		ResetOffloadPointerCache()
		start = time.Now()
		_ = core.PackContext(req, version.Version)
		_ = LoadOffloadPointers(home)
		packOffloadCold = append(packOffloadCold, time.Since(start).Nanoseconds())

		_ = LoadOffloadPointers(home) // ensure cached
		start = time.Now()
		_ = core.PackContext(req, version.Version)
		_ = LoadOffloadPointers(home)
		packOffloadCached = append(packOffloadCached, time.Since(start).Nanoseconds())

		start = time.Now()
		for j := 0; j < microIters; j++ {
			_ = HashTask(stable)
		}
		hashColdNS = append(hashColdNS, time.Since(start).Nanoseconds()/microIters)

		_ = HashStablePrefix(stable)
		start = time.Now()
		for j := 0; j < microIters; j++ {
			_ = HashStablePrefix(stable)
		}
		hashMemoNS = append(hashMemoNS, time.Since(start).Nanoseconds()/microIters)

		start = time.Now()
		for j := 0; j < microIters; j++ {
			ResetOffloadPointerCache()
			_ = LoadOffloadPointers(home)
		}
		offloadColdNS = append(offloadColdNS, time.Since(start).Nanoseconds()/microIters)

		_ = LoadOffloadPointers(home)
		start = time.Now()
		for j := 0; j < microIters; j++ {
			_ = LoadOffloadPointers(home)
		}
		offloadCachedNS = append(offloadCachedNS, time.Since(start).Nanoseconds()/microIters)

		// Pre-fix recall path called EnsureLayout (many MkdirAll) every pack.
		const layoutIters = 200
		start = time.Now()
		for j := 0; j < layoutIters; j++ {
			_ = EnsureLayout(home)
		}
		ensureLayoutNS = append(ensureLayoutNS, time.Since(start).Nanoseconds()/layoutIters)
	}

	p50ms := func(ns []int64) float64 { return float64(percentileNS(ns, 50)) / 1e6 }
	p50us := func(ns []int64) float64 { return float64(percentileNS(ns, 50)) / 1e3 }
	packP50 := p50ms(packOnly)
	t.Logf("latency_attrib pack_only_p50_ms=%.3f pack+hash_cold_delta_ms=%.3f pack+hash_memo_delta_ms=%.3f pack+offload_cold_delta_ms=%.3f pack+offload_cached_delta_ms=%.3f hash_cold_p50_us=%.2f hash_memo_p50_us=%.2f offload_stat_miss_p50_us=%.2f offload_cached_p50_us=%.2f ensure_layout_p50_us=%.2f stable_bytes=%d",
		packP50,
		p50ms(packHashCold)-packP50,
		p50ms(packHashMemo)-packP50,
		p50ms(packOffloadCold)-packP50,
		p50ms(packOffloadCached)-packP50,
		p50us(hashColdNS), p50us(hashMemoNS),
		p50us(offloadColdNS), p50us(offloadCachedNS),
		p50us(ensureLayoutNS),
		len(stable))
}

func TestHashStablePrefixMemo(t *testing.T) {
	ResetStablePrefixHashCache()
	a := HashStablePrefix("## Skills\nalpha\n")
	b := HashStablePrefix("## Skills\nalpha\n")
	c := HashStablePrefix("## Skills\nbeta\n")
	if a == "" || a != b {
		t.Fatalf("memo reuse failed: %q vs %q", a, b)
	}
	if c == "" || c == a {
		t.Fatalf("changed stable should rehash: a=%q c=%q", a, c)
	}
	if HashStablePrefix("") != "" {
		t.Fatal("empty stable → empty hash")
	}
}

func TestLoadOffloadPointersCacheMissingAndUnchanged(t *testing.T) {
	ResetOffloadPointerCache()
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)

	if got := LoadOffloadPointers(home); got != nil {
		t.Fatalf("missing file should yield nil, got %+v", got)
	}
	// Second miss must hit cache (still nil) without creating layout.
	if _, err := os.Stat(filepath.Join(home, "session")); !os.IsNotExist(err) {
		t.Fatalf("read path must not EnsureLayout; session dir err=%v", err)
	}
	if got := LoadOffloadPointers(home); got != nil {
		t.Fatalf("cached miss should stay nil, got %+v", got)
	}

	EnsureLayout(home)
	ptr := OffloadPointer{Path: filepath.Join(home, "offload", "x.txt"), Preview: "hi", Bytes: 2}
	if err := SaveOffloadPointers(home, ptr); err != nil {
		t.Fatal(err)
	}
	first := LoadOffloadPointers(home)
	if len(first) != 1 || first[0].Path != ptr.Path {
		t.Fatalf("loaded=%+v", first)
	}
	second := LoadOffloadPointers(home)
	if len(second) != 1 || second[0].Path != first[0].Path {
		t.Fatalf("cached load mismatch: %+v vs %+v", second, first)
	}

	path := offloadSessionPath(home)
	// Force mtime/size change and ensure cache refreshes.
	time.Sleep(15 * time.Millisecond)
	st := OffloadSessionState{
		Pointers: []OffloadPointer{{Path: filepath.Join(home, "offload", "y.txt"), Bytes: 9}},
	}
	payload, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	third := LoadOffloadPointers(home)
	if len(third) != 1 || third[0].Path == ptr.Path {
		t.Fatalf("expected refresh after mtime change, got %+v", third)
	}
}

func prepareAttribCatalog(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("bench-skill-%03d", i)
		desc := fmt.Sprintf("Synthetic skill %s for hermes docker sandbox catalog packing", name)
		writeLCSkill(t, root, name, desc, "# "+name+"\n\nBody text for latency benches.\n")
	}
	return root
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
