# blast-radius

[![CI](https://github.com/pfrederiksen/blast-radius/actions/workflows/pr.yml/badge.svg)](https://github.com/pfrederiksen/blast-radius/actions/workflows/pr.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/pfrederiksen/blast-radius)](https://goreportcard.com/report/github.com/pfrederiksen/blast-radius)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Local-first AWS dependency graph CLI to understand blast radius before changes.

## Overview

`blast-radius` is a command-line tool that, given an AWS resource identifier (ARN, ID, or name), discovers related dependencies and produces a dependency graph. This helps operators answer the critical question: **"What will break if I touch this?"**

## Features

- **Local-first**: No backend required, uses your existing AWS credentials
- **Multi-resource support**: ALB/NLB, ECS services, Lambda functions, RDS instances/clusters, and more
- **Flexible output formats**: Human-friendly tree view (default), Graphviz DOT, or JSON
- **Permission-resilient**: Missing permissions won't crash the tool; they're annotated and discovery continues
- **Depth control**: Configure how deep to traverse the dependency graph
- **AWS SSO friendly**: Uses the default AWS credential chain

## Installation

### Homebrew (macOS/Linux)

```bash
brew install pfrederiksen/tap/blast-radius
```

### Go Install

```bash
go install github.com/pfrederiksen/blast-radius@latest
```

### Binary Download

Download the latest release from the [releases page](https://github.com/pfrederiksen/blast-radius/releases).

## Quick Start

```bash
# Analyze an ALB by ARN
blast-radius arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/1234567890abcdef

# Analyze an ECS service
blast-radius arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-service

# Use a specific AWS profile and region
blast-radius my-load-balancer --profile prod --region us-west-2

# Output as Graphviz DOT for visualization
blast-radius arn:aws:lambda:us-east-1:123456789012:function:my-function --format dot

# Control traversal depth
blast-radius my-rds-instance --depth 3

# Enable debug logging
blast-radius my-resource --debug
```

## Usage

```
Usage:
  blast-radius [resource-identifier] [flags]

Flags:
      --depth int          Maximum traversal depth (default: 2)
      --format string      Output format: tree, dot, json (default: "tree")
      --profile string     AWS profile to use
      --region string      AWS region (default: from config/environment)
      --max-nodes int      Maximum nodes to discover (default: 250)
      --debug              Enable debug logging
  -h, --help              help for blast-radius
```

## Supported Resources

### Application/Network Load Balancers (ALB/NLB) ✅
**Status: Fully implemented**
- Listeners and listener rules
- Target groups and registered targets (EC2 instances, IP targets, Lambda functions)
- Security groups and VPC/subnets
- Upstream Route 53 alias records (discovers DNS records pointing to the load balancer)
- Target health status

**Resolution methods:**
- By ARN: `arn:aws:elasticloadbalancing:region:account:loadbalancer/app/name/id`
- By name: `my-load-balancer`

### ECS Services ✅
**Status: Fully implemented**
- Task definitions with container information
- Load balancers and target groups (bidirectional discovery with ALB)
- IAM roles (task role, execution role)
- Security groups and VPC/subnets (from awsvpc network mode)
- Application Auto Scaling policies (target tracking, step scaling)
- Cluster membership

**Resolution methods:**
- By ARN: `arn:aws:ecs:region:account:service/cluster-name/service-name`
- By cluster/service: `cluster-name/service-name`

### Lambda Functions ✅
**Status: Fully implemented**
- IAM execution role
- Event source mappings (SQS, DynamoDB streams, Kinesis streams, Kafka)
- Event source mapping destinations (OnFailure)
- Function event invoke config destinations (OnSuccess, OnFailure)
- Dead letter queue configuration
- VPC configuration (security groups, subnets)
- Function metadata (runtime, handler, memory, timeout, layers)

**Resolution methods:**
- By ARN: `arn:aws:lambda:region:account:function:function-name`
- By name: `function-name`

### RDS Instances/Clusters ✅
**Status: Fully implemented**
- DB instances and Aurora clusters
- Subnet groups and VPC with individual subnets
- Security groups with status
- Parameter groups (instance and cluster-level)
- Cluster membership (instances in clusters, and vice versa)
- Complete instance and cluster metadata (engine, version, storage, multi-AZ, endpoints)
- Heuristic-based upstream discovery (experimental, with `--heuristics rds-endpoint`)

**Resolution methods:**
- Instance by identifier: `my-database-instance`
- Cluster by identifier: `my-aurora-cluster`
- By ARN: `arn:aws:rds:region:account:db:instance-name`
- By ARN: `arn:aws:rds:region:account:cluster:cluster-name`

## Architecture

`blast-radius` uses a breadth-first traversal algorithm to discover dependencies:

1. **Resource Identification**: Parses input (ARN or name) and determines resource type
2. **Graph Building**: Uses AWS SDK v2 to query related resources
3. **BFS Traversal**: Expands outward from the starting resource up to specified depth
4. **Output Formatting**: Renders the dependency graph in the requested format

Each node in the graph contains:
- ID, type, ARN, name
- Region and account
- Tags and metadata

Each edge contains:
- Source and target nodes
- Relationship type
- Evidence (API call and key fields)

### Discovery Implementation

**ALB/NLB Discovery:**
- Resolves load balancers by name or ARN
- Discovers listeners via `DescribeListeners` (with pagination)
- Discovers listener rules via `DescribeRules` (with pagination)
- Discovers target groups via `DescribeTargetGroups`
- Discovers target health and registered targets via `DescribeTargetHealth`
- Maps targets to EC2 instances, IP addresses, or Lambda functions based on target type
- Discovers security groups and subnets from load balancer configuration
- Discovers upstream Route 53 alias records by:
  - Listing all hosted zones via `ListHostedZones`
  - Searching each zone for alias records via `ListResourceRecordSets`
  - Matching alias target DNS names to load balancer DNS names

**Permission Requirements:**
- `elasticloadbalancing:DescribeLoadBalancers`
- `elasticloadbalancing:DescribeListeners`
- `elasticloadbalancing:DescribeRules`
- `elasticloadbalancing:DescribeTargetGroups`
- `elasticloadbalancing:DescribeTargetHealth`
- `route53:ListHostedZones`
- `route53:ListResourceRecordSets`

**ECS Service Discovery:**
- Resolves services by ARN or cluster/service name via `DescribeServices`
- Discovers task definitions via `DescribeTaskDefinition`
- Extracts container definitions with CPU/memory allocation
- Discovers IAM roles (task role and execution role) from task definition
- Discovers security groups and subnets from awsvpc network configuration
- Links to target groups (bidirectional discovery with ALB)
- Discovers Application Auto Scaling policies via:
  - `DescribeScalableTargets` to find auto-scaling configuration
  - `DescribeScalingPolicies` to get scaling policies (target tracking, step scaling)
- Discovers cluster membership

**Permission Requirements:**
- `ecs:DescribeServices`
- `ecs:DescribeTaskDefinition`
- `application-autoscaling:DescribeScalableTargets`
- `application-autoscaling:DescribeScalingPolicies`

**Lambda Function Discovery:**
- Resolves functions by name or ARN via `GetFunction`
- Discovers IAM execution role from function configuration
- Discovers VPC configuration (security groups and subnets) if configured
- Discovers dead letter queue (DLQ) from function configuration
- Discovers event source mappings via `ListEventSourceMappings` (with pagination):
  - Identifies source type from ARN (SQS, DynamoDB, Kinesis, Kafka)
  - Tracks mapping state and batch size
  - Discovers OnFailure destinations for event source mappings
- Discovers function event invoke config destinations via `GetFunctionEventInvokeConfig`:
  - OnSuccess destinations (SNS, SQS, Lambda, EventBridge)
  - OnFailure destinations (SNS, SQS, Lambda, EventBridge)
- Extracts function metadata: runtime, handler, memory, timeout, code size, layers

**Permission Requirements:**
- `lambda:GetFunction`
- `lambda:ListEventSourceMappings`
- `lambda:GetFunctionEventInvokeConfig`

**RDS Instance/Cluster Discovery:**
- Resolves instances by identifier or ARN via `DescribeDBInstances`
- Resolves clusters by identifier or ARN via `DescribeDBClusters`
- Discovers subnet groups and individual subnets from DB configuration
- Discovers security groups with status from VPC configuration
- Discovers parameter groups (instance and cluster-level) with apply status
- Discovers cluster membership:
  - For instances: identifies parent cluster if instance is part of Aurora cluster
  - For clusters: lists all member instances with writer/reader role
- Extracts complete metadata: engine, version, storage, multi-AZ, endpoints (including reader endpoint for clusters)
- Heuristic-based upstream discovery (experimental):
  - When `--heuristics rds-endpoint` flag is enabled
  - Attempts to find Lambda functions and ECS services that connect to RDS endpoint
  - Marks connections with `Heuristic: true` in evidence

**Permission Requirements:**
- `rds:DescribeDBInstances`
- `rds:DescribeDBClusters`

Missing permissions will be logged as warnings and discovery will continue with available data.

## Examples

### Real-World Scenarios

#### 1. Understanding Load Balancer Impact

Before making changes to a load balancer, understand its full dependency graph:

```bash
# Analyze by ALB name
blast-radius my-production-alb

# Analyze by ARN with increased depth
blast-radius arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123 --depth 3

# Output as visualization
blast-radius my-production-alb --format dot | dot -Tpng -o alb-dependencies.png
```

**Example output:**
```
[Level 0] Root
└─ LoadBalancer: my-production-alb (arn:aws:elasticloadbalancing:...)
   Tags: Environment=production, Team=platform

[Level 1] Direct Dependencies
├─ Listener: HTTPS:443 (arn:aws:elasticloadbalancing:...)
│  DefaultAction: forward
├─ Listener: HTTP:80 (arn:aws:elasticloadbalancing:...)
│  DefaultAction: redirect to HTTPS
├─ SecurityGroup: alb-sg (sg-abc123)
└─ Route53Record: api.example.com (A ALIAS)

[Level 2] Transitive Dependencies
├─ TargetGroup: api-tg-blue
│  ├─ ECSService: prod-cluster/api-service
│  ├─ Instance: i-0a1b2c3d (healthy, t3.large)
│  └─ Instance: i-4e5f6g7h (healthy, t3.large)
└─ TargetGroup: api-tg-green
   └─ ECSService: prod-cluster/api-service-canary
```

#### 2. ECS Service Dependency Analysis

Understanding what an ECS service depends on before scaling or updating:

```bash
# Analyze ECS service
blast-radius prod-cluster/api-service

# Use specific region and profile
blast-radius prod-cluster/api-service --region us-west-2 --profile production
```

**Example output:**
```
[Level 0] Root
└─ ECSService: prod-cluster/api-service
   DesiredCount: 4, RunningCount: 4

[Level 1] Direct Dependencies
├─ ECSTaskDefinition: api-service:42
│  CPU: 512, Memory: 1024
│  Containers: api-app, datadog-agent
├─ IAMRole: ecs-task-execution-role
├─ IAMRole: api-service-task-role
├─ TargetGroup: api-tg-blue (arn:aws:elasticloadbalancing:...)
├─ SecurityGroup: ecs-service-sg (sg-xyz789)
├─ Subnet: subnet-1a2b3c4d (us-east-1a)
├─ Subnet: subnet-5e6f7g8h (us-east-1b)
└─ ScalingPolicy: target-tracking-cpu

[Level 2] Transitive Dependencies
└─ LoadBalancer: my-production-alb
   └─ Route53Record: api.example.com
```

#### 3. Lambda Function Analysis

Discover what triggers a Lambda function and what it connects to:

```bash
# Analyze Lambda function
blast-radius my-data-processor

# Find all dependencies including event sources
blast-radius arn:aws:lambda:us-east-1:123456789012:function:my-data-processor --depth 2
```

**Example output:**
```
[Level 0] Root
└─ Lambda: my-data-processor
   Runtime: python3.11, Memory: 512MB, Timeout: 30s

[Level 1] Direct Dependencies
├─ IAMRole: lambda-execution-role
├─ SQSQueue: data-ingestion-queue
│  Trigger: EventSourceMapping (BatchSize: 10)
├─ EventDestination: dead-letter-queue (OnFailure)
├─ SecurityGroup: lambda-sg (sg-def456)
└─ Subnet: subnet-9i0j1k2l (us-east-1a)

[Level 2] Transitive Dependencies
└─ VPC: vpc-main
```

#### 4. RDS Database Impact Assessment

Before migrating or modifying a database, see what depends on it:

```bash
# Analyze RDS instance
blast-radius my-production-db

# Analyze Aurora cluster
blast-radius my-aurora-cluster --heuristics rds-endpoint
```

**Example output:**
```
[Level 0] Root
└─ RDSInstance: my-production-db
   Engine: postgres 14.7, Class: db.r5.xlarge
   MultiAZ: true, Endpoint: my-production-db.abc.us-east-1.rds.amazonaws.com

[Level 1] Direct Dependencies
├─ DBSubnetGroup: prod-db-subnet-group
│  ├─ Subnet: subnet-db-1a (us-east-1a)
│  ├─ Subnet: subnet-db-1b (us-east-1b)
│  └─ Subnet: subnet-db-1c (us-east-1c)
├─ SecurityGroup: rds-sg (sg-rds123)
│  Status: active
├─ DBParameterGroup: postgres14-custom
│  Status: in-sync
└─ RDSCluster: my-aurora-cluster
   (if instance is part of a cluster)
```

#### 5. Analyzing Aurora Clusters

```bash
# Discover all instances in a cluster
blast-radius my-aurora-cluster --depth 2
```

**Example output:**
```
[Level 0] Root
└─ RDSCluster: my-aurora-cluster
   Engine: aurora-postgresql 14.7
   Endpoint: cluster-endpoint.us-east-1.rds.amazonaws.com
   ReaderEndpoint: cluster-ro-endpoint.us-east-1.rds.amazonaws.com

[Level 1] Direct Dependencies
├─ RDSInstance: my-aurora-cluster-instance-1
│  IsClusterWriter: true, Class: db.r5.large
├─ RDSInstance: my-aurora-cluster-instance-2
│  IsClusterWriter: false, Class: db.r5.large
├─ RDSInstance: my-aurora-cluster-instance-3
│  IsClusterWriter: false, Class: db.r5.large
├─ DBSubnetGroup: aurora-subnet-group
├─ SecurityGroup: aurora-sg (sg-aurora123)
└─ DBClusterParameterGroup: aurora-pg14-custom
```

### Output Formats

#### Tree (Default) - Human-Friendly

```bash
blast-radius my-resource
```

Best for: Quick analysis, terminal output, understanding dependency hierarchy

#### DOT - Graphviz Visualization

```bash
# Generate PNG
blast-radius my-alb --format dot | dot -Tpng -o graph.png

# Generate SVG (better for large graphs)
blast-radius my-alb --format dot | dot -Tsvg -o graph.svg

# Generate PDF
blast-radius my-alb --format dot | dot -Tpdf -o graph.pdf
```

Best for: Documentation, presentations, visual analysis

#### JSON - Machine-Readable

```bash
# Full graph as JSON
blast-radius my-resource --format json

# Filter specific resource types
blast-radius my-alb --format json | jq '.nodes[] | select(.type == "ECSService")'

# Extract all security groups
blast-radius my-resource --format json | jq '.nodes[] | select(.type == "SecurityGroup") | .name'

# Find all resources in a specific region
blast-radius my-resource --format json | jq '.nodes[] | select(.region == "us-east-1")'

# Count dependencies by type
blast-radius my-resource --format json | jq '.nodes | group_by(.type) | map({type: .[0].type, count: length})'
```

Best for: Automation, CI/CD integration, custom processing

### Common Workflows

#### Pre-Deployment Safety Check

```bash
#!/bin/bash
# Check if critical resources will be affected by changes

RESOURCE=$1
MAX_NODES=100

echo "Analyzing blast radius for: $RESOURCE"
blast-radius "$RESOURCE" --max-nodes $MAX_NODES --format json > blast-radius.json

# Check if any production resources are affected
PROD_COUNT=$(jq '.nodes[] | select(.tags.Environment == "production") | .id' blast-radius.json | wc -l)

if [ "$PROD_COUNT" -gt 0 ]; then
    echo "WARNING: $PROD_COUNT production resources will be affected!"
    echo "Manual approval required."
    exit 1
fi
```

#### Document Infrastructure Dependencies

```bash
#!/bin/bash
# Generate documentation for all critical services

SERVICES=("my-alb" "api-service" "data-processor" "production-db")

for service in "${SERVICES[@]}"; do
    echo "Documenting: $service"
    blast-radius "$service" --format dot | dot -Tpng -o "docs/${service}-dependencies.png"
    blast-radius "$service" > "docs/${service}-dependencies.txt"
done
```

#### Multi-Region Analysis

```bash
#!/bin/bash
# Analyze the same resource across multiple regions

RESOURCE="my-service"
REGIONS=("us-east-1" "us-west-2" "eu-west-1")

for region in "${REGIONS[@]}"; do
    echo "=== $region ==="
    blast-radius "$RESOURCE" --region "$region" --profile prod
    echo ""
done
```

## Development

### Prerequisites

- Go 1.22+
- AWS credentials configured

### Build

```bash
go build -o blast-radius
```

### Test

```bash
go test ./...
```

### Lint

```bash
golangci-lint run
```

### Test Coverage

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### CI/CD

The project uses GitHub Actions for:
- **PR Checks**: Automated testing, linting, and build verification on every pull request
- **Releases**: Automated binary builds for multiple platforms and Homebrew tap updates on version tags

To create a release:
```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

## Contributing

Contributions welcome! Please open an issue or submit a pull request.

## License

Apache-2.0. See [LICENSE](LICENSE) for details.

## Acknowledgments

Built with:
- [AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)
- [Cobra](https://github.com/spf13/cobra)
- [Viper](https://github.com/spf13/viper)
