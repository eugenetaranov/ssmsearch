package ssm

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type awsClient struct {
	svc    *ssm.Client
	stsSvc *sts.Client
}

func newAWSClient(ctx context.Context, opts ClientOptions) (*awsClient, error) {
	var cfgOpts []func(*awsconfig.LoadOptions) error

	if opts.Region != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithRegion(opts.Region))
	}
	if opts.Profile != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithSharedConfigProfile(opts.Profile))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	var ssmOpts []func(*ssm.Options)
	if opts.Endpoint != "" {
		ssmOpts = append(ssmOpts, func(o *ssm.Options) {
			o.BaseEndpoint = aws.String(opts.Endpoint)
		})
	}

	return &awsClient{
		svc:    ssm.NewFromConfig(cfg, ssmOpts...),
		stsSvc: sts.NewFromConfig(cfg),
	}, nil
}

func (c *awsClient) ListParameters(ctx context.Context, opts ListOptions) ([]Parameter, error) {
	var params []Parameter

	input := &ssm.GetParametersByPathInput{
		Path:           aws.String(opts.Path),
		Recursive:      aws.Bool(opts.Recursive),
		WithDecryption: aws.Bool(opts.Decrypt),
	}

	paginator := ssm.NewGetParametersByPathPaginator(c.svc, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list parameters: %w", err)
		}

		for _, p := range page.Parameters {
			params = append(params, convertParameter(p))
		}
	}

	return params, nil
}

func convertParameter(p types.Parameter) Parameter {
	param := Parameter{
		Name:    aws.ToString(p.Name),
		Value:   aws.ToString(p.Value),
		Type:    string(p.Type),
		Version: p.Version,
	}
	if p.LastModifiedDate != nil {
		param.LastModified = *p.LastModifiedDate
	}
	return param
}

func (c *awsClient) GetAccountID(ctx context.Context) (string, error) {
	out, err := c.stsSvc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get account ID: %w", err)
	}
	return aws.ToString(out.Account), nil
}

func (c *awsClient) GetParameters(ctx context.Context, names []string, decrypt bool) ([]Parameter, error) {
	if len(names) == 0 {
		return nil, nil
	}

	var params []Parameter

	// SSM GetParameters API allows max 10 names per call
	const batchSize = 10
	for i := 0; i < len(names); i += batchSize {
		end := i + batchSize
		if end > len(names) {
			end = len(names)
		}
		batch := names[i:end]

		input := &ssm.GetParametersInput{
			Names:          batch,
			WithDecryption: aws.Bool(decrypt),
		}

		out, err := c.svc.GetParameters(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to get parameters: %w", err)
		}

		for _, p := range out.Parameters {
			params = append(params, convertParameter(p))
		}
	}

	return params, nil
}
