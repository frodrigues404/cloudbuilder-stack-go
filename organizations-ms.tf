module "create_secret_keys_lambda" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = "${var.project}-create-keys-ms"
  description                             = "Create Access Keys and store in Secrets Manager"
  handler                                 = "cmd/organizations-ms/create-key/main.handler"
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
    Version   = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = [
            "secretsmanager:CreateSecret", 
            "secretsmanager:PutSecretValue", 
            "secretsmanager:UpdateSecret",
            "secretsmanager:TagResource"
        ]
        Resource = "*"
      },
    ]
  })

  source_path = [
    {
      path = "${path.module}/cmd/organizations-ms/create-key"
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