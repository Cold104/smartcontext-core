# Changelog — SmartContext Go MCP

## [Unreleased]

Bug fixes + minimal Pro hooks (no new Core features).

### Fixed

- **Sticky carry-over:** persisted session sticky names are reused only when the
  stored `task_hash` matches the current task, or (phrasing drift mid-task) when
  `updated_at` is within `SMARTCONTEXT_STICKY_TTL` (default `30m`; `0` = same
  task only). Legacy state without `task_hash`/`updated_at` never carries over.
  Explicit `sticky_names` and `clear_sticky` unchanged.
- **Sticky score leak:** sticky items keep their real rank score (was pinned at
  `1e9`, leaking into `score:*` lines and telemetry); precedence is by position.
- **Discovery:** `.claude/skills/synced/<account>/…` (Claude desktop account
  mirror) is denied; `.claude/skills/synced/SKILL.md` and `synced` dirs
  elsewhere stay discoverable.
- **Rank noise:** more function-word stopwords; generic dev verbs (add, new,
  update, fix, …) weighted ×0.3; path-only token matches weighted ×0.25; query
  tokens in >50% of a catalog of ≥8 entries ignored; `tags` now count as
  selection text.
- **Session watermark:** watermark stores `roots_hash`; a different (or legacy,
  unhashed) root set re-baselines with `delta.watermark_reset` /
  `"roots changed, watermark reset"` instead of listing every skill as
  added/removed.
- **Test isolation:** packages that exercise lifecycle set a temp
  `SMARTCONTEXT_HOME` in `TestMain` (tests previously wrote sticky/telemetry
  into the real `~/.smartcontext`).

### Added (hooks for Pro)

- `PackItem.rank`, `PackItem.sticky`, `PackItem.estimated_tokens` (chars/4, as
  the budget gate).
- `mcp.ServerOptions.PostProcessStable` and `core.ReplaceMarkdownStable` to
  post-process `markdown_stable` before `EnrichPack` with `markdown` /
  `estimated_tokens` kept consistent. `core.RenumberItems` after reorders.
- CI: `.github/workflows/ci.yml` (gofmt, vet, test on ubuntu + windows).

## [1.3.0] — 2026-10-04

G-1 Prompt-Cache Aligned stable reuse. Publication gate **OPEN for Core only**.

### Added / changed

- **G-1 `markdown_stable` reuse:** process-local memo keyed by
  `StableMemoKey(selected_names, version)` (sorted name set + version). When the
  key is unchanged across packs, Core reuses prior `markdown_stable` bytes and
  only rebuilds `markdown_dynamic` (task/budget/scores). Marketed as
  **Prompt-Cache Aligned**. Docs: `docs/PREFIX_CACHE_PACK.md`, `docs/LATENCY.md`.
- **Tests:** `TestMarkdownStableReuseWhenSelectedNamesUnchanged` (hit + dynamic
  task update) and selection-change miss.
- **Version:** `version.Version` → **1.3.0**.

## [1.2.1] — 2026-10-04

Post-1.2 latency harden + trajectory/opt-out lock. Publication gate **CLOSED**.

### Added / changed

- **Pack hot-path attribution:** memoize `stable_prefix_hash` when
  `markdown_stable` is unchanged; cache empty/unchanged offload session pointer
  loads (skip redundant `EnsureLayout` on recall). Docs: `docs/LATENCY.md`;
  `TestPackLatencyAttribution`.
- **Lifecycle-on remasure:** MCP/IPC/WS benches refresh `docs/IPC_OVERHEAD.md`
  + corpus telemetry evidence (token-suite pack p50 residual within
  `TestLatencyBudgets` ceiling).
- **Mock trajectory expansion:** sticky-OFF control, 5-turn sticky-stable
  `stable_prefix_hash`, tight-budget needle/truncation policy, mid-trajectory
  offload → `markdown_dynamic` recall hint (`trajectory_test.go`). Live-LLM
  goal-drift stays Deferred.
- **Opt-out boundary tests:** table-driven `OffloadEnabled` for
  `0|false|off|no|disabled` (case/whitespace); docs/README/MCP description
  aligned; dead `truthyEnv` helper cleaned up in `flags.go`.
- **Version:** `version.Version` → **1.2.1**.

## [1.2.0] — 2026-10-04

RC engineering gaps (Phases 1–3). Publication gate **CLOSED**.

1.1.2 toolchain/hygiene items are **rolled into this 1.2.0 cut** (no separate
`v1.1.2` tag); changelog section kept below for traceability.

### Added / changed (Phase 2 — offload)

- **Offload default-on:** `offload_tool_result` registers unless
  `SMARTCONTEXT_OFFLOAD=0|false|off|no|disabled`. Hosts still call the tool (no silent
  interception of arbitrary host tools).
- **8KB threshold:** content ≥ **8192** bytes stores under
  `~/.smartcontext/offload/`; under threshold with `force=false` returns clear
  JSON no-op (`stored=false`); `force=true` stores anyway.
- **Preview default 200:** runtime default is
  `IndustryAskOffloadPreviewBytes` (200); `LegacyOffloadPreviewBytes` (2048)
  kept as named constant (`preview_bytes` override).
- **Session recall hints:** last offload pointer(s) persist; next
  `pack_context` injects path + preview into `markdown_dynamic` (episodic
  tail) for **tool-result-recall**. Docs: `docs/TOOL_RESULT_OFFLOAD.md`.
- **Version:** `version.Version` → **1.2.0**.

### Added / changed (Phase 3 — trajectory + cache-hit proxy)

- **5-turn sticky harness:** `TestFiveTurnStickyTrajectoryDrift` — task drift across
  five packs; needle + recall@5 / sticky names hold with `SMARTCONTEXT_STICKY_SESSION`.
- **Cache-hit proxy:** `IdenticalPrefixOverlapPct` + sequential-pack tests assert
  `markdown_stable` byte equality and 100% prefix overlap for identical selection.
- **Pack telemetry fields:** `stable_prefix_hash` (SHA-256 of `markdown_stable`) and
  `cache_friendly` on `type=pack` JSONL (`docs/TELEMETRY_CONTRACT.md`).

## [1.1.2] — 2026-10-04 (rolled into 1.2.0)

Toolchain parity + root hygiene. Shipped as part of **1.2.0** (no standalone
tag). Publication gate **CLOSED**.

### Added / changed

- **Native latency benches:** `BenchmarkPackContext`, `BenchmarkBuildCatalog`,
  `BenchmarkDiscoverSkills` in `internal/core`; p50 gate via
  `TestLatencyBudgets` with product vs CI 2× ceilings (`ResolveLatencyBudget`,
  env `CI` / `SMARTCONTEXT_BENCH_CI`). Docs: `docs/LATENCY.md`. CI `go-mcp`
  job runs short benches + gate. Corpus eval gap “Budget.for_ci not ported”
  → **ported**.
- **Version sync:** discover `Note` uses `version.Version` (drops stale
  `v1.1.0-dev POC` wording). Intermediate SemVer was **1.1.2**; release binary
  is **1.2.0**.
- **Home-cwd diagnostics:** `cwd_is_home`, `workspace_seed_source`
  (`ide_env` | `cwd_walk` | `home_only` | env/explicit), and `warning` when
  only home skill dirs resolve — on `discover_skills` + `host_info`.
  Regression: chdir home + `CURSOR_WORKSPACE`. Docs: stronger Cursor
  `mcp.json` guidance in `SKILL_ROOT_DISCOVERY.md` + README.

## [Unreleased] — post-1.1.1

### Added / changed (local telemetry contract)

- **Pack JSONL contract** under `~/.smartcontext/telemetry/events.jsonl`:
  `timestamp`, `version`, `os`, `arch`, `root_count`, `catalog_size`,
  `selected_count`, `budget_preset`, `budget_tokens`, `estimated_tokens`,
  `latency_ms`, `empty_catalog`, `scrub_flag_count`, optional `task_hash`,
  optional `stable_prefix_hash`, `cache_friendly`
  — never raw task text. See `docs/TELEMETRY_CONTRACT.md` (retention + readout).
- `PackResult.catalog_size` for pack/telemetry alignment.

### Added / changed (Go core pack)

- **Skill/rule schema:** discover parses locked frontmatter (`tags`, `triggers`,
  `os`, `kind`/`type`, `related`, `aliases`, `applies_to`, `governed_by`) for
  `SKILL.md` and `RULE.md`; catalog key `(kind,name)` with documented tie-breaks.
  Docs: `docs/SKILL_RULE_FORMAT.md`. Boilerplate index: `baseline/INDEX.json`.
- **Thin IDF rank** with local name-boost weights (stdlib only), OS preference,
  alias/trigger boosts, skill-then-rule attach, deterministic score↓/name↑ ties.
- **Budget tiers:** `super_saver|balanced|max|auto` → 1000/2000/8000/heuristic
  (aliases `small|medium|large`); plan-mode 60s no-answer → `auto`
  (`docs/BUDGET_TIERS.md`). Legacy `minimum|maximum` naïve fractions kept.
- **Budget dial:** `budget_pct` or absolute `budget_tokens`; floor 256;
  empty-catalog fallback 2000; default preset `balanced`.
- **Pack-time scrub:** high-confidence secret redaction + report-only injection
  flags on included bodies (parity with Python `pack_security`).
- **Lifecycle port (POC):** triage prefs, skill aliases, local telemetry
  (memory + JSONL), `session_start`, `skills_refine_list` (read-only proposals).
- **Consent-gated factory:** `factory_status` / `factory_create` / `factory_apply`
  under sandbox or project `.cursor/skills` only. Default OFF via
  `SMARTCONTEXT_FACTORY_CONSENT` + `i_consent_create` / `i_consent_update`.
- **Post-1.1 orchestration stubs** (`internal/orchestration`): pack-at-boundary,
  Pro loop step, hash/pointer handoff envelope (feature-flagged; default OFF).
- **Core AST** (`internal/core/ast`): `Filter` interface + stub (Proposed; no
  tree-sitter). Grammar tests skipped until a grammar is linked.
- **Opt-in MCP tools** `pack_at_boundary` / `pro_loop` when
  `SMARTCONTEXT_ORCH_TOOLS=1`.
- Docs: `docs/LIFECYCLE.md`, `docs/FACTORY.md`, `docs/ORCHESTRATION_ADVANCED.md`,
  `docs/GO_BEST_PRACTICES_REVIEW.md`, `docs/TELEMETRY_CONTRACT.md`.

Default `pack_context` JSON is unchanged when orchestration flags are off.

## [1.1.1] — 2026-10-03

Industry-demand alignment (KV/prefix-cache + observation masking). Publication
gate **CLOSED** — engineering notes only.

### Added / changed

- **Cache-stable pack layout:** `markdown_stable` (skills, name-sorted, no task) +
  `markdown_dynamic` (task/budget/scores/security). Hosts should pin the stable
  prefix across turns. Docs: `docs/PREFIX_CACHE_PACK.md`. Hardened with
  `pack_cache_test.go` byte-identity coverage.
- **Sticky session names:** lifecycle persists `selected_names` and re-merges on
  the next pack (mid-task recoverability / goal skills). Opt out:
  `SMARTCONTEXT_STICKY_SESSION=0`. MCP: `sticky_names`, `clear_sticky`.
- **IPC bench:** `scripts/bench_ipc.go` + `docs/IPC_OVERHEAD.md`; `format=json`
  omits duplicate markdown Content blob on the wire.
- **Tool-result offload (flagged):** `offload_tool_result` when
  `SMARTCONTEXT_OFFLOAD=1`; baseline skill `tool-result-recall`. Optional
  `IndustryAskOffloadPreviewBytes` (~200) vs default 2048 preview; MCP
  `preview_bytes` override. Core stays skill/rule only. Docs:
  `docs/TOOL_RESULT_OFFLOAD.md`.

## [1.1.0] — 2026-10-03

SmartContext **1.1** Go MCP POC: singular Go binary with MCP as the primary
entry. Python package **`0.2.0b2`** remains in-tree for reference and gates.
Publication gate **CLOSED** — engineering/product notes only; no patent claim
language.

### Added

- **MCP-primary binary** (`go/cmd/smartcontext`): `smartcontext mcp` stdio
  server via official `modelcontextprotocol/go-sdk`.
- **Tools:** `pack_context`, `discover_skills`, `host_info`.
- **Core path (POC):** multi-root / glob skill discovery, catalog, simple
  token rank, pack markdown cards.
- **Workspace env:** `SMARTCONTEXT_WORKSPACE_ROOTS` / `SMARTCONTEXT_SKILL_ROOTS`
  plus walk-up and capped nested glob (see `docs/SKILL_ROOT_DISCOVERY.md`).
- **`host_info`:** cross-OS snapshot (OS, arch, CPU, memory, cwd, home, user)
  for agent-agnostic hosts (`docs/HOST_INFO.md`).
- **Cursor wiring:** project `.cursor/mcp.json` sample + Terraform POC skills
  under `.cursor/skills/` (file-based skill creation; no Go factory).
- **Windows no-flash:** PE GUI subsystem build guidance (`-H windowsgui`),
  `FreeConsole` at MCP entry, `CREATE_NO_WINDOW` / `HideWindow` for smoke
  spawners (`internal/winproc`).
- **Terraform skill-creation POC:** five common TF ~1.5-era errors as
  `SKILL.md` + fixtures under `testdata/terraform-poc/` (providers/state
  under `.terraform/` gitignored — rebuild locally with `terraform init`).
- **Eval:** in-process + MCP pack latency / selection hit-rate captured in
  `docs/POC_EVAL_RESULTS.md` (+ JSON).
- **CI:** `go-mcp` job (`go test ./…` + build) on Ubuntu and Windows.


### Also in unreleased (CI / discovery)

- **CI:** `go-mcp` matrix on **ubuntu / windows / macos** with artifact upload
  (`smartcontext-linux-amd64`, `smartcontext-windows-amd64.exe` GUI subsystem,
  `smartcontext-darwin-arm64`); `go vet` + `gofmt` gate; `CGO_ENABLED=0`.
- **Host-auto workspace roots:** when `SMARTCONTEXT_WORKSPACE_ROOTS` is unset,
  seed from common IDE env vars (`CURSOR_WORKSPACE`, `WORKSPACE_FOLDER`,
  `VSCODE_*`, `PWD`, ...) plus cwd walk-up. Documented in
  `docs/SKILL_ROOT_DISCOVERY.md` (MCP `roots/list` deprecated per SEP-2577).

### Decisions (summary)

| Topic | Decision |
|-------|----------|
| Entry | MCP-primary, agent-agnostic singular Go binary |
| Python | Retained (`0.2.0b2`); CLI-first pack deprecated for the 1.1 train |
| Concepts | Keep lifecycle/core contracts; port over time |
| Factory | Consent-gated Go factory (default OFF); sandbox or project `.cursor/skills` |
| Deferred | Pro loop, Core AST grammar, context cancel through WalkDir |

Durable ADR + mirrored POC docs live in **smartcontext-docs**
(`DETAILED/architecture/go-mcp-v1.1-decisions.md`, `EVIDENCE/GO_MCP_1.1/`).

### Excluded from git

- Built binaries (`go/**/*.exe`, `go/smartcontext`) — rebuild per `go/README.md`.
- Terraform provider caches under `testdata/**/.terraform/`.

### Local redesign board

Cursor canvas under the IDE projects folder (not in this repo) may hold a
local redesign board; not part of the tagged release artifact.
