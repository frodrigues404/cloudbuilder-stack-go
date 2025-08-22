package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handler(ctx context.Context, req LoginRequest) (string, error) {
	sess := session.Must(session.NewSession())
	svc := cognitoidentityprovider.New(sess)

	clientID := os.Getenv("CLIENT_ID")

	authInput := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: aws.String("USER_PASSWORD_AUTH"),
		ClientId: aws.String(clientID),
		AuthParameters: map[string]*string{
			"USERNAME": aws.String(req.Email),
			"PASSWORD": aws.String(req.Password),
		},
	}

	resp, err := svc.InitiateAuth(authInput)
	if err != nil {
		return "", fmt.Errorf("falha no login: %v", err)
	}

	return fmt.Sprintf("Login OK. AccessToken: %s", *resp.AuthenticationResult.AccessToken), nil
}

func main() {
	lambda.Start(handler)
}
