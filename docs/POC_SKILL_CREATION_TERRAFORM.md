# POC: Skill creation test drive (Terraform ~1.5 errors)

**Date:** 2026-10-03 (local)  
**Host Terraform:** `v1.7.3` windows_amd64 (target behaviors: Terraform **1.5.x** era; noted where 1.7 messaging differs)  
**Scope:** Write real `SKILL.md` files + fixtures; **do not** implement Go skill factory.  
**Publication gate:** CLOSED — local POC only.

## Why these five errors

Chosen from practitioner troubleshooting sources (state/providers/deps/for_each/backend) and reproduced locally with `hashicorp/null` + local backend (no cloud creds):

| # | Error class | Why common | 1.5 relevance |
|---|-------------|------------|---------------|
| 1 | **Invalid `for_each`** (list instead of set/map) | Extremely frequent when vars are `list(string)` | Same rule since 0.12+; still top plan-time footgun in 1.5 |
| 2 | **Cycle** via mutual `depends_on` | Easy to introduce when “making order explicit” | Unchanged; `terraform graph` still the debug aid |
| 3 | **Provider constraint unsatisfiable** | Bad pins / module range clashes / lock drift | `required_providers` + `.terraform.lock.hcl` workflow mature in 1.5 |
| 4 | **Error acquiring the state lock** | CI concurrency + crashed applies | Local backend uses OS locks; remote Lock ID + `force-unlock` still the runbook |
| 5 | **Backend configuration / state migration** | Path/bucket/key edits after state exists | Interactive: “Backend configuration changed”; TF 1.7 `-input=false`: migration approval disabled |

## Solutions (minimal)

| # | Fix |
|---|-----|
| 1 | `for_each = toset(var.names)` (or use a map); never key on unknown-at-plan attributes |
| 2 | Remove one `depends_on` edge; prefer attribute references |
| 3 | Realistic constraint (e.g. `~> 3.2`); `terraform init -upgrade`; commit lock file |
| 4 | Wait for live run; else `terraform force-unlock <ID>` (remote) or let concurrent local apply finish |
| 5 | `'yes' \| terraform init -migrate-state` (preserve) or `'no' \| …` (empty new backend) |

## Paths

### Fixtures

Root: `F:/Projects/SmartContext/Repos/smartcontext/go/testdata/terraform-poc/`

| Fixture | broken / fixed |
|---------|----------------|
| `01-invalid-foreach/` | list `for_each` → `toset()` |
| `02-cycle-depends/` | mutual `depends_on` → one-way |
| `03-provider-constraint/` | `= 99.99.99` → `~> 3.2` |
| `04-state-lock/` | `hold_lock.ps1` concurrent apply; `fixed/` clean |
| `05-backend-reconfigure/` | `setup_broken.ps1` then migrate; `fixed/` clean init |

Scratch probes (not product): `F:/Projects/Scratch/terraform-skill-poc/`

### Skills (discoverable)

Root: `F:/Projects/SmartContext/Repos/smartcontext/.cursor/skills/`

| Skill | Path |
|-------|------|
| `terraform-invalid-foreach` | `.cursor/skills/terraform-invalid-foreach/SKILL.md` (+ `reference.md`) |
| `terraform-cycle-depends` | `.cursor/skills/terraform-cycle-depends/SKILL.md` |
| `terraform-provider-constraint` | `.cursor/skills/terraform-provider-constraint/SKILL.md` |
| `terraform-state-lock` | `.cursor/skills/terraform-state-lock/SKILL.md` |
| `terraform-backend-reconfigure` | `.cursor/skills/terraform-backend-reconfigure/SKILL.md` |

Helper drivers (ignore-tagged):

- `go/scripts/pack_terraform_skills.go` — pack with `SMARTCONTEXT_SKILL_ROOTS=../.cursor/skills`
- `go/scripts/discover_default_roots.go` — default-roots discover for a chosen cwd

## Terraform plan/apply results (host 1.7.3)

| Fixture | Command | Result |
|---------|---------|--------|
| 01 broken | `init` + `validate`/`plan` | **FAIL** — `Invalid for_each argument` (list of string) |
| 01 fixed | `init` + `validate` + `plan` | **OK** — Plan: 2 to add |
| 02 broken | `init` + `plan` | **FAIL** — `Cycle: null_resource.b, null_resource.a` |
| 02 fixed | `init` + `plan` | **OK** — Plan: 2 to add |
| 03 broken | `init` | **FAIL** — `no available releases match the given constraints 99.99.99` |
| 03 fixed | `init` + `plan` | **OK** — Plan: 1 to add |
| 04 broken | concurrent `apply` + `plan` via `hold_lock.ps1` | **FAIL** — `Error acquiring the state lock` |
| 04 fixed | `init` + `plan` | **OK** |
| 05 broken | `setup_broken.ps1` then `init -input=false` | **FAIL** — migration approval disabled (1.7 non-interactive form of backend change) |
| 05 fix | `'yes' \| terraform init -migrate-state` | **OK** |
| 05 fixed dir | `init` + `plan` | **OK** |

**Apply:** used for 04 (lock hold) and 05 (seed state before backend change). Other fixtures stopped at **plan** (sufficient).

**Note:** Pre-seeded `terraform.tfstate.lock.info` alone does **not** block plan on local backend under 1.7.3 — OS lock during a live command does.

## Pack / discover results

### Isolated pack (`SMARTCONTEXT_SKILL_ROOTS` → project skills)

`go run ./scripts/pack_terraform_skills.go` → catalog **5/5** terraform skills. Selection (top-1 correct for all tasks):

| Task (abbrev) | Top selected |
|---------------|--------------|
| Invalid for_each… | `terraform-invalid-foreach` |
| Cycle depends_on… | `terraform-cycle-depends` |
| Failed to query provider… | `terraform-provider-constraint` |
| Error acquiring the state lock… | `terraform-state-lock` |
| Backend configuration changed… | `terraform-backend-reconfigure` |

JSON: `go/testdata/terraform-poc/_pack_results.json`

### Default roots by cwd

| cwd | terraform-* in catalog |
|-----|------------------------|
| `repos/smartcontext/go` | **5/5** (walk-up to project `.git` / `.cursor`) |
| `repos/smartcontext` | **5/5** |
| TEMP + `SMARTCONTEXT_WORKSPACE_ROOTS` | **5/5** |

### Live MCP `pack_context` (Cursor)

Set `SMARTCONTEXT_WORKSPACE_ROOTS` in mcp.json (project + user). Optional per-call `roots` on `pack_context` / `discover_skills`. See `docs/SKILL_ROOT_DISCOVERY.md`.

## Compare: Python lifecycle factory vs Go 1.1 POC

| Capability | Python (`docs/SKILL_FACTORY.md`) | Go 1.1 POC |
|------------|-----------------------------------|------------|
| Discover `SKILL.md` | Yes (catalog / index) | **Yes** (`DiscoverSkills` / `BuildCatalog`) |
| Pack / rank | Yes (budgeted) | **Yes** (simple token rank stub) |
| Propose drafts | Lifecycle refine / chat-scan | **No** |
| Consent-gated **create/apply** | `skills factory apply` + prefs | **N/A — not implemented** |
| MCP create tool | Lifecycle-adjacent (not Core invention) | MCP tools/list: **`pack_context` only** |

This POC **wrote skills as files** (manual creation) so discover/pack can consume them — matching the instruction to not fake an MCP factory.

## Gaps / follow-ups

1. **Go skill creation / factory** still absent (by design for this POC).
2. **MCP cwd / roots**: Mitigated by walk-up + `SMARTCONTEXT_WORKSPACE_ROOTS` in mcp.json + optional `roots` on pack — see `docs/SKILL_ROOT_DISCOVERY.md`.
3. **Rank quality**: with only terraform skills in roots, hit-rate is trivial; with large home catalogs, token rank needs better TF-specific weighting (or project-root priority).
4. **Terraform version drift**: fixtures validated on **1.7.3**; backend error string differs from classic 1.5 interactive banner — skills document both.
5. No shallow clone of hashicorp/terraform was required; errors grounded via local binary + public troubleshooting consensus.

## Quick re-run

```powershell
Set-Location F:\Projects\SmartContext\Repos\smartcontext\go
go run ./scripts/pack_terraform_skills.go
go run ./scripts/discover_default_roots.go F:\Projects\SmartContext\Repos\smartcontext

Set-Location .\testdata\terraform-poc\01-invalid-foreach\broken
terraform init -input=false; terraform validate
```
