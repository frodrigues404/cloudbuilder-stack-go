package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
)

type Resp struct {
	Message string `json:"message"`
}

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// 1) Garantir que há authorizer e JWT
	log.Printf("Request recebido: %+v\n", req)
	if req.RequestContext.Authorizer.JWT == nil {
		log.Println("JWT ausente no request (rota não protegida ou token inválido)")
		return events.APIGatewayV2HTTPResponse{StatusCode: 401, Body: `{"message":"unauthorized"}`}, nil
	}
	claims := req.RequestContext.Authorizer.JWT.Claims
	log.Printf("Claims recebidos: %+v\n", claims)
	if claims == nil {
		log.Println("claims nil")
		return events.APIGatewayV2HTTPResponse{StatusCode: 401, Body: `{"message":"unauthorized"}`}, nil
	}

	// 2) Pegar o username do Cognito a partir dos claims
	username := claims["cognito:username"]
	sub := claims["sub"]

	// Se não veio cognito:username, tentar resolver via sub (opcional)
	// AdminDeleteUser requer o *Username* do user pool, não apenas o sub.
	userPoolID := os.Getenv("USER_POOL_ID")
	if userPoolID == "" {
		log.Println("USER_POOL_ID vazio")
		return events.APIGatewayV2HTTPResponse{StatusCode: 500, Body: `{"message":"server misconfigured"}`}, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Println("config err:", err)
		return events.APIGatewayV2HTTPResponse{StatusCode: 500, Body: `{"message":"internal error"}`}, nil
	}
	client := cip.NewFromConfig(cfg)

	if username == "" && sub != "" {
		// Fallback: procurar usuário pelo sub
		out, err := client.ListUsers(ctx, &cip.ListUsersInput{
			UserPoolId: aws.String(userPoolID),
			Filter:     aws.String(fmt.Sprintf(`sub = "%s"`, sub)),
			Limit:      aws.Int32(1),
		})
		if err != nil {
			log.Println("ListUsers err:", err)
			return events.APIGatewayV2HTTPResponse{StatusCode: 401, Body: `{"message":"unauthorized"}`}, nil
		}
		if len(out.Users) > 0 && out.Users[0].Username != nil {
			username = *out.Users[0].Username
		}
	}

	if username == "" {
		// Sem username válido, não dá para deletar com AdminDeleteUser
		return events.APIGatewayV2HTTPResponse{StatusCode: 401, Body: `{"message":"unauthorized"}`}, nil
	}

	// 3) Deletar a própria conta
	_, err = client.AdminDeleteUser(ctx, &cip.AdminDeleteUserInput{
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(username),
	})
	if err != nil {
		log.Println("AdminDeleteUser err:", err)
		return events.APIGatewayV2HTTPResponse{StatusCode: 500, Body: `{"message":"delete failed"}`}, nil
	}

	b, _ := json.Marshal(Resp{Message: "user deleted"})
	return events.APIGatewayV2HTTPResponse{StatusCode: 200, Body: string(b)}, nil
}

func main() { lambda.Start(Handler) }
