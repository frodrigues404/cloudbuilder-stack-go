package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	csc "github.com/aws/aws-sdk-go-v2/service/codestarconnections"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// --------------------------------------------------------
// Structs e tipos locais
// --------------------------------------------------------

type GetConnectionsResponseBody struct {
	Message     string                   `json:"message"`
	Account     string                   `json:"account"`
	Owner       string                   `json:"owner"`
	Connections []CodeStarConnectionInfo `json:"connections"`
	RetrievedAt string                   `json:"retrieved_at"`
}

type CodeStarConnectionInfo struct {
	ConnectionName string `json:"connectionName"`
	ConnectionArn  string `json:"connectionArn"`
	ProviderType   string `json:"providerType"`
	Status         string `json:"status"`
}

type SecretKeys struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
}

// --------------------------------------------------------
// Utilitários
// --------------------------------------------------------

// Extrai o "owner" do token JWT (mock simplificado)
func getOwnerFromJWT(req events.APIGatewayV2HTTPRequest) (string, error) {
	authHeader := req.Headers["authorization"]
	if authHeader == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid Authorization header format")
	}
	token := parts[1]
	if !strings.Contains(token, ".") {
		return "", errors.New("invalid JWT format")
	}
	return "ferrodrigues", nil
}

// Busca as chaves do Secrets Manager
func getAccountCreds(ctx context.Context, smClient *sm.Client, secretName string) (*SecretKeys, error) {
	out, err := smClient.GetSecretValue(ctx, &sm.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return nil, err
	}
	var keys SecretKeys
	if err := json.Unmarshal([]byte(*out.SecretString), &keys); err != nil {
		return nil, fmt.Errorf("invalid secret JSON: %w", err)
	}
	return &keys, nil
}

// Cria o config AWS usando apenas AccessKey + SecretAccessKey
func buildTargetConfig(ctx context.Context, baseCfg aws.Config, keys *SecretKeys) (aws.Config, error) {
	customCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(baseCfg.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				keys.AccessKeyID,
				keys.SecretAccessKey,
				"", // sem session token
			),
		),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to build target AWS config: %w", err)
	}

	// Valida se as credenciais funcionam
	stsClient := sts.NewFromConfig(customCfg)
	id, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return aws.Config{}, fmt.Errorf("invalid credentials (STS): %w", err)
	}
	log.Printf("[INFO] Using credentials from account: %s | ARN: %s", aws.ToString(id.Account), aws.ToString(id.Arn))

	return customCfg, nil
}

// Resposta padrão JSON
func jsonResponse(status int, body interface{}) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(body)
	return events.APIGatewayV2HTTPResponse{
		StatusCode:      status,
		Headers:         map[string]string{"Content-Type": "application/json"},
		Body:            string(b),
		IsBase64Encoded: false,
	}
}

// --------------------------------------------------------
// Handler principal
// --------------------------------------------------------

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("[INFO] ListConnections request: %s %s", req.RequestContext.HTTP.Method, req.RequestContext.HTTP.Path)

	accountName := req.QueryStringParameters["accountName"]
	if accountName == "" {
		return jsonResponse(400, map[string]string{"message": "accountName is required"}), nil
	}

	// Config base da Lambda
	baseCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return jsonResponse(500, map[string]string{"message": fmt.Sprintf("failed to load AWS config: %v", err)}), nil
	}

	// Extrai owner (mock do Cognito)
	owner, err := getOwnerFromJWT(req)
	if err != nil {
		return jsonResponse(401, map[string]string{"message": fmt.Sprintf("unauthorized: %v", err)}), nil
	}

	// Busca secret do Secrets Manager
	secretName := fmt.Sprintf("%s/%s/access_keys", owner, accountName)
	smClient := sm.NewFromConfig(baseCfg)
	keys, err := getAccountCreds(ctx, smClient, secretName)
	if err != nil {
		return jsonResponse(400, map[string]string{"message": fmt.Sprintf("failed to get account credentials: %v", err)}), nil
	}

	// Monta config autenticada
	targetCfg, err := buildTargetConfig(ctx, baseCfg, keys)
	if err != nil {
		return jsonResponse(400, map[string]string{"message": fmt.Sprintf("failed to build target config: %v", err)}), nil
	}

	// Lista conexões do CodeStar
	connClient := csc.NewFromConfig(targetCfg)
	out, err := connClient.ListConnections(ctx, &csc.ListConnectionsInput{})
	if err != nil {
		return jsonResponse(400, map[string]string{"message": fmt.Sprintf("failed to list codestar connections: %v", err)}), nil
	}

	connections := make([]CodeStarConnectionInfo, 0, len(out.Connections))
	for _, conn := range out.Connections {
		name := aws.ToString(conn.ConnectionName)
		if strings.Contains(strings.ToLower(name), "shared") {
			continue
		}
		connections = append(connections, CodeStarConnectionInfo{
			ConnectionName: name,
			ConnectionArn:  aws.ToString(conn.ConnectionArn),
			ProviderType:   string(conn.ProviderType),
			Status:         string(conn.ConnectionStatus),
		})
	}

	resp := GetConnectionsResponseBody{
		Message:     "connections retrieved successfully",
		Account:     accountName,
		Owner:       owner,
		Connections: connections,
		RetrievedAt: time.Now().Format(time.RFC3339),
	}

	return jsonResponse(200, resp), nil
}

// --------------------------------------------------------
// Entrypoint da Lambda
// --------------------------------------------------------

func main() {
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" {
		fmt.Println("Running locally for debug...")
	}
	lambda.Start(Handler)
}
