# FIXED: wrap list with toset() so for_each receives a set of strings.
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
  for_each = toset(var.names)

  triggers = {
    name = each.value
  }
}
