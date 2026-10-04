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
