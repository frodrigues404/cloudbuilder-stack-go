package types

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
)

type APIGatewayResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

func (r *APIGatewayResponse) Build() events.APIGatewayV2HTTPResponse {
	return events.APIGatewayV2HTTPResponse{
		StatusCode: r.StatusCode,
		Body:       r.Body,
		Headers:    r.Headers,
	}
}

type AWSKeys struct {
	AccessKeyID     string `json:"acessKeyId"`
	SecretAccessKey string `json:"secretAcessKey"`
	SessionToken    string `json:"sessionToken"`
}

type CreateRequestBody struct {
	Keys         AWSKeys           `json:"keys"`
	StackName    string            `json:"stackName"`
	Template     json.RawMessage   `json:"template"`
	TemplateURL  string            `json:"templateURL,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	OnFailure    string            `json:"onFailure,omitempty"`
}

type ResponseBody struct {
	Message   string `json:"message"`
	StackID   string `json:"stackId,omitempty"`
	StackName string `json:"stackName,omitempty"`
	Account   string `json:"account,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Status    string `json:"status,omitempty"`
}
