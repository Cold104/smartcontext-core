# Skill root discovery (Go 1.1+)

How `pack_context` / `discover_skills` find `SKILL.md` when the MCP process cwd is home, `go/`, or another app’s default directory — with **host-auto** seeds so Cursor and similar hosts need less manual `SMARTCONTEXT_WORKSPACE_ROOTS`.

## Precedence

1. **`roots` tool argument** (optional on `pack_context` and `discover_skills`) — host/agent passes workspace or skill directories for this call. Each path is used as-is and expanded as a workspace seed.
2. **`SMARTCONTEXT_SKILL_ROOTS`** — `;`-separated skill roots; **full override** (existing contract).
3. **`SMARTCONTEXT_WORKSPACE_ROOTS`** — `;`-separated workspace/host folders. When set, **replaces** host-auto seeds (explicit pin). Expanded to `{.cursor,.agents,.claude,.codex}/skills` plus depth-capped nested discovery.
4. **Host-auto workspace seeds** (when `SMARTCONTEXT_WORKSPACE_ROOTS` is unset):
   - Common IDE/host env vars (see below)
   - Process cwd
   - Walk-up from cwd **and** from each IDE seed for markers `.git`, `.cursor`, `.agents` (stop after first `.git` on each walk)
5. **Home + env extras** — `~/…/skills` for agent markers, plus `SMARTCONTEXT_ROOT` / `SMARTCONTEXT_FRIEND_ROOT`.

## Host-auto IDE env vars

Checked in order; empty values skipped. Each value may be a single path or a `;`-separated list (same contract as `SMARTCONTEXT_WORKSPACE_ROOTS`):

| Env var | Typical source |
|---------|----------------|
| `CURSOR_WORKSPACE` / `CURSOR_WORKSPACE_ROOT` | Cursor / wrappers that export the open folder |
| `WORKSPACE_FOLDER` / `WORKSPACE_FOLDERS` | Generic IDE / multi-root hosts |
| `VSCODE_WORKSPACE_FOLDER` / `VSCODE_CWD` | VS Code–family hosts |
| `PROJECT_ROOT` / `PROJECT_DIR` | Scripts / CI / custom launchers |
| `INIT_CWD` | npm / yarn (project that launched the process) |
| `PWD` | POSIX shells when useful as a seed |

Paths are absolutized; a leading `~/` expands via `os.UserHomeDir`. Walk-up then finds the git/project root even if the env points at a nested package directory.

### Recommended mcp.json (required when cwd is home)

Many GUI MCP spawns (including Cursor) start with **cwd = `%USERPROFILE%` / `$HOME`** and **no** `CURSOR_WORKSPACE`. In that case you **must** pin a workspace in `mcp.json` — otherwise discovery only sees home skill dirs (`~/.cursor/skills`, …).

Set **`SMARTCONTEXT_WORKSPACE_ROOTS`** (preferred) or **`CURSOR_WORKSPACE`** to the absolute project folder:

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

Alternate (same effect via host-auto IDE seed):

```json
"env": {
  "CURSOR_WORKSPACE": "F:\\Projects\\SmartContext\\Repos\\smartcontext"
}
```

If the host already exports `CURSOR_WORKSPACE` / `WORKSPACE_FOLDER` to the open folder, you can omit the env pin and rely on host-auto + walk-up.

### Diagnostics (`discover_skills` + `host_info`)

| Field | Meaning |
|-------|---------|
| `cwd_is_home` | Process cwd equals user home |
| `workspace_seed_source` | How seeds were chosen: `ide_env` \| `cwd_walk` \| `home_only` (also `explicit` / `skill_roots_env` / `workspace_env`) |
| `warning` | Set when cwd is home **and** only home skill dirs resolved — fix via `mcp.json` env above |

Call `discover_skills` or `host_info` after reload to confirm `workspace_seed_source` is not `home_only`.

## MCP client `roots` capability (go-sdk)

The official `modelcontextprotocol/go-sdk` exposes `ServerSession.ListRoots` (`roots/list`). As of protocol **2026-07-28 (SEP-2577)** that feature is **deprecated** (still functional during the deprecation window). SmartContext **does not** depend on `ListRoots` for discovery.

Prefer, in order:

1. Tool arg `roots`
2. `SMARTCONTEXT_WORKSPACE_ROOTS` / `SMARTCONTEXT_SKILL_ROOTS`
3. Host-auto IDE env + cwd walk-up

Hosts that still advertise `roots` may pass folders via tool parameters or env instead of expecting the server to call `roots/list`.

## Caps / deny-list

Nested glob under a workspace seed is depth-capped (`maxNestedSkillDepth = 6`) and prunes `node_modules`, `.hermes-work`, `.git`, `.smoke-*`, `eval-repos`, caches, etc. User home is **not** nested-walked (direct `~/.cursor/skills` only).

## How other apps should pass roots

| Mechanism | When to use |
|-----------|-------------|
| Export `CURSOR_WORKSPACE` / `WORKSPACE_FOLDER` (or siblings above) | Host already knows open folders — least friction |
| MCP `env.SMARTCONTEXT_WORKSPACE_ROOTS` | Pin absolute project folder(s) in `mcp.json` |
| MCP `env.SMARTCONTEXT_SKILL_ROOTS` | Pin exactly which skill trees to scan (override) |
| Tool arg `roots: ["D:\\app", "D:\\app\\.cursor\\skills"]` | Per-call from agent/host that knows open workspaces |
| Rely on walk-up | MCP started with cwd inside a git checkout (e.g. `…/go`) |

Use absolute paths on every OS (`F:\\Projects\\…` on Windows, `/home/…` on Linux/macOS). Multiple roots: **semicolon-separated** in env on all platforms (avoids colon/drive-letter ambiguity). Path joining uses `filepath`; home via `os.UserHomeDir` — no hard-coded `C:\`.

## Baseline skill + rule corpus (default ship)

Product baseline content lives in the repo so a normal workspace pin already finds it:

| Kind | Location |
|------|----------|
| Skills | `.cursor/skills/<name>/SKILL.md` |
| Rules | `.cursor/rules/<name>/RULE.md` |
| Index | `.cursor/BASELINE.md` |

**Default:** set `SMARTCONTEXT_WORKSPACE_ROOTS` (or host-auto) to the `Repos/smartcontext` checkout — discovery expands `.cursor/skills` under that workspace. No `SMARTCONTEXT_SKILL_ROOTS` required for day-to-day use.

**Pin / isolate:** `SMARTCONTEXT_SKILL_ROOTS=<repo>/.cursor/skills` (full override). Use a temp empty root plus temp `SMARTCONTEXT_HOME` for cold eval (skill `eval-hygiene`).

## Windows MCP binary (no console flash)

Cursor (and other GUI hosts) spawn the MCP command with redirected stdio. A **CONSOLE**-subsystem `.exe` can still allocate/flash a terminal window.

```powershell
Set-Location -Path 'F:\Projects\SmartContext\Repos\smartcontext\go'
go build -ldflags="-H windowsgui" -o smartcontext.exe ./cmd/smartcontext
```

- Prefer fixing the PE subsystem on the binary itself — do **not** wrap with `.bat`/`.cmd`/`cmd /c`/`powershell` (those create consoles). Point `mcp.json` `command` at `smartcontext.exe` directly.
- `-H windowsgui` does **not** break MCP stdio: stdin/stdout remain usable when the parent attaches pipes.
- At `mcp` entry the binary also calls `FreeConsole` (`internal/winproc`) in case a host attached a console.
- Local smoke spawners set `CREATE_NO_WINDOW` + `HideWindow` on the child (`.\scripts\run_mcp_smoke.ps1`).
- After rebuild, **reload MCP** so Cursor replaces any in-memory pre-GUI process.
- `discover_skills` reports `workspace_env` when `SMARTCONTEXT_WORKSPACE_ROOTS` from `mcp.json` is applied.

## Tools

- **`host_info`** — read-only OS/arch/CPU/memory/cwd/home + root diagnostics (see [HOST_INFO.md](HOST_INFO.md)).
- **`discover_skills`** — read-only: short `host` summary + resolved roots + skill names + root diagnostics (debug before pack).
- **`pack_context`** — same resolution, then token-rank pack; accepts optional `roots`.
