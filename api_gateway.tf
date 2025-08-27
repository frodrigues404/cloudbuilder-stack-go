module "api_gateway" {
  source  = "terraform-aws-modules/apigateway-v2/aws"
  version = "~> 5.0"

  name                  = "${var.project}-${var.environment}-api"
  description           = "${var.project} project API"
  protocol_type         = "HTTP"
  create_certificate    = false
  create_domain_name    = false
  create_domain_records = false
  create_routes_and_integrations = false

  authorizers = {
    cognito = {
      authorizer_type  = "JWT"
      identity_sources = ["$request.header.Authorization"]

      jwt_configuration = {
        audience = [aws_cognito_user_pool_client.client.id]
        issuer   = "https://${aws_cognito_user_pool.user_pool.endpoint}"
      }
    }
  }

  routes = {
    # "GET /week-costs" = {
    #   integration = {
    #     uri                    = module.cost_tracker_lambda.lambda_function_arn
    #     payload_format_version = "2.0"
    #   }
    # },
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
}