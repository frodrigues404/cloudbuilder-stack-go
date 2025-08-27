package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type requestBody struct {
	AccountName     string            `json:"accountName"`
	AccessKeyID     string            `json:"accessKeyId"`
	SecretAccessKey string            `json:"secretAccessKey"`
	Description     string            `json:"description,omitempty"`
	Tags            map[string]string `json:"tags,omitempty"`
}

type responseBody struct {
	Message    string `json:"message"`
	SecretName string `json:"secretName"`
	VersionId  string `json:"versionId,omitempty"`
	Owner      string `json:"owner,omitempty"`
	Account    string `json:"account,omitempty"`
}

func newAWS(ctx context.Context) (aws.Config, error) {
	if region := os.Getenv("AWS_REGION"); region == "" {
		if def := os.Getenv("AWS_DEFAULT_REGION"); def != "" {
			os.Setenv("AWS_REGION", def)
		}
	}
	return config.LoadDefaultConfig(ctx)
}

func newSecretsClient(cfg aws.Config) *sm.Client {
	return sm.NewFromConfig(cfg)
}

func newCognitoClient(cfg aws.Config) *cip.Client {
	return cip.NewFromConfig(cfg)
}

func buildSecretString(accessKeyID, secretAccessKey string) (string, error) {
	payload := map[string]string{
		"accessKeyId":     accessKeyID,
		"secretAccessKey": secretAccessKey,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func resolveUsernameBySub(ctx context.Context, client *cip.Client, userPoolID, sub string) (string, error) {
	out, err := client.ListUsers(ctx, &cip.ListUsersInput{
		UserPoolId: aws.String(userPoolID),
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

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// ===== Auth (Cognito JWT authorizer) =====
	if req.RequestContext.Authorizer.JWT == nil || req.RequestContext.Authorizer.JWT.Claims == nil {
		return apiError(401, errors.New("unauthorized")), nil
	}
	claims := req.RequestContext.Authorizer.JWT.Claims

	var usernameAny, subAny any
	usernameAny = claims["cognito:username"]
	subAny = claims["sub"]

	username, _ := usernameAny.(string)
	sub, _ := subAny.(string)

	userPoolID := os.Getenv("USER_POOL_ID")
	cfg, err := newAWS(ctx)
	if err != nil {
		log.Println("aws config err:", err)
		return apiError(500, errors.New("internal error")), nil
	}
	if username == "" && userPoolID != "" && sub != "" {
		if u, err := resolveUsernameBySub(ctx, newCognitoClient(cfg), userPoolID, sub); err == nil {
			username = u
		}
	}

	owner := username
	if owner == "" {
		owner = sub
	}
	if owner == "" {
		return apiError(401, errors.New("unauthorized")), nil
	}

	// ===== Parse body =====
	rawBody := req.Body
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return apiError(400, fmt.Errorf("invalid base64 body: %w", err)), nil
		}
		rawBody = string(decoded)
	}

	var body requestBody
	if err := json.Unmarshal([]byte(rawBody), &body); err != nil {
		return apiError(400, fmt.Errorf("invalid JSON body: %w", err)), nil
	}
	if body.AccountName == "" {
		return apiError(400, errors.New("field 'accountName' is required")), nil
	}
	if body.AccessKeyID == "" || body.SecretAccessKey == "" {
		return apiError(400, errors.New("fields 'accessKeyId' and 'secretAccessKey' are required")), nil
	}

	// Nome do secret baseado em usuario + conta
	// Ex.: username/account_name/access_keys
	secretName := fmt.Sprintf("%s/%s/access_keys", owner, body.AccountName)

	secretString, err := buildSecretString(body.AccessKeyID, body.SecretAccessKey)
	if err != nil {
		return apiError(500, fmt.Errorf("failed to build secret payload: %w", err)), nil
	}

	smClient := newSecretsClient(cfg)

	// Tags (inclui owner e account)
	if body.Tags == nil {
		body.Tags = map[string]string{}
	}
	body.Tags["owner"] = owner
	body.Tags["account"] = body.AccountName

	var smTags []types.Tag
	for k, v := range body.Tags {
		smTags = append(smTags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}

	// Tenta criar o secret; se já existir, atualiza o valor
	createOut, createErr := smClient.CreateSecret(ctx, &sm.CreateSecretInput{
		Name:         aws.String(secretName),
		Description:  aws.String(body.Description),
		SecretString: aws.String(secretString),
		Tags:         smTags,
	})

	var exists *types.ResourceExistsException
	if createErr != nil && errors.As(createErr, &exists) {
		putOut, putErr := smClient.PutSecretValue(ctx, &sm.PutSecretValueInput{
			SecretId:     aws.String(secretName),
			SecretString: aws.String(secretString),
		})
		if putErr != nil {
			log.Printf("put secret value error: %v", putErr)
			return apiError(500, fmt.Errorf("failed to update existing secret: %w", putErr)), nil
		}
		resp := responseBody{
			Message:    "Secret updated successfully",
			SecretName: secretName,
			VersionId:  aws.ToString(putOut.VersionId),
			Owner:      owner,
			Account:    body.AccountName,
		}
		return apiOK(200, resp), nil
	}

	if createErr != nil {
		log.Printf("create secret error: %v", createErr)
		return apiError(500, fmt.Errorf("failed to create secret: %w", createErr)), nil
	}

	resp := responseBody{
		Message:    "Secret created successfully",
		SecretName: secretName,
		VersionId:  aws.ToString(createOut.VersionId),
		Owner:      owner,
		Account:    body.AccountName,
	}
	return apiOK(201, resp), nil
}

func apiOK(status int, payload responseBody) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(payload)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: string(b),
	}
}

func apiError(status int, err error) events.APIGatewayV2HTTPResponse {
	out := map[string]string{"message": err.Error()}
	b, _ := json.Marshal(out)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: string(b),
	}
}

func main() {
	lambda.Start(handler)
}
