package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
)

func resolveUsernameBySub(ctx context.Context, client *cip.Client, userPoolID, sub string) (string, error) {
	out, err := client.ListUsers(ctx, &cip.ListUsersInput{
		UserPoolId: &userPoolID,
		Filter:     aws.String(fmt.Sprintf(`sub = "%s"`, sub)),
		Limit:      aws.Int32(1),
	})
	if err != nil {
		return "", err
	}
	if len(out.Users) == 0 || out.Users[0].Username == nil {
		return "", errors.New("user not found by sub")
	}
	return *out.Users[0].Username, nil
}

type CognitoDeps interface {
	Cognito() *cip.Client
}

func OwnerFromRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest, cfg aws.Config, deps CognitoDeps) (string, error) {
	if req.RequestContext.Authorizer.JWT == nil || req.RequestContext.Authorizer.JWT.Claims == nil {
		return "", errors.New("unauthorized")
	}
	claims := req.RequestContext.Authorizer.JWT.Claims

	if u, ok := claims["cognito:username"]; ok && u != "" {
		log.Printf("[INFO] Auth owner from cognito:username=%s", u)
		return u, nil
	}
	if sub, ok := claims["sub"]; ok && sub != "" {
		owner := sub
		log.Printf("[INFO] Auth owner from sub=%s", sub)
		if up := os.Getenv("USER_POOL_ID"); up != "" && deps != nil && deps.Cognito() != nil {
			if resolved, err := resolveUsernameBySub(ctx, deps.Cognito(), up, sub); err == nil && resolved != "" {
				log.Printf("[INFO] Resolved username by sub: %s -> %s", sub, resolved)
				owner = resolved
			}
		}
		return owner, nil
	}
	return "", errors.New("unauthorized: no username/sub in claims")
}
