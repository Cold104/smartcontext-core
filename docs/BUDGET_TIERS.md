# Budget tiers (plan-mode + MCP)

Plan-mode / lifecycle UX offers **Super saver**, **Balanced**, or **Max**.
Core receives a resolved `budget_preset` (or absolute `budget_tokens`) — it does
not run a recursive pack loop.

## Presets → token caps

| Agent offers | `budget_preset` | Aliases | Hard token cap |
|--------------|-----------------|---------|----------------|
| Super saver | `super_saver` | `small` | **1000** |
| Balanced | `balanced` | `medium` | **2000** |
| Max | `max` | `large` | **8000** |
| *(unattended / timeout)* | `auto` | — | Heuristic in **[1000, 8000]** |

`auto` = `1000 + 4×estimate_tokens(task) + 25×catalog_size`, clamped to
`[super_saver, max]`.

Default when `budget_preset` is omitted: **`balanced`** (2000).

### Legacy naive-fraction dial

Still accepted for compatibility:

| Preset | Meaning |
|--------|---------|
| `minimum` | 30% of naïve full-corpus estimate (floor 256) |
| `maximum` | 90% of naïve full-corpus estimate |

Prefer absolute tiers for plan-mode. Use `budget_pct` for an explicit naïve
fraction. Absolute `budget_tokens` remains mutually exclusive with preset/`budget_pct`.

## Plan-mode ask → 60s → `auto`

When the job looks large (or the agent wants a budget choice):

1. Ask: *“This looks like a large job. How should we use context — Super saver, Balanced, or Max?”*
2. Map the answer to `super_saver` / `balanced` / `max`.
3. **If there is no answer within 60 seconds, select `auto`.**

Timeout enforcement lives in the **host agent / plan-mode orchestration** (or a
lifecycle helper the agent calls). Constant:
`lifecycle.PlanModeBudgetAskTimeout` (60s). Core stays one-shot.

## MCP `pack_context`

`budget_preset` accepts: `super_saver` | `balanced` | `max` | `auto` |
`small` | `medium` | `large` | `minimum` | `maximum`.

## Skill text (`budget-aware-packing`)

Baseline skill (when shipped) should match this contract:

- Offer Super saver / Balanced / Max in plan mode.
- On **60s** silence → pack with `budget_preset=auto`.
- Prefer cards-first under `super_saver`; allow more bodies under `max`.
- Never treat Max as unbounded — hard cap is 8000 tokens unless the caller
  passes an explicit `budget_tokens`.
