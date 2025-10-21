package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"list-stacks/internal/handler"
)

func HandleRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return handler.Handler(ctx, req)
}

func main() {
	lambda.Start(HandleRequest)
}