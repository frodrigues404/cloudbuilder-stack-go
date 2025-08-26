module "api_gateway" {
  source  = "terraform-aws-modules/apigateway-v2/aws"
  version = "~> 5.0"

  name                  = "${var.project}-${var.environment}-api"
  description           = "${var.project} project API"
  protocol_type         = "HTTP"
  create_certificate    = false
  create_domain_name    = false
  create_domain_records = false

  routes = {
    "GET /week-costs" = {
      integration = {
        uri = module.cost_tracker_lambda.lambda_function_arn
      }
    },
    "POST /register" = {
      integration = {
        uri = module.register_user_lambda.lambda_function_arn
      }
    }
    "POST /login" = {
      integration = {
        uri = module.login_lambda.lambda_function_arn
      }
    }
  }

  tags = local.tags
}