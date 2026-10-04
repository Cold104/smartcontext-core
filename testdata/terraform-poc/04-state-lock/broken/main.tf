# BROKEN setup: use hold_lock.ps1 for concurrent lock reproduction.
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
    note = "use hold_lock.ps1 then plan in another shell"
  }
}
