# Tool-result offload — observation masking

**Publication gate:** CLOSED.

## Value (observation masking)

Industry demand for agent hosts is **observation masking** (RAG-as-compression for
tool I/O): intercept huge tool logs, dump them to disk, and hand the model a short
preview plus a file pointer — not a compressed log folded into skills.

SmartContext’s **`offload_tool_result`** is that path: **path + preview**,
never “stuff the log into the skill catalog.” Core `pack_context` continues to
select **skills/rules only**. Session offload pointers are injected into the next
`markdown_dynamic` (episodic tail) so pack surfaces **tool-result-recall**.

## Problem

Huge tool outputs (e.g. 20k-token logs) should not enter the active prompt or the
procedural pack prefix. Packing them is expensive, blows the budget, and thrashs
prefix cache.

## Solution

Default-on MCP tool **`offload_tool_result`** (disable with `SMARTCONTEXT_OFFLOAD=0|false|off|no|disabled`):

1. When `content` ≥ **8192 bytes** (or `force=true`), writes full content under
   `~/.smartcontext/offload/`.
2. Returns `{ stored, path, preview, bytes, sha256, preview_truncated, threshold_bytes }`.
3. Under threshold with `force=false`: clear JSON **no-op** (`stored=false`, no file).
4. Persists last offload pointer(s) in session; next `pack_context` injects
   path + preview into `markdown_dynamic`.
5. Use baseline skill **`tool-result-recall`** to re-read or search the file.

Hosts still call `offload_tool_result` — SmartContext does **not** silently
intercept every host tool.

Do **not** compress or rewrite tool logs into skills/rules. Offload is lifecycle
storage + a short observation stub for the host.

## Preview budget

| | |
|---|---|
| **Default (1.2+)** | `DefaultOffloadPreviewBytes` = `IndustryAskOffloadPreviewBytes` = **200** (~50-token ask) |
| **Legacy constant** | `LegacyOffloadPreviewBytes` = **2048** (pass `preview_bytes=2048` to keep old budget) |

## Enable / disable

**Default-on** — no env required. To disable registration of the tool:

```json
"env": {
  "SMARTCONTEXT_OFFLOAD": "0"
}
```

Opt-out set (case/whitespace insensitive): `0` | `false` | `off` | `no` | `disabled`.
Empty or unrecognized values stay **on**. Truthy values (`1` / `true` / `on`)
remain accepted but are unnecessary.

## Related

- Procedural / episodic pack split: [PREFIX_CACHE_PACK.md](PREFIX_CACHE_PACK.md)
- Pointer/hash handoff stub (orchestration) for pack cites
- Industry demand gap matrix (internal): `Docs/RESEARCH/2026-10-industry-demand/GAP_MATRIX.md`
