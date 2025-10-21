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

	// Listar todas as stacks
	resp, err := cfClient.ListStacks(ctx, &cf.ListStacksInput{
		// Opcionalmente, você pode filtrar por status de stack
		// StackStatusFilter: []cft.StackStatus{
		//     cft.StackStatusCreateComplete,
		//     cft.StackStatusUpdateComplete,
		// },
	})
	if err != nil {
		log.Printf("[ERROR] Failed to list stacks: %v", err)
		return httpresp.Error(500, fmt.Errorf("failed to list stacks: %w", err)), nil
	}

	// Converter para o formato de resposta
	stacks := make([]map[string]interface{}, 0, len(resp.StackSummaries))
	for _, stack := range resp.StackSummaries {
		stackSummary := map[string]interface{}{
			"stackId":             aws.ToString(stack.StackId),
			"stackName":           aws.ToString(stack.StackName),
			"creationTime":        stack.CreationTime.String(),
			"lastUpdatedTime":     getLastUpdatedTime(stack.LastUpdatedTime),
			"stackStatus":         string(stack.StackStatus),
			"stackStatusReason":   aws.ToString(stack.StackStatusReason),
			"templateDescription": aws.ToString(stack.TemplateDescription),
			"driftInformation":    convertDriftInfo(stack.DriftInformation),
		}
		stacks = append(stacks, stackSummary)
	}

	// Retornar a resposta
	return httpresp.OK(200, map[string]interface{}{
		"stacks": stacks,
	}), nil
}

// Função auxiliar para obter o último horário de atualização
func getLastUpdatedTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.String()
}

// Função auxiliar para converter informações de drift
func convertDriftInfo(info *cft.StackDriftInformationSummary) map[string]interface{} {
	if info == nil {
		return nil
	}
	
	return map[string]interface{}{
		"stackDriftStatus": string(info.StackDriftStatus),
		"lastCheckTimestamp": info.LastCheckTimestamp.String(),
	}
}