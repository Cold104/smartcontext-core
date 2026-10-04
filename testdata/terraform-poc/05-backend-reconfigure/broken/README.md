# Backend configuration changed

Reproduce:

```powershell
# From this directory, simulate prior init with default terraform.tfstate:
Remove-Item -Recurse -Force .terraform -ErrorAction SilentlyContinue
# Temporarily comment out backend block, init, then restore backend and init again
# OR use setup_broken.ps1
```

Expected: `Error: Backend configuration changed` — run `terraform init -reconfigure`
(or `-migrate-state` when preserving state to the new backend).
