package discover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// resolveLoadBalancerByName resolves a load balancer by name
func (d *Discoverer) resolveLoadBalancerByName(ctx context.Context, name string) (*graph.Node, error) {
	slog.Debug("Resolving load balancer by name", "name", name)

	// List all load balancers and filter by name
	paginator := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(d.clients.ELBv2, &elasticloadbalancingv2.DescribeLoadBalancersInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe load balancers: %w", err)
		}

		for i := range output.LoadBalancers {
			lb := &output.LoadBalancers[i]
			if lb.LoadBalancerName != nil && *lb.LoadBalancerName == name {
				return d.loadBalancerToNode(lb), nil
			}
		}
	}

	return nil, fmt.Errorf("load balancer not found: %s", name)
}

// discoverLoadBalancer discovers dependencies for a load balancer
func (d *Discoverer) discoverLoadBalancer(ctx context.Context, node *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering load balancer dependencies", "arn", node.ARN)

	var neighbors []string

	// Get load balancer details
	output, err := d.clients.ELBv2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{node.ARN},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe load balancer: %w", err)
	}

	if len(output.LoadBalancers) == 0 {
		return nil, fmt.Errorf("load balancer not found: %s", node.ARN)
	}

	lb := &output.LoadBalancers[0]

	// Add security groups
	for _, sgID := range lb.SecurityGroups {
		sgNode := &graph.Node{
			ID:      sgID,
			Type:    "SecurityGroup",
			Name:    sgID,
			Region:  node.Region,
			Account: node.Account,
			Metadata: map[string]any{
				"attachedTo": node.Name,
			},
		}
		g.AddNode(sgNode)
		g.AddEdge(&graph.Edge{
			From:         node.ID,
			To:           sgNode.ID,
			RelationType: "uses-security-group",
			Evidence: graph.Evidence{
				APICall: "DescribeLoadBalancers",
				Fields: map[string]any{
					"SecurityGroups": lb.SecurityGroups,
				},
			},
		})
		neighbors = append(neighbors, sgNode.ID)
	}

	// Add subnets
	for _, subnet := range lb.AvailabilityZones {
		if subnet.SubnetId != nil {
			subnetNode := &graph.Node{
				ID:      *subnet.SubnetId,
				Type:    "Subnet",
				Name:    *subnet.SubnetId,
				Region:  node.Region,
				Account: node.Account,
				Metadata: map[string]any{
					"availabilityZone": *subnet.ZoneName,
				},
			}
			g.AddNode(subnetNode)
			g.AddEdge(&graph.Edge{
				From:         node.ID,
				To:           subnetNode.ID,
				RelationType: "uses-subnet",
				Evidence: graph.Evidence{
					APICall: "DescribeLoadBalancers",
					Fields: map[string]any{
						"AvailabilityZones": lb.AvailabilityZones,
					},
				},
			})
			neighbors = append(neighbors, subnetNode.ID)
		}
	}

	// Discover listeners
	listenerNeighbors, err := d.discoverListeners(ctx, node, g)
	if err != nil {
		slog.Warn("Failed to discover listeners", "error", err)
	} else {
		neighbors = append(neighbors, listenerNeighbors...)
	}

	// Discover Route53 upstream (records that alias to this LB)
	if lb.DNSName != nil {
		route53Neighbors, err := d.discoverRoute53Aliases(ctx, *lb.DNSName, node, g)
		if err != nil {
			slog.Warn("Failed to discover Route53 aliases", "error", err)
		} else {
			neighbors = append(neighbors, route53Neighbors...)
		}
	}

	return neighbors, nil
}

// discoverListeners discovers listeners for a load balancer
func (d *Discoverer) discoverListeners(ctx context.Context, lbNode *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering listeners", "loadBalancer", lbNode.ARN)

	var neighbors []string

	paginator := elasticloadbalancingv2.NewDescribeListenersPaginator(d.clients.ELBv2, &elasticloadbalancingv2.DescribeListenersInput{
		LoadBalancerArn: &lbNode.ARN,
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe listeners: %w", err)
		}

		for i := range output.Listeners {
			listener := &output.Listeners[i]
			listenerNode := d.listenerToNode(listener, lbNode.Region, lbNode.Account)
			g.AddNode(listenerNode)
			g.AddEdge(&graph.Edge{
				From:         lbNode.ID,
				To:           listenerNode.ID,
				RelationType: "has-listener",
				Evidence: graph.Evidence{
					APICall: "DescribeListeners",
					Fields: map[string]any{
						"ListenerArn": *listener.ListenerArn,
						"Port":        *listener.Port,
						"Protocol":    listener.Protocol,
					},
				},
			})
			neighbors = append(neighbors, listenerNode.ID)

			// Discover default actions (target groups)
			for _, action := range listener.DefaultActions {
				if action.TargetGroupArn != nil {
					tgNeighbors, err := d.discoverTargetGroup(ctx, *action.TargetGroupArn, listenerNode, g)
					if err != nil {
						slog.Warn("Failed to discover target group", "arn", *action.TargetGroupArn, "error", err)
					} else {
						neighbors = append(neighbors, tgNeighbors...)
					}
				}
			}

			// Discover listener rules
			ruleNeighbors, err := d.discoverListenerRules(ctx, listener, listenerNode, g)
			if err != nil {
				slog.Warn("Failed to discover listener rules", "error", err)
			} else {
				neighbors = append(neighbors, ruleNeighbors...)
			}
		}
	}

	return neighbors, nil
}

// discoverListenerRules discovers rules for a listener
func (d *Discoverer) discoverListenerRules(ctx context.Context, listener *elbv2types.Listener, listenerNode *graph.Node, g *graph.Graph) ([]string, error) {
	var neighbors []string

	paginator := elasticloadbalancingv2.NewDescribeRulesPaginator(d.clients.ELBv2, &elasticloadbalancingv2.DescribeRulesInput{
		ListenerArn: listener.ListenerArn,
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe rules: %w", err)
		}

		for _, rule := range output.Rules {
			// Skip default rule (already handled)
			if rule.IsDefault != nil && *rule.IsDefault {
				continue
			}

			// Process forward actions to target groups
			for _, action := range rule.Actions {
				if action.TargetGroupArn != nil {
					tgNeighbors, err := d.discoverTargetGroup(ctx, *action.TargetGroupArn, listenerNode, g)
					if err != nil {
						slog.Warn("Failed to discover target group from rule", "arn", *action.TargetGroupArn, "error", err)
					} else {
						neighbors = append(neighbors, tgNeighbors...)
					}
				}
			}
		}
	}

	return neighbors, nil
}

// discoverTargetGroup discovers a target group and its targets
func (d *Discoverer) discoverTargetGroup(ctx context.Context, tgARN string, sourceNode *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering target group", "arn", tgARN)

	var neighbors []string

	// Check if we've already discovered this target group
	if g.HasNode(tgARN) {
		// Just add the edge
		g.AddEdge(&graph.Edge{
			From:         sourceNode.ID,
			To:           tgARN,
			RelationType: "forwards-to",
			Evidence: graph.Evidence{
				APICall: "Listener/Rule DefaultActions",
				Fields: map[string]any{
					"TargetGroupArn": tgARN,
				},
			},
		})
		return []string{tgARN}, nil
	}

	// Describe target group
	output, err := d.clients.ELBv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{tgARN},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe target group: %w", err)
	}

	if len(output.TargetGroups) == 0 {
		return nil, fmt.Errorf("target group not found: %s", tgARN)
	}

	tg := &output.TargetGroups[0]
	tgNode := d.targetGroupToNode(tg)
	g.AddNode(tgNode)
	g.AddEdge(&graph.Edge{
		From:         sourceNode.ID,
		To:           tgNode.ID,
		RelationType: "forwards-to",
		Evidence: graph.Evidence{
			APICall: "Listener/Rule DefaultActions",
			Fields: map[string]any{
				"TargetGroupArn": tgARN,
			},
		},
	})
	neighbors = append(neighbors, tgNode.ID)

	// Discover target health
	healthOutput, err := d.clients.ELBv2.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: &tgARN,
	})
	if err != nil {
		slog.Warn("Failed to describe target health", "error", err)
		return neighbors, nil
	}

	// Add targets
	for _, targetHealth := range healthOutput.TargetHealthDescriptions {
		if targetHealth.Target == nil {
			continue
		}

		target := targetHealth.Target
		var targetNode *graph.Node

		switch tg.TargetType {
		case elbv2types.TargetTypeEnumInstance:
			targetNode = &graph.Node{
				ID:      *target.Id,
				Type:    "EC2Instance",
				Name:    *target.Id,
				Region:  tgNode.Region,
				Account: tgNode.Account,
				Metadata: map[string]any{
					"port": target.Port,
				},
			}
		case elbv2types.TargetTypeEnumIp:
			targetNode = &graph.Node{
				ID:      *target.Id,
				Type:    "IPTarget",
				Name:    *target.Id,
				Region:  tgNode.Region,
				Account: tgNode.Account,
				Metadata: map[string]any{
					"port": target.Port,
				},
			}
		case elbv2types.TargetTypeEnumLambda:
			targetNode = &graph.Node{
				ID:      *target.Id,
				Type:    "Lambda",
				ARN:     *target.Id,
				Name:    d.extractLambdaNameFromARN(*target.Id),
				Region:  tgNode.Region,
				Account: tgNode.Account,
			}
		default:
			continue
		}

		g.AddNode(targetNode)
		g.AddEdge(&graph.Edge{
			From:         tgNode.ID,
			To:           targetNode.ID,
			RelationType: "routes-to-target",
			Evidence: graph.Evidence{
				APICall: "DescribeTargetHealth",
				Fields: map[string]any{
					"TargetId":     *target.Id,
					"TargetHealth": targetHealth.TargetHealth,
				},
			},
		})
		neighbors = append(neighbors, targetNode.ID)
	}

	return neighbors, nil
}

// Helper functions to convert AWS types to graph nodes

func (d *Discoverer) loadBalancerToNode(lb *elbv2types.LoadBalancer) *graph.Node {
	var name string
	if lb.LoadBalancerName != nil {
		name = *lb.LoadBalancerName
	}

	// Parse ARN for region and account
	region, account := "", ""
	if lb.LoadBalancerArn != nil {
		parts := strings.Split(*lb.LoadBalancerArn, ":")
		if len(parts) >= 5 {
			region = parts[3]
			account = parts[4]
		}
	}

	metadata := map[string]any{
		"type":   lb.Type,
		"scheme": lb.Scheme,
		"state":  lb.State,
	}
	if lb.DNSName != nil {
		metadata["dnsName"] = *lb.DNSName
	}
	if lb.VpcId != nil {
		metadata["vpcId"] = *lb.VpcId
	}

	tags := make(map[string]string)
	// Note: Tags would need a separate DescribeTags call, skipping for now

	return &graph.Node{
		ID:       *lb.LoadBalancerArn,
		Type:     "LoadBalancer",
		ARN:      *lb.LoadBalancerArn,
		Name:     name,
		Region:   region,
		Account:  account,
		Tags:     tags,
		Metadata: metadata,
	}
}

func (d *Discoverer) listenerToNode(listener *elbv2types.Listener, region, account string) *graph.Node {
	var name string
	if listener.Port != nil && listener.Protocol != "" {
		name = fmt.Sprintf("%s:%d", listener.Protocol, *listener.Port)
	}

	metadata := map[string]any{
		"port":     listener.Port,
		"protocol": listener.Protocol,
	}
	if len(listener.Certificates) > 0 {
		metadata["certificateArn"] = listener.Certificates[0].CertificateArn
	}

	return &graph.Node{
		ID:       *listener.ListenerArn,
		Type:     "Listener",
		ARN:      *listener.ListenerArn,
		Name:     name,
		Region:   region,
		Account:  account,
		Metadata: metadata,
	}
}

func (d *Discoverer) targetGroupToNode(tg *elbv2types.TargetGroup) *graph.Node {
	var name string
	if tg.TargetGroupName != nil {
		name = *tg.TargetGroupName
	}

	// Parse ARN for region and account
	region, account := "", ""
	if tg.TargetGroupArn != nil {
		parts := strings.Split(*tg.TargetGroupArn, ":")
		if len(parts) >= 5 {
			region = parts[3]
			account = parts[4]
		}
	}

	metadata := map[string]any{
		"targetType": tg.TargetType,
		"protocol":   tg.Protocol,
		"port":       tg.Port,
	}
	if tg.VpcId != nil {
		metadata["vpcId"] = *tg.VpcId
	}

	return &graph.Node{
		ID:       *tg.TargetGroupArn,
		Type:     "TargetGroup",
		ARN:      *tg.TargetGroupArn,
		Name:     name,
		Region:   region,
		Account:  account,
		Metadata: metadata,
	}
}

func (d *Discoverer) extractLambdaNameFromARN(arn string) string {
	// ARN format: arn:aws:lambda:region:account:function:function-name
	parts := strings.Split(arn, ":")
	if len(parts) >= 7 && parts[5] == "function" {
		return parts[6]
	}
	return arn
}
