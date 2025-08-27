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
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

type EmailConfirm struct {
	ConfirmationCode string `json:"code"`
	Username         string `json:"username"`
}

type CognitoActions struct {
	CognitoClient *cognitoidentityprovider.Client
}

func (actor CognitoActions) ConfirmEmail(ctx context.Context, clientId string, confirmationCode string, username string) (bool, error) {
	confirmed := false
	output, err := actor.CognitoClient.ConfirmSignUp(ctx, &cognitoidentityprovider.ConfirmSignUpInput{
		ClientId:         aws.String(clientId),
		Username:         aws.String(username),
		ConfirmationCode: aws.String(confirmationCode),
	})

	if err != nil {
		var codeMismatch *types.CodeMismatchException
		if errors.As(err, &codeMismatch) {
			log.Printf("%s", *codeMismatch.Message)
		} else {
			log.Printf("%s", "Couldn't confirm email. Here's why: "+err.Error())
		}
	} else if output != nil {
		log.Println("Confirmation code sent to:" + username)
	}
	confirmed = true
	return confirmed, err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var emailConfirm EmailConfirm
	err := json.Unmarshal([]byte(request.Body), &emailConfirm)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("Invalid request body: %v", err)}, nil
	}

	cfg := aws.Config{Region: os.Getenv("REGION")}
	cognitoClient := cognitoidentityprovider.NewFromConfig(cfg)
	actor := CognitoActions{CognitoClient: cognitoClient}
	clientId := os.Getenv("USER_POOL_CLIENT_ID")

	confirmed, err := actor.ConfirmEmail(context.TODO(), clientId, emailConfirm.ConfirmationCode, emailConfirm.Username)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Error confirming email: %v", err)}, nil
	}
	if !confirmed {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Email confirmation failed"}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Email confirmed successfully"}, nil
}

func main() {
	lambda.Start(handler)
}
