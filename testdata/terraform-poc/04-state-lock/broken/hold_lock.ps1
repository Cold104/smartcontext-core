# Holds the local state lock via a long provisioner so a second `terraform plan`
# in another shell fails with "Error acquiring the state lock".
# Usage: .\hold_lock.ps1   then in another terminal: terraform plan -input=false
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
@'
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

resource "null_resource" "locked" {
  triggers = {
    note = "plan fails while lock.info is present"
    nonce = timestamp()
  }
  provisioner "local-exec" {
    command = "powershell -Command Start-Sleep -Seconds 40"
  }
}
'@ | Set-Content -Path .\main.tf -Encoding utf8
terraform init -input=false -no-color
Write-Host "Starting apply (holds lock ~40s). In another shell: terraform plan -input=false"
terraform apply -auto-approve -input=false -no-color
