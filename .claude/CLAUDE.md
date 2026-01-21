# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 📚 Development Skills & Tools

**⭐ START HERE**: [INDEX.md](INDEX.md) - Complete guide to all skills, commands, and tools

**Critical Skills:**
- [Fix Lint Errors](skills/fix-lint-errors.md) - Fix code when linters fail (NOT config)
- [Fix Test Failures](skills/fix-test-failures.md) - Fix code/tests when tests fail (NOT test config)
- [Auto-Fix Code](skills/auto-fix-code.md) - Use auto-fix tools efficiently

**Quick Commands:**
- `task fmt` - Auto-format all code
- `task lint-fix` - Auto-fix lint issues
- `task ci` - Run all CI checks locally
- See [commands/README.md](commands/README.md) for all commands

## Project Overview

cc-relay is a multi-provider proxy for Claude Code written in Go. It sits between Claude Code and multiple LLM providers (Anthropic, Z.AI, Ollama, AWS Bedrock, Azure Foundry, Vertex AI), enabling rate limit pooling, cost optimization, automatic failover, and flexible provider routing.

## Development Workflow

### Quick Setup

```bash
# First time setup - installs all tools and git hooks
./scripts/setup-tools.sh

# Or using task
task setup
```

### Task Runner (Recommended)

We use [go-task](https://taskfile.dev) for all development tasks. It's faster and more ergonomic than make.

```bash
# See all available tasks
task --list

# Common tasks
task dev          # Run with live reload (uses air)
task build        # Build binary
task test         # Run all tests
task test-short   # Quick test feedback
task ci           # Run all CI checks locally
task lint         # Run linters
task fmt          # Format all code
task pre-commit   # Quick pre-commit checks
```

### Live Reload Development

We use [Air](https://github.com/air-verse/air) for automatic recompilation and restart during development.

```bash
# Start development server with live reload
task dev
# or directly
air

# Air watches for changes in:
# - *.go files
# - *.proto files (if you regenerate)
# - Templates and HTML
```

Configuration in `.air.toml`. Builds to `./tmp/main` and restarts on changes.

### Git Hooks (Lefthook)

Git hooks are automatically installed via `lefthook` and run on every commit. They ensure code quality before changes enter version control.

**Pre-commit hooks:**
- `gofmt`, `goimports`, `gofumpt` - Format Go code
- `golangci-lint` - Comprehensive linting with auto-fix
- `go vet` - Static analysis
- `yamlfmt`, `yamllint` - YAML formatting and linting
- `markdownlint` - Markdown linting with auto-fix
- `buf lint` - Proto file linting
- `go test -short` - Quick test run

**Pre-push hooks:**
- Full test suite with coverage
- `go mod tidy` check
- `govulncheck` - Security vulnerability scanning
- Build verification

**Commit message validation:**
- Enforces [Conventional Commits](https://www.conventionalcommits.org/) format
- Valid types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `build`
- Example: `feat(proxy): add rate limiting support`

### Manual Hook Execution

```bash
# Run pre-commit hooks manually
task hooks-run

# Install hooks
task hooks-install

# Uninstall hooks
task hooks-uninstall
```

### Building

```bash
# Build for current platform
task build

# Build for all platforms
task build-all

# Or manually
go build -o cc-relay ./cmd/cc-relay

# Cross-compilation
GOOS=linux GOARCH=amd64 go build -o cc-relay ./cmd/cc-relay
GOOS=windows GOARCH=amd64 go build -o cc-relay.exe ./cmd/cc-relay
GOOS=darwin GOARCH=arm64 go build -o cc-relay ./cmd/cc-relay
```

### Testing

```bash
# Using task (recommended)
task test              # Run all tests
task test-short        # Quick test run (for pre-commit)
task test-coverage     # Generate coverage report (HTML)
task test-integration  # Run integration tests
task bench             # Run benchmarks

# Manual testing
go test ./...
go test -v ./...
go test -v ./internal/proxy -run TestProxyHandler
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Code Quality

```bash
# Run all quality checks (mimics CI)
task ci

# Individual checks
task fmt          # Auto-format all code
task lint         # Run all linters
task lint-fix     # Run linters with auto-fix
task vet          # Run go vet
task security     # Security scanning (govulncheck + gosec)
task check        # Run fmt + lint + vet + test

# Before committing (runs format + lint + quick tests)
task pre-commit
```

### Protobuf/gRPC

```bash
# Using task
task proto         # Generate code from .proto files
task proto-lint    # Lint proto files
task proto-format  # Format proto files

# Manual
buf generate
buf lint
buf format -w
```

### YAML and Markdown

```bash
# YAML
task yaml-fmt      # Format YAML files
task yaml-lint     # Lint YAML files

# Markdown
task markdown-lint # Lint and fix Markdown files
```

### Running

```bash
# Development with live reload (recommended)
task dev

# Production build and run
task build
./bin/cc-relay serve

# Check server status
./bin/cc-relay status

# Validate configuration
./bin/cc-relay config validate

# Show version
./bin/cc-relay version

# Get help
./bin/cc-relay --help
./bin/cc-relay serve --help

# Manual with custom config
./bin/cc-relay serve --config /path/to/config.yaml
```

### Dependency Management

```bash
task deps         # Download dependencies
task deps-update  # Update all dependencies
task deps-tidy    # Tidy go.mod and verify
```

### Installed Development Tools

The setup script installs these tools (via `scripts/setup-tools.sh`):

**Go Formatters:**
- `gofmt` - Standard Go formatter
- `goimports` - Manages imports and formats
- `gofumpt` - Stricter formatting (superset of gofmt)

**Go Linters:**
- `golangci-lint` - Meta-linter running 40+ linters
- `go vet` - Official Go static analyzer

**Security:**
- `govulncheck` - Official Go vulnerability scanner
- `gosec` - Security-focused linter

**Proto/gRPC:**
- `buf` - Modern protobuf tool (lint, generate, format)

**YAML:**
- `yamlfmt` - YAML formatter
- `yamllint` - YAML linter

**Markdown:**
- `markdownlint-cli` - Markdown linter

**Development:**
- `air` - Live reload for Go apps
- `task` - Task runner (modern make alternative)
- `lefthook` - Fast Git hooks manager

All tools are installed to `$GOPATH/bin` (ensure it's in your `$PATH`).

## Architecture

### Core Components

1. **HTTP Proxy Server** (`internal/proxy/`)
   - Implements `/v1/messages` endpoint matching Anthropic API exactly
   - Handles SSE streaming with correct event sequence
   - Must preserve `tool_use_id` for Claude Code's parallel tool calls
   - Middleware for logging, metrics, auth validation

2. **Router** (`internal/router/`)
   - Implements routing strategies (shuffle, round-robin, failover, cost-based, latency-based, model-based)
   - Manages API key pools with rate limit tracking (RPM/TPM)
   - Selects backend provider + key for each request

3. **Providers** (`internal/providers/`)
   - Provider interface with `TransformRequest`, `TransformResponse`, `Authenticate`, `HealthCheck`
   - Each provider implementation handles API-specific transformations
   - Direct implementations: `anthropic.go`, `zai.go`, `ollama.go`, `bedrock.go`, `azure.go`, `vertex.go`

4. **Health Tracking** (`internal/health/`)
   - Circuit breaker with CLOSED/OPEN/HALF-OPEN states
   - Tracks failures (429s, 5xx, timeouts) per provider
   - Automatic recovery probing after cooldown

5. **Configuration** (`internal/config/`)
   - YAML/TOML config loading with environment variable expansion
   - Hot-reload via fsnotify file watcher
   - Validates provider configs, routing settings

6. **gRPC Management API** (`internal/grpc/`)
   - Service defined in `proto/relay.proto`
   - Exposes stats streaming, provider/key management, config updates
   - Used by TUI, CLI, optional WebUI

7. **TUI** (`ui/tui/`)
   - Built with Bubble Tea (Elm architecture)
   - Connects to daemon via gRPC
   - Real-time stats, provider health, request logs

### Critical API Compatibility Details

**Claude Code expects exact Anthropic API format:**
- Endpoint: `POST /v1/messages`
- Headers: `x-api-key`, `anthropic-version`, `content-type: application/json`
- Body: Standard Anthropic Messages API format
- Streaming: SSE events in exact order (message_start → content_block_start → content_block_delta → content_block_stop → message_delta → message_stop)
- Tool use: Must preserve `tool_use_id` and handle multiple tool blocks atomically
- Extended thinking: Support `thinking` content blocks

**Provider-Specific Transformations:**

| Provider | Key Transformation |
|----------|-------------------|
| **Bedrock** | Model in URL path (not body), `anthropic_version: "bedrock-2023-05-31"`, AWS SigV4 signing |
| **Vertex AI** | Model in URL path, `anthropic_version: "vertex-2023-10-16"`, Google OAuth bearer token |
| **Azure** | Use `x-api-key` header (not `api-key`), deployment names as model IDs |
| **Ollama** | No prompt caching, no PDF support, images must be base64 (no URLs), `budget_tokens` accepted but not enforced |
| **Z.AI** | Fully Anthropic-compatible, use model mapping for GLM models |

### SSE Streaming Requirements

```
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
X-Accel-Buffering: no
Connection: keep-alive
```

Event sequence must match Anthropic's exactly. Use `http.Flusher` interface to flush each SSE event immediately.

## Configuration

- Default location: `~/.config/cc-relay/config.yaml`
- Format: YAML (primary) or TOML
- Environment variables: Use `${VAR_NAME}` syntax
- See `config/example.yaml` for full reference

### Key Config Sections

- `server`: Listen address, timeout, max concurrent requests
- `routing`: Strategy selection, failover chain, cost thresholds
- `providers`: Array of provider configs with keys, rate limits, model mappings
- `grpc`: Management API listen addresses
- `logging`: Level, format (json/text), file output
- `metrics`: Prometheus endpoint configuration
- `health`: Check intervals, circuit breaker thresholds

## Testing with Claude Code

```bash
# Terminal 1: Start cc-relay
./cc-relay serve --config config/example.yaml

# Terminal 2: Point Claude Code to proxy
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_API_KEY="managed-by-cc-relay"  # Any value works
claude
```

The proxy will route requests based on configured strategy and use the actual API keys from config.

## Dependencies

- **Standard Library**: `net/http`, `net/http/httputil`, `encoding/json`, `context`
- **Bubble Tea**: TUI framework (`github.com/charmbracelet/bubbletea`)
- **gRPC**: `google.golang.org/grpc`
- **fsnotify**: File watcher for config hot-reload
- **AWS SDK** (if using Bedrock): SigV4 signing
- **Google Auth** (if using Vertex): OAuth token generation

## Development Phases

The project follows a phased development roadmap (see SPEC.md):

1. **Phase 1 (MVP)**: Basic proxy + Anthropic/Z.AI/Ollama + simple-shuffle
2. **Phase 2**: Multi-key pooling, rate limiting, failover routing
3. **Phase 3**: Cloud providers (Bedrock, Azure, Vertex)
4. **Phase 4**: gRPC API + TUI
5. **Phase 5**: Advanced routing (cost-based, latency-based, model-based)
6. **Phase 6**: WebUI via grpc-web

When implementing new features, follow the phase ordering to maintain logical progression.

## Code Style

- Follow standard Go conventions (`go fmt`, `golint`)
- Use structured logging (consider `log/slog` or `zerolog`)
- Provider implementations should be isolated in separate files
- Keep routing strategies as separate implementations of a common interface
- Use context for cancellation and timeout propagation
- Handle errors explicitly, avoid panic in library code
