# Latency & performance budget (Go MCP)

**Publication gate: CLOSED.** Product vs CI ceilings for native Go pack /
catalog benches — not a multi-host SLA.

Related: Python Core [`docs/LATENCY.md`](../../docs/LATENCY.md) ·
`src/smartcontext/core/benchmark.py` (`Budget.for_ci`) ·
[`IPC_OVERHEAD.md`](IPC_OVERHEAD.md) (stdio Δ via `scripts/bench_ipc.go`).

---

## What we gate (Go)

| Metric | Product p50 ceiling | CI p50 ceiling (2×) | Bench |
|--------|--------------------:|--------------------:|-------|
| `PackContext` wall clock | **36 ms** | **72 ms** | `BenchmarkPackContext` |
| `BuildCatalog` wall clock | **21 ms** | **42 ms** | `BenchmarkBuildCatalog` |

Defaults leave ~3× headroom over local corpus p50 (~6–12 ms pack, ~3–7 ms
discover/build on a 20-skill fixture). **Product** stays tight for the
performance claim; **CI** applies a separate 2× multiplier via `CI=true` /
`SMARTCONTEXT_BENCH_CI=1` (same contract as Python `Budget.for_ci`).

`BenchmarkDiscoverSkills` is available for local profiling; the CI gate is
pack + BuildCatalog only.

---

## How to re-measure

From `Repos/smartcontext/go`:

```powershell
# Short benches (CI shape)
go test ./internal/core/ -bench='BenchmarkPackContext|BenchmarkBuildCatalog' -benchtime=50ms -benchmem -count=1 -run '^$'

# Optional discover microbench
go test ./internal/core/ -bench=BenchmarkDiscoverSkills -benchtime=50ms -benchmem -count=1 -run '^$'

# p50 gate (product locally; set SMARTCONTEXT_BENCH_CI=1 for CI ceilings)
go test ./internal/core/ -run '^TestLatencyBudgets$' -count=1 -v
```

Stdio vs in-process overhead remains `go run ./scripts/bench_ipc.go` (see
[`IPC_OVERHEAD.md`](IPC_OVERHEAD.md)) — not part of this gate.

---

## Lifecycle hot-path attribution (post-1.2)

Corpus pack p50 drifted ~+10% after lifecycle telemetry + offload recall landed
on every pack (`core.PackContext` → `LoadOffloadPointers` →
`HashStablePrefix` / `RecordPackEvent`). Gap-2 offload threshold evaluation is
**not** on this path.

Re-measure (hermetic 20-skill fixture, Windows local):

```powershell
go test ./internal/lifecycle/ -run '^TestPackLatencyAttribution$' -count=1 -v
```

| Cost (p50, process-local) | Pre-mitigation | Post-mitigation (1.2.1 prep) |
|---------------------------|---------------:|-----------------------------:|
| `core.PackContext` only | ~5.7 ms | ~5.7 ms |
| `HashStablePrefix` / SHA-256 of ~1.3 KB `markdown_stable` | ~0.8 µs | ~0 µs memo hit when stable unchanged |
| `LoadOffloadPointers` via `EnsureLayout` + miss | **~330 µs** (`EnsureLayout` MkdirAll) | ~35 µs (`Stat` + cached empty; no layout) |
| Full `markdown_stable` rebuild memo | rebuild every pack | **G-1 (1.3.0):** reuse when `StableMemoKey(selected_names, version)` unchanged |

**Root cause order:** (B) per-pack `EnsureLayout` on offload session read much
greater than (A) SHA-256 of stable markdown, which is much greater than (C)
sample noise. Mitigations: process-local `stable_prefix_hash` memo when
`MarkdownStable` is unchanged; cache `LoadOffloadPointers` when the session
file is missing or mtime/size unchanged; read path no longer calls
`EnsureLayout`.

Pack±hash / pack±offload wall deltas on the full pack loop are usually lost in
timer noise (~±0.3 ms); use the micro rows above for attribution.

### Corpus re-measure note

```powershell
go run ./scripts/eval_corpus_telemetry.go
```

**This run (post-mitigation, 2026-10-04):** token-suite pack p50 **6.217 ms**
(pre-1.2 band ~5.7 ms; residual ~+0.5 ms / within host noise and well under
`TestLatencyBudgets` product ceiling 36 ms). Role-suite pack p50 **11.927 ms**.
See [`CORPUS_TELEMETRY_EVAL.md`](CORPUS_TELEMETRY_EVAL.md).

---

## Explicit caveats

1. Absolute pack latency depends on catalog size, disk, and host load.
2. Gate is in-process `PackContext` / `BuildCatalog`, not MCP round-trip.
3. CI noise on shared runners uses 2× ceilings; do not widen product defaults
   to absorb flakes.
4. Zero pre-flight LLM ≠ zero wall-clock milliseconds.
