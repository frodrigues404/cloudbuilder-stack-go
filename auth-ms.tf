module "register_user_lambda" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = "${var.project}-register-ms"
  description                             = "Register User in Cognito"
  handler                                 = "cmd/auth-ms/register/main.handler"
  runtime                                 = "provided.al2023"
  tracing_mode                            = "Active"
  attach_policy_json                      = false
  attach_tracing_policy                   = true
  create_current_version_allowed_triggers = false
  timeout                                 = 120
  architectures                           = ["arm64"]

  environment_variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }

  allowed_triggers = {
    APIGateway = {
      service    = "apigateway"
      source_arn = "${module.api_gateway.api_execution_arn}/*"
    },
  }

  source_path = [
    {
      path = "${path.module}/cmd/auth-ms/register"
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

module "login_lambda" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = "${var.project}-login-ms"
  description                             = "Login User in Cognito"
  handler                                 = "cmd/auth-ms/login/main.handler"
  runtime                                 = "provided.al2023"
  tracing_mode                            = "Active"
  attach_policy_json                      = false
  attach_tracing_policy                   = true
  create_current_version_allowed_triggers = false
  timeout                                 = 120
  architectures                           = ["arm64"]

  environment_variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }

  allowed_triggers = {
    APIGateway = {
      service    = "apigateway"
      source_arn = "${module.api_gateway.api_execution_arn}/*"
    },
  }

  source_path = [
    {
      path = "${path.module}/cmd/auth-ms/login"
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

module "confirm_email_lambda" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = "${var.project}-confirm-email-ms"
  description                             = "Confirm User Email in Cognito"
  handler                                 = "cmd/auth-ms/confirm-email/main.handler"
  runtime                                 = "provided.al2023"
  tracing_mode                            = "Active"
  attach_policy_json                      = false
  attach_tracing_policy                   = true
  create_current_version_allowed_triggers = false
  timeout                                 = 120
  architectures                           = ["arm64"]

  environment_variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }

  allowed_triggers = {
    APIGateway = {
      service    = "apigateway"
      source_arn = "${module.api_gateway.api_execution_arn}/*"
    },
  }

  source_path = [
    {
      path = "${path.module}/cmd/auth-ms/confirm-email"
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

module "delete_user_lambda" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = "${var.project}-delete-user-ms"
  description                             = "Delete User in Cognito"
  handler                                 = "cmd/auth-ms/delete-user/main.handler"
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
        Action   = ["cognito-idp:AdminDeleteUser"]
        Resource = aws_cognito_user_pool.user_pool.arn
      },
    ]
  })

  source_path = [
    {
      path = "${path.module}/cmd/auth-ms/delete-user"
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