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

resource "aws_ecs_cluster" "test" {
  name = var.name
  tags = {
    terradozer = "test-acc"
  }
}
