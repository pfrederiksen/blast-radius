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

### RDS Instances/Clusters 🚧
**Status: Planned (Checkpoint 6)**
- Subnet groups and VPC
- Security groups
- Parameter groups
- Upstream applications (heuristic-based with `--heuristics rds-endpoint`)

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

Missing permissions will be logged as warnings and discovery will continue with available data.

## Examples

### Tree Output (Default)

```
$ blast-radius arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123

[Level 0] Root
└─ LoadBalancer: my-alb (arn:aws:elasticloadbalancing:...)

[Level 1] Direct Dependencies
├─ Listener: HTTPS:443 (arn:aws:elasticloadbalancing:...)
│  └─ [Forward] → TargetGroup: my-tg
├─ SecurityGroup: alb-sg (sg-12345)
└─ Route53Record: myapp.example.com (A ALIAS)

[Level 2] Transitive Dependencies
└─ TargetGroup: my-tg
   ├─ ECSService: my-cluster/my-service
   └─ Instance: i-1234567890 (t3.large)
```

### DOT Output

```bash
blast-radius my-alb --format dot | dot -Tpng -o graph.png
```

### JSON Output

```bash
blast-radius my-alb --format json | jq '.nodes[] | select(.type == "ECSService")'
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
