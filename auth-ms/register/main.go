package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
)

type User struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func handler(ctx context.Context, user User) (string, error) {
	sess := session.Must(session.NewSession())
	svc := cognitoidentityprovider.New(sess)

	userPoolID := os.Getenv("USER_POOL_ID")

	input := &cognitoidentityprovider.AdminCreateUserInput{
		UserPoolId: &userPoolID,
		Username:   &user.Email,
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{Name: awsString("email"), Value: awsString(user.Email)},
			{Name: awsString("name"), Value: awsString(user.Name)},
		},
		MessageAction: awsString("SUPPRESS"),
	}

	_, err := svc.AdminCreateUser(input)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Usuário %s criado com sucesso", user.Email), nil
}

func awsString(s string) *string {
	return &s
}

func main() {
	lambda.Start(handler)
}
