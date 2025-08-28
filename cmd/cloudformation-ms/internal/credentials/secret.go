package credentials

import (
	"context"
	"encoding/json"
	"errors"

	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"cloudformation-ms/internal/types"
)

func GetAccountCreds(ctx context.Context, smc *sm.Client, secretName string) (types.AWSKeys, error) {
	var sk types.AWSKeys
	out, err := smc.GetSecretValue(ctx, &sm.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return types.AWSKeys{}, err
	}
	if out.SecretString == nil {
		return types.AWSKeys{}, errors.New("secret string is nil")
	}
	if err := json.Unmarshal([]byte(*out.SecretString), &sk); err != nil {
		return types.AWSKeys{}, err
	}
	if sk.AccessKeyID == "" || sk.SecretAccessKey == "" {
		return types.AWSKeys{}, errors.New("empty keys")
	}
	return sk, nil
}
