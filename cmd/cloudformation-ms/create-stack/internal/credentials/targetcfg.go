package credentials

import (
	"context"
	"fmt"
	"log"
	"time"

	"create-stack-ms/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// BuildTargetConfig aplica credenciais do secret (e opcionalmente AssumeRole) e valida com STS.
func BuildTargetConfig(ctx context.Context, base aws.Config, keys types.SecretKeys) (aws.Config, error) {
	target := base

	// Credenciais do secret (long-term ou temporárias)
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
