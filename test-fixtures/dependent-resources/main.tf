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

resource "aws_iam_role" "test" {
  name = "test_role"

  assume_role_policy = data.aws_iam_policy_document.role.json

  tags = {
    tag-key = "tag-value"
  }
}

data "aws_iam_policy_document" "role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["s3.amazonaws.com"]
    }
  }
}

resource "aws_iam_policy" "test" {
  name        = "test_policy"
  path        = "/"
  description = "My test policy"

  policy = data.aws_iam_policy_document.policy.json
}

data "aws_iam_policy_document" "policy" {
  statement {
    actions = [
      "s3:ListAllMyBuckets",
    ]

    resources = [
      "*",
    ]
  }
}

resource "aws_iam_role_policy_attachment" "this" {
  role       = aws_iam_role.test.name
  policy_arn = aws_iam_policy.test.arn
}
