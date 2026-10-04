# Go lifecycle (post-1.1 POC)

Lifecycle sits between MCP tools and Core: triage prefs, skill aliases, local
telemetry, `session_start`, and refine proposal listing. Forge/Lathe is out of
tree. Skill **writes** are in [`FACTORY.md`](FACTORY.md) (consent-gated).

## On / off

Lifecycle is **on by default**.

| Opt out | Effect |
|---------|--------|
| `SMARTCONTEXT_LIFECYCLE=0` (also `false` / `off` / `no`) | No triage/alias expansion/telemetry/session refine scan |

Opting out of lifecycle **never** grants factory write consent.

## State layout (`SMARTCONTEXT_HOME` or `~/.smartcontext`)

| Path | Purpose |
|------|---------|
| `prefs/triage.json` | pin / keep / watch / deprecate |
| `prefs/skill_aliases.json` | user alias overrides (merged with packaged defaults) |
| `telemetry/events.jsonl` | append-only local pack sensors (see [`TELEMETRY_CONTRACT.md`](TELEMETRY_CONTRACT.md)) |
| `refine/queue/*.json` | propose-only refine drafts |
| `session/last_cursor_start.json` | watermark |
| `session/last_delta.json` | last skill/refine delta |

## MCP tools

| Tool | Role |
|------|------|
| `session_start` | Catalog delta vs watermark + pending refine list; optional `mark_session` |
| `skills_refine_list` | Read-only list of refine proposals (`status=pending\|all`) |
| `pack_context` | Via lifecycle: alias expand + triage filter + telemetry emit |

## Pack behavior

When lifecycle is enabled:

1. Expand task tokens via alias map (`tf` -> `terraform`, ...).
2. Core discover + simple token rank.
3. Drop **deprecate**, move **pin** to front of selected items.
4. Emit local `pack` telemetry per [`TELEMETRY_CONTRACT.md`](TELEMETRY_CONTRACT.md)
   (`task_hash` optional; never raw task text).

## Refine

`session_start` runs a lightweight thin-description scan and enqueues
**update** proposals under `refine/queue`. Listing is read-only; applying a
draft requires factory consent (see `FACTORY.md`).
