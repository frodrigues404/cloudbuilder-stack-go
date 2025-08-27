package main

import (
	"context"

	getcosts "cost-tracker/getcosts"

	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context) (string, error) {
	err := getcosts.GetCosts()
	if err != nil {
		return "", err
	}

	return "Custos atualizados com sucesso!", nil
}

func main() {
	lambda.Start(handler)
}
