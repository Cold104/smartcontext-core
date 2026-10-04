# SmartContext 1.1 POC demo checklist

## Success criteria

Cursor invokes Go MCP `pack_context` and gets a pack with **≥1 real skill** from disk.

## Steps

1. **Build** (Windows: GUI subsystem — no console flash under Cursor)
   ```powershell
   Set-Location -Path 'F:\Projects\SmartContext\Repos\smartcontext\go'
   go build -ldflags="-H windowsgui" -o smartcontext.exe ./cmd/smartcontext
   .\smartcontext.exe version
   # expect: smartcontext 1.1.2
   ```

2. **Automated proof (no Cursor UI)**
   ```powershell
   go test ./...
   .\scripts\run_mcp_smoke.ps1
   # expect: tools/list has pack_context + host_info; selected_count >= 1; SMOKE_OK
   go run ./scripts/spawn_noflash_probe.go
   # expect: NOFLASH_OK
   ```

3. **Cursor MCP wiring** (already written; reload if needed)
   - Project: `F:\Projects\SmartContext\Repos\smartcontext\.cursor\mcp.json`
   - User: `C:\Users\cmaso\.cursor\mcp.json`
   - Both point at `F:\Projects\SmartContext\Repos\smartcontext\go\smartcontext.exe` with args `["mcp"]`
   - In Cursor: reload MCP / reopen window so `smartcontext` server appears
   - Confirm tool `pack_context` is listed

4. **Call from Cursor**
   - Ask the agent to call MCP tool `pack_context` with e.g.  
     `task: "Windows PowerShell SmartContext development"`
   - Expect JSON/markdown with `empty_catalog: false`, `selected_count >= 1`, and a named skill (e.g. `smartcontext-windows-dev` from `~\.cursor\skills`)

## If Cursor does not see the tool

- Rebuild `smartcontext.exe` (step 1)
- Confirm mcp.json paths use the absolute `.exe` path
- Reload MCP servers / restart Cursor
- Check MCP logs for spawn failures (wrong path, AV quarantine)
