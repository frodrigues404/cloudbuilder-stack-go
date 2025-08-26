package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CognitoActions struct {
	CognitoClient *cognitoidentityprovider.Client
}

func (actor CognitoActions) SignIn(ctx context.Context, clientId string, userName string, password string) (*types.AuthenticationResultType, error) {
	var authResult *types.AuthenticationResultType
	output, err := actor.CognitoClient.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow:       "USER_PASSWORD_AUTH",
		ClientId:       aws.String(clientId),
		AuthParameters: map[string]string{"USERNAME": userName, "PASSWORD": password},
	})
	if err != nil {
		var resetRequired *types.PasswordResetRequiredException
		if errors.As(err, &resetRequired) {
			log.Println(*resetRequired.Message)
		} else {
			log.Printf("Couldn't sign in user %v. Here's why: %v\n", userName, err)
		}
	} else {
		authResult = output.AuthenticationResult
	}
	return authResult, err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var user LoginRequest
	err := json.Unmarshal([]byte(request.Body), &user)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("Invalid request body: %v", err)}, nil
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	cognitoClient := cognitoidentityprovider.NewFromConfig(cfg)
	actor := CognitoActions{CognitoClient: cognitoClient}

	clientId := os.Getenv("USER_POOL_CLIENT_ID")
	authResult, err := actor.SignIn(context.TODO(), clientId, user.Username, user.Password)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Error signing in user: %v", err)}, nil
	}

	if authResult == nil {
		return events.APIGatewayProxyResponse{StatusCode: 401, Body: "Authentication failed"}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: 200, Body: *authResult.AccessToken}, nil
}

func main() {
	lambda.Start(handler)
}
