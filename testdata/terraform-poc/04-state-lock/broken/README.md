# State lock (local backend)

Pre-placing `terraform.tfstate.lock.info` alone is **not** enough on Terraform 1.7+
local backend (OS file lock is authoritative). Reproduce a real lock:

```powershell
# Terminal A
.\hold_lock.ps1

# Terminal B (while A is sleeping in local-exec)
terraform plan -input=false
```

Expected: `Error: Error acquiring the state lock`.

After the apply finishes, plan succeeds again (see `../fixed/` for a clean tree).

Remote backends (S3+DynamoDB, etc.): confirm no live run, then `terraform force-unlock <LOCK_ID>`.
