package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

type EmailConfirm struct {
	ConfirmationCode string `json:"code"`
}

type CognitoActions struct {
	CognitoClient *cognitoidentityprovider.Client
}

func (actor CognitoActions) ConfirmEmail(ctx context.Context, clientId string, confirmationCode string) (bool, error) {
	confirmed := false
	output, err := actor.CognitoClient.ConfirmSignUp(ctx, &cognitoidentityprovider.ConfirmSignUpInput{
		ClientId:         aws.String(clientId),
		ConfirmationCode: aws.String(confirmationCode),
	})
	if err != nil {
		var codeMismatch *types.CodeMismatchException
		if errors.As(err, &codeMismatch) {
			log.Panic(*codeMismatch.Message)
		} else {
			log.Panic("Couldn't confirm email. Here's why: " + err.Error())
		}
	}
	_ = output
	confirmed = true
	return confirmed, err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var emailConfirm EmailConfirm
	err := json.Unmarshal([]byte(request.Body), &emailConfirm)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("Invalid request body: %v", err)}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Email confirmed successfully"}, nil
}
