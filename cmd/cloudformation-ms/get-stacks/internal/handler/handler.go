package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"cloudformation-ms/shared/auth"
	"cloudformation-ms/shared/awsconfig"
	"cloudformation-ms/shared/credentials"
	"cloudformation-ms/shared/httpresp"
	"cloudformation-ms/shared/types"
)

type deps struct {
	cognito *cip.Client
}

func (d *deps) Cognito() *cip.Client {
	return d.cognito
}

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("[INFO] GetStacks request: method=%s path=%s", req.RequestContext.HTTP.Method, req.RequestContext.HTTP.Path)

	var body types.GetStacksRequestBody
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return httpresp.Error(400, fmt.Errorf("invalid JSON: %w", err)), nil
	}

	if body.AccountName == "" {
		return httpresp.Error(400, fmt.Errorf("accountName is required")), nil
	}

	baseCfg, err := awsconfig.Base(ctx)
	if err != nil {
		return httpresp.Error(500, fmt.Errorf("AWS config failed: %w", err)), nil
	}

	depsInst := &deps{cognito: cip.NewFromConfig(baseCfg)}
	owner, err := auth.OwnerFromRequest(ctx, req, baseCfg, depsInst)
	if err != nil {
		return httpresp.Error(401, err), nil
	}

	secretName := fmt.Sprintf("%s/%s/access_keys", owner, body.AccountName)
	smClient := sm.NewFromConfig(baseCfg)
	keys, err := credentials.GetAccountCreds(ctx, smClient, secretName)
	if err != nil {
		return httpresp.Error(400, fmt.Errorf("failed to get account credentials from secrets manager (%s): %w", secretName, err)), nil
	}

	targetCfg, err := credentials.BuildTargetConfig(ctx, baseCfg, keys)
	if err != nil {
		return httpresp.Error(400, fmt.Errorf("failed to build target config: %w", err)), nil
	}

	cfnClient := cf.NewFromConfig(targetCfg)

	input := &cf.ListStacksInput{}

	// Optional: filter by stack name if provided
	if body.StackName != "" {
		describeInput := &cf.DescribeStacksInput{
			StackName: aws.String(body.StackName),
		}
		describeOut, err := cfnClient.DescribeStacks(ctx, describeInput)
		if err != nil {
			return httpresp.Error(400, fmt.Errorf("describe stack failed: %w", err)), nil
		}

		stacks := make([]types.StackInfo, 0, len(describeOut.Stacks))
		for _, stack := range describeOut.Stacks {
			stackInfo := types.StackInfo{
				StackName:    aws.ToString(stack.StackName),
				StackID:      aws.ToString(stack.StackId),
				StackStatus:  string(stack.StackStatus),
				CreationTime: stack.CreationTime.Format(time.RFC3339),
				Description:  aws.ToString(stack.Description),
			}
			if stack.LastUpdatedTime != nil {
				stackInfo.LastUpdatedTime = stack.LastUpdatedTime.Format(time.RFC3339)
			}

			if len(stack.Tags) > 0 {
				stackInfo.Tags = make(map[string]string)
				for _, tag := range stack.Tags {
					stackInfo.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}
			}

			stacks = append(stacks, stackInfo)
		}

		resp := types.GetStacksResponseBody{
			Message: "stacks retrieved successfully",
			Account: body.AccountName,
			Owner:   owner,
			Stacks:  stacks,
		}
		return httpresp.OK(200, resp), nil
	}

	out, err := cfnClient.ListStacks(ctx, input)
	if err != nil {
		return httpresp.Error(400, fmt.Errorf("list stacks failed: %w", err)), nil
	}

	stacks := make([]types.StackInfo, 0, len(out.StackSummaries))
	for _, summary := range out.StackSummaries {
		if summary.StackStatus == "DELETE_COMPLETE" {
			continue
		}

		stackInfo := types.StackInfo{
			StackName:    aws.ToString(summary.StackName),
			StackID:      aws.ToString(summary.StackId),
			StackStatus:  string(summary.StackStatus),
			CreationTime: summary.CreationTime.Format(time.RFC3339),
		}
		if summary.LastUpdatedTime != nil {
			stackInfo.LastUpdatedTime = summary.LastUpdatedTime.Format(time.RFC3339)
		}

		stacks = append(stacks, stackInfo)
	}

	log.Printf("[INFO] Found %d stacks for account %s", len(stacks), body.AccountName)

	resp := types.GetStacksResponseBody{
		Message: "stacks retrieved successfully",
		Account: body.AccountName,
		Owner:   owner,
		Stacks:  stacks,
	}
	return httpresp.OK(200, resp), nil
}
