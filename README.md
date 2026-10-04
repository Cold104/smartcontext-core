# SmartContext Core (Go MCP)

**Prompt-Cache Aligned** skill/rule packer for agent hosts — discover skills under
workspace roots, pack structured markdown under a token budget, expose MCP over
**stdio**. Apache-2.0.

| | |
|---|---|
| **Line** | **smartcontext-core** (OSS) |
| **Module** | `github.com/Cold104/smartcontext-core` |
| **Version** | `1.2.1` |
| **License** | [Apache-2.0](LICENSE) · [NOTICE](NOTICE) |
| **Location** | Staged extract at `F:/Projects/smartcontext-core` (from monorepo `Repos/smartcontext/go`) |
| **Pro** | Private `github.com/Cold104/smartcontext-pro` imports this module |

SmartContext packs **skills and rules** — it is **not** a monorepo/source-tree
compressor or a replacement for IDE codebase indexing.

**Depth-0 (Core):** skill-root discovery + markdown packing.  
**Not Core:** Stage-2 Depth-1 document graphs, multimodal contracts, harvest,
self-learn, factory writes, orch/Pro loop, Lathe runtime — those belong to
private **smartcontext-pro**, which imports this module. See docs-house
[`OVERVIEW/OSS_PRO_BOUNDARY.md`](../../../Docs/OVERVIEW/OSS_PRO_BOUNDARY.md)
(local: `F:/Projects/SmartContext/Docs/OVERVIEW/OSS_PRO_BOUNDARY.md`).

Publication gate is **OPEN for Core only**. Agents never Patent Center submit.

Dual-track versions: `F:/Projects/SmartContext/Docs/OVERVIEW/VERSION_POLICY.md`.

## Value: Prompt-Cache Aligned

`pack_context` returns:

| Field | Role |
|-------|------|
| `markdown_stable` | **Procedural** skill bodies — pin across turns for provider KV/prefix-cache hits |
| `markdown_dynamic` | **Episodic** task/meta — expected to change every turn |

Hosts should inject the stable block once (or when `selected_names` changes) and
mutate only the dynamic tail. Details: [`docs/PREFIX_CACHE_PACK.md`](docs/PREFIX_CACHE_PACK.md).

## Requirements

- Go 1.22+ (developed with Go 1.27 on Windows)

## Build / run

```powershell
Set-Location -Path 'F:\Projects\smartcontext-core'
# Windows: GUI subsystem so Cursor/GUI hosts do not flash a console window.
# Stdio MCP still works — parent-attached pipes are unchanged; only AllocConsole is skipped.
go build -ldflags="-H windowsgui" -o smartcontext.exe ./cmd/smartcontext
.\smartcontext.exe version   # smartcontext 1.2.1
.\smartcontext.exe mcp       # stdio MCP server
```

On non-Windows hosts, omit `-ldflags="-H windowsgui"` (that flag is Windows PE–only).

## MCP tools (Core)

- `pack_context` — skill-root discovery, OS-aware IDF rank, budget tiers, scrub; returns Prompt-Cache Aligned `markdown_stable` + `markdown_dynamic`; session sticky names for mid-task re-packs; `format=json` optional. See `docs/PREFIX_CACHE_PACK.md`, `docs/BUDGET_TIERS.md`.
- `discover_skills` — read-only: short `host` (os/arch/cpu) + resolved roots + skill names; includes `cwd_is_home` / `workspace_seed_source` / `warning` when the daemon cwd is home.
- `host_info` — read-only: full host snapshot plus the same root-resolution diagnostics.
- `session_start` — catalog delta vs watermark + pending refine proposals.
- `skills_refine_list` — list refine queue proposals (pending/all).
- `offload_tool_result` — observation masking (default-on; disable with `SMARTCONTEXT_OFFLOAD=0|false|off|no|disabled`): store large tool blobs → disk; return path + preview; session pointers inject into next `markdown_dynamic`. See `docs/TOOL_RESULT_OFFLOAD.md`.

**Pro-bound (not in this tree):** `factory_*`, `pack_at_boundary`, `pro_loop`
live in private **smartcontext-pro**, which imports this module. See
[EXTRACT.md](EXTRACT.md).

Docs:

- Skill-root discovery: [`docs/SKILL_ROOT_DISCOVERY.md`](docs/SKILL_ROOT_DISCOVERY.md)
- Skill/rule frontmatter: [`docs/SKILL_RULE_FORMAT.md`](docs/SKILL_RULE_FORMAT.md)
- Baseline index scaffolding: [`baseline/`](baseline/)
- Host OS/resources: [`docs/HOST_INFO.md`](docs/HOST_INFO.md)
- Lifecycle: [`docs/LIFECYCLE.md`](docs/LIFECYCLE.md)
- Local telemetry: [`docs/TELEMETRY_CONTRACT.md`](docs/TELEMETRY_CONTRACT.md) (remote upload opt-in is Next; default OFF)
- IPC overhead: [`docs/IPC_OVERHEAD.md`](docs/IPC_OVERHEAD.md)
- Latency: [`docs/LATENCY.md`](docs/LATENCY.md)

## Cursor MCP wiring (stdio)

Default transport is **stdio** — no ports or daemons. Absolute path to the built
binary (no `.bat` wrapper). Reload MCP / restart Cursor after changing the binary
or `mcp.json`.

**No console flash (Windows):**

1. **Binary:** PE subsystem **WINDOWS_GUI** (`-H windowsgui`) + `FreeConsole` at `mcp` entry — Cursor must launch the `.exe` directly.
2. **Smoke / local spawners:** `CREATE_NO_WINDOW` + `HideWindow` on the child (`internal/winproc.HideConsole`).

**Project** — `F:\Projects\SmartContext\Repos\smartcontext\.cursor\mcp.json`:

```json
{
  "mcpServers": {
    "smartcontext": {
      "command": "F:\\Projects\\SmartContext\\Repos\\smartcontext\\go\\smartcontext.exe",
      "args": ["mcp"],
      "env": {
        "SMARTCONTEXT_WORKSPACE_ROOTS": "F:\\Projects\\SmartContext\\Repos\\smartcontext"
      }
    }
  }
}
```

**User** (same snippet) — `C:\Users\cmaso\.cursor\mcp.json`

**Required when daemon cwd is `%USERPROFILE%` / `$HOME`:** always set
`SMARTCONTEXT_WORKSPACE_ROOTS` (preferred) or `CURSOR_WORKSPACE` in `mcp.json`
`env` to your project folder — otherwise `discover_skills` / `host_info` report
`cwd_is_home=true` and a warning. See `docs/SKILL_ROOT_DISCOVERY.md`.

Other apps: set `SMARTCONTEXT_WORKSPACE_ROOTS` (or `SMARTCONTEXT_SKILL_ROOTS`),
pass `roots` on `pack_context` / `discover_skills`, or rely on **host-auto**
(`CURSOR_WORKSPACE` / `WORKSPACE_FOLDER` / `PWD` + walk-up).

Multi-arch GitHub Releases + install scripts are **Next** (see docs-house roadmap).

## Test

**Preferred on Windows (no child console flash):**

```powershell
go test ./...
.\scripts\run_mcp_smoke.ps1
# optional: assert child has no console HWND
go run ./scripts/spawn_noflash_probe.go   # expect NOFLASH_OK
```

## Layout

```
cmd/smartcontext/     # main: mcp | version | help
internal/mcp/         # stdio MCP server (official go-sdk)
internal/lifecycle/   # triage, aliases, telemetry, session, refine list
internal/core/        # roots, discover, catalog, IDF rank, budget, scrub
internal/core/ast/    # Filter interface + stub
internal/winproc/     # Windows HideConsole / FreeConsole / console probe
internal/version/     # 1.2.1
baseline/             # INDEX.json + skill/rule scaffolding (ADD only)
docs/                 # PREFIX_CACHE_PACK, SKILL_ROOT_DISCOVERY, TELEMETRY_CONTRACT, …
```

Pro-bound packages still present until extract: `internal/factory/`,
`internal/orchestration/` — not Core OSS claims.

## Contributing / security

- [CONTRIBUTING.md](../CONTRIBUTING.md)
- [SECURITY.md](../SECURITY.md)
- Docs-house roadmap: `Docs/PLANS/ROADMAP_CURRENT.md`
