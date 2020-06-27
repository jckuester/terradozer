provider "aws" {
  version = "~> 2.0"

  profile = var.profile
  region  = var.region
}

terraform {
  # The configuration for this backend will be filled in by Terragrunt
  backend "s3" {
  }
}

resource "aws_vpc" "test" {
  cidr_block = "10.0.0.0/16"

  tags = {
    Name       = var.name
    terradozer = "test-acc"
  }
}
