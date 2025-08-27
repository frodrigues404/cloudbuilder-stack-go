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
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cft "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type secretKeys struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken,omitempty"`
	RoleARN         string `json:"roleArn,omitempty"`
	ExternalID      string `json:"externalId,omitempty"`
}

type requestBody struct {
	AccountName        string            `json:"accountName"`
	StackName          string            `json:"stackName"`
	Template           json.RawMessage   `json:"template"`              // Inline JSON (TemplateBody)
	TemplateURL        string            `json:"templateUrl,omitempty"` // Alternativa: URL
	Parameters         map[string]string `json:"parameters,omitempty"`
	Capabilities       []string          `json:"capabilities,omitempty"` // ["CAPABILITY_IAM", "CAPABILITY_NAMED_IAM", "CAPABILITY_AUTO_EXPAND"]
	RoleARN            string            `json:"roleArn,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
	OnFailure          string            `json:"onFailure,omitempty"` // "DO_NOTHING" | "ROLLBACK" | "DELETE"
	DisableRollback    *bool             `json:"disableRollback,omitempty"`
	TimeoutInMinutes   *int32            `json:"timeoutInMinutes,omitempty"`
	ClientRequestToken string            `json:"clientRequestToken,omitempty"`
}

type responseBody struct {
	Message   string `json:"message"`
	StackID   string `json:"stackId,omitempty"`
	StackName string `json:"stackName,omitempty"`
	Account   string `json:"account,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Status    string `json:"status,omitempty"`
}

func awsConfigBase(ctx context.Context) (aws.Config, error) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") != "" {
		os.Setenv("AWS_REGION", os.Getenv("AWS_DEFAULT_REGION"))
	}
	return config.LoadDefaultConfig(ctx)
}

func newSecretsClient(cfg aws.Config) *sm.Client  { return sm.NewFromConfig(cfg) }
func newCognitoClient(cfg aws.Config) *cip.Client { return cip.NewFromConfig(cfg) }

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

func getAccountCreds(ctx context.Context, smc *sm.Client, secretName string) (secretKeys, error) {
	out, err := smc.GetSecretValue(ctx, &sm.GetSecretValueInput{SecretId: &secretName})
	if err != nil {
		return secretKeys{}, err
	}
	var sk secretKeys
	if out.SecretString == nil {
		return secretKeys{}, errors.New("secret has no SecretString")
	}
	if err := json.Unmarshal([]byte(*out.SecretString), &sk); err != nil {
		return secretKeys{}, fmt.Errorf("invalid secret json: %w", err)
	}
	if sk.AccessKeyID == "" || sk.SecretAccessKey == "" {
		return secretKeys{}, errors.New("secret missing accessKeyId or secretAccessKey")
	}
	return sk, nil
}

func cfCapabilities(vals []string) []cft.Capability {
	var out []cft.Capability
	for _, v := range vals {
		switch strings.ToUpper(strings.TrimSpace(v)) {
		case "CAPABILITY_IAM":
			out = append(out, cft.CapabilityCapabilityIam)
		case "CAPABILITY_NAMED_IAM":
			out = append(out, cft.CapabilityCapabilityNamedIam)
		case "CAPABILITY_AUTO_EXPAND":
			out = append(out, cft.CapabilityCapabilityAutoExpand)
		}
	}
	return out
}

func cfParameters(m map[string]string) []cft.Parameter {
	if len(m) == 0 {
		return nil
	}
	out := make([]cft.Parameter, 0, len(m))
	for k, v := range m {
		key := k
		val := v
		out = append(out, cft.Parameter{
			ParameterKey:   &key,
			ParameterValue: &val,
		})
	}
	return out
}

func cfTags(m map[string]string) []cft.Tag {
	if len(m) == 0 {
		return nil
	}
	out := make([]cft.Tag, 0, len(m))
	for k, v := range m {
		kc, vc := k, v
		out = append(out, cft.Tag{Key: &kc, Value: &vc})
	}
	return out
}

func buildTargetConfig(ctx context.Context, base aws.Config, keys secretKeys) (aws.Config, error) {
	target := base

	if keys.AccessKeyID != "" && keys.SecretAccessKey != "" {
		target.Credentials = aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(
				keys.AccessKeyID, keys.SecretAccessKey, keys.SessionToken,
			),
		)
	}

	// AssumeRole (opcional)
	if keys.RoleARN != "" {
		stsClient := sts.NewFromConfig(target)
		sessionName := fmt.Sprintf("cfn-%d", time.Now().Unix())
		p := stscreds.NewAssumeRoleProvider(stsClient, keys.RoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = sessionName
			if keys.ExternalID != "" {
				o.ExternalID = &keys.ExternalID
			}
		})
		target.Credentials = aws.NewCredentialsCache(p)
		log.Printf("[INFO] Using AssumeRole: roleArn=%s sessionName=%s", keys.RoleARN, sessionName)
	}

	// Validação STS
	idOut, err := sts.NewFromConfig(target).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return target, fmt.Errorf("STS GetCallerIdentity failed (creds inválidas/expiradas?): %w", err)
	}
	log.Printf("[INFO] Caller identity: Account=%s ARN=%s UserId=%s",
		aws.ToString(idOut.Account), aws.ToString(idOut.Arn), aws.ToString(idOut.UserId))

	return target, nil
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("[INFO] Incoming request: reqId=%s method=%s path=%s",
		req.RequestContext.RequestID, req.RequestContext.HTTP.Method, req.RawPath)

	if req.RequestContext.Authorizer.JWT == nil || req.RequestContext.Authorizer.JWT.Claims == nil {
		return apiError(401, errors.New("unauthorized")), nil
	}
	claims := req.RequestContext.Authorizer.JWT.Claims
	var owner string
	if u, ok := claims["cognito:username"]; ok && u != "" {
		owner = u
	} else if sub, ok := claims["sub"]; ok && sub != "" {
		owner = sub
		if up := os.Getenv("USER_POOL_ID"); up != "" {
			cfg, _ := awsConfigBase(ctx)
			if resolved, err := resolveUsernameBySub(ctx, newCognitoClient(cfg), up, sub); err == nil && resolved != "" {
				owner = resolved
			}
		}
	}
	if owner == "" {
		return apiError(401, errors.New("unauthorized")), nil
	}
	log.Printf("[INFO] Authenticated owner=%s", owner)

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
	log.Printf("[INFO] Payload summary: accountName=%s stackName=%s templateInline=%t templateUrlSet=%t params=%d tags=%d caps=%v",
		body.AccountName, body.StackName, len(body.Template) > 0, body.TemplateURL != "", len(body.Parameters), len(body.Tags), body.Capabilities)

	if body.AccountName == "" || body.StackName == "" {
		return apiError(400, errors.New("fields 'accountName' and 'stackName' are required")), nil
	}

	var templateBody *string
	if len(body.Template) > 0 {
		var tmp map[string]any
		if err := json.Unmarshal(body.Template, &tmp); err != nil {
			return apiError(400, fmt.Errorf("template must be valid JSON: %v", err)), nil
		}
		str := string(body.Template)
		if len(str) > 51200 {
			return apiError(400, errors.New("template exceeds 51,200 bytes (use templateUrl instead)")), nil
		}
		templateBody = &str
		log.Printf("[INFO] Using inline template (size=%d bytes)", len(str))
	} else if body.TemplateURL != "" {
		log.Printf("[INFO] Using template URL: %s", body.TemplateURL)
	} else {
		return apiError(400, errors.New("either 'template' or 'templateUrl' is required")), nil
	}

	cfg, err := awsConfigBase(ctx)
	if err != nil {
		return apiError(500, fmt.Errorf("aws config error: %w", err)), nil
	}
	smClient := newSecretsClient(cfg)

	secretName := fmt.Sprintf("%s/%s/access_keys", owner, body.AccountName)
	log.Printf("[INFO] Fetching credentials from secret: %s", secretName)

	keys, err := getAccountCreds(ctx, smClient, secretName)
	if err != nil {
		return apiError(404, fmt.Errorf("failed to get credentials from secrets manager: %w", err)), nil
	}

	targetCfg, err := buildTargetConfig(ctx, cfg, keys)
	if err != nil {
		return apiError(401, fmt.Errorf("invalid credentials for account '%s': %w", body.AccountName, err)), nil
	}

	cfn := cf.NewFromConfig(targetCfg)

	in := &cf.CreateStackInput{
		StackName:    &body.StackName,
		Capabilities: cfCapabilities(body.Capabilities),
		Parameters:   cfParameters(body.Parameters),
		Tags:         cfTags(body.Tags),
	}
	if body.ClientRequestToken != "" {
		in.ClientRequestToken = aws.String(body.ClientRequestToken)
	}
	if body.DisableRollback != nil {
		in.DisableRollback = body.DisableRollback
	}
	if body.TimeoutInMinutes != nil {
		in.TimeoutInMinutes = body.TimeoutInMinutes
	}
	if templateBody != nil {
		in.TemplateBody = templateBody
	} else {
		in.TemplateURL = aws.String(body.TemplateURL)
	}
	switch strings.ToUpper(body.OnFailure) {
	case "ROLLBACK":
		in.OnFailure = cft.OnFailureRollback
	case "DELETE":
		in.OnFailure = cft.OnFailureDelete
	case "DO_NOTHING", "":
		in.OnFailure = cft.OnFailureDoNothing
	default:
		return apiError(400, fmt.Errorf("invalid onFailure: %s", body.OnFailure)), nil
	}

	log.Printf("[INFO] Calling CreateStack: stackName=%s onFailure=%s caps=%v params=%d tags=%d templateMode=%s",
		body.StackName, in.OnFailure, body.Capabilities, len(in.Parameters), len(in.Tags),
		func() string {
			if templateBody != nil {
				return "INLINE"
			}
			return "URL"
		}(),
	)

	out, err := cfn.CreateStack(ctx, in)
	if err != nil {
		return apiError(400, fmt.Errorf("create stack failed: %w", err)), nil
	}

	log.Printf("[INFO] CreateStack started successfully: stackId=%s", aws.ToString(out.StackId))

	resp := responseBody{
		Message:   "stack creation started",
		StackID:   aws.ToString(out.StackId),
		StackName: body.StackName,
		Account:   body.AccountName,
		Owner:     owner,
		Status:    "CREATE_IN_PROGRESS",
	}
	return apiOK(200, resp), nil
}

func apiOK(status int, payload responseBody) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(payload)
	log.Printf("[INFO] Response %d: %s", status, string(b))
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
	log.Printf("[ERROR] Response %d: %s", status, string(b))
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: string(b),
	}
}

func main() { lambda.Start(handler) }
