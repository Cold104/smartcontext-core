# Security Policy — SmartContext

SmartContext indexes local agent skills/rules and builds **token-budgeted
context packs**. Security work focuses on **context least privilege** and
**shift-left** pack controls. That is aligned with Zero Trust *principles*
(NIST SP 800-207) but is **not** a claim of a complete Zero Trust Architecture.

**Product fence:** **Core OSS** (Go MCP packer, Apache-2.0) vs private **Pro**
(factory / harvest / self-learn / Lathe). See docs-house
`OVERVIEW/OSS_PRO_BOUNDARY.md`. Publication gate is **OPEN for Core only**.
Agents never Patent Center submit.

## Supported controls (Core)

### Go MCP Core (`go/`)

- **Task-ranked selection** — only top-N relevant catalog entries enter a pack.
- **Token budgets** — packs fail soft by truncating; benches/CI guard regressions.
- **Skill-root allowlists** — workspace roots via `SMARTCONTEXT_WORKSPACE_ROOTS`
  / discovery rules; hostile paths should not be seeded as roots.
- **Pack-time secret redaction** (default on) — high-confidence patterns replaced
  with redaction placeholders; injection markers flagged (report-only).
- **Provenance** — included bodies record path / hash / length where applicable.
- **Telemetry** — local JSONL by default; aggregates only (no raw task, skill
  bodies, or home paths). Remote upload (when enabled) is opt-in and fail-open.

### Python Core (`smartcontext.core`) — compat until sunset

- **Source root allow/deny** — `SMARTCONTEXT_SOURCE_POLICY` (or built-in deny
  substrings: `.env`, `secrets`, `credentials`).
- **Host-path-free / stdlib-only Core** — CI guards reject third-party imports and
  embedded drive-letter or home-dir literals.
- Same pack-time redaction / injection report patterns as above.

Disable redaction only when necessary (Python CLI):

```powershell
smartcontext pack "your task" --no-redact
```

Token-reduction benchmarks intentionally exclude the security report overlay from
counted tokens so historical §101 baselines stay frozen and reproducible.
Product packs still include the overlay by default.

## Reporting a vulnerability

Please do **not** open a public issue with live secrets or exploit details.

1. Redact credentials before sharing logs or packs.
2. Describe the affected component (`go/` MCP pack path, Python `core`, Forge
   persona, or Lathe API).
3. Include reproduction steps that use **synthetic** secrets (for example `AKIA`
   test IDs).
4. Contact the repository maintainer through a private channel for this
   workspace (or GitHub private security advisory when the Core repo is public).

## Trust boundaries (summary)

| Zone | Write access |
|------|----------------|
| Main tree (`go/`, `src/`, `docs/`, tests) | Human / Cursor merge only |
| Skill roots | Read-only for pack/discover |
| `.hermes-work/` sandbox | Persona RW; critic / cycle gate before merge (Forge/Pro neighbor) |
| Pro private repo | Not in Core OSS tree |

See `docs/security/THREAT_MODEL.md` for assets, actors, threats, and residual risks.

## Related docs

- [CONTRIBUTING.md](CONTRIBUTING.md) — Core vs Pro contribution fence
- [LICENSE](LICENSE) / [NOTICE](NOTICE) — Apache-2.0 Core
- `go/README.md` — Go MCP Core
- `go/docs/TELEMETRY_CONTRACT.md` — local (and future remote) telemetry rules
- `docs/security/THREAT_MODEL.md`
- Docs-house: `OVERVIEW/OSS_PRO_BOUNDARY.md`

## References (primary)

- NIST SP 800-207 — Zero Trust Architecture — https://doi.org/10.6028/NIST.SP.800-207
- NIST SP 800-218 — SSDF 1.1 — https://doi.org/10.6028/NIST.SP.800-218
- OWASP Top 10 for LLM Applications 2025 — https://genai.owasp.org/llm-top-10/
- NIST NCCoE — AI agent identity & authorization concept paper (2026-02) —
  https://csrc.nist.gov/pubs/other/2026/02/05/accelerating-the-adoption-of-software-and-ai-agent/ipd
- CISA — Careful Adoption of Agentic AI (2026-05-01 announcement) —
  https://www.cisa.gov/news-events/news/cisa-us-and-international-partners-release-guide-secure-adoption-agentic-ai
