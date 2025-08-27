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

type User struct {
	Email    string `json:"email"`
	Name     string `json:"username"`
	Password string `json:"password"`
}

type CognitoActions struct {
	CognitoClient *cognitoidentityprovider.Client
}

// SignUp signs up a user with Amazon Cognito.
func (actor CognitoActions) SignUp(ctx context.Context, clientId string, userName string, password string, userEmail string) (bool, error) {
	confirmed := false
	output, err := actor.CognitoClient.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(clientId),
		Password: aws.String(password),
		Username: aws.String(userName),
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(userEmail)},
		},
	})
	if err != nil {
		var invalidPassword *types.InvalidPasswordException
		if errors.As(err, &invalidPassword) {
			log.Println(*invalidPassword.Message)
		} else {
			log.Printf("Couldn't sign up user %v. Here's why: %v\n", userName, err)
		}
	} else {
		confirmed = output.UserConfirmed
	}
	return confirmed, err
}

// Send a confirmation code to the user's email address.
func (actor CognitoActions) SendConfirmationCode(ctx context.Context, clientId string, userName string) error {
	_, err := actor.CognitoClient.ResendConfirmationCode(ctx, &cognitoidentityprovider.ResendConfirmationCodeInput{
		ClientId: aws.String(clientId),
		Username: aws.String(userName),
	})
	if err != nil {
		var userNotFound *types.UserNotFoundException
		if errors.As(err, &userNotFound) {
			log.Println(*userNotFound.Message)
		} else {
			log.Printf("Couldn't resend confirmation code to user %v. Here's why: %v\n", userName, err)
		}
	}
	return err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var user User
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
	confirmed, err := actor.SignUp(context.TODO(), clientId, user.Name, user.Password, user.Email)

	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Error signing up user: %v", err)}, nil
	}
	if !confirmed {
		// err := actor.SendConfirmationCode(context.TODO(), clientId, user.Name)
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Error sending confirmation code: %v", err)}, nil
		}
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: "User signed up but not confirmed. Please check your email for confirmation instructions."}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "User signed up and confirmed successfully."}, nil
}

func main() {
	lambda.Start(handler)
}
