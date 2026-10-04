# BROKEN: for_each requires a map or set of strings, not a list.
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

variable "names" {
  type    = list(string)
  default = ["web", "api"]
}

resource "null_resource" "svc" {
  for_each = var.names

  triggers = {
    name = each.value
  }
}
