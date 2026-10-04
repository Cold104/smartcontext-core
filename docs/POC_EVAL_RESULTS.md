# SmartContext 1.1 Go POC evaluation results

**Version:** `1.1.0`  
**Captured:** 2026-10-03T05:37:32Z (UTC)  
**Host note:** Windows local POC eval; in-process core + in-memory MCP  
**Publication gate:** CLOSED — local POC QA only; no Python parity claim.

## Prior eval methodology (reused shape)

Modeled on existing SmartContext evidence / harness patterns (not Lathe stack-eval LLM ingest):

- **Token / role benchmarks** — gold skill lists per task prompt, recall@k / hit-rate (`Repos/smartcontext/src/smartcontext/core/token_benchmark.py`, `role_token_benchmark.py`).
- **Case studies A/B** — cold empty catalog vs pre-seeded roots; pack + lifecycle refine/factory (`docs/CASE_STUDIES_SKILLS.md`, `tests/test_case_studies_skills.py`).
- **Eval hygiene** — isolated `SMARTCONTEXT_SKILL_ROOTS` / home; do not mix warm-home numbers (`docs/EVAL_HYGIENE.md`).
- **Latency budget docs** — local pack wall-clock, no pre-flight LLM (`docs/LATENCY.md`); Python Core micro-bench under `Docs/EVIDENCE/`.
- **Diehard ladder** — held-out success rate under skill-gated solver (Temp stamp pointer in `docs/DIEHARD_EVAL.md`) — **not** re-run here.
- **Alpha / Lathe stack-eval** — ingest latency + token reduction (`Docs/EVIDENCE/ALPHA_EVAL_v0.1.0-alpha.3.md`) — methodology reference only; out of scope for Go pack POC.

## Headline metrics

| Metric | Value |
|--------|------:|
| Selection hit-rate (scored tasks) | **100%** (6/6) |
| Mean recall@5 | 1.00 |
| Pack latency p50 (in-process) | **3.603 ms** |
| Pack latency avg (in-process) | 3.642 ms |
| Pack samples | 56 (8 iters × tasks) |
| Discover/catalog p50 (fixture, 15 skills) | 2.280 ms |
| Discover/catalog avg | 3.861 ms |
| MCP pack_context p50 | 4.143 ms |
| MCP pack_context avg | 4.552 ms |
| Default-roots discover (live) | 7.544 ms · roots=14 · catalog=18 |
| Re-pack after catalog change | 5.168 ms |

## Functionality status matrix

| Capability | Status | Evidence |
|------------|--------|----------|
| Skill discovery | **pass** | Fixture discover found 15/15 expected SKILL.md; default-roots live catalog=18 |
| Skill packing / selection | **pass** | MCP tool `pack_context` present; hit-rate 100%; simple token rank |
| Skill refinement | **partial** | No refine loop / proposal queue in Go; measured re-pack after new SKILL.md only (omega-skill selected after mutation) |
| Skill creation / factory | **N/A** | NOT IN POC — MCP tools/list has no factory/create tool; lifecycle factory is stub-only |

### MCP tools observed

- `discover_skills`
- `host_info`
- `pack_context`

## Latency table (per task, in-process pack)

| Task ID | p50 ms | avg ms | selected_count | root_count |
|---------|-------:|-------:|---------------:|-----------:|
| hermes-team | 3.611 | 3.552 | 5 | 1 |
| skill-index | 4.018 | 3.883 | 5 | 1 |
| windows-smartcontext | 3.322 | 3.655 | 5 | 1 |
| token-ingest | 3.615 | 3.660 | 5 | 1 |
| cycle-review | 3.603 | 3.642 | 5 | 1 |
| distractor-cooking | 3.598 | 3.599 | 5 | 1 |
| negative-astronomy | 3.575 | 3.505 | 5 | 1 |

## Accuracy table (selection)

| Task ID | Hit | recall@5 | Gold | Selected | Notes |
|---------|:---:|---------:|------|----------|-------|
| hermes-team | hit | 1.00 | hermes-agent-team, docker-personas | hermes-agent-team, docker-personas, cycle-review, context-budget, frontmatter-parse |  |
| skill-index | hit | 1.00 | skill-catalog, frontmatter-parse | skill-catalog, frontmatter-parse, token-pack, context-budget, cycle-review |  |
| windows-smartcontext | hit | 1.00 | windows-dev, powershell-paths | windows-dev, powershell-paths, context-budget, cycle-review, docker-personas |  |
| token-ingest | hit | 1.00 | token-pack, context-budget | token-pack, context-budget, skill-catalog, cycle-review, docker-personas |  |
| cycle-review | hit | 1.00 | cycle-review, merge-gate | cycle-review, merge-gate, hermes-agent-team, context-budget, docker-personas |  |
| distractor-cooking | hit | 1.00 | unrelated-cooking | unrelated-cooking, context-budget, cycle-review, docker-personas, frontmatter-parse |  |
| negative-astronomy | ok | 0.00 | — | context-budget, cycle-review, docker-personas, frontmatter-parse, hermes-agent-team | negative OK (no strong false lead on windows/token/hermes) |

## Gaps vs prior Python eval

| Python capability | Go POC |
|--------------------|--------|
| IDF / fuzzy rank + triage pin/deprecate | Thin IDF + name boost shipped; fuzzy/triage still deferred |
| Token budget + body scrub | Budget dial + pack-time secret scrub / injection flags |
| Lifecycle refine proposals + factory apply + consent | **Not in POC** |
| Session start / Canvas / telemetry | Stubs only |
| Token-reduction % vs naïve corpus | Naive tokens + budget_pct on pack result |
| Diehard held-out ladder / SWE-bench | Not run |
| Lathe stack-eval ingest | Out of scope |
| Core micro-bench Budget.for_ci gates | Ported — go/docs/LATENCY.md |

## How to reproduce

```powershell
Set-Location -Path 'F:\Projects\SmartContext\Repos\smartcontext\go'
go test ./internal/core/ -run TestPocEval -count=1 -v
go test ./...
go run ./scripts/eval_poc.go
# refreshes docs/POC_EVAL_RESULTS.md + docs/poc_eval_results.json
```

## Recommended next POC hardening (max 3)

1. **Optional fuzzy alias rank** (stdlib) on top of thin IDF for typo recall — keep weights local.
2. **Lifecycle refine stub → real proposal list** (read-only) without factory writes; expose `session_start` or `skills_refine_list` on MCP for parity demos.
3. **Telemetry emit** for pack latency / budget_tokens / scrub counts (local JSONL only).

---

_Raw JSON: [`poc_eval_results.json`](poc_eval_results.json) · harness: `scripts/eval_poc.go` · tests: `internal/core/poc_eval_test.go`_
