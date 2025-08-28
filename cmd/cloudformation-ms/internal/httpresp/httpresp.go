package httpresp

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"

	"cloudformation-ms/internal/types"
)

func OK(status int, payload any) events.APIGatewayV2HTTPResponse {
	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ERROR] %s", err.Error())
	}

	log.Printf("[INFO] Response %d: %s", status, string(b))

	resp := &types.APIGatewayResponse{
		StatusCode: status,
		Body:       string(b),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	return resp.Build()
}

func Error(status int, err error) events.APIGatewayV2HTTPResponse {
	log.Printf("[ERROR] Response %d: %s", status, err.Error())

	resp := &types.APIGatewayResponse{
		StatusCode: status,
		Body:       err.Error(),
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}

	return resp.Build()
}
