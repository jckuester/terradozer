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

resource "aws_s3_bucket" "test" {
  bucket = var.name

  tags = {
    Name       = var.name
    terradozer = "test-acc"
  }
}

resource "aws_s3_bucket_object" "object" {
  bucket = aws_s3_bucket.test.bucket
  key    = "test_object"
  source = "test.txt"
}
