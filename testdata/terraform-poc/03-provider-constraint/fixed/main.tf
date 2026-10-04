# FIXED: realistic constraint that the registry can satisfy.
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

resource "null_resource" "ok" {
  triggers = {
    note = "init succeeds with a matching provider"
  }
}
