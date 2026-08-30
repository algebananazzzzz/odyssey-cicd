terraform {
  required_version = ">= 1.14"

  # R2 speaks the S3 API; Terraform ships no R2 backend.
  backend "s3" {}
}
