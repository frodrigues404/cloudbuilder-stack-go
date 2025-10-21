module "create_secret_keys_lambda" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = "${var.project}-create-keys-ms"
  description                             = "Create Access Keys and store in Secrets Manager"
  handler                                 = "cmd/organizations-ms/account-keys/create-account/main.handler"
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
        Effect = "Allow"
        Action = [
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
      path = "${path.module}/cmd/organizations-ms/account-keys/create-account"
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


module "create_codestar_connections_lambda" {
  source            = "./modules/lambda"
  name              = "${var.project}-git-connection-ms"
  description       = "Create CodeStar Connections"
  handler           = "${path.module}/cmd/organizations-ms/git-connection/create-connection/main.handler"
  path              = "${path.module}/cmd/organizations-ms/git-connection/create-connection"
  api_execution_arn = module.api_gateway.api_execution_arn
  variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }
  attach_policy_json = true
  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = "*"
      }
    ]
  })
}

module "list_codestar_connections_lambda" {
  source            = "./modules/lambda"
  name              = "${var.project}-list-git-connection-ms"
  description       = "List CodeStar Connections"
  handler           = "${path.module}/cmd/organizations-ms/git-connection/list-connections/main.handler"
  path              = "${path.module}/cmd/organizations-ms/git-connection/list-connections/"
  api_execution_arn = module.api_gateway.api_execution_arn
  variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }
  attach_policy_json = true
  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = "*"
      }
    ]
  })
}

module "list_account_lambda" {
  source            = "./modules/lambda"
  name              = "${var.project}-list-account-ms"
  description       = "List AWS Accounts"
  handler           = "${path.module}/cmd/organizations-ms/account-keys/list-accounts/main.handler"
  path              = "${path.module}/cmd/organizations-ms/account-keys/list-accounts/"
  api_execution_arn = module.api_gateway.api_execution_arn
  variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }
  attach_policy_json = true
  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["secretsmanager:ListSecrets"]
        Resource = "*"
      }
    ]
  })
}

module "create_beanstalk_env_lambda" {
  source            = "./modules/lambda"
  name              = "${var.project}-create-beanstalk-env-ms"
  description       = "Create Elastic Beanstalk Environment"
  handler           = "${path.module}/cmd/beanstalk-ms/main.handler"
  path              = "${path.module}/cmd/beanstalk-ms/"
  api_execution_arn = module.api_gateway.api_execution_arn
  variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }
  attach_policy_json = true
  policy_json = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["elasticbeanstalk:*"]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["s3:*"]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["codepipeline:*"]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["iam:*"]
        Resource = "*"
      }
    ]
  })
}