# blast-radius

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

### From source

```bash
go install github.com/pfrederiksen/blast-radius@latest
```

### From binary

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

### Application/Network Load Balancers (ALB/NLB)
- Listeners and listener rules
- Target groups and registered targets
- Security groups and VPC/subnets
- Upstream Route 53 alias records
- Upstream CloudFront distributions (best-effort)

### ECS Services
- Task definitions and tasks
- Load balancers and target groups
- IAM roles (task role, execution role)
- Security groups and network configuration
- Auto Scaling policies (best-effort)

### Lambda Functions
- IAM execution role
- Event source mappings (SQS, DynamoDB, Kinesis)
- EventBridge rules
- API Gateway integrations (best-effort)
- Dead letter queues and destinations
- VPC configuration

### RDS Instances/Clusters
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

### Test Coverage

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
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
