package eval

import (
	"fmt"
	"strings"
)

// RenderMarkdown writes CORPUS_TELEMETRY_EVAL.md content from a measured report.
func RenderMarkdown(r Report) string {
	var b strings.Builder
	b.WriteString("# SmartContext Go corpus + telemetry evaluation\n\n")
	b.WriteString(fmt.Sprintf("**Version:** `%s`  \n", r.Version))
	b.WriteString(fmt.Sprintf("**Captured:** %s (UTC)  \n", r.CapturedAt))
	b.WriteString(fmt.Sprintf("**Host note:** %s  \n", r.HostNote))
	b.WriteString(fmt.Sprintf("**Publication gate:** %s\n\n", r.PublicationGate))

	b.WriteString("## Prior test types (inventory)\n\n")
	b.WriteString("| Type | Python / docs path | Go status in this run |\n")
	b.WriteString("|------|--------------------|-----------------------|\n")
	b.WriteString("| Token benchmark (gold recall@k + naive reduction) | `src/smartcontext/core/token_benchmark.py`, `tests/test_benchmark.py` | **Ran** — token_suite |\n")
	b.WriteString("| Role token benchmark (multi-role noise catalog) | `src/smartcontext/core/role_token_benchmark.py`, `tests/test_role_token_benchmark.py` | **Ran** — role_suite |\n")
	b.WriteString("| Lifecycle telemetry (local JSONL, no raw task) | `src/smartcontext/lifecycle/telemetry.py`, `tests/test_lifecycle.py` | **Ran** — local events.jsonl |\n")
	b.WriteString("| Refund receipt / pack aggregates | `telemetry.build_refund_receipt`, `metrics_summary` | **N/A** — missing on Go |\n")
	b.WriteString("| Case studies A/B cold vs seeded | `docs/CASE_STUDIES_SKILLS.md`, `tests/test_case_studies_skills.py` | **Partial** — empty vs seeded pack only |\n")
	b.WriteString("| Eval hygiene (isolated roots/home) | `docs/EVAL_HYGIENE.md` | **Applied** |\n")
	b.WriteString("| Latency budgets (pack wall-clock) | `go/docs/LATENCY.md`, Core `Budget.for_ci` | **Ported** — native `TestLatencyBudgets` + CI 2× benches |\n")
	b.WriteString("| Diehard held-out ladder | `docs/DIEHARD_EVAL.md` | **N/A** — not run |\n")
	b.WriteString("| Lathe / alpha stack-eval ingest | `docs/evidence/ALPHA_EVAL_*.md`, `Docs/EVIDENCE/` | **N/A** — out of scope |\n")
	b.WriteString("| Go POC fixture eval | `go/scripts/eval_poc.go`, `go/docs/POC_EVAL_RESULTS.md` | Prior sibling harness; this expands corpora |\n\n")

	b.WriteString("## Headline metrics (Go, this run)\n\n")
	b.WriteString("| Metric | Token suite | Role suite |\n|--------|------------:|-----------:|\n")
	b.WriteString(fmt.Sprintf("| Catalog skills | %d | %d |\n", r.TokenSuite.SkillCount, r.RoleSuite.SkillCount))
	b.WriteString(fmt.Sprintf("| Selection hit-rate | %.0f%% (%d/%d) | %.0f%% (%d/%d) |\n",
		r.TokenSuite.HitRate*100, r.TokenSuite.Hits, r.TokenSuite.ScoredTasks,
		r.RoleSuite.HitRate*100, r.RoleSuite.Hits, r.RoleSuite.ScoredTasks))
	b.WriteString(fmt.Sprintf("| Mean recall@%d | %.2f | %.2f |\n", r.TopN, r.TokenSuite.MeanRecallAtK, r.RoleSuite.MeanRecallAtK))
	b.WriteString(fmt.Sprintf("| Median token reduction vs naive | %.1f%% | %.1f%% |\n",
		r.TokenSuite.MedianReduction*100, r.RoleSuite.MedianReduction*100))
	b.WriteString(fmt.Sprintf("| Pack latency p50 / p95 | %.3f / %.3f ms | %.3f / %.3f ms |\n",
		r.TokenSuite.PackMSP50, r.TokenSuite.PackMSP95, r.RoleSuite.PackMSP50, r.RoleSuite.PackMSP95))
	b.WriteString(fmt.Sprintf("| Discover p50 / p95 | %.3f / %.3f ms | %.3f / %.3f ms |\n",
		r.TokenSuite.DiscoverMSP50, r.TokenSuite.DiscoverMSP95, r.RoleSuite.DiscoverMSP50, r.RoleSuite.DiscoverMSP95))
	b.WriteString(fmt.Sprintf("| Gate (reduction>=%.0f%%, recall>=%.0f%%) | %s | %s |\n\n",
		r.MinReduction*100, r.MinRecall*100,
		gateLabel(r.TokenSuite.GateOK), gateLabel(r.RoleSuite.GateOK)))

	b.WriteString("### MCP latency (token fixture)\n\n")
	b.WriteString("| Metric | Value |\n|--------|------:|\n")
	b.WriteString(fmt.Sprintf("| discover_skills p50 / p95 | %.3f / %.3f ms |\n", r.MCPDiscoverMSP50, r.MCPDiscoverMSP95))
	b.WriteString(fmt.Sprintf("| pack_context p50 / p95 | %.3f / %.3f ms |\n", r.MCPPackMSP50, r.MCPPackMSP95))
	b.WriteString(fmt.Sprintf("| Tools | %s |\n\n", strings.Join(r.MCPToolNames, ", ")))

	b.WriteString("### Telemetry (local)\n\n")
	b.WriteString("| Field | Value |\n|-------|------:|\n")
	b.WriteString(fmt.Sprintf("| Lifecycle enabled | %v |\n", r.Telemetry.Enabled))
	b.WriteString(fmt.Sprintf("| Pack events in JSONL | %d |\n", r.Telemetry.PackEventsWritten))
	b.WriteString(fmt.Sprintf("| JSONL lines / bytes | %d / %d |\n", r.Telemetry.JSONLLines, r.Telemetry.JSONLBytes))
	b.WriteString(fmt.Sprintf("| Raw task leaked | %v |\n", r.Telemetry.RawTaskLeaked))
	b.WriteString(fmt.Sprintf("| Run measure log lines | %d |\n", r.Telemetry.RunLogLines))
	b.WriteString(fmt.Sprintf("| Note | %s |\n\n", r.Telemetry.Note))

	b.WriteString("### Case study (thin A/B)\n\n")
	b.WriteString("| Field | Value |\n|-------|------:|\n")
	b.WriteString(fmt.Sprintf("| Cold empty_catalog | %v (selected=%d) |\n", r.CaseStudy.ColdEmptyCatalog, r.CaseStudy.ColdSelected))
	b.WriteString(fmt.Sprintf("| Seeded catalog size | %d |\n", r.CaseStudy.SeededCatalog))
	b.WriteString(fmt.Sprintf("| Seeded pack hit windows-dev | %v |\n", r.CaseStudy.SeededHit))
	b.WriteString(fmt.Sprintf("| Note | %s |\n\n", r.CaseStudy.Note))

	b.WriteString("## Comparison vs cited Python evidence\n\n")
	b.WriteString("Python numbers below are **cited from product docs**, not re-measured in this run.\n\n")
	b.WriteString("| Source | Median reduction | Mean recall@5 |\n|--------|-----------------:|--------------:|\n")
	if mr, ok := r.PythonCited["recorded_median_reduction"].(float64); ok {
		rr, _ := r.PythonCited["recorded_mean_recall_at_5"].(float64)
		b.WriteString(fmt.Sprintf("| Python `docs/requirements.md` (recorded) | %.1f%% | %.1f%% |\n", mr*100, rr*100))
	}
	b.WriteString(fmt.Sprintf("| Go token_suite (this run) | %.1f%% | %.1f%% |\n",
		r.TokenSuite.MedianReduction*100, r.TokenSuite.MeanRecallAtK*100))
	b.WriteString(fmt.Sprintf("| Go role_suite (this run) | %.1f%% | %.1f%% |\n\n",
		r.RoleSuite.MedianReduction*100, r.RoleSuite.MeanRecallAtK*100))
	b.WriteString("Methodology deltas (not apples-to-apples): Go pack markdown includes scrub/security lines; ")
	b.WriteString("Python token gate used `include_security_report=False`. Go fixture omits `.mdc` rules. ")
	b.WriteString("Absolute `budget_tokens=2000` + body include matches Python harness shape.\n\n")

	writeSuiteTable(&b, "Token suite accuracy", r.TokenSuite)
	writeSuiteTable(&b, "Role suite accuracy", r.RoleSuite)

	b.WriteString("## Gaps vs Python suite\n\n")
	for _, g := range r.Gaps {
		b.WriteString(fmt.Sprintf("- %s\n", g))
	}
	b.WriteString("\n")

	b.WriteString("## How to reproduce\n\n")
	b.WriteString("```powershell\n")
	b.WriteString("Set-Location -Path 'F:\\Projects\\SmartContext\\Repos\\smartcontext\\go'\n")
	b.WriteString("go test ./internal/eval/ -count=1 -v\n")
	b.WriteString("go run ./scripts/eval_corpus_telemetry.go\n")
	b.WriteString("# refreshes docs/CORPUS_TELEMETRY_EVAL.md + docs/corpus_telemetry_eval.json\n")
	b.WriteString("# + docs/corpus_telemetry_run.jsonl\n")
	b.WriteString("```\n\n")

	b.WriteString("---\n\n")
	b.WriteString("_Raw JSON: [`corpus_telemetry_eval.json`](corpus_telemetry_eval.json) · ")
	b.WriteString("harness: `internal/eval` + `scripts/eval_corpus_telemetry.go`_\n")
	return b.String()
}

func gateLabel(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func writeSuiteTable(b *strings.Builder, title string, s SuiteSummary) {
	b.WriteString(fmt.Sprintf("## %s\n\n", title))
	if len(s.Violations) > 0 {
		b.WriteString("Violations:\n")
		for _, v := range s.Violations {
			b.WriteString(fmt.Sprintf("- %s\n", v))
		}
		b.WriteString("\n")
	}
	b.WriteString("| Task | Hit | recall@k | reduction | est_tokens | naive | selected |\n")
	b.WriteString("|------|:---:|---------:|----------:|-----------:|------:|----------|\n")
	for _, t := range s.Tasks {
		hit := "miss"
		if t.Hit {
			hit = "hit"
		}
		if t.Negative {
			if t.NegativeOK {
				hit = "ok"
			} else {
				hit = "fail"
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.1f%% | %d | %d | %s |\n",
			t.ID, hit, t.RecallAtK, t.Reduction*100, t.EstimatedTokens, t.NaiveTokens,
			strings.Join(t.Selected, ", ")))
	}
	b.WriteString("\n")
}
