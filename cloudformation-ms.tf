module "create_stack_lambda" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = "${var.project}-create-stack-ms"
  description                             = "Create CloudFormation Stack in target account"
  handler                                 = "cmd/cloudformation-ms/create-stack/main.handler"
  runtime                                 = "provided.al2023"
  tracing_mode                            = "Active"
  attach_policy_json                      = true
  attach_tracing_policy                   = true
  create_current_version_allowed_triggers = false
  timeout                                 = 120
  architectures                           = ["arm64"]

  environment_variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    USER_POOL_ID        = aws_cognito_user_pool.user_pool.id
    REGION              = var.region
  }

  allowed_triggers = {
    APIGateway = {
      service    = "apigateway"
      source_arn = "${module.api_gateway.api_execution_arn}/*"
    },
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

  source_path = [
    {
      path = "${path.module}/cmd/cloudformation-ms/create-stack/cmd/lambda"
      commands = [
        "GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bootstrap main.go",
        ":zip",
      ]
      patterns = [
        "!.*",
        "bootstrap",
      ]
    }
  ]
}