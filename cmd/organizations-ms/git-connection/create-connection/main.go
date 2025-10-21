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

	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/aws/aws-sdk-go-v2/service/codestarconnections"
	ccTypes "github.com/aws/aws-sdk-go-v2/service/codestarconnections/types"

	sharedAuth "cloudformation-ms/shared/auth"
	sharedAWS "cloudformation-ms/shared/awsconfig"
	sharedCreds "cloudformation-ms/shared/credentials"
	sharedHTTP "cloudformation-ms/shared/httpresp"
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

// removed: accessKeys (use shared types)

// ---------- Deps for shared auth ----------
type deps struct{ cip *cip.Client }

func (d *deps) Cognito() *cip.Client { return d.cip }

// removed: resolveUsernameBySub (use shared auth)

// removed: getAccountCreds/buildTargetConfig (use shared credentials)

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

// removed: local http helpers (use shared httpresp)

// ---------- Lambda handler ----------

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Auth via API GW/Cognito
	if req.RequestContext.Authorizer.JWT == nil || req.RequestContext.Authorizer.JWT.Claims == nil {
		return sharedHTTP.Error(401, errors.New("unauthorized")), nil
	}
	claims := req.RequestContext.Authorizer.JWT.Claims // map[string]string no API GW HTTP API
	username := claims["cognito:username"]
	sub := claims["sub"]

	// Base cfg (role da Lambda)
	cfg, err := sharedAWS.Base(ctx)
	if err != nil {
		log.Println("aws config err:", err)
		return sharedHTTP.Error(500, errors.New("internal error")), nil
	}

	// Resolve owner (se faltar username)
	userPoolID := os.Getenv("USER_POOL_ID")
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

	// Body (suporta base64)
	raw := req.Body
	if req.IsBase64Encoded {
		b, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return sharedHTTP.Error(400, fmt.Errorf("invalid base64 body: %w", err)), nil
		}
		raw = string(b)
	}
	var body requestBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return sharedHTTP.Error(400, fmt.Errorf("invalid JSON body: %w", err)), nil
	}
	if strings.TrimSpace(body.ProviderType) == "" {
		return sharedHTTP.Error(400, errors.New("field 'providerType' is required")), nil
	}
	provider, err := normalizeProvider(body.ProviderType)
	if err != nil {
		return sharedHTTP.Error(400, err), nil
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
	smClient := sm.NewFromConfig(cfg)
	secretName := fmt.Sprintf("%s/%s/access_keys", owner, strings.TrimSpace(body.AccountName))
	keys, err := sharedCreds.GetAccountCreds(ctx, smClient, secretName)
	if err != nil {
		return sharedHTTP.Error(404, fmt.Errorf("failed to get credentials from secrets manager: %w", err)), nil
	}

	// target cfg com as credenciais do secret
	targetCfg, err := sharedCreds.BuildTargetConfig(ctx, cfg, keys)
	if err != nil {
		return sharedHTTP.Error(401, fmt.Errorf("invalid credentials for account '%s': %w", body.AccountName, err)), nil
	}

	// Client do CodeStar Connections NA CONTA-ALVO
	cc := codestarconnections.NewFromConfig(targetCfg)

	// Idempotência opcional por nome
	if body.IdempotentByName {
		if arn, st, found, err := listConnectionByName(ctx, cc, connectionName); err != nil {
			log.Printf("list connections error: %v", err)
			return sharedHTTP.Error(500, fmt.Errorf("failed to list connections: %w", err)), nil
		} else if found {
			resp := responseBody{
				Message:        "Connection already exists",
				ConnectionName: connectionName,
				ConnectionArn:  arn,
				Status:         st,
				Owner:          owner,
				Account:        body.AccountName,
			}
			return sharedHTTP.OK(200, resp), nil
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
				return sharedHTTP.OK(200, resp), nil
			}
		}
		log.Printf("create connection error: %v", createErr)
		return sharedHTTP.Error(500, fmt.Errorf("failed to create connection: %w", createErr)), nil
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
	return sharedHTTP.OK(201, resp), nil
}

func main() {
	lambda.Start(handler)
}
