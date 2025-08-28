package credentials

import (
	"context"

	"cloudformation-ms/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func BuildTargetConfig(ctx context.Context, base aws.Config, keys types.AWSKeys) (aws.Config, error) {
	target := base

	if keys.AccessKeyID != "" && keys.SecretAccessKey != "" {
		target.Credentials = aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(
				keys.AccessKeyID,
				keys.SecretAccessKey,
				keys.SessionToken,
			),
		)
	}

	return target, nil
}
