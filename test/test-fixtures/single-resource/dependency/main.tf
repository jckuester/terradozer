provider "aws" {
  version = "~> 2.0"

  profile = var.profile
  region  = var.region
}

resource "aws_subnet" "test" {
  availability_zone = "us-west-2b"

  vpc_id = var.vpc_id
  cidr_block = "10.0.1.0/24"

  tags = {
    Name = var.name
  }
}
