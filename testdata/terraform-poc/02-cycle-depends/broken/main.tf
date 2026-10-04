# BROKEN: mutual depends_on creates a cycle Terraform cannot order.
terraform {
  required_version = ">= 1.5.0"
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

resource "null_resource" "a" {
  depends_on = [null_resource.b]
  triggers = {
    role = "a"
  }
}

resource "null_resource" "b" {
  depends_on = [null_resource.a]
  triggers = {
    role = "b"
  }
}
