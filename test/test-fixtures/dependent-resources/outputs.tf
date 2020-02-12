output "vpc_id" {
  value = aws_vpc.test.id
}

output "role_name" {
  description = "The name of the test role"
  value = aws_iam_role.test.id
}

output "policy_arn" {
  description = "The ARN of the test policy"
  value = aws_iam_policy.test.arn
}