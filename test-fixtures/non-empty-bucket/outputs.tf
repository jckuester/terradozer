output "bucket_name" {
  description = "The name of the test bucket"
  value = aws_s3_bucket.test.bucket
}