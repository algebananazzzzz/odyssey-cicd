terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  default_tags {
    tags = {
      Environment = var.env
      Project     = var.project_code
      ManagedBy   = "terraform"
    }
  }
}
