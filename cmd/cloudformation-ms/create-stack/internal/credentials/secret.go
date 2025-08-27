package credentials

import (
	"context"
	"encoding/json"
	"errors"

	"create-stack-ms/internal/types"

	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func GetAccountCreds(ctx context.Context, smc *sm.Client, secretName string) (types.SecretKeys, error) {
	out, err := smc.GetSecretValue(ctx, &sm.GetSecretValueInput{SecretId: &secretName})
	if err != nil {
		return types.SecretKeys{}, err
	}
	if out.SecretString == nil {
		return types.SecretKeys{}, errors.New("secret has no SecretString")
	}
	var sk types.SecretKeys
	if err := json.Unmarshal([]byte(*out.SecretString), &sk); err != nil {
		return types.SecretKeys{}, err
	}
	if sk.AccessKeyID == "" || sk.SecretAccessKey == "" {
		return types.SecretKeys{}, errors.New("secret missing accessKeyId or secretAccessKey")
	}
	return sk, nil
}
