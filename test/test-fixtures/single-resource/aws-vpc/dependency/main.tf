provider "aws" {
  version = "~> 3.0"

  profile = var.profile
  region  = var.region
}

terraform {
  # The configuration for this backend will be filled in by Terragrunt
  backend "s3" {
  }
}

resource "aws_subnet" "test" {
  availability_zone = "us-west-2b"

  vpc_id     = var.vpc_id
  cidr_block = "10.0.1.0/24"

  tags = {
    Name       = var.name
    terradozer = "test-acc"
  }
}
