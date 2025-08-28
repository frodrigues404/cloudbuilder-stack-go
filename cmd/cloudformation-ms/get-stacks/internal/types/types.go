package types

type RequestBody struct {
	StackName       string `json:"stackName"`
	AccessKeyID     string `json:"acessKey"`
	SecretAccessKey string `json:"secretAccessKey"`
	Region          string `json:"region"`
}
