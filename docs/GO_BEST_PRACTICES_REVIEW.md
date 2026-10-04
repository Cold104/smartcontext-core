# Go 1.1 / post-1.1 best-practices review

Review of the SmartContext Go tree (`go/`) against idiomatic Go and project hygiene while landing post-1.1 PRs in precedence order (docs → CI/roots → pack core → lifecycle/factory → orchestration stubs).

Publication gate remains **CLOSED**; this note does not expand claim language.

## Findings (fixed)

| Area | Finding | Fix |
|------|---------|-----|
| Line endings | `.go` sources were CRLF on Windows; `gofmt` / Linux CI disagreed | Added `.gitattributes` (`*.go` / `go.mod` / `go.sum` → `eol=lf`); ran `gofmt -w`; CI **Format check** fails on drift |
| CI hygiene | Only `go test` on Go matrix | CI runs `go vet ./...`, `gofmt -l`, then `go test`; build keeps `CGO_ENABLED=0`; `-H windowsgui` only on Windows matrix entry |
| Ranking | Hand-rolled selection sort for IDF order | `sort.SliceStable` with score then name/path tie-break; `nameBoost` unexported |
| Budget API | `BudgetResolution.Err` field (non-idiomatic) | `ResolveBudget(...) (BudgetResolution, error)` |
| Source encoding | Mixed `naïve` / mojibake risk in comments/strings | Prefer ASCII `naive` in Go source / MCP descriptions |
| Factory errors | Bare `err.Error()` / ignored prefs & audit failures | `%w` wrapping on I/O; side-note aggregation; still emit telemetry after successful write |
| MCP pack wiring | Lifecycle PR dropped `budget_pct` / `budget_tokens` into Core | Restored fields on `handlePackContext` so dial survives factory/lifecycle merge |

## Findings (deferred)

| Area | Why deferred |
|------|----------------|
| `context.Context` through discover/pack WalkDir | Signature / call-graph change across Core; MCP handlers already receive `ctx` but pack is sync and short for POC catalogs |
| MCP handler timeouts | Prefer cancelable Core first; avoid orphan work after timeout |
| `staticcheck` in CI | `honnef.co/go-tools` module path not resolvable from this host at review time; re-add when toolchain pin is clear |
| Exported `NaiveBudgetPresets` map | Useful for tests/docs; could freeze behind a getter later |
| Orchestration stubs | Opt-in env flags already default OFF; no deeper refactor until Lathe boundary ownership is decided |
| Full LF rewrite of untouched history | New checkouts honor `.gitattributes`; historical blobs may still be CRLF until touched |
| Shared `Enabled()` helpers | `orchestration.Enabled` and factory/lifecycle truthy parsers are near-duplicates; extract later if a third copy appears |

## Precedence landings (this review window)

1. Docs house sunset narrative + app pointer
2. CI matrix + host-auto roots (+ hygiene above)
3. IDF / budget / scrub (+ budget error / sort fixes)
4. Lifecycle + consent-gated factory (+ pack dial wiring / error wrap)
5. Orchestration advanced stubs (flags default OFF; default pack path unchanged)

## Invariants preserved

- Default pack path unchanged unless opt-in env flags / factory consent are enabled
- Factory consent defaults **OFF** (`SMARTCONTEXT_FACTORY_CONSENT`)
- Binaries not committed (`dist/`, `*.exe` gitignored); CI uploads artifacts only
- `internal/` layout; `cmd/smartcontext` is the singular binary entry
