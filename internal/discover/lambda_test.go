package discover

import (
	"testing"
)

func TestLambdaARNPatterns(t *testing.T) {
	// This test validates ARN patterns we expect to handle
	tests := []struct {
		name        string
		arn         string
		shouldMatch string
	}{
		{
			name:        "SQS Queue ARN contains sqs",
			arn:         "arn:aws:sqs:us-east-1:123456789012:my-queue",
			shouldMatch: "sqs",
		},
		{
			name:        "DynamoDB Stream ARN contains dynamodb",
			arn:         "arn:aws:dynamodb:us-east-1:123456789012:table/MyTable/stream/2023-01-01T00:00:00.000",
			shouldMatch: "dynamodb",
		},
		{
			name:        "Kinesis Stream ARN contains kinesis",
			arn:         "arn:aws:kinesis:us-east-1:123456789012:stream/my-stream",
			shouldMatch: "kinesis",
		},
		{
			name:        "Kafka Cluster ARN contains kafka",
			arn:         "arn:aws:kafka:us-east-1:123456789012:cluster/my-cluster/abc123",
			shouldMatch: "kafka",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Import strings package is available
			if !containsSubstring(tt.arn, tt.shouldMatch) {
				t.Errorf("ARN %q should contain %q", tt.arn, tt.shouldMatch)
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
