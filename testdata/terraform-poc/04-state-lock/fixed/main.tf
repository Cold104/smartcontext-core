# FIXED: no stale lock.info — plan/apply can acquire the lock.
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
    note = "lock cleared; safe to plan"
  }
}
