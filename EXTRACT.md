# Core extract notes

Staged from `F:/Projects/SmartContext/Repos/smartcontext/go` by
`SmartContext/scripts/extract-core-pro.ps1`.

| | |
|---|---|
| **Module** | `github.com/Cold104/smartcontext-core` |
| **Target repo** | `https://github.com/Cold104/smartcontext-core` (public) |
| **Removed from this tree** | `factory/`, `orchestration/` (live in smartcontext-pro) |
| **Package layout** | Former `internal/{core,lifecycle,mcp,...}` lifted to module root so Pro can import |

Monorepo `Repos/smartcontext` remains the transitional working tree until cutover;
do not use `//go:build pro` as the product fence.
