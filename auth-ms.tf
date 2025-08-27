module "register_user_lambda" {
  source            = "./modules/lambda"
  name              = "${var.project}-register-ms"
  description       = "Register User in Cognito"
  handler           = "${path.module}/cmd/auth-ms/register/main.handler"
  path              = "${path.module}/cmd/auth-ms/register"
  api_execution_arn = module.api_gateway.api_execution_arn
  variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }
}

module "login_lambda" {
  source            = "./modules/lambda"
  name              = "${var.project}-login-ms"
  description       = "Login User in Cognito"
  handler           = "${path.module}/cmd/auth-ms/login/main.handler"
  path              = "${path.module}/cmd/auth-ms/login"
  api_execution_arn = module.api_gateway.api_execution_arn
  variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }
}

module "confirm_email_lambda" {
  source            = "./modules/lambda"
  name              = "${var.project}-confirm-email-ms"
  description       = "Confirm User Email in Cognito"
  handler           = "${path.module}/cmd/auth-ms/confirm-email/main.handler"
  path              = "${path.module}/cmd/auth-ms/confirm-email"
  api_execution_arn = module.api_gateway.api_execution_arn
  variables = {
    USER_POOL_CLIENT_ID = aws_cognito_user_pool_client.client.id
    REGION              = var.region
  }
}

module "delete_user_lambda" {
  source             = "./modules/lambda"
  name               = "${var.project}-delete-user-ms"
  description        = "Delete User in Cognito"
  handler            = "cmd/auth-ms/delete-user/main.handler"
  path               = "${path.module}/cmd/auth-ms/delete-user"
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
        Action   = ["cognito-idp:AdminDeleteUser"]
        Resource = aws_cognito_user_pool.user_pool.arn
      },
    ]
  })
}