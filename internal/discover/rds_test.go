package discover

import (
	"testing"

	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

func TestRDSInstanceToNode(t *testing.T) {
	d := &Discoverer{}

	arn := "arn:aws:rds:us-east-1:123456789012:db:my-database"
	identifier := "my-database"
	engine := "postgres"
	engineVersion := "14.7"
	status := "available"
	instanceClass := "db.t3.micro"
	allocatedStorage := int32(20)
	storageType := "gp2"
	multiAZ := true
	publiclyAccessible := false
	endpoint := "my-database.abc123.us-east-1.rds.amazonaws.com"
	port := int32(5432)

	instance := &rdstypes.DBInstance{
		DBInstanceArn:        &arn,
		DBInstanceIdentifier: &identifier,
		Engine:               &engine,
		EngineVersion:        &engineVersion,
		DBInstanceStatus:     &status,
		DBInstanceClass:      &instanceClass,
		AllocatedStorage:     &allocatedStorage,
		StorageType:          &storageType,
		MultiAZ:              &multiAZ,
		PubliclyAccessible:   &publiclyAccessible,
		Endpoint: &rdstypes.Endpoint{
			Address: &endpoint,
			Port:    &port,
		},
	}

	node := d.rdsInstanceToNode(instance)

	if node.ID != arn {
		t.Errorf("Expected ID %s, got %s", arn, node.ID)
	}
	if node.Type != "RDSInstance" {
		t.Errorf("Expected Type RDSInstance, got %s", node.Type)
	}
	if node.ARN != arn {
		t.Errorf("Expected ARN %s, got %s", arn, node.ARN)
	}
	if node.Name != identifier {
		t.Errorf("Expected Name %s, got %s", identifier, node.Name)
	}
	if node.Region != "us-east-1" {
		t.Errorf("Expected Region us-east-1, got %s", node.Region)
	}
	if node.Account != "123456789012" {
		t.Errorf("Expected Account 123456789012, got %s", node.Account)
	}

	// Check metadata
	if node.Metadata["engine"] != &engine {
		t.Errorf("Expected engine %s in metadata", engine)
	}
	if node.Metadata["endpoint"] != endpoint {
		t.Errorf("Expected endpoint %s in metadata", endpoint)
	}
	if node.Metadata["port"] != port {
		t.Errorf("Expected port %d in metadata", port)
	}
}

func TestRDSClusterToNode(t *testing.T) {
	d := &Discoverer{}

	arn := "arn:aws:rds:us-east-1:123456789012:cluster:my-cluster"
	identifier := "my-cluster"
	engine := "aurora-postgresql"
	engineVersion := "14.7"
	status := "available"
	allocatedStorage := int32(100)
	storageType := "aurora"
	multiAZ := true
	endpoint := "my-cluster.cluster-abc123.us-east-1.rds.amazonaws.com"
	readerEndpoint := "my-cluster.cluster-ro-abc123.us-east-1.rds.amazonaws.com"
	port := int32(5432)

	cluster := &rdstypes.DBCluster{
		DBClusterArn:        &arn,
		DBClusterIdentifier: &identifier,
		Engine:              &engine,
		EngineVersion:       &engineVersion,
		Status:              &status,
		AllocatedStorage:    &allocatedStorage,
		StorageType:         &storageType,
		MultiAZ:             &multiAZ,
		Endpoint:            &endpoint,
		ReaderEndpoint:      &readerEndpoint,
		Port:                &port,
	}

	node := d.rdsClusterToNode(cluster)

	if node.ID != arn {
		t.Errorf("Expected ID %s, got %s", arn, node.ID)
	}
	if node.Type != "RDSCluster" {
		t.Errorf("Expected Type RDSCluster, got %s", node.Type)
	}
	if node.ARN != arn {
		t.Errorf("Expected ARN %s, got %s", arn, node.ARN)
	}
	if node.Name != identifier {
		t.Errorf("Expected Name %s, got %s", identifier, node.Name)
	}
	if node.Region != "us-east-1" {
		t.Errorf("Expected Region us-east-1, got %s", node.Region)
	}
	if node.Account != "123456789012" {
		t.Errorf("Expected Account 123456789012, got %s", node.Account)
	}

	// Check metadata
	if node.Metadata["engine"] != &engine {
		t.Errorf("Expected engine %s in metadata", engine)
	}
	if node.Metadata["endpoint"] != endpoint {
		t.Errorf("Expected endpoint %s in metadata", endpoint)
	}
	if node.Metadata["readerEndpoint"] != readerEndpoint {
		t.Errorf("Expected readerEndpoint %s in metadata", readerEndpoint)
	}
	if node.Metadata["port"] != port {
		t.Errorf("Expected port %d in metadata", port)
	}
}

func TestHasHeuristic(t *testing.T) {
	tests := []struct {
		name       string
		heuristics []string
		search     string
		expected   bool
	}{
		{
			name:       "Heuristic found",
			heuristics: []string{"rds-endpoint", "lambda-env"},
			search:     "rds-endpoint",
			expected:   true,
		},
		{
			name:       "Heuristic not found",
			heuristics: []string{"rds-endpoint", "lambda-env"},
			search:     "nonexistent",
			expected:   false,
		},
		{
			name:       "Empty heuristics",
			heuristics: []string{},
			search:     "rds-endpoint",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Discoverer{
				opts: &Options{
					Heuristics: tt.heuristics,
				},
			}
			result := d.hasHeuristic(tt.search)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
