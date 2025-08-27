package types

import "encoding/json"

type SecretKeys struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken,omitempty"`
	RoleARN         string `json:"roleArn,omitempty"`
	ExternalID      string `json:"externalId,omitempty"`
}

type RequestBody struct {
	AccountName        string            `json:"accountName"`
	StackName          string            `json:"stackName"`
	Template           json.RawMessage   `json:"template"`              // Inline JSON (TemplateBody)
	TemplateURL        string            `json:"templateUrl,omitempty"` // Alternativa: URL
	Parameters         map[string]string `json:"parameters,omitempty"`
	Capabilities       []string          `json:"capabilities,omitempty"` // ["CAPABILITY_IAM", ...]
	RoleARN            string            `json:"roleArn,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
	OnFailure          string            `json:"onFailure,omitempty"` // "DO_NOTHING" | "ROLLBACK" | "DELETE"
	DisableRollback    *bool             `json:"disableRollback,omitempty"`
	TimeoutInMinutes   *int32            `json:"timeoutInMinutes,omitempty"`
	ClientRequestToken string            `json:"clientRequestToken,omitempty"`
}

type ResponseBody struct {
	Message   string `json:"message"`
	StackID   string `json:"stackId,omitempty"`
	StackName string `json:"stackName,omitempty"`
	Account   string `json:"account,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Status    string `json:"status,omitempty"`
}
