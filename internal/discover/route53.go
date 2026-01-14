package discover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/pfrederiksen/blast-radius/internal/graph"
)

// discoverRoute53Aliases discovers Route53 records that alias to a given DNS name
func (d *Discoverer) discoverRoute53Aliases(ctx context.Context, dnsName string, targetNode *graph.Node, g *graph.Graph) ([]string, error) {
	slog.Debug("Discovering Route53 aliases", "dnsName", dnsName)

	var neighbors []string

	// Normalize DNS name (remove trailing dot if present)
	dnsName = strings.TrimSuffix(dnsName, ".")

	// List all hosted zones
	hostedZones, err := d.listHostedZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosted zones: %w", err)
	}

	// Search each hosted zone for alias records pointing to this DNS name
	for _, zone := range hostedZones {
		if zone.Id == nil {
			continue
		}

		records, err := d.findAliasRecordsInZone(ctx, *zone.Id, dnsName)
		if err != nil {
			slog.Warn("Failed to search hosted zone for aliases",
				"zoneId", *zone.Id,
				"error", err)
			continue
		}

		for _, record := range records {
			recordNode := d.route53RecordToNode(record, zone, targetNode.Region, targetNode.Account)
			g.AddNode(recordNode)
			g.AddEdge(&graph.Edge{
				From:         recordNode.ID,
				To:           targetNode.ID,
				RelationType: "aliases-to",
				Evidence: graph.Evidence{
					APICall: "ListResourceRecordSets",
					Fields: map[string]any{
						"Name":            record.Name,
						"Type":            record.Type,
						"AliasTarget":     record.AliasTarget,
						"HostedZoneId":    *zone.Id,
						"HostedZoneName":  zone.Name,
					},
				},
			})
			neighbors = append(neighbors, recordNode.ID)
		}
	}

	return neighbors, nil
}

// listHostedZones lists all Route53 hosted zones
func (d *Discoverer) listHostedZones(ctx context.Context) ([]route53types.HostedZone, error) {
	var zones []route53types.HostedZone

	paginator := route53.NewListHostedZonesPaginator(d.clients.Route53, &route53.ListHostedZonesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list hosted zones: %w", err)
		}
		zones = append(zones, output.HostedZones...)
	}

	return zones, nil
}

// findAliasRecordsInZone finds alias records in a hosted zone that point to the given DNS name
func (d *Discoverer) findAliasRecordsInZone(ctx context.Context, hostedZoneID, targetDNS string) ([]route53types.ResourceRecordSet, error) {
	var matchingRecords []route53types.ResourceRecordSet

	// Normalize target DNS name
	targetDNS = strings.TrimSuffix(targetDNS, ".")
	targetDNSWithDot := targetDNS + "."

	paginator := route53.NewListResourceRecordSetsPaginator(d.clients.Route53, &route53.ListResourceRecordSetsInput{
		HostedZoneId: &hostedZoneID,
	})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list resource record sets: %w", err)
		}

		for _, record := range output.ResourceRecordSets {
			// Check if this is an alias record
			if record.AliasTarget == nil || record.AliasTarget.DNSName == nil {
				continue
			}

			// Normalize alias target DNS name
			aliasDNS := strings.TrimSuffix(*record.AliasTarget.DNSName, ".")

			// Check if it matches our target
			if aliasDNS == targetDNS || *record.AliasTarget.DNSName == targetDNSWithDot {
				matchingRecords = append(matchingRecords, record)
			}
		}
	}

	return matchingRecords, nil
}

// route53RecordToNode converts a Route53 record to a graph node
func (d *Discoverer) route53RecordToNode(record route53types.ResourceRecordSet, zone route53types.HostedZone, region, account string) *graph.Node {
	var name string
	if record.Name != nil {
		name = strings.TrimSuffix(*record.Name, ".")
	}

	// Route53 is a global service, but we'll use the region of the target resource
	// for consistency in the graph
	metadata := map[string]any{
		"type": record.Type,
	}
	if zone.Id != nil {
		metadata["hostedZoneId"] = *zone.Id
	}
	if zone.Name != nil {
		metadata["hostedZoneName"] = strings.TrimSuffix(*zone.Name, ".")
	}
	if record.AliasTarget != nil {
		metadata["aliasTarget"] = map[string]any{
			"dnsName":              record.AliasTarget.DNSName,
			"hostedZoneId":         record.AliasTarget.HostedZoneId,
			"evaluateTargetHealth": record.AliasTarget.EvaluateTargetHealth,
		}
	}
	if record.SetIdentifier != nil {
		metadata["setIdentifier"] = *record.SetIdentifier
	}

	// Generate a unique ID for the record
	id := fmt.Sprintf("route53:%s:%s:%s", *zone.Id, name, record.Type)

	return &graph.Node{
		ID:       id,
		Type:     "Route53Record",
		Name:     name,
		Region:   region,
		Account:  account,
		Metadata: metadata,
	}
}
