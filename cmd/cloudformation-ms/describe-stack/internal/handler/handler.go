package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cft "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"cloudformation-ms/shared/auth"
	"cloudformation-ms/shared/awsconfig"
	"cloudformation-ms/shared/httpresp"
)

type deps struct {
	cip *cip.Client
	sm  *sm.Client
}

func (d *deps) Cognito() *cip.Client { return d.cip }

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("[INFO] Incoming request: reqId=%s method=%s path=%s",
		req.RequestContext.RequestID, req.RequestContext.HTTP.Method, req.RawPath)

	cfg, err := awsconfig.Base(ctx)
	if err != nil {
		return httpresp.Error(500, fmt.Errorf("aws config error: %w", err)), nil
	}

	d := &deps{
		cip: cip.NewFromConfig(cfg),
		sm:  sm.NewFromConfig(cfg),
	}

	owner, err := auth.OwnerFromRequest(ctx, req, cfg, d)
	if err != nil || owner == "" {
		return httpresp.Error(401, errors.New("unauthorized")), nil
	}

	// Obter o nome da stack dos parâmetros de consulta (query parameters)
	stackName := req.QueryStringParameters["stackName"]
	if stackName == "" {
		return httpresp.Error(400, errors.New("stackName query parameter is required")), nil
	}

	// Obter o nome da conta dos parâmetros de consulta (query parameters)
	accountName := req.QueryStringParameters["accountName"]
	if accountName == "" {
		return httpresp.Error(400, errors.New("accountName query parameter is required")), nil
	}

	// Gerar o nome do segredo com base no accountName e owner
	secretName := fmt.Sprintf("%s-%s", owner, accountName)
	log.Printf("[INFO] Generated secret name: %s", secretName)

	// Usar a configuração base para este exemplo
	// Em um cenário real, você implementaria a função ForTarget no pacote awsconfig
	targetCfg := cfg

	cfClient := cf.NewFromConfig(targetCfg)

	// Descrever a stack
	resp, err := cfClient.DescribeStacks(ctx, &cf.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		log.Printf("[ERROR] Failed to describe stack: %v", err)
		return httpresp.Error(500, fmt.Errorf("failed to describe stack: %w", err)), nil
	}

	if len(resp.Stacks) == 0 {
		return httpresp.Error(404, fmt.Errorf("stack %s not found", stackName)), nil
	}

	// Converter para o formato de resposta
	stack := resp.Stacks[0]
	stackDetails := map[string]interface{}{
		"stackId":           *stack.StackId,
		"stackName":         *stack.StackName,
		"description":       aws.ToString(stack.Description),
		"creationTime":      stack.CreationTime.String(),
		"lastUpdatedTime":   getLastUpdatedTime(stack.LastUpdatedTime),
		"stackStatus":       string(stack.StackStatus),
		"stackStatusReason": aws.ToString(stack.StackStatusReason),
		"disableRollback":   stack.DisableRollback,
		"notificationARNs":  stack.NotificationARNs,
		"roleARN":           aws.ToString(stack.RoleARN),
		"tags":              convertTags(stack.Tags),
		"outputs":           convertOutputs(stack.Outputs),
		"parameters":        convertParameters(stack.Parameters),
		"capabilities":      convertCapabilities(stack.Capabilities),
	}

	return httpresp.OK(200, stackDetails), nil
}

func getLastUpdatedTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.String()
}

func convertTags(tags []cft.Tag) []map[string]string {
	result := make([]map[string]string, len(tags))
	for i, tag := range tags {
		result[i] = map[string]string{
			"key":   *tag.Key,
			"value": *tag.Value,
		}
	}
	return result
}

func convertOutputs(outputs []cft.Output) []map[string]string {
	result := make([]map[string]string, len(outputs))
	for i, output := range outputs {
		result[i] = map[string]string{
			"outputKey":   *output.OutputKey,
			"outputValue": aws.ToString(output.OutputValue),
			"description": aws.ToString(output.Description),
			"exportName":  aws.ToString(output.ExportName),
		}
	}
	return result
}

func convertParameters(params []cft.Parameter) []map[string]interface{} {
	result := make([]map[string]interface{}, len(params))
	for i, param := range params {
		result[i] = map[string]interface{}{
			"parameterKey":     *param.ParameterKey,
			"parameterValue":   aws.ToString(param.ParameterValue),
			"usePreviousValue": param.UsePreviousValue,
			"resolvedValue":    aws.ToString(param.ResolvedValue),
		}
	}
	return result
}

func convertCapabilities(capabilities []cft.Capability) []string {
	result := make([]string, len(capabilities))
	for i, capability := range capabilities {
		result[i] = string(capability)
	}
	return result
}
