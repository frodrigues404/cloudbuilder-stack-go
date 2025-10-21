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
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	sharedAuth "cloudformation-ms/shared/auth"
	sharedAWS "cloudformation-ms/shared/awsconfig"
	sharedHTTP "cloudformation-ms/shared/httpresp"
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

type deps struct{ cip *cip.Client }

func (d *deps) Cognito() *cip.Client { return d.cip }

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

// removed: resolveUsernameBySub (use shared auth)

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.Authorizer.JWT == nil || req.RequestContext.Authorizer.JWT.Claims == nil {
		return sharedHTTP.Error(401, errors.New("unauthorized")), nil
	}
	claims := req.RequestContext.Authorizer.JWT.Claims

	var usernameAny, subAny any
	usernameAny = claims["cognito:username"]
	subAny = claims["sub"]

	username, _ := usernameAny.(string)
	sub, _ := subAny.(string)

	userPoolID := os.Getenv("USER_POOL_ID")
	cfg, err := sharedAWS.Base(ctx)
	if err != nil {
		log.Println("aws config err:", err)
		return sharedHTTP.Error(500, errors.New("internal error")), nil
	}
	if username == "" && userPoolID != "" && sub != "" {
		d := &deps{cip: cip.NewFromConfig(cfg)}
		if u, err := sharedAuth.OwnerFromRequest(ctx, req, cfg, d); err == nil && u != "" {
			username = u
		}
	}

	owner := username
	if owner == "" {
		owner = sub
	}
	if owner == "" {
		return sharedHTTP.Error(401, errors.New("unauthorized")), nil
	}

	rawBody := req.Body
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return sharedHTTP.Error(400, fmt.Errorf("invalid base64 body: %w", err)), nil
		}
		rawBody = string(decoded)
	}

	var body requestBody
	if err := json.Unmarshal([]byte(rawBody), &body); err != nil {
		return sharedHTTP.Error(400, fmt.Errorf("invalid JSON body: %w", err)), nil
	}
	if body.AccountName == "" {
		return sharedHTTP.Error(400, errors.New("field 'accountName' is required")), nil
	}
	if body.AccessKeyID == "" || body.SecretAccessKey == "" {
		return sharedHTTP.Error(400, errors.New("fields 'accessKeyId' and 'secretAccessKey' are required")), nil
	}

	secretName := fmt.Sprintf("%s/%s/access_keys", owner, body.AccountName)

	secretString, err := buildSecretString(body.AccessKeyID, body.SecretAccessKey)
	if err != nil {
		return sharedHTTP.Error(500, fmt.Errorf("failed to build secret payload: %w", err)), nil
	}

	smClient := sm.NewFromConfig(cfg)

	if body.Tags == nil {
		body.Tags = map[string]string{}
	}
	body.Tags["owner"] = owner
	body.Tags["account"] = body.AccountName

	var smTags []types.Tag
	for k, v := range body.Tags {
		smTags = append(smTags, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}

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
			return sharedHTTP.Error(500, fmt.Errorf("failed to update existing secret: %w", putErr)), nil
		}
		resp := responseBody{
			Message:    "Secret updated successfully",
			SecretName: secretName,
			VersionId:  aws.ToString(putOut.VersionId),
			Owner:      owner,
			Account:    body.AccountName,
		}
		return sharedHTTP.OK(200, resp), nil
	}

	if createErr != nil {
		log.Printf("create secret error: %v", createErr)
		return sharedHTTP.Error(500, fmt.Errorf("failed to create secret: %w", createErr)), nil
	}

	resp := responseBody{
		Message:    "Secret created successfully",
		SecretName: secretName,
		VersionId:  aws.ToString(createOut.VersionId),
		Owner:      owner,
		Account:    body.AccountName,
	}
	return sharedHTTP.OK(201, resp), nil
}

// removed local apiOK/apiError in favor of shared http resp

func main() {
	lambda.Start(handler)
}
