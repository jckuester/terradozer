provider "aws" {
  version = "~> 2.0"

  profile = var.profile
  region  = var.region
}

resource "aws_s3_bucket" "test" {
  bucket = var.name

  tags = {
    Name = var.name
  }
}

resource "aws_s3_bucket_object" "object" {
  bucket = aws_s3_bucket.test.bucket
  key    = "test_object"
  source = "test.txt"
}