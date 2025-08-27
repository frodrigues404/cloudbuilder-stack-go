module "create_stack_lambda" {
  source             = "./modules/lambda"
  name               = "${var.project}-create-stack-ms"
  description        = "Create CloudFormation Stack in target account"
  handler            = "${path.module}/cmd/cloudformation-ms/create-stack/main.handler"
  path               = "${path.module}/cmd/cloudformation-ms/create-stack/cmd/lambda"
  api_execution_arn  = module.api_gateway.api_execution_arn
  attach_policy_json = true
  variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    USER_POOL_ID        = aws_cognito_user_pool.user_pool.id
    REGION              = var.region
  }
  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["cloudformation:*"]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = "*"
      }
    ]
  })
}