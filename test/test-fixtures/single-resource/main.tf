provider "aws" {
  version = "~> 2.0"

  profile = var.profile
  region  = var.region
}

resource "aws_vpc" "test" {
  cidr_block       = "10.0.0.0/16"

  tags = {
    Name = var.name
  }
}
