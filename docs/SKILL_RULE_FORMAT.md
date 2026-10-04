# Skill / Rule frontmatter format (Go 1.1+)

Locked schema for `SKILL.md` (workflows) and `RULE.md` (limitations). Discover
parses these fields into catalog `Entry` rows. Agents **add** rows and files to
the baseline scaffolding; they do not invent alternate column names.

## Kind model

| Kind | Meaning | Selection role |
|------|---------|----------------|
| **skill** | A **workflow** / playbook | Rank for “which job?” — prefer one primary skill (+ optional `related` fallbacks) |
| **rule** | A **limitation** / guardrail | Attach many rules to matching skills (`applies_to`); one rule → many skills |

`kind` and `type` are synonyms in frontmatter; `kind` wins if both are set.

## Required / supported fields

Shared (both kinds):

| Field | Type | Notes |
|-------|------|--------|
| `name` | string | Stable kebab-case id; primary reference key |
| `description` | string | When/what — selection text |
| `tags` | list | Coarse topics |
| `triggers` | list | Errors / products / phrases that boost rank |
| `os` | enum | `windows` \| `linux` \| `macos` \| `any` (empty → `any`) |
| `kind` / `type` | enum | `skill` \| `rule` |
| `related` | list of names | Correlation / failure fallbacks |
| `aliases` | list | Phrasing → same id (find); not the same as `related` |

Extra:

| Field | On | Notes |
|-------|-----|--------|
| `applies_to` | **rule** | Skill names, `tag:…`, or `*` — which skills this limits |
| `governed_by` | **skill** | Rule names that constrain this workflow (optional; can be derived from rules) |

### Example skill

```yaml
---
name: smartcontext-pack-operator
description: Discover roots and pack_context for a task under budget
tags: [pack, mcp, discover]
triggers: [pack_context, discover_skills, empty catalog]
os: any
kind: skill
related: [budget-aware-packing, host-os-windows]
aliases: [pack operator, how do I pack]
governed_by: [secure-pack-hygiene]
---
```

### Example rule

```yaml
---
name: secure-pack-hygiene
description: Do not pack secrets; factory stays consent-gated
tags: [security, pack]
triggers: [secret, API_KEY, .env]
os: any
kind: rule
applies_to: [smartcontext-pack-operator, budget-aware-packing, tag:factory]
aliases: [secure hygiene]
related: []
---
```

## List syntax

Discover accepts:

- Inline YAML: `related: [a, b]`
- Block lists (flattened by the thin frontmatter parser): 

```yaml
related:
  - a
  - b
```

- Comma-separated: `tags: pack, mcp`

## Catalog key and tie-breaks

**Identity:** `(kind, name)` via `CatalogKey` — `name` compared case-insensitively.

**Duplicates:** first wins in discovery order:

1. Roots in `ResolveRoots` / `SMARTCONTEXT_*` order (earlier root wins).
2. Within a root, absolute path **ascending** (lexicographic).

A `skill` and a `rule` may share the same `name`; they are different keys.

## Aliases vs related

| Field | Job |
|-------|-----|
| `aliases` | User/agent **phrasing** → find this id |
| `related` | **Correlation / recover** — if this skill fails or is thin, try these names next |

## Boilerplate index

Shipped scaffolding (agents only **ADD**):

- [`go/baseline/README.md`](../baseline/README.md)
- [`go/baseline/INDEX.json`](../baseline/INDEX.json)

Full baseline skill/rule **bodies** may land in a separate PR under `.cursor/skills` /
`.cursor/rules` or `go/baseline/{skills,rules}/`. Index rows should stay in sync
when names/`related`/`applies_to` change.

## Discovery paths

`DiscoverSkills` walks configured roots for:

- `**/SKILL.md` — default `kind: skill`
- `**/RULE.md` — default `kind: rule`

Agent layouts (`.cursor|.agents|.claude|.codex`)/(skills|rules) and shallow
fixture / `go/baseline` trees are included under the same depth / deny rules as
skill-root discovery ([`SKILL_ROOT_DISCOVERY.md`](SKILL_ROOT_DISCOVERY.md)).
