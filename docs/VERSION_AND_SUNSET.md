# Version path and Python retention (pointer)

**Source of truth** is the docs house (`Cold104/smartcontext-docs`), not this file:

| Topic | Docs-house path |
|-------|-----------------|
| Go MCP 1.1 ADR | `DETAILED/architecture/go-mcp-v1.1-decisions.md` |
| Dual-track versions + sunset criteria | `OVERVIEW/VERSION_POLICY.md` |
| Post-1.1 Now/Next/Later | `PLANS/ROADMAP_CURRENT.md` |
| POC evidence mirrors | `EVIDENCE/GO_MCP_1.1/` |

**Summary:** Go MCP **`1.1.2`** is the product path (patch on the 1.1 train).
Python **`0.2.0b2`** stays for Forge/compat and Core CI until explicit sunset
criteria in the docs house are met. Do not delete the Python package based on
this pointer alone.
