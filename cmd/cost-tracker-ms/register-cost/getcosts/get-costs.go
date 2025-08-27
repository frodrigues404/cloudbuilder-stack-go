package getcosts

import (
	"context"
	"fmt"
	"time"

	dynamosave "cost-tracker/dynamosave"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func GetCosts() error {
	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	client := costexplorer.NewFromConfig(cfg)

	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"UnblendedCost"},
		GroupBy: []types.GroupDefinition{
			{
				Type: types.GroupDefinitionTypeDimension,
				Key:  aws.String("SERVICE"),
			},
		},
	}

	output, err := client.GetCostAndUsage(ctx, input)
	if err != nil {
		return err
	}

	for _, result := range output.ResultsByTime {
		fmt.Println("Date:", *result.TimePeriod.Start)
		for _, group := range result.Groups {
			service := group.Keys[0]
			amount := group.Metrics["UnblendedCost"].Amount
			ddb := dynamodb.NewFromConfig(cfg)
			err := dynamosave.SaveToDynamo(ddb, *result.TimePeriod.Start, service, *amount)
			if err != nil {
				fmt.Println("Erro salvando no Dynamo:", err)
			}
			fmt.Printf("  %s: $%s\n", service, *amount)
		}
	}
	return nil
}
