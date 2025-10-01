package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	cp "github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cpTypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	eb "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebTypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	s3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type requestBody struct {
	AccountName            string            `json:"accountName"`
	ApplicationName        string            `json:"applicationName"`
	EnvironmentName        string            `json:"environmentName"`
	PlatformArn            string            `json:"platformArn,omitempty"`
	SolutionStackName      string            `json:"solutionStackName,omitempty"`
	BeanstalkServiceRole   string            `json:"beanstalkServiceRole,omitempty"`
	InstanceProfile        string            `json:"instanceProfile,omitempty"`
	EnvTags                map[string]string `json:"envTags,omitempty"`
	PipelineName           string            `json:"pipelineName"`
	PipelineServiceRoleArn string            `json:"pipelineServiceRoleArn"`
	ArtifactBucket         string            `json:"artifactBucket,omitempty"`
	ConnectionArn          string            `json:"connectionArn"`
	Repository             string            `json:"repository"` // "owner/repo"
	Branch                 string            `json:"branch"`
	Idempotent             bool              `json:"idempotent,omitempty"`
}

type responseBody struct {
	Message         string `json:"message"`
	ApplicationName string `json:"applicationName"`
	EnvironmentName string `json:"environmentName"`
	PipelineName    string `json:"pipelineName"`
	ArtifactBucket  string `json:"artifactBucket"`
	Account         string `json:"account"`
	Owner           string `json:"owner"`
}

type accessKeys struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken,omitempty"`
}

// ---------- AWS base ----------

func newAWS(ctx context.Context) (aws.Config, error) {
	if region := os.Getenv("AWS_REGION"); region == "" {
		if def := os.Getenv("AWS_DEFAULT_REGION"); def != "" {
			os.Setenv("AWS_REGION", def)
		}
	}
	return config.LoadDefaultConfig(ctx)
}

func newCognitoClient(cfg aws.Config) *cip.Client { return cip.NewFromConfig(cfg) }
func newSecretsClient(cfg aws.Config) *sm.Client  { return sm.NewFromConfig(cfg) }
func newEBClient(cfg aws.Config) *eb.Client       { return eb.NewFromConfig(cfg) }
func newCPClient(cfg aws.Config) *cp.Client       { return cp.NewFromConfig(cfg) }
func newS3Client(cfg aws.Config) *s3.Client       { return s3.NewFromConfig(cfg) }

// ---------- Auth helpers ----------

func resolveUsernameBySub(ctx context.Context, client *cip.Client, userPoolID, sub string) (string, error) {
	out, err := client.ListUsers(ctx, &cip.ListUsersInput{
		UserPoolId: aws.String(userPoolID),
		Filter:     aws.String(fmt.Sprintf(`sub = "%s"`, sub)),
		Limit:      aws.Int32(1),
	})
	if err != nil {
		return "", err
	}
	if len(out.Users) == 0 || out.Users[0].Username == nil {
		return "", errors.New("user not found by sub")
	}
	return *out.Users[0].Username, nil
}

// ---------- Secrets & target config ----------

func getAccountCreds(ctx context.Context, smc *sm.Client, secretName string) (*accessKeys, error) {
	out, err := smc.GetSecretValue(ctx, &sm.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return nil, err
	}
	var keys accessKeys
	if err := json.Unmarshal([]byte(aws.ToString(out.SecretString)), &keys); err != nil {
		return nil, err
	}
	if keys.AccessKeyID == "" || keys.SecretAccessKey == "" {
		return nil, fmt.Errorf("secret %s missing accessKeyId/secretAccessKey", secretName)
	}
	return &keys, nil
}

func buildTargetConfig(ctx context.Context, base aws.Config, keys *accessKeys) (aws.Config, error) {
	cfg := base
	cfg.Credentials = aws.NewCredentialsCache(
		credentials.NewStaticCredentialsProvider(keys.AccessKeyID, keys.SecretAccessKey, keys.SessionToken),
	)
	return cfg, nil
}

// ---------- S3 helpers (artifact store) ----------

func ensureBucket(ctx context.Context, s3c *s3.Client, bucket, region string) (string, error) {
	if bucket == "" {
		bucket = fmt.Sprintf("cp-artifacts-%s-%d", region, time.Now().Unix())
	}
	_, headErr := s3c.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if headErr == nil {
		return bucket, nil
	}
	_, err := s3c.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		},
	})
	if err != nil {
		return "", fmt.Errorf("create bucket %s: %w", bucket, err)
	}
	return bucket, nil
}

// ---------- Elastic Beanstalk ----------

func ensureEBApplication(ctx context.Context, ebc *eb.Client, appName string, tags map[string]string) error {
	_, err := ebc.DescribeApplications(ctx, &eb.DescribeApplicationsInput{
		ApplicationNames: []string{appName},
	})
	if err == nil {
		return nil
	}
	var ebTags []ebTypes.Tag
	for k, v := range tags {
		ebTags = append(ebTags, ebTypes.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	_, err = ebc.CreateApplication(ctx, &eb.CreateApplicationInput{
		ApplicationName: aws.String(appName),
		Tags:            ebTags,
	})
	return err
}

func ensureEBEnvironment(ctx context.Context, ebc *eb.Client, appName, envName, platformArn, solutionStack, serviceRole, instanceProfile string, tags map[string]string) error {
	envs, err := ebc.DescribeEnvironments(ctx, &eb.DescribeEnvironmentsInput{
		ApplicationName:  aws.String(appName),
		EnvironmentNames: []string{envName},
		IncludeDeleted:   aws.Bool(false),
	})
	if err == nil && len(envs.Environments) > 0 {
		return nil
	}
	var opts []ebTypes.ConfigurationOptionSetting
	if serviceRole != "" {
		opts = append(opts, ebTypes.ConfigurationOptionSetting{
			Namespace:  aws.String("aws:elasticbeanstalk:environment"),
			OptionName: aws.String("ServiceRole"),
			Value:      aws.String(serviceRole),
		})
	}
	if instanceProfile != "" {
		opts = append(opts, ebTypes.ConfigurationOptionSetting{
			Namespace:  aws.String("aws:autoscaling:launchconfiguration"),
			OptionName: aws.String("IamInstanceProfile"),
			Value:      aws.String(instanceProfile),
		})
	}
	var ebTags []ebTypes.Tag
	for k, v := range tags {
		ebTags = append(ebTags, ebTypes.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	in := &eb.CreateEnvironmentInput{
		ApplicationName: aws.String(appName),
		EnvironmentName: aws.String(envName),
		Tier: &ebTypes.EnvironmentTier{
			Name: aws.String("WebServer"),
			Type: aws.String("Standard"),
		},
		OptionSettings: opts,
		Tags:           ebTags,
	}
	if platformArn != "" {
		in.PlatformArn = aws.String(platformArn)
	} else if solutionStack != "" {
		in.SolutionStackName = aws.String(solutionStack)
	} else {
		return errors.New("must provide platformArn or solutionStackName")
	}
	_, err = ebc.CreateEnvironment(ctx, in)
	return err
}

// ---------- CodePipeline ----------

func ensurePipeline(ctx context.Context, cpc *cp.Client, region, pipelineName, pipelineRoleArn, artifactBucket, connectionArn, repo, branch, appName, envName string, idempotent bool) error {
	if idempotent {
		_, err := cpc.GetPipeline(ctx, &cp.GetPipelineInput{Name: aws.String(pipelineName)})
		if err == nil {
			return nil
		}
	}

	artifactStore := cpTypes.ArtifactStore{
		Type:     cpTypes.ArtifactStoreTypeS3,
		Location: aws.String(artifactBucket),
	}

	sourceAction := cpTypes.ActionDeclaration{
		Name: aws.String("Source"),
		ActionTypeId: &cpTypes.ActionTypeId{
			Category: cpTypes.ActionCategorySource,
			Owner:    cpTypes.ActionOwnerAws,
			Provider: aws.String("CodeStarSourceConnection"),
			Version:  aws.String("1"),
		},
		Configuration: map[string]string{
			"ConnectionArn":        connectionArn,
			"FullRepositoryId":     repo,
			"BranchName":           branch,
			"OutputArtifactFormat": "CODE_ZIP",
		},
		OutputArtifacts: []cpTypes.OutputArtifact{
			{Name: aws.String("SourceOutput")},
		},
		RunOrder: aws.Int32(1),
	}

	deployAction := cpTypes.ActionDeclaration{
		Name: aws.String("DeployToElasticBeanstalk"),
		ActionTypeId: &cpTypes.ActionTypeId{
			Category: cpTypes.ActionCategoryDeploy,
			Owner:    cpTypes.ActionOwnerAws,
			Provider: aws.String("ElasticBeanstalk"),
			Version:  aws.String("1"),
		},
		Configuration: map[string]string{
			"ApplicationName": appName,
			"EnvironmentName": envName,
		},
		InputArtifacts: []cpTypes.InputArtifact{
			{Name: aws.String("SourceOutput")},
		},
		RunOrder: aws.Int32(1),
	}

	stages := []cpTypes.StageDeclaration{
		{Name: aws.String("Source"), Actions: []cpTypes.ActionDeclaration{sourceAction}},
		{Name: aws.String("Deploy"), Actions: []cpTypes.ActionDeclaration{deployAction}},
	}

	p := &cpTypes.PipelineDeclaration{
		Name:          aws.String(pipelineName),
		RoleArn:       aws.String(pipelineRoleArn),
		ArtifactStore: &artifactStore,
		Stages:        stages,
		Version:       aws.Int32(1),
	}

	_, err := cpc.CreatePipeline(ctx, &cp.CreatePipelineInput{
		Pipeline: p,
		Tags:     []cpTypes.Tag{{Key: aws.String("app"), Value: aws.String(appName)}},
	})
	if err != nil {
		var conflict *cpTypes.PipelineNameInUseException
		if errors.As(err, &conflict) {
			_, upErr := cpc.UpdatePipeline(ctx, &cp.UpdatePipelineInput{Pipeline: p})
			if upErr != nil {
				return upErr
			}
			// Emula "restart on update"
			_, _ = cpc.StartPipelineExecution(ctx, &cp.StartPipelineExecutionInput{
				Name: aws.String(pipelineName),
			})
			return nil
		}
		return err
	}

	// Opcional: start após criação
	_, _ = cpc.StartPipelineExecution(ctx, &cp.StartPipelineExecutionInput{
		Name: aws.String(pipelineName),
	})
	return nil
}

// ---------- HTTP helpers ----------

func apiOK(status int, payload any) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(payload)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json", "Access-Control-Allow-Origin": "*"},
		Body:       string(b),
	}
}

func apiError(status int, err error) events.APIGatewayV2HTTPResponse {
	out := map[string]string{"message": err.Error()}
	b, _ := json.Marshal(out)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json", "Access-Control-Allow-Origin": "*"},
		Body:       string(b),
	}
}

// ---------- Lambda handler ----------

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	if req.RequestContext.Authorizer.JWT == nil || req.RequestContext.Authorizer.JWT.Claims == nil {
		return apiError(401, errors.New("unauthorized")), nil
	}
	claims := req.RequestContext.Authorizer.JWT.Claims
	username := claims["cognito:username"]
	sub := claims["sub"]

	cfg, err := newAWS(ctx)
	if err != nil {
		log.Println("aws config err:", err)
		return apiError(500, errors.New("internal error")), nil
	}

	userPoolID := os.Getenv("USER_POOL_ID")
	if username == "" && userPoolID != "" && sub != "" {
		if u, err := resolveUsernameBySub(ctx, newCognitoClient(cfg), userPoolID, sub); err == nil {
			username = u
		}
	}
	owner := username
	if owner == "" {
		owner = sub
	}
	if owner == "" {
		return apiError(401, errors.New("unauthorized")), nil
	}

	raw := req.Body
	if req.IsBase64Encoded {
		b, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return apiError(400, fmt.Errorf("invalid base64 body: %w", err)), nil
		}
		raw = string(b)
	}
	var body requestBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return apiError(400, fmt.Errorf("invalid JSON body: %w", err)), nil
	}

	// validações
	if strings.TrimSpace(body.AccountName) == "" ||
		strings.TrimSpace(body.ApplicationName) == "" ||
		strings.TrimSpace(body.EnvironmentName) == "" ||
		strings.TrimSpace(body.PipelineName) == "" ||
		strings.TrimSpace(body.PipelineServiceRoleArn) == "" ||
		strings.TrimSpace(body.ConnectionArn) == "" ||
		strings.TrimSpace(body.Repository) == "" ||
		strings.TrimSpace(body.Branch) == "" {
		return apiError(400, errors.New("required: accountName, applicationName, environmentName, pipelineName, pipelineServiceRoleArn, connectionArn, repository, branch")), nil
	}
	if body.PlatformArn == "" && body.SolutionStackName == "" {
		return apiError(400, errors.New("provide platformArn or solutionStackName")), nil
	}

	// credenciais da conta-alvo
	smClient := newSecretsClient(cfg)
	secretName := fmt.Sprintf("%s/%s/access_keys", owner, body.AccountName)
	keys, err := getAccountCreds(ctx, smClient, secretName)
	if err != nil {
		return apiError(404, fmt.Errorf("failed to get credentials from secrets manager: %w", err)), nil
	}

	targetCfg, err := buildTargetConfig(ctx, cfg, keys)
	if err != nil {
		return apiError(401, fmt.Errorf("invalid credentials for account '%s': %w", body.AccountName, err)), nil
	}
	region := targetCfg.Region

	// clients conta-alvo
	ebc := newEBClient(targetCfg)
	cpc := newCPClient(targetCfg)
	s3c := newS3Client(targetCfg)

	// EB app/env
	appTags := map[string]string{"owner": owner, "app": body.ApplicationName}
	if err := ensureEBApplication(ctx, ebc, body.ApplicationName, appTags); err != nil {
		return apiError(500, fmt.Errorf("create beanstalk application failed: %w", err)), nil
	}
	serviceRole := body.BeanstalkServiceRole
	if serviceRole == "" {
		serviceRole = "aws-elasticbeanstalk-service-role"
	}
	instanceProfile := body.InstanceProfile
	if instanceProfile == "" {
		instanceProfile = "aws-elasticbeanstalk-ec2-role"
	}
	if err := ensureEBEnvironment(ctx, ebc, body.ApplicationName, body.EnvironmentName, body.PlatformArn, body.SolutionStackName, serviceRole, instanceProfile, body.EnvTags); err != nil {
		return apiError(500, fmt.Errorf("create beanstalk environment failed: %w", err)), nil
	}

	// artifact bucket
	bucket, err := ensureBucket(ctx, s3c, body.ArtifactBucket, region)
	if err != nil {
		return apiError(500, fmt.Errorf("ensure artifact bucket failed: %w", err)), nil
	}

	// pipeline
	if err := ensurePipeline(ctx, cpc, region, body.PipelineName, body.PipelineServiceRoleArn, bucket, body.ConnectionArn, body.Repository, body.Branch, body.ApplicationName, body.EnvironmentName, body.Idempotent); err != nil {
		return apiError(500, fmt.Errorf("create/update pipeline failed: %w", err)), nil
	}

	resp := responseBody{
		Message:         "Elastic Beanstalk app/env and CodePipeline created/ensured",
		ApplicationName: body.ApplicationName,
		EnvironmentName: body.EnvironmentName,
		PipelineName:    body.PipelineName,
		ArtifactBucket:  bucket,
		Account:         body.AccountName,
		Owner:           owner,
	}
	return apiOK(201, resp), nil
}

func main() {
	lambda.Start(handler)
}
