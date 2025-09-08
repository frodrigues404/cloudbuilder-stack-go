package main

import (
	"get-stacks-ms/internal/handler"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler.Handler)
}