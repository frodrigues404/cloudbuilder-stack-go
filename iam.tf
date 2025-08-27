module "openapi-bucket-policy" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-policy"

  name        = "${var.project}-openapi-bucket"
  path        = "/"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "s3:getObject",
        ]
        Effect   = "Allow"
        Resource = "*"
      },
    ]
 })
}