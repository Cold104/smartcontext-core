# Cache-stable pack layout (KV / prefix cache)

**Publication gate:** CLOSED — engineering guidance only.

## Value (KV-cache first)

SmartContext structures packs for **maximum provider prefix-cache (KV-cache) hits**,
not token cut alone. Token reduction is supporting evidence; a rewritten pack that
misses the provider prefix cache can still burn cost even when skill bodies are
unchanged.

Primary product line for hosts: pin procedural skill bodies at the top so the
provider can reuse the cached prefix; mutate only the episodic tail each turn.

## Procedural vs episodic

Industry multi-tier memory maps directly onto the Go pack split:

| Memory tier | Pack field | Contents | Cache expectation |
|-------------|------------|----------|-------------------|
| **Procedural** | `markdown_stable` | `# SmartContext pack` + version + `## Skills` (name-sorted cards/bodies; **no task**) | Pin across turns when the skill set is unchanged — aim for prefix-cache hits |
| **Episodic** | `markdown_dynamic` | `## Call context` (task, budget, scores, selected_names) + security; future session history belongs here | Expected to change; cache miss is normal |
| Convenience | `markdown` | `markdown_stable` + `markdown_dynamic` | Concatenation only — prefer splitting at the host |

Scores live only in the episodic (dynamic) tail so procedural skill bodies stay
byte-stable when the same skills are selected. No new Core algorithm is required
for this naming; it documents the existing layout.

## Host pin checklist

Use this checklist so hosts actually get cache-friendly payloads:

1. **Inject `markdown_stable` once** (or only when the `selected_names` set changes).
2. **Append/replace only `markdown_dynamic`** each turn — do not rebuild the full
   `markdown` string and re-send a shifted prefix.
3. **Preserve byte identity** of the stable block: no host-side re-sort, trim, or
   whitespace rewrite of `markdown_stable` before send.
4. **Enable session sticky** (`sticky_names` / `SMARTCONTEXT_STICKY_SESSION`) so
   mid-task re-packs keep goal skills and the procedural prefix does not thrash.
5. **Do not put task text or tool logs into the stable prefix** — task stays in
   `markdown_dynamic`; huge observations use offload (see below).
6. **Measure on the host/API side** — SmartContext cannot guarantee provider cache
   hits; it only emits **cache-friendly** procedural/episodic payloads.
7. **Correlate via local telemetry** — pack events include `stable_prefix_hash`
   (SHA-256 of `markdown_stable`) and `cache_friendly=true` so hosts can align
   SC packs with provider KV-cache hit rates ([TELEMETRY_CONTRACT.md](TELEMETRY_CONTRACT.md)).

## Related

- Sticky session: `SMARTCONTEXT_STICKY_SESSION` (default on when lifecycle on)
- Observation masking (tool offload): [TOOL_RESULT_OFFLOAD.md](TOOL_RESULT_OFFLOAD.md)
- IPC overhead: [IPC_OVERHEAD.md](IPC_OVERHEAD.md)
- Industry demand gap matrix (internal): `Docs/RESEARCH/2026-10-industry-demand/GAP_MATRIX.md`
