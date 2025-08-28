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
	"github.com/aws/aws-xray-sdk-go/v2/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/v2/xray"
)

type User struct {
	Email    string `json:"email"`
	Name     string `json:"username"`
	Password string `json:"password"`
}

type CognitoActions struct {
	CognitoClient *cognitoidentityprovider.Client
}

func (actor CognitoActions) SignUp(ctx context.Context, clientId, userName, password, userEmail string) (bool, error) {
	var (
		confirmed bool
		err       error
	)

	xray.Capture(ctx, "Cognito.SignUp", func(ctx context.Context) error {
		output, e := actor.CognitoClient.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
			ClientId: aws.String(clientId),
			Password: aws.String(password),
			Username: aws.String(userName),
			UserAttributes: []types.AttributeType{
				{Name: aws.String("email"), Value: aws.String(userEmail)},
			},
		})
		if e != nil {
			xray.AddError(ctx, e)
			err = e
			return e
		}
		confirmed = output.UserConfirmed
		return nil
	})

	if err != nil {
		var invalidPassword *types.InvalidPasswordException
		if errors.As(err, &invalidPassword) {
			log.Println(*invalidPassword.Message)
		} else {
			log.Printf("Couldn't sign up user %v. Here's why: %v\n", userName, err)
		}
	}
	return confirmed, err
}

func (actor CognitoActions) SendConfirmationCode(ctx context.Context, clientId, userName string) error {
	var err error
	xray.Capture(ctx, "Cognito.ResendConfirmationCode", func(ctx context.Context) error {
		_, e := actor.CognitoClient.ResendConfirmationCode(ctx, &cognitoidentityprovider.ResendConfirmationCodeInput{
			ClientId: aws.String(clientId),
			Username: aws.String(userName),
		})
		if e != nil {
			xray.AddError(ctx, e)
			err = e
			return e
		}
		return nil
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

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var user User

	if err := xray.Capture(ctx, "Parse.RequestBody", func(ctx context.Context) error {
		return json.Unmarshal([]byte(request.Body), &user)
	}); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("Invalid request body: %v", err)}, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(os.Getenv("REGION")))
	if err != nil {
		xray.AddError(ctx, err)
		log.Fatalf("unable to load SDK config, %v", err)
	}
	awsv2.AWSV2Instrumentor(&cfg.APIOptions) // ← ponto chave

	cognitoClient := cognitoidentityprovider.NewFromConfig(cfg)
	actor := CognitoActions{CognitoClient: cognitoClient}

	clientId := os.Getenv("USER_POOL_CLIENT_ID")

	confirmed, err := actor.SignUp(ctx, clientId, user.Name, user.Password, user.Email)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: fmt.Sprintf("Error signing up user: %v", err)}, nil
	}
	if !confirmed {
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       "User signed up but not confirmed. Please check your email for confirmation instructions.",
		}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "User signed up and confirmed successfully."}, nil
}

func main() {
	lambda.Start(handler)
}
