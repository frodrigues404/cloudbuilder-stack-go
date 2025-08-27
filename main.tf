resource "aws_cognito_user_pool" "user_pool" {
  name                     = var.project
  auto_verified_attributes = ["email"]
  verification_message_template {
    default_email_option = "CONFIRM_WITH_CODE"
  }
  email_configuration {
    email_sending_account = "COGNITO_DEFAULT"
  }

}

resource "aws_cognito_user_pool_client" "client" {
  name         = var.project
  user_pool_id = aws_cognito_user_pool.user_pool.id

  explicit_auth_flows = ["USER_PASSWORD_AUTH"]
}

module "cost_tracker_dynamodb" {
  source  = "terraform-aws-modules/dynamodb-table/aws"
  version = "~> 5.0"

  name      = "aws_costs"
  hash_key  = "pk"
  range_key = "sk"

  attributes = [
    {
      name = "pk"
      type = "S"
    },
    {
      name = "sk"
      type = "S"
    }
  ]

}

module "cost_tracker_lambda" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = "lambda_cost_tracker"
  description                             = "Save Costs In DynamoDB"
  handler                                 = "cmd/cost-tracker-ms/main.handler"
  runtime                                 = "provided.al2023"
  attach_policy_json                      = true
  tracing_mode                            = "Active"
  attach_tracing_policy                   = true
  create_current_version_allowed_triggers = false
  timeout                                 = 120
  architectures                           = ["arm64"]

  environment_variables = {
    TABLE_NAME = module.cost_tracker_dynamodb.dynamodb_table_id
    REGION     = var.region
  }

  policy_json = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "dynamodb:PutItem",
                "dynamodb:Scan"
            ],
            "Resource": "${module.cost_tracker_dynamodb.dynamodb_table_arn}"
        },
        {
            "Effect": "Allow",
            "Action": [
                "ce:GetCostAndUsage"
            ],
            "Resource": "*"
        }
    ]
}
EOF
  source_path = [
    {
      path = "${path.module}/cmd/cost-tracker-ms"
      commands = [
        "go build -o bootstrap main.go",
        ":zip",
      ]
      patterns = [
        "!.*",
        "bootstrap",
      ]
    }
  ]

}