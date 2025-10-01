package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/aws/aws-sdk-go-v2/service/codestarconnections"
	ccTypes "github.com/aws/aws-sdk-go-v2/service/codestarconnections/types"
)

type requestBody struct {
	// Se não vier, geramos como <owner>/<accountName>
	ConnectionName string `json:"connectionName,omitempty"`
	// "GitHub" | "Bitbucket" | "GitLab" | "GitHubEnterpriseServer" | "GitLabSelfManaged"
	ProviderType string `json:"providerType"`
	// Obrigatório p/ provedores self-managed (GHES/GitLab Self-Managed)
	HostArn          string            `json:"hostArn,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
	Description      string            `json:"description,omitempty"` // apenas informativo
	IdempotentByName bool              `json:"idempotentByName,omitempty"`
	AccountName      string            `json:"accountName,omitempty"` // usado p/ gerar nome padrão
}

type responseBody struct {
	Message        string `json:"message"`
	ConnectionName string `json:"connectionName"`
	ConnectionArn  string `json:"connectionArn,omitempty"`
	Status         string `json:"status,omitempty"`
	Owner          string `json:"owner,omitempty"`
	Account        string `json:"account,omitempty"`
}

type accessKeys struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken,omitempty"`
}

// ---------- AWS helpers ----------

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

// ---------- Auth helpers ----------

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

// ---------- Secrets & target config ----------

func getAccountCreds(ctx context.Context, smClient *sm.Client, secretName string) (*accessKeys, error) {
	out, err := smClient.GetSecretValue(ctx, &sm.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return nil, err
	}
	var keys accessKeys
	if err := json.Unmarshal([]byte(aws.ToString(out.SecretString)), &keys); err != nil {
		return nil, err
	}
	if keys.AccessKeyID == "" || keys.SecretAccessKey == "" {
		return nil, fmt.Errorf("secret %s missing accessKeyId/secretAccessKey", secretName)
	}
	return &keys, nil
}

func buildTargetConfig(ctx context.Context, base aws.Config, keys *accessKeys) (aws.Config, error) {
	cfg := base
	cfg.Credentials = aws.NewCredentialsCache(
		credentials.NewStaticCredentialsProvider(keys.AccessKeyID, keys.SecretAccessKey, keys.SessionToken),
	)
	// Se quiser forçar região aqui (ou vindo do body/env), descomente:
	// cfg.Region = "us-east-1"
	return cfg, nil
}

// ---------- CodeStar Connections helpers ----------

func normalizeProvider(s string) (ccTypes.ProviderType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "github":
		return ccTypes.ProviderTypeGithub, nil
	case "bitbucket":
		return ccTypes.ProviderTypeBitbucket, nil
	case "gitlab":
		return ccTypes.ProviderTypeGitlab, nil
	case "githubenterpriseserver", "github_enterprise_server", "ghes":
		return ccTypes.ProviderTypeGithubEnterpriseServer, nil
	case "gitlabselfmanaged", "gitlab_self_managed":
		return ccTypes.ProviderTypeGitlabSelfManaged, nil
	default:
		// permite passar um valor desconhecido, mas a API provavelmente rejeitará
		pt := ccTypes.ProviderType(s)
		if strings.TrimSpace(s) == "" {
			return "", errors.New("field 'providerType' is required")
		}
		return pt, nil
	}
}

func listConnectionByName(ctx context.Context, client *codestarconnections.Client, name string) (arn string, status string, found bool, err error) {
	p := codestarconnections.NewListConnectionsPaginator(client, &codestarconnections.ListConnectionsInput{
		// ProviderTypeFilter: []ccTypes.ProviderType{...} // opcional
		MaxResults: 50,
	})
	for p.HasMorePages() {
		page, e := p.NextPage(ctx)
		if e != nil {
			return "", "", false, e
		}
		for _, c := range page.Connections {
			if aws.ToString(c.ConnectionName) == name {
				return aws.ToString(c.ConnectionArn), string(c.ConnectionStatus), true, nil
			}
		}
	}
	return "", "", false, nil
}

func getConnectionStatus(ctx context.Context, client *codestarconnections.Client, arn string) (string, error) {
	out, err := client.GetConnection(ctx, &codestarconnections.GetConnectionInput{
		ConnectionArn: aws.String(arn),
	})
	if err != nil {
		return "", err
	}
	return string(out.Connection.ConnectionStatus), nil
}

// ---------- HTTP helpers ----------

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

// ---------- Lambda handler ----------

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Auth via API GW/Cognito
	if req.RequestContext.Authorizer.JWT == nil || req.RequestContext.Authorizer.JWT.Claims == nil {
		return apiError(401, errors.New("unauthorized")), nil
	}
	claims := req.RequestContext.Authorizer.JWT.Claims // map[string]string no API GW HTTP API
	username := claims["cognito:username"]
	sub := claims["sub"]

	// Base cfg (role da Lambda)
	cfg, err := newAWS(ctx)
	if err != nil {
		log.Println("aws config err:", err)
		return apiError(500, errors.New("internal error")), nil
	}

	// Resolve owner (se faltar username)
	userPoolID := os.Getenv("USER_POOL_ID")
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

	// Body (suporta base64)
	raw := req.Body
	if req.IsBase64Encoded {
		b, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return apiError(400, fmt.Errorf("invalid base64 body: %w", err)), nil
		}
		raw = string(b)
	}
	var body requestBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return apiError(400, fmt.Errorf("invalid JSON body: %w", err)), nil
	}
	if strings.TrimSpace(body.ProviderType) == "" {
		return apiError(400, errors.New("field 'providerType' is required")), nil
	}
	provider, err := normalizeProvider(body.ProviderType)
	if err != nil {
		return apiError(400, err), nil
	}

	// Gera connectionName padrão se não veio
	connectionName := strings.TrimSpace(body.ConnectionName)
	if connectionName == "" {
		acc := strings.TrimSpace(body.AccountName)
		if acc == "" {
			acc = "default"
		}
		connectionName = fmt.Sprintf("%s/%s", owner, acc)
	}

	// Monta secret name e pega credenciais da conta-alvo
	smClient := newSecretsClient(cfg)
	secretName := fmt.Sprintf("%s/%s/access_keys", owner, strings.TrimSpace(body.AccountName))
	keys, err := getAccountCreds(ctx, smClient, secretName)
	if err != nil {
		return apiError(404, fmt.Errorf("failed to get credentials from secrets manager: %w", err)), nil
	}

	// target cfg com as credenciais do secret
	targetCfg, err := buildTargetConfig(ctx, cfg, keys)
	if err != nil {
		return apiError(401, fmt.Errorf("invalid credentials for account '%s': %w", body.AccountName, err)), nil
	}

	// Client do CodeStar Connections NA CONTA-ALVO
	cc := codestarconnections.NewFromConfig(targetCfg)

	// Idempotência opcional por nome
	if body.IdempotentByName {
		if arn, st, found, err := listConnectionByName(ctx, cc, connectionName); err != nil {
			log.Printf("list connections error: %v", err)
			return apiError(500, fmt.Errorf("failed to list connections: %w", err)), nil
		} else if found {
			resp := responseBody{
				Message:        "Connection already exists",
				ConnectionName: connectionName,
				ConnectionArn:  arn,
				Status:         st,
				Owner:          owner,
				Account:        body.AccountName,
			}
			return apiOK(200, resp), nil
		}
	}

	// Monta tags
	var tags []ccTypes.Tag
	if body.Tags == nil {
		body.Tags = map[string]string{}
	}
	body.Tags["owner"] = owner
	for k, v := range body.Tags {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		tags = append(tags, ccTypes.Tag{Key: aws.String(k), Value: aws.String(v)})
	}

	// Monta input de criação
	in := &codestarconnections.CreateConnectionInput{
		ConnectionName: aws.String(connectionName),
		ProviderType:   provider, // enum (valor)
		Tags:           tags,
	}
	h := strings.TrimSpace(body.HostArn)
	if h != "" {
		in.HostArn = aws.String(h) // necessário em GHES / GitLab Self-Managed
	}

	// Cria a connection
	createOut, createErr := cc.CreateConnection(ctx, in)
	if createErr != nil {
		// Se já existir, tenta retornar dados (algumas regiões/contas retornam ResourceAlreadyExists)
		var already *ccTypes.ResourceAlreadyExistsException
		if errors.As(createErr, &already) {
			if arn, st, found, err := listConnectionByName(ctx, cc, connectionName); err == nil && found {
				resp := responseBody{
					Message:        "Connection already exists",
					ConnectionName: connectionName,
					ConnectionArn:  arn,
					Status:         st,
					Owner:          owner,
					Account:        body.AccountName,
				}
				return apiOK(200, resp), nil
			}
		}
		log.Printf("create connection error: %v", createErr)
		return apiError(500, fmt.Errorf("failed to create connection: %w", createErr)), nil
	}

	arn := aws.ToString(createOut.ConnectionArn)
	status, _ := getConnectionStatus(ctx, cc, arn) // best-effort

	resp := responseBody{
		Message:        "Connection created (Pending until you authorize it in the Console)",
		ConnectionName: connectionName,
		ConnectionArn:  arn,
		Status:         status,
		Owner:          owner,
		Account:        body.AccountName,
	}
	return apiOK(201, resp), nil
}

func main() {
	lambda.Start(handler)
}
