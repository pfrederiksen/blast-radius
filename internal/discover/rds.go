package discover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// resolveRDSInstance resolves an RDS instance by identifier
func (d *Discoverer) resolveRDSInstance(ctx context.Context, identifier string) (*graph.Node, error) {
	slog.Debug("Resolving RDS instance", "identifier", identifier)

	output, err := d.clients.RDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &identifier,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe RDS instance: %w", err)
	}

	if len(output.DBInstances) == 0 {
		return nil, fmt.Errorf("RDS instance not found: %s", identifier)
	}

	return d.rdsInstanceToNode(&output.DBInstances[0]), nil
}

// resolveRDSCluster resolves an RDS cluster by identifier
func (d *Discoverer) resolveRDSCluster(ctx context.Context, identifier string) (*graph.Node, error) {
	slog.Debug("Resolving RDS cluster", "identifier", identifier)

	output, err := d.clients.RDS.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{
		DBClusterIdentifier: &identifier,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe RDS cluster: %w", err)
	}

	if len(output.DBClusters) == 0 {
		return nil, fmt.Errorf("RDS cluster not found: %s", identifier)
	}

	return d.rdsClusterToNode(&output.DBClusters[0]), nil
}

// discoverRDS discovers dependencies for an RDS instance or cluster
func (d *Discoverer) discoverRDS(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering RDS dependencies", "type", node.Type, "arn", node.ARN)

	switch node.Type {
	case ResourceTypeRDSInstance:
		return d.discoverRDSInstance(ctx, node, g)
	case ResourceTypeRDSCluster:
		return d.discoverRDSCluster(ctx, node, g)
	default:
		return nil, fmt.Errorf("unknown RDS type: %s", node.Type)
	}
}

// discoverRDSInstance discovers dependencies for an RDS instance
func (d *Discoverer) discoverRDSInstance(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering RDS instance dependencies", "name", node.Name)

	var neighbors []string

	// Get instance details
	output, err := d.clients.RDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &node.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe RDS instance: %w", err)
	}

	if len(output.DBInstances) == 0 {
		return nil, fmt.Errorf("RDS instance not found: %s", node.Name)
	}

	instance := &output.DBInstances[0]

	// Discover subnet group
	if instance.DBSubnetGroup != nil && instance.DBSubnetGroup.DBSubnetGroupName != nil {
		subnetGroupNode := &graph.Node{
			ID:      *instance.DBSubnetGroup.DBSubnetGroupName,
			Type:    ResourceTypeDBSubnetGroup,
			Name:    *instance.DBSubnetGroup.DBSubnetGroupName,
			Region:  node.Region,
			Account: node.Account,
			Metadata: map[string]any{
				"vpcId": instance.DBSubnetGroup.VpcId,
			},
		}
		if instance.DBSubnetGroup.DBSubnetGroupDescription != nil {
			subnetGroupNode.Metadata["description"] = *instance.DBSubnetGroup.DBSubnetGroupDescription
		}
		g.AddNode(subnetGroupNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           subnetGroupNode.ID,
			RelationType: "uses-subnet-group",
			Evidence: graph.Evidence{
				APICall: "DescribeDBInstances",
				Fields: map[string]any{
					"DBSubnetGroupName": *instance.DBSubnetGroup.DBSubnetGroupName,
				},
			},
		})
		neighbors = append(neighbors, subnetGroupNode.ID)

		// Add individual subnets
		for i := range instance.DBSubnetGroup.Subnets {
			subnet := &instance.DBSubnetGroup.Subnets[i]
			if subnet.SubnetIdentifier == nil {
				continue
			}
			subnetNode := &graph.Node{
				ID:      *subnet.SubnetIdentifier,
				Type:    ResourceTypeSubnet,
				Name:    *subnet.SubnetIdentifier,
				Region:  node.Region,
				Account: node.Account,
			}
			if subnet.SubnetAvailabilityZone != nil && subnet.SubnetAvailabilityZone.Name != nil {
				subnetNode.Metadata = map[string]any{
					"availabilityZone": *subnet.SubnetAvailabilityZone.Name,
				}
			}
			g.AddNode(subnetNode)
			g.AddEdge(&graph.Edge{
				From:         subnetGroupNode.ID,
				To:           subnetNode.ID,
				RelationType: "contains",
				Evidence: graph.Evidence{
					APICall: "DescribeDBInstances",
					Fields: map[string]any{
						"SubnetIdentifier": *subnet.SubnetIdentifier,
					},
				},
			})
			neighbors = append(neighbors, subnetNode.ID)
		}
	}

	// Discover security groups
	for i := range instance.VpcSecurityGroups {
		sg := &instance.VpcSecurityGroups[i]
		if sg.VpcSecurityGroupId == nil {
			continue
		}
		sgNode := &graph.Node{
			ID:      *sg.VpcSecurityGroupId,
			Type:    ResourceTypeSecurityGroup,
			Name:    *sg.VpcSecurityGroupId,
			Region:  node.Region,
			Account: node.Account,
			Metadata: map[string]any{
				"status": sg.Status,
			},
		}
		g.AddNode(sgNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           sgNode.ID,
			RelationType: "uses-security-group",
			Evidence: graph.Evidence{
				APICall: "DescribeDBInstances",
				Fields: map[string]any{
					"VpcSecurityGroupId": *sg.VpcSecurityGroupId,
				},
			},
		})
		neighbors = append(neighbors, sgNode.ID)
	}

	// Discover parameter group
	if len(instance.DBParameterGroups) > 0 {
		for i := range instance.DBParameterGroups {
			pg := &instance.DBParameterGroups[i]
			if pg.DBParameterGroupName == nil {
				continue
			}
			pgNode := &graph.Node{
				ID:      *pg.DBParameterGroupName,
				Type:    ResourceTypeDBParameterGroup,
				Name:    *pg.DBParameterGroupName,
				Region:  node.Region,
				Account: node.Account,
				Metadata: map[string]any{
					"status": pg.ParameterApplyStatus,
				},
			}
			g.AddNode(pgNode)
			g.AddEdge(&graph.Edge{
				From:         node.ID,
				To:           pgNode.ID,
				RelationType: "uses-parameter-group",
				Evidence: graph.Evidence{
					APICall: "DescribeDBInstances",
					Fields: map[string]any{
						"DBParameterGroupName": *pg.DBParameterGroupName,
					},
				},
			})
			neighbors = append(neighbors, pgNode.ID)
		}
	}

	// Discover cluster membership (if instance is part of a cluster)
	if instance.DBClusterIdentifier != nil {
		clusterNode := &graph.Node{
			ID:      *instance.DBClusterIdentifier,
			Type:    ResourceTypeRDSCluster,
			Name:    *instance.DBClusterIdentifier,
			Region:  node.Region,
			Account: node.Account,
		}
		g.AddNode(clusterNode)
		g.AddEdge(&graph.Edge{
			From:         clusterNode.ID,
			To:           node.ID,
			RelationType: "contains",
			Evidence: graph.Evidence{
				APICall: "DescribeDBInstances",
				Fields: map[string]any{
					"DBClusterIdentifier": *instance.DBClusterIdentifier,
				},
			},
		})
		neighbors = append(neighbors, clusterNode.ID)
	}

	// Discover upstream connections using heuristics if enabled
	if d.hasHeuristic("rds-endpoint") && instance.Endpoint != nil && instance.Endpoint.Address != nil {
		upstreamNeighbors, heuristicErr := d.discoverRDSUpstream(ctx, *instance.Endpoint.Address, node, g)
		if heuristicErr != nil {
			slog.Warn("Failed to discover RDS upstream connections", "error", heuristicErr)
		} else {
			neighbors = append(neighbors, upstreamNeighbors...)
		}
	}

	return neighbors, nil
}

// discoverRDSCluster discovers dependencies for an RDS cluster
func (d *Discoverer) discoverRDSCluster(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering RDS cluster dependencies", "name", node.Name)

	var neighbors []string

	// Get cluster details
	output, err := d.clients.RDS.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{
		DBClusterIdentifier: &node.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe RDS cluster: %w", err)
	}

	if len(output.DBClusters) == 0 {
		return nil, fmt.Errorf("RDS cluster not found: %s", node.Name)
	}

	cluster := &output.DBClusters[0]

	// Discover cluster members (instances)
	for i := range cluster.DBClusterMembers {
		member := &cluster.DBClusterMembers[i]
		if member.DBInstanceIdentifier == nil {
			continue
		}
		instanceNode := &graph.Node{
			ID:      *member.DBInstanceIdentifier,
			Type:    ResourceTypeRDSInstance,
			Name:    *member.DBInstanceIdentifier,
			Region:  node.Region,
			Account: node.Account,
			Metadata: map[string]any{
				"isClusterWriter": member.IsClusterWriter,
			},
		}
		g.AddNode(instanceNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           instanceNode.ID,
			RelationType: "contains",
			Evidence: graph.Evidence{
				APICall: "DescribeDBClusters",
				Fields: map[string]any{
					"DBInstanceIdentifier": *member.DBInstanceIdentifier,
					"IsClusterWriter":      member.IsClusterWriter,
				},
			},
		})
		neighbors = append(neighbors, instanceNode.ID)
	}

	// Discover subnet group
	if cluster.DBSubnetGroup != nil {
		subnetGroupNode := &graph.Node{
			ID:      *cluster.DBSubnetGroup,
			Type:    ResourceTypeDBSubnetGroup,
			Name:    *cluster.DBSubnetGroup,
			Region:  node.Region,
			Account: node.Account,
		}
		g.AddNode(subnetGroupNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           subnetGroupNode.ID,
			RelationType: "uses-subnet-group",
			Evidence: graph.Evidence{
				APICall: "DescribeDBClusters",
				Fields: map[string]any{
					"DBSubnetGroup": *cluster.DBSubnetGroup,
				},
			},
		})
		neighbors = append(neighbors, subnetGroupNode.ID)
	}

	// Discover security groups
	for i := range cluster.VpcSecurityGroups {
		sg := &cluster.VpcSecurityGroups[i]
		if sg.VpcSecurityGroupId == nil {
			continue
		}
		sgNode := &graph.Node{
			ID:      *sg.VpcSecurityGroupId,
			Type:    ResourceTypeSecurityGroup,
			Name:    *sg.VpcSecurityGroupId,
			Region:  node.Region,
			Account: node.Account,
			Metadata: map[string]any{
				"status": sg.Status,
			},
		}
		g.AddNode(sgNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           sgNode.ID,
			RelationType: "uses-security-group",
			Evidence: graph.Evidence{
				APICall: "DescribeDBClusters",
				Fields: map[string]any{
					"VpcSecurityGroupId": *sg.VpcSecurityGroupId,
				},
			},
		})
		neighbors = append(neighbors, sgNode.ID)
	}

	// Discover parameter group
	if cluster.DBClusterParameterGroup != nil {
		pgNode := &graph.Node{
			ID:      *cluster.DBClusterParameterGroup,
			Type:    ResourceTypeDBClusterParameterGroup,
			Name:    *cluster.DBClusterParameterGroup,
			Region:  node.Region,
			Account: node.Account,
		}
		g.AddNode(pgNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           pgNode.ID,
			RelationType: "uses-parameter-group",
			Evidence: graph.Evidence{
				APICall: "DescribeDBClusters",
				Fields: map[string]any{
					"DBClusterParameterGroup": *cluster.DBClusterParameterGroup,
				},
			},
		})
		neighbors = append(neighbors, pgNode.ID)
	}

	// Discover upstream connections using heuristics if enabled
	if d.hasHeuristic("rds-endpoint") && cluster.Endpoint != nil {
		upstreamNeighbors, heuristicErr := d.discoverRDSUpstream(ctx, *cluster.Endpoint, node, g)
		if heuristicErr != nil {
			slog.Warn("Failed to discover RDS upstream connections", "error", heuristicErr)
		} else {
			neighbors = append(neighbors, upstreamNeighbors...)
		}
	}

	return neighbors, nil
}

// discoverRDSUpstream discovers upstream resources that connect to an RDS endpoint
// This uses heuristic-based discovery by searching for Lambda functions and ECS services
// that have environment variables containing the RDS endpoint
func (d *Discoverer) discoverRDSUpstream(ctx context.Context, endpoint string, rdsNode *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering RDS upstream connections (heuristic)", "endpoint", endpoint)

	var neighbors []string

	// This is a heuristic approach - we would need to:
	// 1. List all Lambda functions and check their environment variables
	// 2. List all ECS task definitions and check their environment variables
	// 3. Look for the RDS endpoint in connection strings
	//
	// For MVP, we'll log that this is a placeholder for heuristic discovery
	// and return empty list. Full implementation would be more complex.

	slog.Debug("RDS upstream heuristic discovery not yet fully implemented", "endpoint", endpoint)

	return neighbors, nil
}

// hasHeuristic checks if a specific heuristic is enabled
func (d *Discoverer) hasHeuristic(name string) bool {
	for _, h := range d.opts.Heuristics {
		if h == name {
			return true
		}
	}
	return false
}

// Helper function to convert RDS instance to graph node
func (d *Discoverer) rdsInstanceToNode(instance *rdstypes.DBInstance) *graph.Node {
	var name string
	if instance.DBInstanceIdentifier != nil {
		name = *instance.DBInstanceIdentifier
	}

	region, account := "", ""
	if instance.DBInstanceArn != nil {
		parts := strings.Split(*instance.DBInstanceArn, ":")
		if len(parts) >= 5 {
			region = parts[3]
			account = parts[4]
		}
	}

	metadata := map[string]any{
		"engine":        instance.Engine,
		"engineVersion": instance.EngineVersion,
		"status":        instance.DBInstanceStatus,
	}
	if instance.DBInstanceClass != nil {
		metadata["instanceClass"] = *instance.DBInstanceClass
	}
	if instance.AllocatedStorage != nil {
		metadata["allocatedStorage"] = *instance.AllocatedStorage
	}
	if instance.StorageType != nil {
		metadata["storageType"] = *instance.StorageType
	}
	if instance.MultiAZ != nil {
		metadata["multiAZ"] = *instance.MultiAZ
	}
	if instance.PubliclyAccessible != nil {
		metadata["publiclyAccessible"] = *instance.PubliclyAccessible
	}
	if instance.Endpoint != nil {
		if instance.Endpoint.Address != nil {
			metadata["endpoint"] = *instance.Endpoint.Address
		}
		if instance.Endpoint.Port != nil {
			metadata["port"] = *instance.Endpoint.Port
		}
	}

	return &graph.Node{
		ID:       *instance.DBInstanceArn,
		Type:     ResourceTypeRDSInstance,
		ARN:      *instance.DBInstanceArn,
		Name:     name,
		Region:   region,
		Account:  account,
		Metadata: metadata,
	}
}

// Helper function to convert RDS cluster to graph node
func (d *Discoverer) rdsClusterToNode(cluster *rdstypes.DBCluster) *graph.Node {
	var name string
	if cluster.DBClusterIdentifier != nil {
		name = *cluster.DBClusterIdentifier
	}

	region, account := "", ""
	if cluster.DBClusterArn != nil {
		parts := strings.Split(*cluster.DBClusterArn, ":")
		if len(parts) >= 5 {
			region = parts[3]
			account = parts[4]
		}
	}

	metadata := map[string]any{
		"engine":        cluster.Engine,
		"engineVersion": cluster.EngineVersion,
		"status":        cluster.Status,
	}
	if cluster.AllocatedStorage != nil {
		metadata["allocatedStorage"] = *cluster.AllocatedStorage
	}
	if cluster.StorageType != nil {
		metadata["storageType"] = *cluster.StorageType
	}
	if cluster.MultiAZ != nil {
		metadata["multiAZ"] = *cluster.MultiAZ
	}
	if cluster.Endpoint != nil {
		metadata["endpoint"] = *cluster.Endpoint
	}
	if cluster.Port != nil {
		metadata["port"] = *cluster.Port
	}
	if cluster.ReaderEndpoint != nil {
		metadata["readerEndpoint"] = *cluster.ReaderEndpoint
	}

	return &graph.Node{
		ID:       *cluster.DBClusterArn,
		Type:     ResourceTypeRDSCluster,
		ARN:      *cluster.DBClusterArn,
		Name:     name,
		Region:   region,
		Account:  account,
		Metadata: metadata,
	}
}
