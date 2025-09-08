package types

import "encoding/json"

type SecretKeys struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken,omitempty"`
	RoleARN         string `json:"roleArn,omitempty"`
	ExternalID      string `json:"externalId,omitempty"`
}

// CreateStack types
type CreateStackRequestBody struct {
	AccountName        string            `json:"accountName"`
	StackName          string            `json:"stackName"`
	Template           json.RawMessage   `json:"template"`               // Inline JSON (TemplateBody)
	TemplateURL        string            `json:"templateUrl,omitempty"`  // Alternativa: URL
	Capabilities       []string          `json:"capabilities,omitempty"` // ["CAPABILITY_IAM", ...]
	RoleARN            string            `json:"roleArn,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
	OnFailure          string            `json:"onFailure,omitempty"` // "DO_NOTHING" | "ROLLBACK" | "DELETE"
	DisableRollback    *bool             `json:"disableRollback,omitempty"`
	TimeoutInMinutes   *int32            `json:"timeoutInMinutes,omitempty"`
	ClientRequestToken string            `json:"clientRequestToken,omitempty"`
}

// GetStacks types
type GetStacksRequestBody struct {
	AccountName string `json:"accountName"`
	StackName   string `json:"stackName,omitempty"` // Optional: filter by specific stack
}

type StackInfo struct {
	StackName        string            `json:"stackName"`
	StackID          string            `json:"stackId"`
	StackStatus      string            `json:"stackStatus"`
	CreationTime     string            `json:"creationTime"`
	LastUpdatedTime  string            `json:"lastUpdatedTime,omitempty"`
	Description      string            `json:"description,omitempty"`
	Tags             map[string]string `json:"tags,omitempty"`
}

type GetStacksResponseBody struct {
	Message string      `json:"message"`
	Account string      `json:"account"`
	Owner   string      `json:"owner"`
	Stacks  []StackInfo `json:"stacks"`
}

// Common response type
type ResponseBody struct {
	Message   string `json:"message"`
	StackID   string `json:"stackId,omitempty"`
	StackName string `json:"stackName,omitempty"`
	Account   string `json:"account,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Status    string `json:"status,omitempty"`
}