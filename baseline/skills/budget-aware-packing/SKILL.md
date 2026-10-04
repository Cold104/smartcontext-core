---
name: budget-aware-packing
description: >-
  Plan-mode budget ask (Super saver / Balanced / Max) with 60s timeout → auto;
  maps presets to hard token caps for pack_context.
tags: [budget, packing, plan-mode, smartcontext]
triggers: [budget, token budget, super saver, balanced, max context, save tokens]
os: any
kind: skill
related: [smartcontext-pack-operator, budget-policy]
aliases: [budget tiers, context budget, save context, max detail]
governed_by: [budget-policy]
---

# Budget-aware packing

## When to use

Before a large `pack_context` call, or when the user (or plan mode) asks how much
context to spend.

## Plan-mode ask

Ask:

> This looks like a large job. How should we use context — **Super saver**, **Balanced**, or **Max**?

| Answer | `budget_preset` | Token cap |
|--------|-----------------|-----------|
| Super saver | `super_saver` (alias `small`) | 1000 |
| Balanced | `balanced` (alias `medium`) | 2000 |
| Max | `max` (alias `large`) | 8000 |

## 60s timeout → `auto`

If there is **no answer within 60 seconds**, pack with `budget_preset=auto`.
`auto` is a heuristic from task length + catalog size, clamped to **[1000, 8000]**.

Do not block the pack forever waiting for a budget choice.

## MCP call shape

```json
{
  "task": "<user task>",
  "budget_preset": "balanced",
  "top_n": 5
}
```

Use `auto` after timeout; use absolute `budget_tokens` only when the user names a number.

## Cards vs bodies

- **Super saver** — prefer skill cards; few or no bodies.
- **Balanced** — default; bodies when they fit.
- **Max** — more bodies, still hard-capped at 8000 (not unbounded).

See `go/docs/BUDGET_TIERS.md`.
