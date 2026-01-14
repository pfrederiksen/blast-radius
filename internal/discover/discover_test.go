package discover

import (
	"testing"
)

func TestParseARN(t *testing.T) {
	tests := []struct {
		name        string
		arn         string
		wantType    string
		wantName    string
		wantRegion  string
		wantAccount string
		wantErr     bool
	}{
		{
			name:        "ALB ARN",
			arn:         "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/1234567890abcdef",
			wantType:    "LoadBalancer",
			wantName:    "my-alb",
			wantRegion:  "us-east-1",
			wantAccount: "123456789012",
			wantErr:     false,
		},
		{
			name:        "NLB ARN",
			arn:         "arn:aws:elasticloadbalancing:us-west-2:123456789012:loadbalancer/net/my-nlb/abcdef1234567890",
			wantType:    "LoadBalancer",
			wantName:    "my-nlb",
			wantRegion:  "us-west-2",
			wantAccount: "123456789012",
			wantErr:     false,
		},
		{
			name:        "ECS Service ARN",
			arn:         "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-service",
			wantType:    "ECSService",
			wantName:    "my-service",
			wantRegion:  "us-east-1",
			wantAccount: "123456789012",
			wantErr:     false,
		},
		{
			name:        "Lambda Function ARN",
			arn:         "arn:aws:lambda:us-east-1:123456789012:function:my-function",
			wantType:    "Lambda",
			wantName:    "my-function",
			wantRegion:  "us-east-1",
			wantAccount: "123456789012",
			wantErr:     false,
		},
		{
			name:        "RDS Instance ARN",
			arn:         "arn:aws:rds:us-east-1:123456789012:db:my-database",
			wantType:    "RDSInstance",
			wantName:    "my-database",
			wantRegion:  "us-east-1",
			wantAccount: "123456789012",
			wantErr:     false,
		},
		{
			name:        "RDS Cluster ARN",
			arn:         "arn:aws:rds:us-east-1:123456789012:cluster:my-cluster",
			wantType:    "RDSCluster",
			wantName:    "my-cluster",
			wantRegion:  "us-east-1",
			wantAccount: "123456789012",
			wantErr:     false,
		},
		{
			name:    "Invalid ARN - too short",
			arn:     "arn:aws:service",
			wantErr: true,
		},
		{
			name:    "Invalid ARN - unsupported service",
			arn:     "arn:aws:s3:us-east-1:123456789012:bucket/my-bucket",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Discoverer{}
			node, err := d.parseARN(tt.arn)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseARN() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseARN() unexpected error: %v", err)
				return
			}

			if node.Type != tt.wantType {
				t.Errorf("parseARN() Type = %v, want %v", node.Type, tt.wantType)
			}
			if node.Name != tt.wantName {
				t.Errorf("parseARN() Name = %v, want %v", node.Name, tt.wantName)
			}
			if node.Region != tt.wantRegion {
				t.Errorf("parseARN() Region = %v, want %v", node.Region, tt.wantRegion)
			}
			if node.Account != tt.wantAccount {
				t.Errorf("parseARN() Account = %v, want %v", node.Account, tt.wantAccount)
			}
			if node.ARN != tt.arn {
				t.Errorf("parseARN() ARN = %v, want %v", node.ARN, tt.arn)
			}
			if node.ID != tt.arn {
				t.Errorf("parseARN() ID = %v, want %v", node.ID, tt.arn)
			}
		})
	}
}

func TestExtractLambdaNameFromARN(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		wantName string
	}{
		{
			name:     "Standard Lambda ARN",
			arn:      "arn:aws:lambda:us-east-1:123456789012:function:my-function",
			wantName: "my-function",
		},
		{
			name:     "Lambda with version",
			arn:      "arn:aws:lambda:us-east-1:123456789012:function:my-function:1",
			wantName: "my-function",
		},
		{
			name:     "Lambda with alias",
			arn:      "arn:aws:lambda:us-east-1:123456789012:function:my-function:prod",
			wantName: "my-function",
		},
		{
			name:     "Invalid ARN",
			arn:      "not-an-arn",
			wantName: "not-an-arn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Discoverer{}
			got := d.extractLambdaNameFromARN(tt.arn)
			if got != tt.wantName {
				t.Errorf("extractLambdaNameFromARN() = %v, want %v", got, tt.wantName)
			}
		})
	}
}
