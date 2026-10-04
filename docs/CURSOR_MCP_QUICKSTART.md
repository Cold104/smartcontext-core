# Cursor MCP quickstart (stdio)

SmartContext Core is an MCP server over **stdio**. Cursor launches the binary
directly — **no ports, no HTTP daemon, no wrapper scripts**.

## 1. Install the binary

**From GitHub Releases** (preferred):

```bash
# Linux / macOS — default repo Cold104/smartcontext (override with REPO=… or SMARTCONTEXT_REPO)
curl -fsSL https://raw.githubusercontent.com/Cold104/smartcontext/main/install.sh | bash

# Pin version / alternate Core extract repo
REPO=Cold104/smartcontext-core VERSION=v1.2.1 bash install.sh
```

```powershell
# Windows
irm https://raw.githubusercontent.com/Cold104/smartcontext/main/install.ps1 | iex

# Pin version / alternate repo
$env:SMARTCONTEXT_REPO = "Cold104/smartcontext-core"
$env:SMARTCONTEXT_VERSION = "v1.2.1"
.\install.ps1
```

Scripts live at repo root: [`install.sh`](../../install.sh), [`install.ps1`](../../install.ps1).
They download the matching Release asset (`darwin/arm64`, `linux/amd64`,
`windows/amd64`) and print a ready-to-paste `mcp.json` snippet.

**From source** (Go 1.22+):

```powershell
Set-Location go
# Windows: GUI subsystem avoids a console flash under Cursor
go build -ldflags="-H windowsgui" -o smartcontext.exe ./cmd/smartcontext
# Linux / macOS:
# go build -o smartcontext ./cmd/smartcontext
```

## 2. Wire Cursor `mcp.json`

Put the absolute path to the binary in `command`, with `args: ["mcp"]`.
Reload MCP / restart Cursor after changes.

**User global:** `~/.cursor/mcp.json` (Windows: `%USERPROFILE%\.cursor\mcp.json`)  
**Project local:** `<workspace>/.cursor/mcp.json`

```json
{
  "mcpServers": {
    "smartcontext": {
      "command": "/absolute/path/to/smartcontext",
      "args": ["mcp"],
      "env": {
        "SMARTCONTEXT_WORKSPACE_ROOTS": "/absolute/path/to/your/project"
      }
    }
  }
}
```

Windows example:

```json
{
  "mcpServers": {
    "smartcontext": {
      "command": "C:\\Users\\you\\.local\\bin\\smartcontext.exe",
      "args": ["mcp"],
      "env": {
        "SMARTCONTEXT_WORKSPACE_ROOTS": "C:\\Users\\you\\Projects\\myapp"
      }
    }
  }
}
```

### Rules

| Do | Don't |
|----|--------|
| Point `command` at the binary | Use `.bat` / `.cmd` / `powershell -c` wrappers |
| Pass `args: ["mcp"]` only | Open TCP ports or run a background daemon |
| Set `SMARTCONTEXT_WORKSPACE_ROOTS` when Cursor cwd is `$HOME` | Rely on home-only skill dirs for project work |

See [`SKILL_ROOT_DISCOVERY.md`](SKILL_ROOT_DISCOVERY.md) for root resolution and
cwd-is-home warnings.

## 3. Smoke check

```bash
smartcontext version
# Leaves process waiting on stdio (Ctrl+C to stop):
smartcontext mcp
```

In Cursor, call `discover_skills` or `host_info` once after reload.

## Prompt-Cache Aligned hosts

Pin `markdown_stable` across turns; mutate `markdown_dynamic` per turn.
Details: [`PREFIX_CACHE_PACK.md`](PREFIX_CACHE_PACK.md).

## Release automation

Cutting Core binaries: [`../../RELEASE.md`](../../RELEASE.md) ·
workflow [`.github/workflows/release.yml`](../../.github/workflows/release.yml).
