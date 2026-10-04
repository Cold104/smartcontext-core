// POC eval driver: discover / pack latency + selection accuracy + functionality matrix.
// Usage (from go/): go run ./scripts/eval_poc.go
//
// Writes docs/POC_EVAL_RESULTS.md with measured numbers (not invented).
//
//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cold104/smartcontext-core/core"
	scmcp "github.com/Cold104/smartcontext-core/mcp"
	"github.com/Cold104/smartcontext-core/version"
)

const packIters = 8
const topN = 5

type evalTask struct {
	ID       string
	Prompt   string
	Gold     []string
	Negative bool
}

type taskResult struct {
	ID            string   `json:"id"`
	Prompt        string   `json:"prompt"`
	Gold          []string `json:"gold"`
	Negative      bool     `json:"negative"`
	Selected      []string `json:"selected"`
	Hit           bool     `json:"hit"`
	RecallAtK     float64  `json:"recall_at_k"`
	LeadScore     float64  `json:"lead_score"`
	LatencyMSP50  float64  `json:"latency_ms_p50"`
	LatencyMSAvg  float64  `json:"latency_ms_avg"`
	SelectedCount int      `json:"selected_count"`
	CatalogSize   int      `json:"catalog_size_hint"`
	RootCount     int      `json:"root_count"`
	NegativeOK    bool     `json:"negative_ok,omitempty"`
}

type report struct {
	Version              string            `json:"version"`
	CapturedAt           string            `json:"captured_at"`
	HostNote             string            `json:"host_note"`
	FixtureSkillCount    int               `json:"fixture_skill_count"`
	DiscoverFixtureMSP50 float64           `json:"discover_fixture_ms_p50"`
	DiscoverFixtureMSAvg float64           `json:"discover_fixture_ms_avg"`
	CatalogBuildMSP50    float64           `json:"catalog_build_ms_p50"`
	CatalogBuildMSAvg    float64           `json:"catalog_build_ms_avg"`
	PackMSP50            float64           `json:"pack_ms_p50"`
	PackMSAvg            float64           `json:"pack_ms_avg"`
	PackSamples          int               `json:"pack_samples"`
	HitRate              float64           `json:"selection_hit_rate"`
	Hits                 int               `json:"hits"`
	ScoredTasks          int               `json:"scored_tasks"`
	MeanRecallAtK        float64           `json:"mean_recall_at_k"`
	DefaultRootsDiscover float64           `json:"default_roots_discover_ms"`
	DefaultRootCount     int               `json:"default_root_count"`
	DefaultCatalogSize   int               `json:"default_catalog_size"`
	RepackAfterChangeMS  float64           `json:"repack_after_catalog_change_ms"`
	MCPToolNames         []string          `json:"mcp_tool_names"`
	MCPHasPack           bool              `json:"mcp_has_pack_context"`
	MCPHasCreate         bool              `json:"mcp_has_create_or_factory"`
	MCPHasRefine         bool              `json:"mcp_has_refine"`
	MCPPackMSP50         float64           `json:"mcp_pack_ms_p50"`
	MCPPackMSAvg         float64           `json:"mcp_pack_ms_avg"`
	Tasks                []taskResult      `json:"tasks"`
	Functionality        map[string]string `json:"functionality"`
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fatalf("getcwd: %v", err)
	}
	// Allow running from repo root or go/
	goRoot := wd
	if filepath.Base(wd) != "go" {
		cand := filepath.Join(wd, "go")
		if st, e := os.Stat(cand); e == nil && st.IsDir() {
			goRoot = cand
		}
	}

	fixtureRoot, err := os.MkdirTemp("", "sc-poc-eval-*")
	if err != nil {
		fatalf("temp: %v", err)
	}
	defer os.RemoveAll(fixtureRoot)

	skillCount := writeCorpus(fixtureRoot)
	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", fixtureRoot)
	_ = os.Unsetenv("SMARTCONTEXT_ROOT")
	_ = os.Unsetenv("SMARTCONTEXT_FRIEND_ROOT")

	rep := report{
		Version:           version.Version,
		CapturedAt:        time.Now().UTC().Format(time.RFC3339),
		HostNote:          "Windows local POC eval; in-process core + in-memory MCP",
		FixtureSkillCount: skillCount,
		Functionality:     map[string]string{},
	}

	// --- Discover / catalog build ---
	discoverMS := make([]float64, 0, packIters)
	var lastCat core.Catalog
	for i := 0; i < packIters; i++ {
		start := time.Now()
		lastCat = core.BuildCatalog(nil)
		discoverMS = append(discoverMS, msSince(start))
	}
	sort.Float64s(discoverMS)
	rep.DiscoverFixtureMSP50 = percentile(discoverMS, 50)
	rep.DiscoverFixtureMSAvg = mean(discoverMS)
	rep.CatalogBuildMSP50 = rep.DiscoverFixtureMSP50
	rep.CatalogBuildMSAvg = rep.DiscoverFixtureMSAvg
	if len(lastCat.Entries) != skillCount {
		fatalf("discover count=%d want %d", len(lastCat.Entries), skillCount)
	}

	// --- Pack accuracy + latency ---
	tasks := evalTasks()
	var allPackMS []float64
	var hits, scored int
	var recallSum float64
	for _, task := range tasks {
		taskMS := make([]float64, 0, packIters)
		var last core.PackResult
		for i := 0; i < packIters; i++ {
			start := time.Now()
			last = core.PackContext(core.PackRequest{Task: task.Prompt, TopN: topN}, version.Version)
			taskMS = append(taskMS, msSince(start))
		}
		allPackMS = append(allPackMS, taskMS...)
		sort.Float64s(taskMS)

		tr := taskResult{
			ID:            task.ID,
			Prompt:        task.Prompt,
			Gold:          task.Gold,
			Negative:      task.Negative,
			Selected:      last.SelectedNames,
			SelectedCount: last.SelectedCount,
			RootCount:     last.RootCount,
			LatencyMSP50:  percentile(taskMS, 50),
			LatencyMSAvg:  mean(taskMS),
		}
		if len(last.Items) > 0 {
			tr.LeadScore = last.Items[0].Score
		}
		// catalog size from empty_catalog / selected path — use lastCat
		tr.CatalogSize = len(lastCat.Entries)

		if task.Negative {
			ok := true
			if len(last.Items) > 0 && last.Items[0].Score > 0 {
				lead := strings.ToLower(last.Items[0].Name)
				if lead == "windows-dev" || lead == "token-pack" || lead == "hermes-agent-team" {
					ok = false
				}
			}
			tr.NegativeOK = ok
			tr.Hit = ok
			rep.Tasks = append(rep.Tasks, tr)
			continue
		}

		scored++
		tr.Hit = hitAny(last.SelectedNames, task.Gold)
		tr.RecallAtK = recallAtK(last.SelectedNames, task.Gold, topN)
		recallSum += tr.RecallAtK
		if tr.Hit {
			hits++
		}
		rep.Tasks = append(rep.Tasks, tr)
	}
	sort.Float64s(allPackMS)
	rep.PackMSP50 = percentile(allPackMS, 50)
	rep.PackMSAvg = mean(allPackMS)
	rep.PackSamples = len(allPackMS)
	rep.Hits = hits
	rep.ScoredTasks = scored
	if scored > 0 {
		rep.HitRate = float64(hits) / float64(scored)
		rep.MeanRecallAtK = recallSum / float64(scored)
	}

	// --- Default roots (live machine) ---
	_ = os.Unsetenv("SMARTCONTEXT_SKILL_ROOTS")
	start := time.Now()
	defCat := core.BuildCatalog(nil)
	rep.DefaultRootsDiscover = msSince(start)
	rep.DefaultRootCount = len(defCat.Roots)
	rep.DefaultCatalogSize = len(defCat.Entries)

	// Restore fixture for remaining checks
	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", fixtureRoot)

	// --- Refinement stand-in: re-pack after catalog change ---
	omegaDir := filepath.Join(fixtureRoot, "omega-skill")
	_ = os.MkdirAll(omegaDir, 0o755)
	_ = os.WriteFile(filepath.Join(omegaDir, "SKILL.md"), []byte(
		"---\nname: omega-skill\ndescription: Brand new omega workflow patterns for packing\n---\n\n# Omega\n",
	), 0o644)
	start = time.Now()
	after := core.PackContext(core.PackRequest{Task: "brand new omega workflow", TopN: 3}, version.Version)
	rep.RepackAfterChangeMS = msSince(start)
	repackHit := hitAny(after.SelectedNames, []string{"omega-skill"})

	// --- MCP tools probe (creation / refine status) ---
	mcpTools, mcpPackP50, mcpPackAvg := probeMCP(fixtureRoot)
	rep.MCPToolNames = mcpTools
	for _, name := range mcpTools {
		switch name {
		case "pack_context":
			rep.MCPHasPack = true
		case "skills_factory_apply", "factory_apply", "create_skill", "skills_create", "skill_create":
			rep.MCPHasCreate = true
		case "skills_refine", "refine_scan", "refine_list", "session_start":
			rep.MCPHasRefine = true
		}
	}
	rep.MCPPackMSP50 = mcpPackP50
	rep.MCPPackMSAvg = mcpPackAvg

	// Functionality matrix
	rep.Functionality["discovery"] = "pass"
	if rep.HitRate >= 0.8 && rep.MCPHasPack {
		rep.Functionality["packing"] = "pass"
	} else if rep.MCPHasPack {
		rep.Functionality["packing"] = "partial"
	} else {
		rep.Functionality["packing"] = "fail"
	}
	if repackHit {
		rep.Functionality["refinement"] = "partial" // re-pack after change only; no refine loop
	} else {
		rep.Functionality["refinement"] = "N/A"
	}
	if rep.MCPHasCreate {
		rep.Functionality["creation"] = "pass"
	} else {
		rep.Functionality["creation"] = "N/A" // NOT IN POC
	}

	outMD := filepath.Join(goRoot, "docs", "POC_EVAL_RESULTS.md")
	outJSON := filepath.Join(goRoot, "docs", "poc_eval_results.json")
	if err := os.MkdirAll(filepath.Dir(outMD), 0o755); err != nil {
		fatalf("mkdir docs: %v", err)
	}
	b, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(outJSON, b, 0o644); err != nil {
		fatalf("write json: %v", err)
	}
	if err := os.WriteFile(outMD, []byte(renderMarkdown(rep, repackHit)), 0o644); err != nil {
		fatalf("write md: %v", err)
	}

	fmt.Printf("Wrote %s\n", outMD)
	fmt.Printf("Wrote %s\n", outJSON)
	fmt.Printf("hit_rate=%.2f pack_ms p50=%.3f avg=%.3f discovery=%s packing=%s refinement=%s creation=%s\n",
		rep.HitRate, rep.PackMSP50, rep.PackMSAvg,
		rep.Functionality["discovery"], rep.Functionality["packing"],
		rep.Functionality["refinement"], rep.Functionality["creation"])
}

func probeMCP(fixtureRoot string) (tools []string, packP50, packAvg float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", fixtureRoot)

	server := scmcp.NewServer()
	client := mcp.NewClient(&mcp.Implementation{Name: "poc-eval", Version: "0.0.1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp server connect: %v\n", err)
		return nil, 0, 0
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp client connect: %v\n", err)
		return nil, 0, 0
	}
	defer cs.Close()

	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tools/list: %v\n", err)
		return nil, 0, 0
	}
	for _, t := range listed.Tools {
		tools = append(tools, t.Name)
	}

	durs := make([]float64, 0, packIters)
	for i := 0; i < packIters; i++ {
		start := time.Now()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{
			Name: "pack_context",
			Arguments: map[string]any{
				"task":  "Windows PowerShell SmartContext development on F drive paths",
				"top_n": topN,
			},
		})
		_ = res
		if err != nil {
			fmt.Fprintf(os.Stderr, "mcp pack_context: %v\n", err)
			continue
		}
		durs = append(durs, msSince(start))
	}
	if len(durs) == 0 {
		return tools, 0, 0
	}
	sort.Float64s(durs)
	return tools, percentile(durs, 50), mean(durs)
}

func writeCorpus(root string) int {
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
		dir := filepath.Join(root, s.name)
		_ = os.MkdirAll(dir, 0o755)
		body := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n# %s\n\nGuidance body.\n", s.name, s.desc, s.name)
		_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644)
	}
	return len(skills)
}

func evalTasks() []evalTask {
	return []evalTask{
		{ID: "hermes-team", Prompt: "launch Hermes engineering team containers docker persona indexer", Gold: []string{"hermes-agent-team", "docker-personas"}},
		{ID: "skill-index", Prompt: "index Cursor skills and rules into a catalog with frontmatter", Gold: []string{"skill-catalog", "frontmatter-parse"}},
		{ID: "windows-smartcontext", Prompt: "Windows PowerShell SmartContext development on F drive paths", Gold: []string{"windows-dev", "powershell-paths"}},
		{ID: "token-ingest", Prompt: "reduce token usage when ingesting code into Cursor chat context", Gold: []string{"token-pack", "context-budget"}},
		{ID: "cycle-review", Prompt: "evaluate Hermes cycle with lint smoke review before DONE merge", Gold: []string{"cycle-review", "merge-gate"}},
		{ID: "distractor-cooking", Prompt: "sourdough bread recipes and kitchen timer techniques", Gold: []string{"unrelated-cooking"}},
		{ID: "negative-astronomy", Prompt: "orbital mechanics of exoplanet transit photometry", Negative: true},
	}
}

func hitAny(selected, gold []string) bool {
	if len(gold) == 0 {
		return false
	}
	set := map[string]struct{}{}
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

func recallAtK(selected, gold []string, k int) float64 {
	if len(gold) == 0 {
		return 0
	}
	top := selected
	if len(top) > k {
		top = top[:k]
	}
	set := map[string]struct{}{}
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

func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
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

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func renderMarkdown(r report, repackHit bool) string {
	var b strings.Builder
	b.WriteString("# SmartContext 1.1 Go POC evaluation results\n\n")
	b.WriteString(fmt.Sprintf("**Version:** `%s`  \n", r.Version))
	b.WriteString(fmt.Sprintf("**Captured:** %s (UTC)  \n", r.CapturedAt))
	b.WriteString(fmt.Sprintf("**Host note:** %s  \n", r.HostNote))
	b.WriteString("**Publication gate:** CLOSED — local POC QA only; no Python parity claim.\n\n")

	b.WriteString("## Prior eval methodology (reused shape)\n\n")
	b.WriteString("Modeled on existing SmartContext evidence / harness patterns (not Lathe stack-eval LLM ingest):\n\n")
	b.WriteString("- **Token / role benchmarks** — gold skill lists per task prompt, recall@k / hit-rate (`Repos/smartcontext/src/smartcontext/core/token_benchmark.py`, `role_token_benchmark.py`).\n")
	b.WriteString("- **Case studies A/B** — cold empty catalog vs pre-seeded roots; pack + lifecycle refine/factory (`docs/CASE_STUDIES_SKILLS.md`, `tests/test_case_studies_skills.py`).\n")
	b.WriteString("- **Eval hygiene** — isolated `SMARTCONTEXT_SKILL_ROOTS` / home; do not mix warm-home numbers (`docs/EVAL_HYGIENE.md`).\n")
	b.WriteString("- **Latency budget docs** — local pack wall-clock, no pre-flight LLM (`docs/LATENCY.md`); Python Core micro-bench under `Docs/EVIDENCE/`.\n")
	b.WriteString("- **Diehard ladder** — held-out success rate under skill-gated solver (Temp stamp pointer in `docs/DIEHARD_EVAL.md`) — **not** re-run here.\n")
	b.WriteString("- **Alpha / Lathe stack-eval** — ingest latency + token reduction (`Docs/EVIDENCE/ALPHA_EVAL_v0.1.0-alpha.3.md`) — methodology reference only; out of scope for Go pack POC.\n\n")

	b.WriteString("## Headline metrics\n\n")
	b.WriteString("| Metric | Value |\n|--------|------:|\n")
	b.WriteString(fmt.Sprintf("| Selection hit-rate (scored tasks) | **%.0f%%** (%d/%d) |\n", r.HitRate*100, r.Hits, r.ScoredTasks))
	b.WriteString(fmt.Sprintf("| Mean recall@%d | %.2f |\n", topN, r.MeanRecallAtK))
	b.WriteString(fmt.Sprintf("| Pack latency p50 (in-process) | **%.3f ms** |\n", r.PackMSP50))
	b.WriteString(fmt.Sprintf("| Pack latency avg (in-process) | %.3f ms |\n", r.PackMSAvg))
	b.WriteString(fmt.Sprintf("| Pack samples | %d (%d iters × tasks) |\n", r.PackSamples, packIters))
	b.WriteString(fmt.Sprintf("| Discover/catalog p50 (fixture, %d skills) | %.3f ms |\n", r.FixtureSkillCount, r.DiscoverFixtureMSP50))
	b.WriteString(fmt.Sprintf("| Discover/catalog avg | %.3f ms |\n", r.DiscoverFixtureMSAvg))
	b.WriteString(fmt.Sprintf("| MCP pack_context p50 | %.3f ms |\n", r.MCPPackMSP50))
	b.WriteString(fmt.Sprintf("| MCP pack_context avg | %.3f ms |\n", r.MCPPackMSAvg))
	b.WriteString(fmt.Sprintf("| Default-roots discover (live) | %.3f ms · roots=%d · catalog=%d |\n", r.DefaultRootsDiscover, r.DefaultRootCount, r.DefaultCatalogSize))
	b.WriteString(fmt.Sprintf("| Re-pack after catalog change | %.3f ms |\n\n", r.RepackAfterChangeMS))

	b.WriteString("## Functionality status matrix\n\n")
	b.WriteString("| Capability | Status | Evidence |\n|------------|--------|----------|\n")
	b.WriteString(fmt.Sprintf("| Skill discovery | **%s** | Fixture discover found %d/%d expected SKILL.md; default-roots live catalog=%d |\n",
		r.Functionality["discovery"], r.FixtureSkillCount, r.FixtureSkillCount, r.DefaultCatalogSize))
	b.WriteString(fmt.Sprintf("| Skill packing / selection | **%s** | MCP tool `pack_context` present; hit-rate %.0f%%; simple token rank |\n",
		r.Functionality["packing"], r.HitRate*100))
	refineNote := "No refine loop / proposal queue in Go; measured re-pack after new SKILL.md only"
	if repackHit {
		refineNote += " (omega-skill selected after mutation)"
	} else {
		refineNote += " (mutation re-pack miss)"
	}
	b.WriteString(fmt.Sprintf("| Skill refinement | **%s** | %s |\n", r.Functionality["refinement"], refineNote))
	createNote := "NOT IN POC — MCP tools/list has no factory/create tool; lifecycle factory is stub-only"
	if r.MCPHasCreate {
		createNote = "create/factory tool present in MCP"
	}
	b.WriteString(fmt.Sprintf("| Skill creation / factory | **%s** | %s |\n\n", r.Functionality["creation"], createNote))

	b.WriteString("### MCP tools observed\n\n")
	if len(r.MCPToolNames) == 0 {
		b.WriteString("(none / probe failed)\n\n")
	} else {
		for _, n := range r.MCPToolNames {
			b.WriteString(fmt.Sprintf("- `%s`\n", n))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Latency table (per task, in-process pack)\n\n")
	b.WriteString("| Task ID | p50 ms | avg ms | selected_count | root_count |\n|---------|-------:|-------:|---------------:|-----------:|\n")
	for _, t := range r.Tasks {
		b.WriteString(fmt.Sprintf("| %s | %.3f | %.3f | %d | %d |\n", t.ID, t.LatencyMSP50, t.LatencyMSAvg, t.SelectedCount, t.RootCount))
	}
	b.WriteString("\n")

	b.WriteString("## Accuracy table (selection)\n\n")
	b.WriteString("| Task ID | Hit | recall@5 | Gold | Selected | Notes |\n|---------|:---:|---------:|------|----------|-------|\n")
	for _, t := range r.Tasks {
		notes := ""
		if t.Negative {
			if t.NegativeOK {
				notes = "negative OK (no strong false lead on windows/token/hermes)"
			} else {
				notes = "negative FAIL"
			}
		}
		hit := "miss"
		if t.Hit {
			hit = "hit"
		}
		if t.Negative {
			hit = map[bool]string{true: "ok", false: "fail"}[t.NegativeOK]
		}
		gold := "—"
		if len(t.Gold) > 0 {
			gold = strings.Join(t.Gold, ", ")
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %.2f | %s | %s | %s |\n",
			t.ID, hit, t.RecallAtK, gold, strings.Join(t.Selected, ", "), notes))
	}
	b.WriteString("\n")

	b.WriteString("## Gaps vs prior Python eval\n\n")
	b.WriteString("| Python capability | Go POC |\n|--------------------|--------|\n")
	b.WriteString("| IDF / fuzzy rank + triage pin/deprecate | Thin IDF + name boost shipped; fuzzy/triage still deferred |\n")
	b.WriteString("| Token budget + body scrub | Budget dial + pack-time secret scrub / injection flags |\n")
	b.WriteString("| Lifecycle refine proposals + factory apply + consent | **Not in POC** |\n")
	b.WriteString("| Session start / Canvas / telemetry | Stubs only |\n")
	b.WriteString("| Token-reduction % vs naïve corpus | Naive tokens + budget_pct on pack result |\n")
	b.WriteString("| Diehard held-out ladder / SWE-bench | Not run |\n")
	b.WriteString("| Lathe stack-eval ingest | Out of scope |\n")
	b.WriteString("| Core micro-bench Budget.for_ci gates | Ported — go/docs/LATENCY.md |\n\n")

	b.WriteString("## How to reproduce\n\n")
	b.WriteString("```powershell\n")
	b.WriteString("Set-Location -Path 'F:\\Projects\\SmartContext\\Repos\\smartcontext\\go'\n")
	b.WriteString("go test ./internal/core/ -run TestPocEval -count=1 -v\n")
	b.WriteString("go test ./...\n")
	b.WriteString("go run ./scripts/eval_poc.go\n")
	b.WriteString("# refreshes docs/POC_EVAL_RESULTS.md + docs/poc_eval_results.json\n")
	b.WriteString("```\n\n")

	b.WriteString("## Recommended next POC hardening (max 3)\n\n")
	b.WriteString("1. **Optional fuzzy alias rank** (stdlib) on top of thin IDF for typo recall — keep weights local.\n")
	b.WriteString("2. **Lifecycle refine stub → real proposal list** (read-only) without factory writes; expose `session_start` or `skills_refine_list` on MCP for parity demos.\n")
	b.WriteString("3. **Telemetry emit** for pack latency / budget_tokens / scrub counts (local JSONL only).\n")

	b.WriteString("\n---\n\n")
	b.WriteString(fmt.Sprintf("_Raw JSON: [`poc_eval_results.json`](poc_eval_results.json) · harness: `scripts/eval_poc.go` · tests: `internal/core/poc_eval_test.go`_\n"))
	return b.String()
}
