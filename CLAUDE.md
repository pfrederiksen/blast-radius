# Built with Claude Code

This project was built with assistance from [Claude Code](https://claude.com/claude-code), Anthropic's CLI tool for AI-assisted software development.

## Development Approach

The `blast-radius` CLI tool was developed incrementally using Claude Code (Sonnet 4.5) as a senior Go + AWS platform engineer. The development followed strict workflows:

### Workflow Principles

1. **Branch-based development**: All work done on feature branches, never committing directly to `main`
2. **Pull Request workflow**: Each checkpoint merged via PR for review and documentation
3. **Test-driven**: High test coverage maintained throughout with unit tests, table-driven tests, and golden tests
4. **Documentation-first**: README, command help, and architecture docs kept in sync with implementation

### Implementation Checkpoints

The project was built in structured phases:

1. **Checkpoint 1**: Repository scaffold + Cobra CLI + config loading + structured logging (slog) ✅
2. **Checkpoint 3**: ALB/NLB discovery + Route53 alias upstream discovery + tests + docs ✅
3. **Checkpoint 4**: ECS service discovery + tests + docs (Planned)
4. **Checkpoint 5**: Lambda function discovery + event sources + tests + docs (Planned)
5. **Checkpoint 6**: RDS discovery + heuristic-based upstream discovery + tests + docs (Planned)
6. **Checkpoint 7**: Polish (max-nodes limits, evidence annotations, error handling) + examples + coverage optimization (Planned)

### Key Technical Decisions

- **AWS SDK v2**: Modern SDK with better performance and smaller binary size
- **Cobra + Viper**: Industry-standard CLI framework with excellent config management
- **BFS traversal**: Ensures level-by-level discovery and natural depth limiting
- **Permission resilience**: Continues discovery even with partial AWS permissions, annotating unavailable data
- **Evidence tracking**: Each edge includes the API call and fields that established the relationship
- **Local-first**: No backend, no telemetry, respects user privacy and uses their existing AWS credentials

### Testing Strategy

- **Unit tests**: Core logic (ARN parsing, graph traversal, output formatting)
- **Table-driven tests**: Comprehensive coverage of edge cases
- **Golden tests**: Output format stability (tree and DOT rendering)
- **Coverage targets**: High coverage for internal packages, especially graph and discovery logic

## AI Assistance Details

Claude Code assisted with:
- Architecture design and incremental planning
- Go best practices and idiomatic code patterns
- AWS SDK v2 usage patterns and pagination handling
- Test coverage strategies
- Documentation and examples
- Git workflow and PR management

The AI followed a **local-first, permission-resilient, operator-friendly** design philosophy throughout, prioritizing:
- Operator experience (clear output, helpful errors)
- Robustness (handles missing permissions gracefully)
- Performance (pagination, configurable limits)
- Maintainability (clear structure, comprehensive tests)

## Reproducing the Build

To reproduce this project structure with Claude Code:

```bash
# Start Claude Code in a new directory
claude-code

# Provide the complete specification (see original prompt)
# Claude will:
# 1. Create GitHub repo with proper topics/license
# 2. Initialize git with branch workflow
# 3. Implement each checkpoint incrementally
# 4. Create PRs and merge to main
# 5. Maintain tests and docs throughout
```

## Version

- Built with: Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)
- Date: January 2026
- Repository: https://github.com/pfrederiksen/blast-radius

---

For questions about the AI-assisted development process, see [Claude Code documentation](https://claude.com/claude-code).
