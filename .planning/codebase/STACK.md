# Technology Stack

**Analysis Date:** 2026-01-20

## Languages

**Primary:**
- Go 1.24.7 - Core implementation language for HTTP proxy, CLI, daemon, and management API

**Secondary:**
- Protocol Buffers (protobuf3) - gRPC service definitions in `relay.proto`

## Runtime

**Environment:**
- Go 1.24.7 runtime
- Supports cross-platform compilation via `GOOS` and `GOARCH` (Linux, Windows, macOS)

**Package Manager:**
- Go Modules (go.mod)
- Lockfile: Present (go.mod only, no go.sum in repository - dependencies not yet vendored)

## Frameworks

**Core:**
- Standard Go library (`net/http`, `net/http/httputil`) - HTTP server and reverse proxy implementation
- `google.golang.org/grpc` (planned) - gRPC management API server

**TUI:**
- `github.com/charmbracelet/bubbletea` (planned) - Terminal user interface framework using Elm architecture

**Configuration & Watching:**
- `github.com/fsnotify/fsnotify` (planned) - File watcher for hot-reload of config files
- `gopkg.in/yaml.v3` (planned) - YAML configuration parsing

**Build & Code Generation:**
- `protoc` (protocol buffers compiler) - Generates Go code from `relay.proto`
- `buf` (optional) - Alternative protobuf toolchain for code generation

## Key Dependencies

**Infrastructure (Planned):**
- AWS SDK (`github.com/aws/aws-sdk-go`) - AWS SigV4 signing for Bedrock provider
- Google Cloud Client Library - Google OAuth token generation for Vertex AI
- Prometheus client (`github.com/prometheus/client_golang`) - Metrics export

**Standard Library (Critical):**
- `net/http` - HTTP server and client functionality
- `net/http/httputil` - Reverse proxy utilities
- `context` - Request cancellation and timeouts
- `encoding/json` - JSON marshaling/unmarshaling
- `log/slog` or similar - Structured logging

**Configuration:**
- YAML v3 (`gopkg.in/yaml.v3`) - Primary config format parsing
- TOML (optional) - Alternative config format

## Configuration

**Environment:**
- Environment variable expansion in config: `${VAR_NAME}` syntax
- Config location: `~/.config/cc-relay/config.yaml` (default)
- Supports YAML and TOML formats
- Example config: `example.yaml` (216 lines with comprehensive documentation)

**Build:**
- Standard `go build` - No special build tools required
- Protobuf compilation: `protoc` with Go plugins
- Example commands:
  ```bash
  go build -o cc-relay ./cmd/cc-relay
  GOOS=linux GOARCH=amd64 go build -o cc-relay ./cmd/cc-relay
  ```

**Configuration Sections:**
- `server` - Listen address, timeout settings, max concurrent requests
- `routing` - Strategy selection, failover chain configuration
- `providers` - Array of provider configs with API keys, rate limits, model mappings
- `grpc` - Management API listen addresses (standard and gRPC-web)
- `logging` - Level (debug/info/warn/error), format (json/text), file output
- `metrics` - Prometheus metrics endpoint configuration
- `health` - Circuit breaker thresholds, check intervals

## Platform Requirements

**Development:**
- Go 1.24.7 or later
- `protoc` compiler (for proto code generation)
- Standard C build tools (for cgo dependencies in some providers, optional)
- POSIX shell (for build scripts)

**Production:**
- Linux, macOS, or Windows systems
- Local filesystem for configuration storage (`~/.config/cc-relay/`)
- Network connectivity to:
  - Claude Code client (default: `127.0.0.1:8787`)
  - Backend LLM providers (Anthropic, Z.AI, Ollama, AWS Bedrock, Azure, Vertex AI)
  - gRPC management API port (default: `127.0.0.1:9090`)
  - Prometheus metrics endpoint (default: `127.0.0.1:9100`)
- Optional: Google Cloud credentials for Vertex AI provider
- Optional: AWS credentials for Bedrock provider

## Testing Infrastructure

**Framework (Planned):**
- Go built-in `testing` package
- No external test framework specified in current spec

**Test Coverage:**
- Unit tests for provider transformers, routing strategies, key pool logic
- Integration tests for proxy flow, SSE streaming, multi-provider failover
- Test commands:
  ```bash
  go test ./...
  go test -v ./internal/proxy
  go test -race ./...
  go test -cover ./...
  go test -bench=. ./internal/router
  ```

## Entry Points

**CLI Application:**
- `cmd/cc-relay/main.go` - Command-line interface with subcommands

**HTTP Server:**
- `internal/proxy/server.go` - Serves `POST /v1/messages` endpoint (Anthropic API compatible)

**gRPC Server:**
- `internal/grpc/server.go` - Management API for TUI/CLI/WebUI communication

**TUI Application:**
- `ui/tui/app.go` - Bubble Tea based terminal interface

---

*Stack analysis: 2026-01-20*
