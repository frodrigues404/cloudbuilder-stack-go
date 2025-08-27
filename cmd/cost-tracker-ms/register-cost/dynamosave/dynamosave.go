package dynamosave

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go/aws"
)

func SaveToDynamo(client *dynamodb.Client, date, service, amount string) error {
	input := &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("TABLE_NAME")),
		Item: map[string]types.AttributeValue{
			"pk":      &types.AttributeValueMemberS{Value: "COST#SERVICE"},
			"sk":      &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%s", date, service)},
			"service": &types.AttributeValueMemberS{Value: service},
			"amount":  &types.AttributeValueMemberS{Value: amount},
			"date":    &types.AttributeValueMemberS{Value: date},
		},
	}
	_, err := client.PutItem(context.TODO(), input)
	return err
}
