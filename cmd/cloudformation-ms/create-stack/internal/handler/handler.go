package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cft "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"create-stack-ms/internal/auth"
	"create-stack-ms/internal/awsconfig"
	"create-stack-ms/internal/cfn"
	"create-stack-ms/internal/credentials"
	"create-stack-ms/internal/httpresp"
	"create-stack-ms/internal/types"
)

// deps simples para injetar clientes Cognito/SM se precisar (facilita testes)
type deps struct {
	cip *cip.Client
	sm  *sm.Client
}

func (d *deps) Cognito() *cip.Client { return d.cip }

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("[INFO] Incoming request: reqId=%s method=%s path=%s",
		req.RequestContext.RequestID, req.RequestContext.HTTP.Method, req.RawPath)

	// ---- SDK base ----
	cfg, err := awsconfig.Base(ctx)
	if err != nil {
		return httpresp.Error(500, fmt.Errorf("aws config error: %w", err)), nil
	}

	d := &deps{
		cip: cip.NewFromConfig(cfg),
		sm:  sm.NewFromConfig(cfg),
	}

	// ---- Auth (Cognito) ----
	owner, err := auth.OwnerFromRequest(ctx, req, cfg, d)
	if err != nil || owner == "" {
		return httpresp.Error(401, errors.New("unauthorized")), nil
	}
	log.Printf("[INFO] Authenticated owner=%s", owner)

	// ---- Parse body ----
	raw := req.Body
	if req.IsBase64Encoded {
		b, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return httpresp.Error(400, fmt.Errorf("invalid base64 body: %w", err)), nil
		}
		raw = string(b)
	}

	var body types.RequestBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return httpresp.Error(400, fmt.Errorf("invalid JSON body: %w", err)), nil
	}
	log.Printf("[INFO] Payload summary: accountName=%s stackName=%s templateInline=%t templateUrlSet=%t params=%d tags=%d caps=%v",
		body.AccountName, body.StackName, len(body.Template) > 0, body.TemplateURL != "", len(body.Parameters), len(body.Tags), body.Capabilities)

	if body.AccountName == "" || body.StackName == "" {
		return httpresp.Error(400, errors.New("fields 'accountName' and 'stackName' are required")), nil
	}

	// Template (inline ou URL)
	var templateBody *string
	if len(body.Template) > 0 {
		var tmp map[string]any
		if err := json.Unmarshal(body.Template, &tmp); err != nil {
			return httpresp.Error(400, fmt.Errorf("template must be valid JSON: %v", err)), nil
		}
		str := string(body.Template)
		if len(str) > 51200 {
			return httpresp.Error(400, errors.New("template exceeds 51,200 bytes (use templateUrl instead)")), nil
		}
		templateBody = &str
		log.Printf("[INFO] Using inline template (size=%d bytes)", len(str))
	} else if body.TemplateURL != "" {
		log.Printf("[INFO] Using template URL: %s", body.TemplateURL)
	} else {
		return httpresp.Error(400, errors.New("either 'template' or 'templateUrl' is required")), nil
	}

	// ---- Secrets Manager: credenciais da conta alvo ----
	secretName := fmt.Sprintf("%s/%s/access_keys", owner, body.AccountName)
	log.Printf("[INFO] Fetching credentials from secret: %s", secretName)

	keys, err := credentials.GetAccountCreds(ctx, d.sm, secretName)
	if err != nil {
		return httpresp.Error(404, fmt.Errorf("failed to get credentials from secrets manager: %w", err)), nil
	}

	// ---- Config alvo (credenciais / assume role / sts check) ----
	targetCfg, err := credentials.BuildTargetConfig(ctx, cfg, keys)
	if err != nil {
		return httpresp.Error(401, fmt.Errorf("invalid credentials for account '%s': %w", body.AccountName, err)), nil
	}

	// ---- CloudFormation: CreateStack (não aguarda conclusão) ----
	cfnClient := cf.NewFromConfig(targetCfg)

	in := &cf.CreateStackInput{
		StackName:    &body.StackName,
		Capabilities: cfn.Capabilities(body.Capabilities),
		Parameters:   cfn.Parameters(body.Parameters),
		Tags:         cfn.Tags(body.Tags),
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
		return httpresp.Error(400, fmt.Errorf("invalid onFailure: %s", body.OnFailure)), nil
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

	out, err := cfnClient.CreateStack(ctx, in)
	if err != nil {
		return httpresp.Error(400, fmt.Errorf("create stack failed: %w", err)), nil
	}
	log.Printf("[INFO] CreateStack started successfully: stackId=%s", aws.ToString(out.StackId))

	// Retorna imediatamente, sem esperar o completion
	resp := types.ResponseBody{
		Message:   "stack creation started",
		StackID:   aws.ToString(out.StackId),
		StackName: body.StackName,
		Account:   body.AccountName,
		Owner:     owner,
		Status:    "CREATE_IN_PROGRESS",
	}
	return httpresp.OK(200, resp), nil
}
