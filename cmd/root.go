package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/pfrederiksen/blast-radius/internal/awsx"
	"github.com/pfrederiksen/blast-radius/internal/discover"
	"github.com/pfrederiksen/blast-radius/internal/graph"
	"github.com/pfrederiksen/blast-radius/internal/output"
)

var (
	// Global flags
	profile    string
	region     string
	depth      int
	format     string
	maxNodes   int
	debug      bool
	heuristics []string
)

var rootCmd = &cobra.Command{
	Use:   "blast-radius [resource-identifier]",
	Short: "Discover AWS resource dependencies and visualize blast radius",
	Long: `blast-radius discovers dependencies for AWS resources and produces a dependency graph.

Given an AWS resource identifier (ARN, ID, or name), blast-radius will discover
related dependencies and show you what might break if you modify the resource.

Supported resources:
  - Application/Network Load Balancers (ALB/NLB)
  - ECS Services
  - Lambda Functions
  - RDS Instances/Clusters

Examples:
  # Analyze an ALB by ARN
  blast-radius arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123

  # Analyze with specific profile and region
  blast-radius my-load-balancer --profile prod --region us-west-2

  # Output as Graphviz DOT
  blast-radius my-function --format dot

  # Control traversal depth
  blast-radius my-rds-instance --depth 3

  # Enable heuristics for RDS endpoint discovery
  blast-radius my-rds --heuristics rds-endpoint`,
	Args: cobra.ExactArgs(1),
	RunE: runGraph,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&profile, "profile", "", "AWS profile to use")
	rootCmd.Flags().StringVar(&region, "region", "", "AWS region (default: from config/environment)")
	rootCmd.Flags().IntVar(&depth, "depth", 2, "Maximum traversal depth")
	rootCmd.Flags().StringVar(&format, "format", "tree", "Output format: tree, dot, json")
	rootCmd.Flags().IntVar(&maxNodes, "max-nodes", 250, "Maximum nodes to discover")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.Flags().StringSliceVar(&heuristics, "heuristics", []string{}, "Enable heuristics: env-arn, rds-endpoint")
}

func runGraph(cmd *cobra.Command, args []string) error {
	// Setup logging
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	resourceID := args[0]
	ctx := context.Background()

	slog.Info("Starting blast-radius discovery",
		"resource", resourceID,
		"depth", depth,
		"maxNodes", maxNodes,
		"format", format)

	// Load AWS config
	cfg, err := awsx.LoadConfig(ctx, profile, region)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	slog.Debug("AWS config loaded",
		"region", cfg.Region,
		"profile", profile)

	// Initialize clients
	clients, err := awsx.NewClients(&cfg)
	if err != nil {
		return fmt.Errorf("failed to create AWS clients: %w", err)
	}

	// Create graph
	g := graph.New()

	// Discover dependencies
	discoverer := discover.New(clients, &discover.Options{
		MaxDepth:   depth,
		MaxNodes:   maxNodes,
		Heuristics: heuristics,
	})

	if err := discoverer.Discover(ctx, resourceID, g); err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	slog.Info("Discovery complete",
		"nodes", len(g.Nodes()),
		"edges", len(g.Edges()))

	// Output results
	switch format {
	case "tree":
		return output.RenderTree(os.Stdout, g, resourceID)
	case "dot":
		return output.RenderDOT(os.Stdout, g)
	case "json":
		return output.RenderJSON(os.Stdout, g)
	default:
		return fmt.Errorf("unknown format: %s (must be tree, dot, or json)", format)
	}
}
