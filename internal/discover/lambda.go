package discover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// resolveLambdaFunction resolves a Lambda function by name
func (d *Discoverer) resolveLambdaFunction(ctx context.Context, name string) (*graph.Node, error) {
	slog.Debug("Resolving Lambda function", "name", name)

	output, err := d.clients.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get Lambda function: %w", err)
	}

	return d.lambdaFunctionToNode(output.Configuration), nil
}

// discoverLambda discovers dependencies for a Lambda function
func (d *Discoverer) discoverLambda(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering Lambda function dependencies", "arn", node.ARN)

	var neighbors []string

	// Extract function name from ARN or node
	functionName := node.Name
	if functionName == "" {
		functionName = node.ARN
	}

	// Get function configuration
	output, err := d.clients.Lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: &functionName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get Lambda function: %w", err)
	}

	config := output.Configuration

	// Discover IAM execution role
	if config.Role != nil {
		roleNode := &graph.Node{
			ID:      *config.Role,
			Type:    "IAMRole",
			ARN:     *config.Role,
			Name:    extractRoleNameFromARN(*config.Role),
			Region:  node.Region,
			Account: node.Account,
		}
		g.AddNode(roleNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           roleNode.ID,
			RelationType: "uses-execution-role",
			Evidence: graph.Evidence{
				APICall: "GetFunction",
				Fields: map[string]any{
					"Role": *config.Role,
				},
			},
		})
		neighbors = append(neighbors, roleNode.ID)
	}

	// Discover VPC configuration
	if config.VpcConfig != nil && config.VpcConfig.VpcId != nil {
		// Add security groups
		for _, sgID := range config.VpcConfig.SecurityGroupIds {
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
					APICall: "GetFunction",
					Fields: map[string]any{
						"VpcConfig": config.VpcConfig,
					},
				},
			})
			neighbors = append(neighbors, sgNode.ID)
		}

		// Add subnets
		for _, subnetID := range config.VpcConfig.SubnetIds {
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
					APICall: "GetFunction",
					Fields: map[string]any{
						"VpcConfig": config.VpcConfig,
					},
				},
			})
			neighbors = append(neighbors, subnetNode.ID)
		}
	}

	// Discover Dead Letter Queue
	if config.DeadLetterConfig != nil && config.DeadLetterConfig.TargetArn != nil {
		dlqNode := &graph.Node{
			ID:      *config.DeadLetterConfig.TargetArn,
			Type:    "DLQ",
			ARN:     *config.DeadLetterConfig.TargetArn,
			Name:    extractNameFromARN(*config.DeadLetterConfig.TargetArn),
			Region:  node.Region,
			Account: node.Account,
		}
		g.AddNode(dlqNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           dlqNode.ID,
			RelationType: "sends-failures-to",
			Evidence: graph.Evidence{
				APICall: "GetFunction",
				Fields: map[string]any{
					"TargetArn": *config.DeadLetterConfig.TargetArn,
				},
			},
		})
		neighbors = append(neighbors, dlqNode.ID)
	}

	// Discover event source mappings
	eventSourceNeighbors, eventSourceErr := d.discoverEventSourceMappings(ctx, node.ARN, node, g)
	if eventSourceErr != nil {
		slog.Warn("Failed to discover event source mappings", "error", eventSourceErr)
	} else {
		neighbors = append(neighbors, eventSourceNeighbors...)
	}

	// Discover function event invoke config (destinations)
	destinationNeighbors, destErr := d.discoverFunctionDestinations(ctx, functionName, node, g)
	if destErr != nil {
		slog.Warn("Failed to discover function destinations", "error", destErr)
	} else {
		neighbors = append(neighbors, destinationNeighbors...)
	}

	return neighbors, nil
}

// discoverEventSourceMappings discovers event source mappings for a Lambda function
func (d *Discoverer) discoverEventSourceMappings(ctx context.Context, functionARN string, lambdaNode *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering event source mappings", "functionArn", functionARN)

	var neighbors []string

	paginator := lambda.NewListEventSourceMappingsPaginator(d.clients.Lambda, &lambda.ListEventSourceMappingsInput{
		FunctionName: &functionARN,
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list event source mappings: %w", err)
		}

		for i := range output.EventSourceMappings {
			mapping := &output.EventSourceMappings[i]
			if mapping.EventSourceArn == nil {
				continue
			}

			// Determine source type from ARN
			sourceType := "EventSource"
			switch {
			case strings.Contains(*mapping.EventSourceArn, ":sqs:"):
				sourceType = "SQSQueue"
			case strings.Contains(*mapping.EventSourceArn, ":dynamodb:"):
				sourceType = "DynamoDBStream"
			case strings.Contains(*mapping.EventSourceArn, ":kinesis:"):
				sourceType = "KinesisStream"
			case strings.Contains(*mapping.EventSourceArn, ":kafka:"):
				sourceType = "KafkaCluster"
			}

			sourceNode := &graph.Node{
				ID:      *mapping.EventSourceArn,
				Type:    sourceType,
				ARN:     *mapping.EventSourceArn,
				Name:    extractNameFromARN(*mapping.EventSourceArn),
				Region:  lambdaNode.Region,
				Account: lambdaNode.Account,
				Metadata: map[string]any{
					"state":     mapping.State,
					"batchSize": mapping.BatchSize,
				},
			}

			if mapping.UUID != nil {
				sourceNode.Metadata["uuid"] = *mapping.UUID
			}

			g.AddNode(sourceNode)
			g.AddEdge(&graph.Edge{
				From:         sourceNode.ID,
				To:           lambdaNode.ID,
				RelationType: "triggers",
				Evidence: graph.Evidence{
					APICall: "ListEventSourceMappings",
					Fields: map[string]any{
						"EventSourceArn": *mapping.EventSourceArn,
						"UUID":           mapping.UUID,
						"State":          mapping.State,
					},
				},
			})
			neighbors = append(neighbors, sourceNode.ID)

			// Discover destination on failure for event source mapping
			if mapping.DestinationConfig != nil && mapping.DestinationConfig.OnFailure != nil && mapping.DestinationConfig.OnFailure.Destination != nil {
				destNode := &graph.Node{
					ID:      *mapping.DestinationConfig.OnFailure.Destination,
					Type:    "EventDestination",
					ARN:     *mapping.DestinationConfig.OnFailure.Destination,
					Name:    extractNameFromARN(*mapping.DestinationConfig.OnFailure.Destination),
					Region:  lambdaNode.Region,
					Account: lambdaNode.Account,
				}
				g.AddNode(destNode)
				g.AddEdge(&graph.Edge{
					From:         sourceNode.ID,
					To:           destNode.ID,
					RelationType: "sends-failures-to",
					Evidence: graph.Evidence{
						APICall: "ListEventSourceMappings",
						Fields: map[string]any{
							"Destination": *mapping.DestinationConfig.OnFailure.Destination,
						},
					},
				})
				neighbors = append(neighbors, destNode.ID)
			}
		}
	}

	return neighbors, nil
}

// discoverFunctionDestinations discovers Lambda function event invoke config destinations
func (d *Discoverer) discoverFunctionDestinations(ctx context.Context, functionName string, lambdaNode *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering function destinations", "function", functionName)

	var neighbors []string

	// Get function event invoke config
	output, err := d.clients.Lambda.GetFunctionEventInvokeConfig(ctx, &lambda.GetFunctionEventInvokeConfigInput{
		FunctionName: &functionName,
	})
	if err != nil {
		// Not all functions have event invoke config, this is not an error
		slog.Debug("No event invoke config found", "function", functionName)
		return neighbors, nil
	}

	if output.DestinationConfig == nil {
		return neighbors, nil
	}

	// Discover OnSuccess destination
	if output.DestinationConfig.OnSuccess != nil && output.DestinationConfig.OnSuccess.Destination != nil {
		destNode := &graph.Node{
			ID:      *output.DestinationConfig.OnSuccess.Destination,
			Type:    "EventDestination",
			ARN:     *output.DestinationConfig.OnSuccess.Destination,
			Name:    extractNameFromARN(*output.DestinationConfig.OnSuccess.Destination),
			Region:  lambdaNode.Region,
			Account: lambdaNode.Account,
			Metadata: map[string]any{
				"destinationType": "OnSuccess",
			},
		}
		g.AddNode(destNode)
		g.AddEdge(&graph.Edge{
			From:         lambdaNode.ID,
			To:           destNode.ID,
			RelationType: "sends-success-to",
			Evidence: graph.Evidence{
				APICall: "GetFunctionEventInvokeConfig",
				Fields: map[string]any{
					"Destination": *output.DestinationConfig.OnSuccess.Destination,
				},
			},
		})
		neighbors = append(neighbors, destNode.ID)
	}

	// Discover OnFailure destination
	if output.DestinationConfig.OnFailure != nil && output.DestinationConfig.OnFailure.Destination != nil {
		destNode := &graph.Node{
			ID:      *output.DestinationConfig.OnFailure.Destination,
			Type:    "EventDestination",
			ARN:     *output.DestinationConfig.OnFailure.Destination,
			Name:    extractNameFromARN(*output.DestinationConfig.OnFailure.Destination),
			Region:  lambdaNode.Region,
			Account: lambdaNode.Account,
			Metadata: map[string]any{
				"destinationType": "OnFailure",
			},
		}
		g.AddNode(destNode)
		g.AddEdge(&graph.Edge{
			From:         lambdaNode.ID,
			To:           destNode.ID,
			RelationType: "sends-failures-to",
			Evidence: graph.Evidence{
				APICall: "GetFunctionEventInvokeConfig",
				Fields: map[string]any{
					"Destination": *output.DestinationConfig.OnFailure.Destination,
				},
			},
		})
		neighbors = append(neighbors, destNode.ID)
	}

	return neighbors, nil
}

// Helper function to convert Lambda function to graph node
func (d *Discoverer) lambdaFunctionToNode(config *lambdatypes.FunctionConfiguration) *graph.Node {
	var name string
	if config.FunctionName != nil {
		name = *config.FunctionName
	}

	region, account := "", ""
	if config.FunctionArn != nil {
		parts := strings.Split(*config.FunctionArn, ":")
		if len(parts) >= 5 {
			region = parts[3]
			account = parts[4]
		}
	}

	metadata := map[string]any{
		"runtime":      config.Runtime,
		"handler":      config.Handler,
		"state":        config.State,
		"lastModified": config.LastModified,
	}
	if config.MemorySize != nil {
		metadata["memorySize"] = *config.MemorySize
	}
	if config.Timeout != nil {
		metadata["timeout"] = *config.Timeout
	}
	if config.CodeSize != 0 {
		metadata["codeSize"] = config.CodeSize
	}
	if config.Description != nil {
		metadata["description"] = *config.Description
	}

	// Add environment variables count (not the actual values for security)
	if config.Environment != nil && config.Environment.Variables != nil {
		metadata["environmentVariablesCount"] = len(config.Environment.Variables)
	}

	// Add layers
	if len(config.Layers) > 0 {
		layers := make([]string, 0, len(config.Layers))
		for i := range config.Layers {
			layer := &config.Layers[i]
			if layer.Arn != nil {
				layers = append(layers, *layer.Arn)
			}
		}
		metadata["layers"] = layers
	}

	return &graph.Node{
		ID:       *config.FunctionArn,
		Type:     "Lambda",
		ARN:      *config.FunctionArn,
		Name:     name,
		Region:   region,
		Account:  account,
		Metadata: metadata,
	}
}
