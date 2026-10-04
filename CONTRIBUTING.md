# Contributing to SmartContext Core

Thanks for interest in **SmartContext Core** — the Go MCP skill/rule packer
(Apache-2.0). This repository may still contain Python Forge/compat and
Pro-bound packages until the public Core extract completes. Please keep
contributions aligned with the Core fence.

**Boundary:** public Core vs private Pro —
docs-house `OVERVIEW/OSS_PRO_BOUNDARY.md` (local: `F:/Projects/SmartContext/Docs/OVERVIEW/OSS_PRO_BOUNDARY.md`).

## What belongs in Core PRs

- MCP stdio server, pack/discover/host_info
- Skill-root discovery and markdown packing (**Depth-0**)
- Sticky selection, tool-result offload, budgets, scrub
- Prompt-Cache Aligned `markdown_stable` / `markdown_dynamic` behavior
- Local telemetry and (when present) opt-in remote aggregates
- Baseline skills, evals, benches, Core docs

## What does **not** belong in Core PRs

- Factory writes, conversation→skill harvest, self-learn loops
- Orch / Pro loop, Lathe runtime connectors
- Stage-2 Depth-1 graphs or multimodal contracts
- `//go:build pro` fences that hide proprietary source in a public tree

Those land in the private **smartcontext-pro** line that imports Core.

## Development (Go Core)

```powershell
Set-Location -Path 'go'
go test ./...
# Windows MCP smoke (no child console flash):
.\scripts\run_mcp_smoke.ps1
```

Requirements: Go 1.22+ (developed with Go 1.27 on Windows).

## Pull requests

1. Prefer focused PRs with tests for pack/lifecycle changes.
2. Do not add raw task text, skill bodies, or home paths to telemetry events.
3. Do not claim Depth-1 / multimodal / harvest in Core README or release notes.
4. Agents never Patent Center submit; disclosure questions go to patent-advisor /
   the inventor.

## License

By contributing, you agree your contributions are licensed under the
**Apache License 2.0** (see `LICENSE` and `NOTICE`).

## Security

See [SECURITY.md](SECURITY.md). Do not open public issues with live secrets.
