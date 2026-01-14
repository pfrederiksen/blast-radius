package discover

import (
	"testing"
)

func TestExtractNameFromARN(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		wantName string
	}{
		{
			name:     "Target group ARN",
			arn:      "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/1234567890abcdef",
			wantName: "1234567890abcdef",
		},
		{
			name:     "Simple path",
			arn:      "service/cluster/my-service",
			wantName: "my-service",
		},
		{
			name:     "No slashes",
			arn:      "simple-name",
			wantName: "simple-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNameFromARN(tt.arn)
			if got != tt.wantName {
				t.Errorf("extractNameFromARN() = %v, want %v", got, tt.wantName)
			}
		})
	}
}

func TestExtractRoleNameFromARN(t *testing.T) {
	tests := []struct {
		name     string
		arn      string
		wantName string
	}{
		{
			name:     "IAM role ARN",
			arn:      "arn:aws:iam::123456789012:role/my-task-role",
			wantName: "my-task-role",
		},
		{
			name:     "IAM role with path",
			arn:      "arn:aws:iam::123456789012:role/service-role/my-exec-role",
			wantName: "my-exec-role",
		},
		{
			name:     "No slashes",
			arn:      "simple-role",
			wantName: "simple-role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRoleNameFromARN(tt.arn)
			if got != tt.wantName {
				t.Errorf("extractRoleNameFromARN() = %v, want %v", got, tt.wantName)
			}
		})
	}
}
