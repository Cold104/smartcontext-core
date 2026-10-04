# SmartContext Go corpus + telemetry evaluation

**Version:** `1.2.1`  
**Captured:** 2026-10-04T15:40:52Z (UTC)  
**Host note:** Windows local corpus+telemetry eval; isolated SMARTCONTEXT_HOME + SKILL_ROOTS  
**Publication gate:** CLOSED — local measurement only; no Python parity / publication claim

## Prior test types (inventory)

| Type | Python / docs path | Go status in this run |
|------|--------------------|-----------------------|
| Token benchmark (gold recall@k + naive reduction) | `src/smartcontext/core/token_benchmark.py`, `tests/test_benchmark.py` | **Ran** — token_suite |
| Role token benchmark (multi-role noise catalog) | `src/smartcontext/core/role_token_benchmark.py`, `tests/test_role_token_benchmark.py` | **Ran** — role_suite |
| Lifecycle telemetry (local JSONL, no raw task) | `src/smartcontext/lifecycle/telemetry.py`, `tests/test_lifecycle.py` | **Ran** — local events.jsonl |
| Refund receipt / pack aggregates | `telemetry.build_refund_receipt`, `metrics_summary` | **N/A** — missing on Go |
| Case studies A/B cold vs seeded | `docs/CASE_STUDIES_SKILLS.md`, `tests/test_case_studies_skills.py` | **Partial** — empty vs seeded pack only |
| Eval hygiene (isolated roots/home) | `docs/EVAL_HYGIENE.md` | **Applied** |
| Latency budgets (pack wall-clock) | `go/docs/LATENCY.md`, Core `Budget.for_ci` | **Ported** — native `TestLatencyBudgets` + CI 2× benches |
| Diehard held-out ladder | `docs/DIEHARD_EVAL.md` | **N/A** — not run |
| Lathe / alpha stack-eval ingest | `docs/evidence/ALPHA_EVAL_*.md`, `Docs/EVIDENCE/` | **N/A** — out of scope |
| Go POC fixture eval | `go/scripts/eval_poc.go`, `go/docs/POC_EVAL_RESULTS.md` | Prior sibling harness; this expands corpora |

## Headline metrics (Go, this run)

| Metric | Token suite | Role suite |
|--------|------------:|-----------:|
| Catalog skills | 15 | 48 |
| Selection hit-rate | 100% (5/5) | 100% (8/8) |
| Mean recall@5 | 1.00 | 1.00 |
| Median token reduction vs naive | 72.5% | 91.5% |
| Pack latency p50 / p95 | 6.217 / 7.245 ms | 11.927 / 12.978 ms |
| Discover p50 / p95 | 2.870 / 9.509 ms | 8.230 / 29.859 ms |
| Gate (reduction>=70%, recall>=80%) | pass | pass |

### MCP latency (token fixture)

| Metric | Value |
|--------|------:|
| discover_skills p50 / p95 | 3.442 / 4.003 ms |
| pack_context p50 / p95 | 8.253 / 8.733 ms |
| Tools | discover_skills, factory_apply, factory_create, factory_status, host_info, offload_tool_result, pack_context, session_start, skills_refine_list |

### Telemetry (local)

| Field | Value |
|-------|------:|
| Lifecycle enabled | true |
| Pack events in JSONL | 117 |
| JSONL lines / bytes | 117 / 53611 |
| Raw task leaked | false |
| Run measure log lines | 13 |
| Note | Local JSONL under SMARTCONTEXT_HOME/telemetry; no upload. Pack suites also emit via lifecycle.PackContext. |

### Case study (thin A/B)

| Field | Value |
|-------|------:|
| Cold empty_catalog | true (selected=0) |
| Seeded catalog size | 1 |
| Seeded pack hit windows-dev | true |
| Note | Thin stand-in for docs/CASE_STUDIES_SKILLS.md A/B; no factory/refine/Canvas |

## Comparison vs cited Python evidence

Python numbers below are **cited from product docs**, not re-measured in this run.

| Source | Median reduction | Mean recall@5 |
|--------|-----------------:|--------------:|
| Python `docs/requirements.md` (recorded) | 86.3% | 92.7% |
| Go token_suite (this run) | 72.5% | 100.0% |
| Go role_suite (this run) | 91.5% | 100.0% |

Methodology deltas (not apples-to-apples): Go pack markdown includes scrub/security lines; Python token gate used `include_security_report=False`. Go fixture omits `.mdc` rules. Absolute `budget_tokens=2000` + body include matches Python harness shape.

## Token suite accuracy

| Task | Hit | recall@k | reduction | est_tokens | naive | selected |
|------|:---:|---------:|----------:|-----------:|------:|----------|
| hermes-team | hit | 1.00 | 72.3% | 2032 | 7347 | hermes-agent-team, docker-personas, cycle-review, context-budget, frontmatter-parse |
| skill-index | hit | 1.00 | 72.4% | 2025 | 7347 | skill-catalog, frontmatter-parse, token-pack, context-budget, cycle-review |
| windows-paths | hit | 1.00 | 72.5% | 2023 | 7347 | powershell-paths, windows-dev, context-budget, cycle-review, docker-personas |
| token-ingest | hit | 1.00 | 72.5% | 2024 | 7347 | token-pack, context-budget, skill-catalog, cycle-review, docker-personas |
| cycle-review | hit | 1.00 | 72.5% | 2024 | 7347 | cycle-review, merge-gate, hermes-agent-team, context-budget, docker-personas |

## Role suite accuracy

| Task | Hit | recall@k | reduction | est_tokens | naive | selected |
|------|:---:|---------:|----------:|-----------:|------:|----------|
| data_analytics | hit | 1.00 | 91.4% | 2097 | 24329 | dbt-semantic-layer, amplitude-analyze-chart, looker-explore-router, dbt-analytics-engineering, amplitude-weekly-product-review |
| product_management | hit | 1.00 | 91.5% | 2075 | 24329 | figma-spec-handoff, pendo-friction-themes, amplitude-weekly-product-review, launchdarkly-flag-rollout, docker-review |
| software_development | hit | 1.00 | 91.5% | 2064 | 24329 | nestjs-module-patterns, django-orm-migrations, aspire-service-defaults, write-tests, dbt-analytics-engineering |
| networking | hit | 1.00 | 91.5% | 2061 | 24329 | cisco-aci-epg-contracts, haproxy-acl-tuning, istio-traffic-management, data-contracts-dq, service-mesh-mtls |
| security | hit | 1.00 | 91.6% | 2046 | 24329 | crowdstrike-detection-triage, prisma-cloud-misconfig, aws-iam, gitlab-ci-oidc, pipeline-review |
| devops | hit | 1.00 | 91.5% | 2059 | 24329 | gitlab-ci-oidc, pulumi-preview-review, release-readiness, spinnaker-pipeline-templates, sonarqube-quality-gate |
| devsecops | hit | 1.00 | 91.5% | 2073 | 24329 | owasp-zap-baseline, sonarqube-quality-gate, checkov-policy-suppressions, crowdstrike-detection-triage, pipeline-review |
| platform_engineering | hit | 1.00 | 91.5% | 2057 | 24329 | port-blueprint, crossplane-composition, kubernetes-ops, cisco-aci-epg-contracts, amplitude-analyze-chart |

## Gaps vs Python suite

- Python refund-receipt / metrics_summary aggregates (load_aggregates, export_refund_receipt) — Go has local JSONL RecordEvent only; no receipt builder
- Python case studies refine_scan + factory apply + Canvas CTA — Go case study is cold empty vs seeded pack only
- Python include_security_report=False for token gate — Go pack markdown always includes scrub report lines (methodology delta)
- Python .cursor/rules/*.mdc in token fixture — Go discover is SKILL.md-only in this harness
- Diehard held-out ladder / Lathe stack-eval ingest — out of scope
- Core micro-bench Budget.for_ci latency gates — ported (go/docs/LATENCY.md; TestLatencyBudgets + CI benches)
- Remote / upload telemetry lifecycle — intentionally not required; local JSONL only

## How to reproduce

```powershell
Set-Location -Path 'F:\Projects\SmartContext\Repos\smartcontext\go'
go test ./internal/eval/ -count=1 -v
go run ./scripts/eval_corpus_telemetry.go
# refreshes docs/CORPUS_TELEMETRY_EVAL.md + docs/corpus_telemetry_eval.json
# + docs/corpus_telemetry_run.jsonl
```

---

_Raw JSON: [`corpus_telemetry_eval.json`](corpus_telemetry_eval.json) · harness: `internal/eval` + `scripts/eval_corpus_telemetry.go`_
