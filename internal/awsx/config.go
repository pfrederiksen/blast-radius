package awsx

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// Clients holds all AWS service clients
type Clients struct {
	ELBv2                *elasticloadbalancingv2.Client
	ECS                  *ecs.Client
	Lambda               *lambda.Client
	RDS                  *rds.Client
	Route53              *route53.Client
	EC2                  *ec2.Client
	ApplicationAutoScaling *applicationautoscaling.Client
}

// LoadConfig loads AWS configuration with optional profile and region overrides
func LoadConfig(ctx context.Context, profile, region string) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("unable to load AWS config: %w", err)
	}

	return cfg, nil
}

// NewClients creates all AWS service clients from config
func NewClients(cfg aws.Config) (*Clients, error) {
	return &Clients{
		ELBv2:                  elasticloadbalancingv2.NewFromConfig(cfg),
		ECS:                    ecs.NewFromConfig(cfg),
		Lambda:                 lambda.NewFromConfig(cfg),
		RDS:                    rds.NewFromConfig(cfg),
		Route53:                route53.NewFromConfig(cfg),
		EC2:                    ec2.NewFromConfig(cfg),
		ApplicationAutoScaling: applicationautoscaling.NewFromConfig(cfg),
	}, nil
}
