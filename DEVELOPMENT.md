# Development Guide for cc-relay

This guide covers the development workflow, tools, and best practices for contributing to cc-relay.

## Quick Start

```bash
# 1. Install all development tools
./scripts/setup-tools.sh

# 2. Verify installation
task doctor

# 3. Start developing with live reload
task dev
```

## Development Tools

### Tool Management (mise)

We use [mise](https://mise.jdx.dev/) for tool version management. All tool versions are defined in `.mise.toml`.

```bash
# Install all tools from .mise.toml
mise install

# Update all tools
mise upgrade

# Check tool versions
mise list
```

### Task Runner

We use [go-task](https://taskfile.dev) instead of Make for all development tasks.

```bash
# See all available tasks
task --list

# Common development tasks
task dev              # Start with live reload
task build            # Build binary
task test             # Run tests
task test-coverage    # Run tests with coverage
task ci               # Run all CI checks locally
task lint             # Run linters
task fmt              # Format code
task pre-commit       # Quick pre-commit checks
```

## Code Quality Tools

### Go Formatters

Three formatters run automatically on commit:

1. **gofmt** - Standard Go formatter
2. **goimports** - Manages imports and formats
3. **gofumpt** - Stricter formatting (superset of gofmt)

```bash
# Manual formatting
task fmt

# Or individually
gofmt -w .
goimports -w .
gofumpt -w .
```

### Go Linters

**golangci-lint** runs 40+ linters with comprehensive rules (see `.golangci.yml`).

```bash
# Run linters
task lint

# Run with auto-fix
task lint-fix

# Run manually
golangci-lint run --config .golangci.yml
golangci-lint run --fix
```

Key linters enabled:

- **errcheck** - Unchecked errors
- **gosec** - Security issues
- **govet** - Suspicious constructs
- **staticcheck** - Static analysis
- **revive** - Code style
- **unused** - Unused code detection
- **gocyclo** - Cyclomatic complexity
- **goconst** - Repeated strings
- And 30+ more...

### Security Scanning

```bash
# Run security checks
task security

# Or individually
govulncheck ./...    # Vulnerability scanning
gosec ./...          # Security-focused linting
```

### YAML Tools

```bash
# Format YAML
task yaml-fmt
yamlfmt -w .

# Lint YAML
task yaml-lint
yamllint .
```

Configuration:

- `.yamlfmt` - YAML formatter config
- `.yamllint` - YAML linter rules

### Markdown Linting

```bash
# Lint and fix Markdown
task markdown-lint
markdownlint --fix '**/*.md'
```

Configuration: `.markdownlint.json`

### Protobuf/gRPC

```bash
# Generate code from .proto files
task proto
buf generate

# Lint proto files
task proto-lint
buf lint

# Format proto files
task proto-format
buf format -w
```

## Git Hooks (Lefthook)

Git hooks run automatically on commit/push via [lefthook](https://github.com/evilmartians/lefthook).

### Pre-commit Hooks (runs on every commit)

- ✓ Format Go code (gofmt, goimports, gofumpt)
- ✓ Lint Go code (golangci-lint with auto-fix)
- ✓ Static analysis (go vet)
- ✓ Format YAML files (yamlfmt)
- ✓ Lint YAML files (yamllint)
- ✓ Lint Markdown (markdownlint with auto-fix)
- ✓ Lint proto files (buf)
- ✓ Run quick tests (go test -short)

### Pre-push Hooks (runs before git push)

- ✓ Full test suite with coverage
- ✓ Dependency check (go mod tidy)
- ✓ Security scanning (govulncheck)
- ✓ Build verification

### Commit Message Validation

All commits must follow [Conventional Commits](https://www.conventionalcommits.org/) format:

```
type(scope?): subject

Valid types: feat, fix, docs, style, refactor, perf, test, chore, ci, build
```

Examples:

```bash
git commit -m "feat(proxy): add rate limiting support"
git commit -m "fix: correct SSE streaming bug"
git commit -m "docs: update installation guide"
git commit -m "test(router): add failover test cases"
```

### Manual Hook Execution

```bash
# Run pre-commit hooks manually
task hooks-run
lefthook run pre-commit

# Install/reinstall hooks
task hooks-install
lefthook install

# Uninstall hooks
task hooks-uninstall
lefthook uninstall
```

Configuration: `lefthook.yml`

## Live Reload Development

Use [Air](https://github.com/air-verse/air) for automatic recompilation during development:

```bash
# Start with live reload (recommended)
task dev

# Or directly
air
```

Air watches for changes in:

- `*.go` files
- `*.proto` files (after regeneration)
- Templates and HTML files

Configuration: `.air.toml`

## Testing

```bash
# Run all tests
task test
go test ./...

# Quick feedback (for pre-commit)
task test-short
go test -short ./...

# With coverage
task test-coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Integration tests
task test-integration
go test -tags=integration ./...

# Benchmarks
task bench
go test -bench=. -benchmem ./...
```

## Building

```bash
# Build for current platform
task build

# Build for all platforms
task build-all

# Individual platforms
task build-linux
task build-darwin
task build-windows

# Manual build
go build -o bin/cc-relay ./cmd/cc-relay

# Cross-compilation
GOOS=linux GOARCH=amd64 go build -o cc-relay ./cmd/cc-relay
```

## CI/CD

### Local CI Simulation

Run the same checks that CI runs:

```bash
task ci
```

This runs:

1. Code formatting
2. Linting
3. Static analysis
4. Full test suite with coverage
5. Build verification
6. Security scanning

### GitHub Actions

Two workflows are configured:

**`.github/workflows/ci.yml`** - Runs on PRs and pushes:

- Linting (golangci-lint)
- Testing with coverage
- Security scanning (govulncheck, gosec)
- YAML linting
- Markdown linting
- Proto linting
- Multi-platform builds

**`.github/workflows/merge.yml`** - Runs after merge to main:

- Full test suite
- Dependency audit
- Build release artifacts
- Success notifications

## Dependency Management

```bash
# Download dependencies
task deps
go mod download

# Update dependencies
task deps-update
go get -u ./...
go mod tidy

# Tidy and verify
task deps-tidy
go mod tidy
go mod verify
```

## Project Structure

```
cc-relay/
├── .air.toml              # Air live reload config
├── .golangci.yml          # golangci-lint configuration
├── .markdownlint.json     # Markdown linting rules
├── .mise.toml             # Tool version management
├── .yamlfmt               # YAML formatter config
├── .yamllint              # YAML linting rules
├── lefthook.yml           # Git hooks configuration
├── Taskfile.yml           # Task runner definitions
├── scripts/
│   └── setup-tools.sh     # Development tools installer
└── .github/
    └── workflows/
        ├── ci.yml         # PR/push CI checks
        └── merge.yml      # Post-merge workflows
```

## Troubleshooting

### Tools Not Found

Ensure `$GOPATH/bin` is in your PATH:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Add to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
export GOPATH="$HOME/go"
export PATH="$PATH:$GOPATH/bin"
```

### Hooks Not Running

Reinstall hooks:

```bash
lefthook install
```

### Hook Failures

Run hooks manually to see detailed output:

```bash
lefthook run pre-commit
```

Skip hooks temporarily (not recommended):

```bash
git commit --no-verify
```

### Slow Commits

If commits are too slow, you can:

1. Disable specific hooks in `lefthook.yml`
2. Use `task pre-commit` before committing to catch issues early
3. Skip slow checks on commit and run them on push instead

## Best Practices

1. **Run `task ci` before creating a PR** - Catches issues locally
2. **Use `task dev` for development** - Instant feedback on changes
3. **Commit frequently** - Hooks catch issues early
4. **Write meaningful commit messages** - Follow Conventional Commits
5. **Run `task test-coverage` regularly** - Maintain test coverage
6. **Keep dependencies updated** - Run `task deps-update` periodically

## Additional Resources

- [go-task documentation](https://taskfile.dev)
- [lefthook documentation](https://github.com/evilmartians/lefthook)
- [golangci-lint linters](https://golangci-lint.run/usage/linters/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Air documentation](https://github.com/air-verse/air)
- [buf documentation](https://buf.build/docs)
