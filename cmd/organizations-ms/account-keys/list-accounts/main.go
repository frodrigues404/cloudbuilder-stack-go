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
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// --------------------------------------------------------
// Tipos
// --------------------------------------------------------

type AccountInfo struct {
	AccountName string    `json:"accountName"`
	SecretName  string    `json:"secretName"`
	CreatedDate time.Time `json:"createdDate"`
}

type ListAccountsResponse struct {
	Message     string        `json:"message"`
	Owner       string        `json:"owner"`
	Accounts    []AccountInfo `json:"accounts"`
	RetrievedAt string        `json:"retrieved_at"`
}

// --------------------------------------------------------
// Utils
// --------------------------------------------------------

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
	// mock simplificado
	return "ferrodrigues", nil
}

// Filtra os secrets de um owner específico (ex: ferrodrigues/)
func filterSecretsByOwner(secrets []smtypes.SecretListEntry, owner string) []AccountInfo {
	var accounts []AccountInfo
	prefix := fmt.Sprintf("%s/", owner)
	for _, s := range secrets {
		name := aws.ToString(s.Name)
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		// queremos só secrets que terminam com "/access_keys"
		if !strings.HasSuffix(name, "/access_keys") {
			continue
		}
		// extrai o nome da conta do padrão: owner/account/access_keys
		parts := strings.Split(name, "/")
		if len(parts) < 3 {
			continue
		}
		accountName := parts[1]
		accounts = append(accounts, AccountInfo{
			AccountName: accountName,
			SecretName:  name,
			CreatedDate: aws.ToTime(s.CreatedDate),
		})
	}
	return accounts
}

// Resposta JSON padrão
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
	log.Printf("[INFO] ListAccounts request: %s %s", req.RequestContext.HTTP.Method, req.RequestContext.HTTP.Path)

	// Owner via JWT
	owner, err := getOwnerFromJWT(req)
	if err != nil {
		return jsonResponse(401, map[string]string{"message": fmt.Sprintf("unauthorized: %v", err)}), nil
	}

	// Config padrão da Lambda
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return jsonResponse(500, map[string]string{"message": fmt.Sprintf("failed to load AWS config: %v", err)}), nil
	}

	smClient := sm.NewFromConfig(cfg)

	// Lista secrets do usuário
	var allSecrets []smtypes.SecretListEntry
	var nextToken *string
	for {
		out, err := smClient.ListSecrets(ctx, &sm.ListSecretsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return jsonResponse(500, map[string]string{"message": fmt.Sprintf("failed to list secrets: %v", err)}), nil
		}
		allSecrets = append(allSecrets, out.SecretList...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	// Filtra secrets do owner
	accounts := filterSecretsByOwner(allSecrets, owner)
	log.Printf("[INFO] Found %d accounts for user '%s'", len(accounts), owner)

	resp := ListAccountsResponse{
		Message:     "accounts retrieved successfully",
		Owner:       owner,
		Accounts:    accounts,
		RetrievedAt: time.Now().Format(time.RFC3339),
	}

	return jsonResponse(200, resp), nil
}

// --------------------------------------------------------
// Main
// --------------------------------------------------------

func main() {
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" {
		fmt.Println("Running locally for debug...")
	}
	lambda.Start(Handler)
}
