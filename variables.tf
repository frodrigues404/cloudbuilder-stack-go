locals {

  routes = {
    "POST /register" = {
      integration = {
        uri                    = module.register_user_lambda.lambda_function_arn
        payload_format_version = "2.0"
      }
    }
    "POST /login" = {
      integration = {
        uri                    = module.login_lambda.lambda_function_arn
        payload_format_version = "2.0"
      }
    }
    "POST /verify-email" = {
      integration = {
        uri                    = module.confirm_email_lambda.lambda_function_arn
        payload_format_version = "2.0"
      }
    }
    "DELETE /user" = {
      integration = {
        uri                    = module.delete_user_lambda.lambda_function_arn
        payload_format_version = "2.0"
      }
      authorization_type = "JWT"
      authorizer_key     = "cognito"
    }
    "POST /register-keys" = {
      integration = {
        uri                    = module.create_secret_keys_lambda.lambda_function_arn
        payload_format_version = "2.0"
      }
      authorization_type = "JWT"
      authorizer_key     = "cognito"
    }
    "POST /create-stack" = {
      integration = {
        uri                    = module.create_stack_lambda.lambda_function_arn
        payload_format_version = "2.0"
      }
      authorization_type = "JWT"
      authorizer_key     = "cognito"
    }
  }

  tags = {
    Environment = var.environment
    Project     = var.project
  }
}

variable "environment" {
  default = "development"
}

variable "project" {
  default = "cloudbuilder"
}

variable "region" {
  default = "us-east-1"
}