package awsconfig

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

func Base(ctx context.Context) (aws.Config, error) {
	if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") != "" {
		os.Setenv("AWS_REGION", os.Getenv("AWS_DEFAULT_REGION"))
	}
	return config.LoadDefaultConfig(ctx)
}
