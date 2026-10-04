# Local telemetry contract (Go)

SmartContext records **local-only** pack sensors under the operator home.
There is **no upload**, no remote collector, and **no raw task text** in the log.

## Path

| Item | Location |
|------|----------|
| State root | `SMARTCONTEXT_HOME` or `~/.smartcontext` |
| Directory | `<home>/telemetry/` |
| Append log | `<home>/telemetry/events.jsonl` |

Lifecycle creates the directory via `EnsureLayout`. Opt out of all lifecycle
telemetry with `SMARTCONTEXT_LIFECYCLE=0` (also `false` / `off` / `no`).

## Pack event fields (`type=pack`)

One JSON object per line. Flat fields (not nested under `payload`):

| Field | Type | Notes |
|-------|------|-------|
| `type` | string | Always `"pack"` |
| `source` | string | e.g. `"mcp"` |
| `timestamp` | string | UTC RFC3339, second precision |
| `version` | string | Binary / pack version |
| `os` | string | `runtime.GOOS` |
| `arch` | string | `runtime.GOARCH` |
| `root_count` | int | Resolved skill roots for the pack |
| `catalog_size` | int | Entries after kind filter |
| `selected_count` | int | Skills in the pack after triage |
| `budget_preset` | string | Optional (`minimum` / `medium` / `maximum` / …) |
| `budget_tokens` | int | Resolved token budget |
| `estimated_tokens` | int | Pack markdown estimate |
| `latency_ms` | number | Wall time for core pack + triage (ms) |
| `empty_catalog` | bool | No skills under roots |
| `scrub_flag_count` | int | Redactions + injection flags (counts only) |
| `task_hash` | string | Optional SHA-256 hex of task; **never** raw task |
| `stable_prefix_hash` | string | Optional SHA-256 hex of `markdown_stable` bytes (no bodies in the event) |
| `cache_friendly` | bool | `true` when a non-empty procedural (`markdown_stable`) prefix is present for host pin |

### Forbidden in every event

- Raw task / prompt text
- Skill bodies / file contents
- Secret values (scrub reports keep counts and rule ids only)
- Stable device identifiers beyond OS/arch already listed

Non-pack lifecycle events (`session_start`, `triage_set`, …) still use
`timestamp` / `type` / `source` / `payload` and the same exclusions.

## Sample pack line

```json
{
  "type": "pack",
  "source": "mcp",
  "timestamp": "2026-10-03T08:15:42Z",
  "version": "1.1.0",
  "os": "windows",
  "arch": "amd64",
  "root_count": 1,
  "catalog_size": 12,
  "selected_count": 5,
  "budget_preset": "medium",
  "budget_tokens": 2400,
  "estimated_tokens": 1987,
  "latency_ms": 4.842,
  "empty_catalog": false,
  "scrub_flag_count": 0,
  "task_hash": "aea1756e02e4a15e69dc364596d639a903326db5c92d9b753b1acedd2dc97d11",
  "stable_prefix_hash": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "cache_friendly": true
}
```

## Retention

| Policy | Default |
|--------|---------|
| Persistence | Append-only JSONL; operator-owned file |
| Automatic prune | **None** in v1.1 — no silent delete |
| Recommended operator prune | Keep ≤ **90 days** or rotate when `events.jsonl` exceeds ~50 MiB |
| In-process buffer | Last **500** events (diagnostics / tests only; not durable) |

Suggested prune (operator-run, not product code):

```powershell
# Keep last 90 days of pack lines (illustrative)
$p = Join-Path $env:USERPROFILE '.smartcontext\telemetry\events.jsonl'
$cutoff = (Get-Date).ToUniversalTime().AddDays(-90)
Get-Content $p | ForEach-Object {
  $o = $_ | ConvertFrom-Json
  if ([datetime]$o.timestamp -ge $cutoff) { $_ }
} | Set-Content ($p + '.pruned')
```

## Readout (how to use the log)

**Eval / CI floors:** empty-catalog rate, `latency_ms` p95, scrub flag spikes.

**Budget experiments:** compare `selected_count` / `estimated_tokens` vs
`budget_tokens` and `budget_preset` when changing dials.

**Operator debug:** `root_count` + `catalog_size` vs empty packs (roots misconfigured).

PowerShell — last pack events:

```powershell
Get-Content "$env:USERPROFILE\.smartcontext\telemetry\events.jsonl" |
  ForEach-Object { $_ | ConvertFrom-Json } |
  Where-Object { $_.type -eq 'pack' } |
  Select-Object -Last 20 timestamp, version, catalog_size, selected_count, budget_tokens, latency_ms, empty_catalog
```

Assert no raw task leak (tests do this in-process):

```powershell
Select-String -Path "$env:USERPROFILE\.smartcontext\telemetry\events.jsonl" -Pattern '\"task\"\s*:'
# Expect no matches for pack contract lines (task lives only as optional task_hash).
```

## Code

| Piece | Path |
|-------|------|
| Writer | `internal/lifecycle/telemetry.go` (`PackEvent`, `RecordPackEvent`) |
| Emit site | `internal/lifecycle/lifecycle.go` (`PackContext`) |
| Layout | `internal/lifecycle/paths.go` → `telemetry/events.jsonl` |

See also [`LIFECYCLE.md`](LIFECYCLE.md).
