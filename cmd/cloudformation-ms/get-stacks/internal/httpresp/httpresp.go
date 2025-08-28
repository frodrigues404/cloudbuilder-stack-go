package httpresp

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
)

func OK(status int, payload any) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(payload)
	log.Printf("[INFO] Response %d: %s", status, string(b))
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: string(b),
	}
}

func Error(status int, err error) events.APIGatewayV2HTTPResponse {
	out := map[string]string{
		"error": err.Error(),
	}
	b, _ := json.Marshal(out)
	log.Printf("[ERROR] Response %d: %s", status, string(b))
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: string(b),
	}
}
