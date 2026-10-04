# FIXED: one-way dependency (or remove depends_on when implicit refs suffice).
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
  triggers = {
    role = "a"
  }
}

resource "null_resource" "b" {
  depends_on = [null_resource.a]
  triggers = {
    role = "b"
    after = null_resource.a.id
  }
}
