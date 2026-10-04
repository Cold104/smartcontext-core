# SmartContext v1.1 kickoff

**Status:** shipped as Go MCP POC train **`1.1.0`** / tag `v1.1.0`  
**Python package:** remains `0.2.0b2` — do not mass-delete; keep working.

## Why `go/` under the Python repo

| Option | Decision |
|--------|----------|
| `Repos/smartcontext/go` | **Chosen** — same git remote (`Cold104/smartcontext`), outside `src/` so setuptools/`pyproject.toml` are unaffected |
| `Repos/smartcontext-go` | Rejected for kickoff — would sit outside the existing git root |

**Module path:** `github.com/Cold104/smartcontext-core`

## Locked product decisions (do not re-litigate)

1. Agent-agnostic (Cursor, Claude, any MCP host) — KEEP  
2. Roots (skill/rule discovery) — KEEP (contract ported in `internal/core/roots.go`)  
3. Pack-at-boundary — KEEP as edge concept  
4. **MCP server is PRIMARY entry** — `smartcontext mcp`  
5. Lifecycle retained: triage, budget, telemetry, session, factory (stubs)  
6. Core pack retained: discover → catalog → rank → pack → scrub (one-shot, no network)  
7. Language: Python → Go singular binary  
8. Forge/Lathe remains a separate neighbor — **not** in this module  

## Legacy vs primary

| Surface | Role in v1.1 |
|---------|----------------|
| Go `smartcontext mcp` | **Primary** product entry |
| Python `python -m smartcontext.mcp` | Still works; reference implementation during port |
| Python CLI (`smartcontext` / pack CLI-first) | **Legacy** for the v1.1 train — keep for parity/tests; do not expand |

## MCP SDK

Official SDK: [`github.com/modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk)  
Tool stub: `pack_context` → lifecycle → core empty-catalog pack + default roots.

## Stubbed vs deferred

| Area | This slice | Next |
|------|------------|------|
| `default_roots` | Ported | — |
| SKILL.md discover / catalog index | Empty catalog only | Full discover + dedupe |
| Rank / budget dial | Deferred | Port Core select + naive-% budget |
| Pack markdown / scrub | Placeholder markdown; scrub no-op | Real pack + pack_security |
| Lifecycle triage/telemetry/session/factory | Stubs | Port from Python lifecycle |
| Other MCP tools (`session_start`, triage, …) | Not registered yet | Add after pack path solid |
| Forge | Out of tree | Never inside Core binary |

## Suggested next slices (priority)

1. **Discover + catalog** — port `discover_skills` / `discover_rules` + `build_catalog`; wire `index_catalog` MCP tool.  
2. **Rank + budget pack** — port select/budget dial; make `pack_context` return real selections under a hard token budget.  
3. **Lifecycle glue** — triage prefs + local telemetry + `session_start` stub→real; still no Forge inside Core.
