# MCP stdio / serialization overhead

**Publication gate:** CLOSED — local measurement guidance.

## Question

In-process pack (lifecycle-ON product path) is a few ms. Does JSON + stdio MCP dominate end-to-end latency?

## How to measure

From `go/` (Windows GUI binary optional for Cursor; console build fine for bench):

```powershell
go build -o smartcontext.exe ./cmd/smartcontext
go run ./scripts/bench_ipc.go
```

The harness uses **lifecycle ON** (product default), an isolated `SMARTCONTEXT_HOME`, and reports:

- **in_process_pack_p50_ms** — `lifecycle.PackContext` only (empty offload session)
- **in_process_empty_offload_p50_ms** / **in_process_populated_offload_p50_ms** — same pack with missing vs populated `session/last_offload.json`
- **mcp_roundtrip_p50_ms** — full `pack_context` tool call over CommandTransport
- **stdio_delta_p50_ms** — roundtrip − in-process (serialize + IPC + dual text content)
- **ws_before_packs_mb** / **ws_after_n_packs_mb** — Windows `Get-Process` WorkingSet of the MCP child before packs and after N extra packs
- **host_info_memory_*** — `host_info` total/available bytes from the same MCP session

## Example local sample (post-1.2 mitigations)

Windows, modest one-skill fixture, lifecycle ON, hash memo + offload pointer cache, 20 iters, 2026-10-04:

| Metric | Value |
|--------|------:|
| in_process_pack_p50_ms | ~3.6 |
| in_process_empty_offload_p50_ms | ~3.7–4.0 |
| in_process_populated_offload_p50_ms | ~3.6–3.8 |
| mcp_roundtrip_p50_ms | ~4.2–4.6 |
| stdio_delta_p50_ms | ~0.6–1.0 |
| content_bytes_full / json | ~3.9k / ~3.2k |
| ws_before_packs_mb → ws_after_30_packs_mb | ~12.8 → ~18.3 |
| host_info memory avail / total | ~39.9 / ~63.9 GB |

Empty vs populated offload p50 are within host noise after the pointer-cache mitigation; populated is not a measurable pack regression at this fixture size. Stdio Δ stays sub-millisecond to ~1 ms across repeats — it does **not** dominate; keep JSON+markdown default. Use `format=json` when hosts already parse structured results.

> Pre-1.2 harness samples (lifecycle OFF, ~1.0 / ~1.7 / ~0.6 ms) are **superseded** — do not mix them with lifecycle-ON product-path numbers.

## Interpretation

| Observation | Action |
|-------------|--------|
| delta ≪ pack time | Keep default JSON + markdown texts |
| delta ≫ pack time at large budgets | Use `pack_context` `format=json` (omit second markdown Content blob; stable/dynamic still inside JSON) |
| Still dominated at huge payloads | Consider opt-in binary later — **not** default MCP rewrite |
| WS grows slowly across N packs | Expected Go heap/telemetry; still ≪ Tier-1 RAM headroom |

Default contract remains JSON/markdown. Binary/MessagePack is **Later** until benches justify it.

## Related

- [PREFIX_CACHE_PACK.md](PREFIX_CACHE_PACK.md)
- [LATENCY.md](LATENCY.md) — lifecycle hot-path attribution
- `pack_context` arg `format=json`
