# Creates non-empty default local state, then switches backend path so the next
# `terraform init -input=false` refuses interactive migration approval.
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
Remove-Item -Recurse -Force .terraform, terraform.tfstate, terraform.tfstate.backup, moved.tfstate, other.tfstate -ErrorAction SilentlyContinue

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

resource "null_resource" "backend_demo" {
  triggers = {
    note = "seed state before backend path change"
  }
}
'@ | Set-Content -Path .\main.tf -Encoding utf8

terraform init -input=false -no-color
terraform apply -auto-approve -input=false -no-color

@'
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }

  backend "local" {
    path = "moved.tfstate"
  }
}

resource "null_resource" "backend_demo" {
  triggers = {
    note = "init must acknowledge backend change"
  }
}
'@ | Set-Content -Path .\main.tf -Encoding utf8

Write-Host "Now run: terraform init -input=false"
Write-Host "Expect: Can't ask approval for state migration when interactive input is disabled."
Write-Host "        (interactive TF 1.5 often says: Backend configuration changed)"
Write-Host "Fix:  'yes' | terraform init -migrate-state -no-color"
Write-Host "  or: 'no'  | terraform init -migrate-state -no-color   # empty new backend"
