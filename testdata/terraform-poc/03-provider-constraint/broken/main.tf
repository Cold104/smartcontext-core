# BROKEN: impossible provider version — no registry release matches.
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "= 99.99.99"
    }
  }
}

resource "null_resource" "ok" {
  triggers = {
    note = "unreachable until constraint is fixed"
  }
}
