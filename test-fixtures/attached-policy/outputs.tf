output "role_name" {
  description = "The name of the test role"
  value = aws_iam_role.test.id
}