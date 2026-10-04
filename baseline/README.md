# Baseline skill / rule index (scaffolding)

Out-of-box catalog contract for SmartContext Go. Agents and humans **only ADD**
rows and files — do not rename columns, invent alternate schemas, or delete the
index scaffolding.

## Layout

```
go/baseline/
  README.md       ← this file
  INDEX.json      ← machine-readable index (add rows here)
  skills/         ← optional SKILL.md placeholders (bodies may live elsewhere)
  rules/          ← optional RULE.md placeholders
```

Discoverable when `SMARTCONTEXT_SKILL_ROOTS` (or workspace roots) include
`go/baseline` or its `skills` / `rules` children. Project MCP wiring that sets
`SMARTCONTEXT_WORKSPACE_ROOTS` to the app repo also finds `.cursor/skills` —
baseline **content** PRs may place full bodies there; keep `INDEX.json` in sync.

## Agent rules

1. **Add, don’t rewrite** — append `INDEX.json` entries; keep column names stable.
2. **Cite by `name`** — `related`, `aliases`, `applies_to`, and `governed_by` use
   catalog `name` values (kebab-case).
3. **Frontmatter first** — every `SKILL.md` / `RULE.md` must carry the locked
   fields documented in [`../docs/SKILL_RULE_FORMAT.md`](../docs/SKILL_RULE_FORMAT.md).
4. **Skills vs rules** — skills = workflows; rules = limitations (one rule → many
   skills via `applies_to`).
5. **Bodies elsewhere OK** — stub index rows are valid until a corpus PR fills
   `SKILL.md` / `RULE.md` bodies.

## INDEX.json columns

`kind`, `name`, `description`, `os`, `tags`, `triggers`, `related`, `aliases`,
`applies_to`, `governed_by`, `path`

Catalog identity is `(kind, name)` — see tie-breaks in `SKILL_RULE_FORMAT.md`.
