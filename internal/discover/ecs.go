package discover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	appscalingtypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// resolveECSService resolves an ECS service by cluster and service name
func (d *Discoverer) resolveECSService(ctx context.Context, cluster, service string) (*graph.Node, error) {
	slog.Debug("Resolving ECS service", "cluster", cluster, "service", service)

	output, err := d.clients.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{service},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECS service: %w", err)
	}

	if len(output.Services) == 0 {
		return nil, fmt.Errorf("ECS service not found: %s/%s", cluster, service)
	}

	svc := &output.Services[0]
	return d.ecsServiceToNode(svc, cluster), nil
}

// discoverECSService discovers dependencies for an ECS service
func (d *Discoverer) discoverECSService(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering ECS service dependencies", "arn", node.ARN)

	var neighbors []string

	// Extract cluster and service name from metadata or ARN
	cluster, ok := node.Metadata["cluster"].(string)
	if !ok {
		// Try to parse from ARN
		parts := strings.Split(node.ARN, "/")
		if len(parts) >= 3 {
			cluster = parts[1]
		} else {
			return nil, fmt.Errorf("cannot determine cluster for ECS service: %s", node.ARN)
		}
	}

	// Get service details
	output, err := d.clients.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []string{node.ARN},
		Include:  []ecstypes.ServiceField{ecstypes.ServiceFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ECS service: %w", err)
	}

	if len(output.Services) == 0 {
		return nil, fmt.Errorf("ECS service not found: %s", node.ARN)
	}

	svc := &output.Services[0]

	// Discover cluster
	clusterNode := &graph.Node{
		ID:      cluster,
		Type:    "ECSCluster",
		Name:    cluster,
		Region:  node.Region,
		Account: node.Account,
	}
	g.AddNode(clusterNode)
	g.AddEdge(&graph.Edge{
		From:         node.ID,
		To:           clusterNode.ID,
		RelationType: "runs-in",
		Evidence: graph.Evidence{
			APICall: "DescribeServices",
			Fields: map[string]any{
				"ClusterArn": *svc.ClusterArn,
			},
		},
	})
	neighbors = append(neighbors, clusterNode.ID)

	// Discover task definition
	if svc.TaskDefinition != nil {
		tdNeighbors, err := d.discoverTaskDefinition(ctx, *svc.TaskDefinition, node, g)
		if err != nil {
			slog.Warn("Failed to discover task definition", "arn", *svc.TaskDefinition, "error", err)
		} else {
			neighbors = append(neighbors, tdNeighbors...)
		}
	}

	// Discover load balancers
	for i := range svc.LoadBalancers {
		lb := &svc.LoadBalancers[i]
		if lb.TargetGroupArn != nil {
			// Link to target group (which we may have already discovered via ALB)
			if g.HasNode(*lb.TargetGroupArn) {
				g.AddEdge(&graph.Edge{
					From:         node.ID,
					To:           *lb.TargetGroupArn,
					RelationType: "registers-with",
					Evidence: graph.Evidence{
						APICall: "DescribeServices",
						Fields: map[string]any{
							"TargetGroupArn": *lb.TargetGroupArn,
							"ContainerName":  lb.ContainerName,
							"ContainerPort":  lb.ContainerPort,
						},
					},
				})
				neighbors = append(neighbors, *lb.TargetGroupArn)
			} else {
				// Create target group node
				tgNode := &graph.Node{
					ID:      *lb.TargetGroupArn,
					Type:    "TargetGroup",
					ARN:     *lb.TargetGroupArn,
					Name:    extractNameFromARN(*lb.TargetGroupArn),
					Region:  node.Region,
					Account: node.Account,
				}
				g.AddNode(tgNode)
				g.AddEdge(&graph.Edge{
					From:         node.ID,
					To:           tgNode.ID,
					RelationType: "registers-with",
					Evidence: graph.Evidence{
						APICall: "DescribeServices",
						Fields: map[string]any{
							"TargetGroupArn": *lb.TargetGroupArn,
							"ContainerName":  lb.ContainerName,
							"ContainerPort":  lb.ContainerPort,
						},
					},
				})
				neighbors = append(neighbors, tgNode.ID)
			}
		}
	}

	// Discover security groups and subnets from network configuration
	if svc.NetworkConfiguration != nil && svc.NetworkConfiguration.AwsvpcConfiguration != nil {
		awsvpc := svc.NetworkConfiguration.AwsvpcConfiguration

		for _, sgID := range awsvpc.SecurityGroups {
			sgNode := &graph.Node{
				ID:      sgID,
				Type:    "SecurityGroup",
				Name:    sgID,
				Region:  node.Region,
				Account: node.Account,
			}
			g.AddNode(sgNode)
			g.AddEdge(&graph.Edge{
				From:         node.ID,
				To:           sgNode.ID,
				RelationType: "uses-security-group",
				Evidence: graph.Evidence{
					APICall: "DescribeServices",
					Fields: map[string]any{
						"SecurityGroups": awsvpc.SecurityGroups,
					},
				},
			})
			neighbors = append(neighbors, sgNode.ID)
		}

		for _, subnetID := range awsvpc.Subnets {
			subnetNode := &graph.Node{
				ID:      subnetID,
				Type:    "Subnet",
				Name:    subnetID,
				Region:  node.Region,
				Account: node.Account,
			}
			g.AddNode(subnetNode)
			g.AddEdge(&graph.Edge{
				From:         node.ID,
				To:           subnetNode.ID,
				RelationType: "runs-in-subnet",
				Evidence: graph.Evidence{
					APICall: "DescribeServices",
					Fields: map[string]any{
						"Subnets": awsvpc.Subnets,
					},
				},
			})
			neighbors = append(neighbors, subnetNode.ID)
		}
	}

	// Discover Application Auto Scaling policies
	scalingNeighbors, err := d.discoverECSScalingPolicies(ctx, cluster, *svc.ServiceName, node, g)
	if err != nil {
		slog.Warn("Failed to discover scaling policies", "error", err)
	} else {
		neighbors = append(neighbors, scalingNeighbors...)
	}

	return neighbors, nil
}

// discoverTaskDefinition discovers a task definition and its dependencies
func (d *Discoverer) discoverTaskDefinition(ctx context.Context, taskDefARN string, sourceNode *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering task definition", "arn", taskDefARN)

	var neighbors []string

	// Check if we've already discovered this task definition
	if g.HasNode(taskDefARN) {
		g.AddEdge(&graph.Edge{
			From:         sourceNode.ID,
			To:           taskDefARN,
			RelationType: "uses-task-definition",
			Evidence: graph.Evidence{
				APICall: "DescribeServices",
				Fields: map[string]any{
					"TaskDefinition": taskDefARN,
				},
			},
		})
		return []string{taskDefARN}, nil
	}

	output, err := d.clients.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDefARN,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe task definition: %w", err)
	}

	td := output.TaskDefinition
	tdNode := d.taskDefinitionToNode(td, sourceNode.Region, sourceNode.Account)
	g.AddNode(tdNode)
	g.AddEdge(&graph.Edge{
		From:         sourceNode.ID,
		To:           tdNode.ID,
		RelationType: "uses-task-definition",
		Evidence: graph.Evidence{
			APICall: "DescribeServices",
			Fields: map[string]any{
				"TaskDefinition": taskDefARN,
			},
		},
	})
	neighbors = append(neighbors, tdNode.ID)

	// Discover IAM roles
	if td.TaskRoleArn != nil {
		roleNode := &graph.Node{
			ID:      *td.TaskRoleArn,
			Type:    "IAMRole",
			ARN:     *td.TaskRoleArn,
			Name:    extractRoleNameFromARN(*td.TaskRoleArn),
			Region:  sourceNode.Region,
			Account: sourceNode.Account,
		}
		g.AddNode(roleNode)
		g.AddEdge(&graph.Edge{
			From:         tdNode.ID,
			To:           roleNode.ID,
			RelationType: "uses-task-role",
			Evidence: graph.Evidence{
				APICall: "DescribeTaskDefinition",
				Fields: map[string]any{
					"TaskRoleArn": *td.TaskRoleArn,
				},
			},
		})
		neighbors = append(neighbors, roleNode.ID)
	}

	if td.ExecutionRoleArn != nil {
		execRoleNode := &graph.Node{
			ID:      *td.ExecutionRoleArn,
			Type:    "IAMRole",
			ARN:     *td.ExecutionRoleArn,
			Name:    extractRoleNameFromARN(*td.ExecutionRoleArn),
			Region:  sourceNode.Region,
			Account: sourceNode.Account,
		}
		g.AddNode(execRoleNode)
		g.AddEdge(&graph.Edge{
			From:         tdNode.ID,
			To:           execRoleNode.ID,
			RelationType: "uses-execution-role",
			Evidence: graph.Evidence{
				APICall: "DescribeTaskDefinition",
				Fields: map[string]any{
					"ExecutionRoleArn": *td.ExecutionRoleArn,
				},
			},
		})
		neighbors = append(neighbors, execRoleNode.ID)
	}

	return neighbors, nil
}

// discoverECSScalingPolicies discovers Application Auto Scaling policies for an ECS service
func (d *Discoverer) discoverECSScalingPolicies(ctx context.Context, cluster, serviceName string, serviceNode *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering ECS scaling policies", "cluster", cluster, "service", serviceName)

	var neighbors []string

	resourceID := fmt.Sprintf("service/%s/%s", cluster, serviceName)

	// Describe scalable targets
	targetsOutput, err := d.clients.ApplicationAutoScaling.DescribeScalableTargets(ctx, &applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace: appscalingtypes.ServiceNamespaceEcs,
		ResourceIds:      []string{resourceID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe scalable targets: %w", err)
	}

	if len(targetsOutput.ScalableTargets) == 0 {
		// No scaling configured
		return neighbors, nil
	}

	// Describe scaling policies
	policiesOutput, err := d.clients.ApplicationAutoScaling.DescribeScalingPolicies(ctx, &applicationautoscaling.DescribeScalingPoliciesInput{
		ServiceNamespace: appscalingtypes.ServiceNamespaceEcs,
		ResourceId:       &resourceID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe scaling policies: %w", err)
	}

	for i := range policiesOutput.ScalingPolicies {
		policy := &policiesOutput.ScalingPolicies[i]
		policyNode := d.scalingPolicyToNode(policy, serviceNode.Region, serviceNode.Account)
		g.AddNode(policyNode)
		g.AddEdge(&graph.Edge{
			From:         serviceNode.ID,
			To:           policyNode.ID,
			RelationType: "has-scaling-policy",
			Evidence: graph.Evidence{
				APICall: "DescribeScalingPolicies",
				Fields: map[string]any{
					"PolicyName": *policy.PolicyName,
					"PolicyARN":  *policy.PolicyARN,
				},
			},
		})
		neighbors = append(neighbors, policyNode.ID)
	}

	return neighbors, nil
}

// Helper functions to convert AWS types to graph nodes

func (d *Discoverer) ecsServiceToNode(svc *ecstypes.Service, cluster string) *graph.Node {
	var name string
	if svc.ServiceName != nil {
		name = *svc.ServiceName
	}

	region, account := "", ""
	if svc.ServiceArn != nil {
		parts := strings.Split(*svc.ServiceArn, ":")
		if len(parts) >= 5 {
			region = parts[3]
			account = parts[4]
		}
	}

	metadata := map[string]any{
		"cluster":      cluster,
		"status":       svc.Status,
		"desiredCount": svc.DesiredCount,
		"runningCount": svc.RunningCount,
		"launchType":   svc.LaunchType,
	}
	if svc.TaskDefinition != nil {
		metadata["taskDefinition"] = *svc.TaskDefinition
	}

	tags := make(map[string]string)
	for i := range svc.Tags {
		tag := &svc.Tags[i]
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	return &graph.Node{
		ID:       *svc.ServiceArn,
		Type:     "ECSService",
		ARN:      *svc.ServiceArn,
		Name:     name,
		Region:   region,
		Account:  account,
		Tags:     tags,
		Metadata: metadata,
	}
}

func (d *Discoverer) taskDefinitionToNode(td *ecstypes.TaskDefinition, region, account string) *graph.Node {
	var name string
	if td.Family != nil {
		name = fmt.Sprintf("%s:%d", *td.Family, td.Revision)
	}

	metadata := map[string]any{
		"family":                  td.Family,
		"revision":                td.Revision,
		"cpu":                     td.Cpu,
		"memory":                  td.Memory,
		"networkMode":             td.NetworkMode,
		"requiresCompatibilities": td.RequiresCompatibilities,
	}

	// Add container information
	if len(td.ContainerDefinitions) > 0 {
		containers := make([]map[string]any, 0, len(td.ContainerDefinitions))
		for i := range td.ContainerDefinitions {
			container := &td.ContainerDefinitions[i]
			containerInfo := map[string]any{
				"name":  container.Name,
				"image": container.Image,
			}
			if container.Cpu != 0 {
				containerInfo["cpu"] = container.Cpu
			}
			if container.Memory != nil {
				containerInfo["memory"] = *container.Memory
			}
			containers = append(containers, containerInfo)
		}
		metadata["containers"] = containers
	}

	return &graph.Node{
		ID:       *td.TaskDefinitionArn,
		Type:     "TaskDefinition",
		ARN:      *td.TaskDefinitionArn,
		Name:     name,
		Region:   region,
		Account:  account,
		Metadata: metadata,
	}
}

func (d *Discoverer) scalingPolicyToNode(policy *appscalingtypes.ScalingPolicy, region, account string) *graph.Node {
	var name string
	if policy.PolicyName != nil {
		name = *policy.PolicyName
	}

	metadata := map[string]any{
		"policyType": policy.PolicyType,
	}
	if policy.TargetTrackingScalingPolicyConfiguration != nil {
		metadata["targetValue"] = policy.TargetTrackingScalingPolicyConfiguration.TargetValue
	}

	return &graph.Node{
		ID:       *policy.PolicyARN,
		Type:     "ScalingPolicy",
		ARN:      *policy.PolicyARN,
		Name:     name,
		Region:   region,
		Account:  account,
		Metadata: metadata,
	}
}

// Helper to extract name from ARN
func extractNameFromARN(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return arn
}

// Helper to extract role name from IAM role ARN
func extractRoleNameFromARN(arn string) string {
	// ARN format: arn:aws:iam::account:role/role-name
	parts := strings.Split(arn, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return arn
}
