variable "profile" {
  description = "The named profile for the AWS account that will be deployed to"
}

variable "region" {
  description = "The AWS region to deploy to"
}

variable "name" {
  description = "The name of test"
}

variable "vpc_id" {
  description = "The ID of the VPC to create a subnet in"
}