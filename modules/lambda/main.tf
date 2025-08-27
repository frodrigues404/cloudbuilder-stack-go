module "this" {
  source  = "terraform-aws-modules/lambda/aws"
  version = "~> 8.0"

  function_name                           = var.name
  description                             = var.description
  handler                                 = var.handler
  runtime                                 = "provided.al2023"
  tracing_mode                            = "Active"
  attach_policy_json                      = var.attach_policy_json
  attach_tracing_policy                   = true
  create_current_version_allowed_triggers = false
  timeout                                 = var.timeout
  architectures                           = ["arm64"]

  environment_variables = var.variables

  allowed_triggers = {
    APIGateway = {
      service    = "apigateway"
      source_arn = "${var.api_execution_arn}/*"
    },
  }

  policy_json = (var.attach_policy_json) ? var.policy_json : null

  source_path = [
    {
      path = var.path
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