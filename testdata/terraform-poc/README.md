# Terraform skill-creation POC fixtures

Minimal reproducible Terraform ~1.5-era error cases for SmartContext skill discovery/pack demos.

**Local Terraform used for validation:** see `POC_SKILL_CREATION_TERRAFORM.md` (host may differ from 1.5.x; syntax targets 1.5 behaviors).

| # | Fixture dir | Error class | Skill |
|---|-------------|-------------|-------|
| 1 | `01-invalid-foreach` | Invalid `for_each` (list not set/map) | `terraform-invalid-foreach` |
| 2 | `02-cycle-depends` | Cycle via mutual `depends_on` | `terraform-cycle-depends` |
| 3 | `03-provider-constraint` | No provider release matches constraints | `terraform-provider-constraint` |
| 4 | `04-state-lock` | Error acquiring the state lock | `terraform-state-lock` |
| 5 | `05-backend-reconfigure` | Backend configuration changed | `terraform-backend-reconfigure` |

Each fixture has `broken/` (reproduces error) and `fixed/` (minimal fix). Prefer `terraform init` + `terraform plan` / `validate` as noted per fixture; avoid real cloud credentials — use `hashicorp/null` / local backend only.
