# Host info (Go 1.1 POC)

Cross-platform snapshot of the machine running the SmartContext MCP server. Agents should call **`host_info`** (or read the short **`host`** block on **`discover_skills`**) before packing when OS or resources matter.

## Tools

| Tool | Role |
|------|------|
| **`host_info`** | Full read-only snapshot (no args) |
| **`discover_skills`** | Includes `host: { os, arch, num_cpu }` so OS is visible without an extra call |

## Fields (`host_info`)

| Field | Source | Notes |
|-------|--------|--------|
| `os` | `runtime.GOOS` | `windows`, `linux`, `darwin`, … — interpret as Go OS names |
| `arch` | `runtime.GOARCH` | e.g. `amd64`, `arm64` |
| `hostname` | `os.Hostname` | Best-effort |
| `num_cpu` | `runtime.NumCPU` | Logical CPUs available to the process |
| `memory_total_bytes` | OS APIs via `golang.org/x/sys` | Omitted if unavailable |
| `memory_available_bytes` | OS APIs | Windows: avail phys; Linux: freeram; Darwin: free+inactive pages |
| `working_dir` | `os.Getwd` | MCP process cwd (often host default, not project) |
| `home_dir` | `os.UserHomeDir` | Portable; not hard-coded `C:\…` |
| `username` | `os/user` or `USER`/`USERNAME` | Best-effort |
| `cwd_is_home` | cwd vs home | `true` when daemon cwd is the user home directory |
| `workspace_seed_source` | root resolution | `ide_env` \| `cwd_walk` \| `home_only` (or env/explicit) |
| `warning` | root resolution | Present when cwd is home and only home skill dirs resolved |

No Forge telemetry, no persistent sampling — one-shot on tool call.

## How other apps should interpret `os`

- Use Go `GOOS` strings (`windows` / `linux` / `darwin`), not marketing names.
- Choose path examples and shell tips from skills that match `os` (e.g. PowerShell vs bash).
- Do **not** assume drive letters or backslashes unless `os == "windows"`.
- Skill-root discovery is already path-portable (`filepath`, `os.UserHomeDir`); pass absolute roots via env or tool args.

## Example payloads

`host_info`:

```json
{
  "os": "windows",
  "arch": "amd64",
  "hostname": "DESKTOP-EXAMPLE",
  "num_cpu": 16,
  "memory_total_bytes": 34268008448,
  "memory_available_bytes": 12000000000,
  "working_dir": "C:\\Users\\example",
  "home_dir": "C:\\Users\\example",
  "username": "example",
  "cwd_is_home": true,
  "workspace_seed_source": "home_only",
  "warning": "process cwd is user home and only home skill dirs were resolved; set SMARTCONTEXT_WORKSPACE_ROOTS or CURSOR_WORKSPACE in mcp.json (see docs/SKILL_ROOT_DISCOVERY.md)"
}
```

`discover_skills` host summary (+ diagnostics):

```json
{
  "host": {
    "os": "windows",
    "arch": "amd64",
    "num_cpu": 16
  },
  "roots": ["…"],
  "skill_names": ["…"],
  "cwd_is_home": true,
  "workspace_seed_source": "ide_env"
}
```

## Build note (Windows only)

GUI hosts: build with `-ldflags="-H windowsgui"` so the PE subsystem does not flash a console. Omit that flag on Linux/macOS. See [SKILL_ROOT_DISCOVERY.md](SKILL_ROOT_DISCOVERY.md).
