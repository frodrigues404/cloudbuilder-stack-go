output "api_gateway_url" {
  value = module.api_gateway.api_endpoint
}

output "aws_cognito_user_pool_client_id" {
  value = aws_cognito_user_pool_client.client.id
}