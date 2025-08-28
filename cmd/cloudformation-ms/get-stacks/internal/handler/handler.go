package handler

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"

	"get-stacks/internal/httpresp"
)

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Print("[INFO] Incoming request: reqId=%s method=%s path=%s",
		req.RequestContext.RequestID, req.RequestContext.HTTP.Method, req.RawPath)

	cfg, err := awsconfig.Base(ctx)
	if err != nil {
		return httpresp.Error(500, fmt.Errorf("aws config error: %w", err)), nil
	}

}
