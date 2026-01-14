# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive usage examples and real-world scenarios in README
- Edge case tests for ARN parsing and graph operations
- Multi-region analysis example workflows
- Pre-deployment safety check script examples
- Documentation for all three output formats (tree, DOT, JSON)

### Changed
- Improved README with practical operational scenarios
- Enhanced error messages for better debugging

## [0.1.0] - 2026-01-14

### Added

#### Core Features
- Local-first AWS dependency graph discovery
- No backend required - uses existing AWS credentials
- BFS traversal algorithm with configurable depth
- Evidence tracking for all discovered relationships
- Permission-resilient discovery (continues with warnings)

#### Resource Discovery

**ALB/NLB (Application/Network Load Balancers)**
- Listener and listener rule discovery with pagination
- Target group and registered target discovery (EC2, IP, Lambda)
- Security group and VPC/subnet mapping
- Route53 alias record upstream discovery
- Target health status tracking

**ECS Services**
- Task definition discovery with container details
- IAM role discovery (task role and execution role)
- Security group and subnet discovery from awsvpc network mode
- Load balancer target group integration (bidirectional)
- Application Auto Scaling policy discovery (target tracking, step scaling)
- Cluster membership tracking

**Lambda Functions**
- IAM execution role discovery
- VPC configuration (security groups, subnets)
- Dead letter queue (DLQ) configuration
- Event source mapping discovery with pagination (SQS, DynamoDB, Kinesis, Kafka)
- Event source mapping destination discovery (OnFailure)
- Function event invoke config destinations (OnSuccess, OnFailure)
- Complete function metadata (runtime, handler, memory, timeout, layers)

**RDS Instances and Aurora Clusters**
- DB instance and Aurora cluster resolution by identifier or ARN
- Subnet group discovery with individual subnet details
- Security group discovery with status tracking
- Parameter group discovery (instance and cluster-level)
- Cluster membership tracking (bidirectional for instances and clusters)
- Complete metadata (engine, version, storage, multi-AZ, endpoints)
- Experimental heuristic-based upstream discovery framework

#### Output Formats
- **Tree format** (default): Human-friendly hierarchical view organized by BFS depth levels
- **DOT format**: Graphviz-compatible output for visual diagrams
- **JSON format**: Machine-readable output for automation and custom processing

#### CLI Features
- `--depth` flag: Control traversal depth (default: 2)
- `--max-nodes` flag: Limit discovered nodes (default: 250)
- `--format` flag: Choose output format (tree, dot, json)
- `--profile` flag: Use specific AWS profile
- `--region` flag: Override AWS region
- `--heuristics` flag: Enable experimental heuristic discovery (e.g., `rds-endpoint`)
- `--debug` flag: Enable debug logging

#### Resource Resolution Methods
- **By ARN**: Full AWS ARN for any supported resource type
- **By name**: Friendly name for load balancers, Lambda functions, RDS instances/clusters
- **By cluster/service**: ECS services using `cluster-name/service-name` format

#### CI/CD and Quality
- GitHub Actions PR checks (test, lint, build)
- GoReleaser for multi-platform releases (Linux, macOS, Windows on amd64/arm64)
- Homebrew tap integration (`pfrederiksen/tap/blast-radius`)
- golangci-lint with comprehensive linter rules
- High test coverage for core packages (graph: 91.4%, output: 96.4%)
- Branch protection on main branch requiring passing checks

#### Documentation
- Comprehensive README with architecture details
- Installation via Homebrew, Go install, or binary download
- Usage examples for all resource types
- Permission requirements documented per service
- CLAUDE.md documenting AI-assisted development approach

### Technical Details
- Built with AWS SDK v2 for better performance
- Cobra CLI framework with Viper configuration management
- Structured logging with slog
- Thread-safe graph implementation with mutex protection
- Pagination handling for all AWS list operations
- Resource type constants for consistency

## [0.0.1] - 2026-01-10

### Added
- Initial project structure
- Basic Cobra CLI scaffold
- Repository setup on GitHub

[Unreleased]: https://github.com/pfrederiksen/blast-radius/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/pfrederiksen/blast-radius/releases/tag/v0.1.0
[0.0.1]: https://github.com/pfrederiksen/blast-radius/releases/tag/v0.0.1
