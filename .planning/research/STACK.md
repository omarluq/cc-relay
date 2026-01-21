# Stack Research

**Domain:** Multi-provider LLM HTTP Proxy
**Researched:** 2026-01-20
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Go** | 1.23+ | Primary language | Excellent HTTP/2 support, native concurrency for SSE streaming, strong standard library (net/http/httputil.ReverseProxy), statically typed for API transformations |
| **net/http/httputil** | stdlib | HTTP reverse proxy | Built-in ReverseProxy with Rewrite function (modern pattern), handles hop-by-hop headers, connection pooling, X-Forwarded headers automatically |
| **log/slog** | stdlib (Go 1.21+) | Structured logging | Standard library solution (no deps), TextHandler for dev, JSONHandler for prod, integrates with context for request tracing |
| **gRPC** | v1.78.0 | Management API | Industry standard for service-to-service communication, supports streaming stats, bidirectional communication for TUI updates, requires Go 1.23+ |
| **Bubble Tea** | v1.3.10 (v2 available) | Terminal UI | Elm Architecture (functional, testable), battle-tested in production, excellent for real-time dashboards, active ecosystem (Bubbles, Lip Gloss) |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **fsnotify** | v1.8.0+ | Config file watching | Hot-reload for YAML/TOML config changes, cross-platform (Windows/Linux/macOS), 12,768+ packages depend on it |
| **spf13/viper** | Latest | Configuration management | Multi-format support (YAML/TOML/JSON), environment variable expansion, struct unmarshaling, de facto standard for Go config |
| **prometheus/client_golang** | v1.20+ | Metrics instrumentation | Standard for Go observability, Counter/Gauge/Histogram/Summary types, promhttp.Handler() for /metrics endpoint |
| **aws-sdk-go-v2** | v1.33.0+ | AWS Bedrock integration | Official AWS SDK v2 (v1 EOL), BedrockRuntime client for Claude, SigV4 signing built-in, requires Go 1.23+ |
| **google.golang.org/genai** | Latest | Vertex AI integration | NEW preferred SDK (as of June 2025), replaces deprecated cloud.google.com/go/vertexai/genai, OAuth bearer token support |
| **Azure/azure-sdk-for-go/sdk/ai/azopenai** | v0.8.0+ | Azure OpenAI integration | Works with official OpenAI Go client, supports Azure-specific features (On Your Data), v1 API support (Aug 2025+) |
| **ollama/ollama/api** | Latest | Ollama integration | Official Ollama client (CLI uses this), fully typed, respects OLLAMA_HOST env var, safest for compatibility |
| **r3labs/sse** or **tmaxmax/go-sse** | Latest | SSE streaming (if needed) | r3labs for simple client/server, tmaxmax for spec-compliant with LLM support (ChatGPT/Claude streams), jetify-com/sse for dependency-free |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| **buf** | Protobuf tooling | Modern alternative to protoc, handles linting/generation/formatting, `buf generate` for gRPC code |
| **golangci-lint** | Meta-linter | Runs 40+ linters, includes gosec for security, use with `--fix` for auto-fix |
| **govulncheck** | Vulnerability scanning | Official Go security scanner, checks dependencies against vulnerability database |
| **air** | Live reload | Auto-recompile on .go/.proto changes, faster development iteration, config in .air.toml |
| **task** (go-task) | Task runner | Modern alternative to make, faster, better ergonomics, YAML-based task definitions |

## Installation

```bash
# Core dependencies (Go modules)
go get google.golang.org/grpc@v1.78.0
go get github.com/charmbracelet/bubbletea@v1.3.10
go get github.com/fsnotify/fsnotify@latest
go get github.com/spf13/viper@latest
go get github.com/prometheus/client_golang@latest

# Provider SDKs
go get github.com/aws/aws-sdk-go-v2/service/bedrockruntime@latest
go get google.golang.org/genai@latest
go get github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai@latest
go get github.com/ollama/ollama/api@latest

# Development tools (install to $GOPATH/bin)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/bufbuild/buf/cmd/buf@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/air-verse/air@latest
go install github.com/go-task/task/v3/cmd/task@latest
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| **log/slog** (stdlib) | zerolog / zap | High-throughput scenarios (zerolog has zero allocations, zap edges out slog in benchmarks); use slog for standard library benefits and no deps |
| **net/http/httputil** (stdlib) | Custom reverse proxy | Only if you need fundamentally different behavior; stdlib ReverseProxy is production-grade and handles edge cases |
| **Bubble Tea** v1 | Bubble Tea v2 | v2 is available but ecosystem (Bubbles components) may not be fully migrated; use v1 for stability |
| **r3labs/sse** | tmaxmax/go-sse | Use tmaxmax for spec-compliant SSE with LLM-specific features; r3labs for simpler client/server; jetify-com/sse for zero deps |
| **viper** | knadh/koanf | Use koanf if you need S3/etcd backends or extremely lightweight config; viper is more feature-complete |
| **google.golang.org/genai** | cloud.google.com/go/vertexai | OLD SDK is deprecated (June 24, 2025), removed June 24, 2026; always use new genai package |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **aws-sdk-go** (v1) | Reached end-of-support in 2025 | aws-sdk-go-v2 (requires Go 1.23+) |
| **cloud.google.com/go/vertexai/genai** | Deprecated June 24, 2025, removed June 24, 2026 | google.golang.org/genai (new preferred SDK) |
| **http.DefaultClient** | No timeouts configured, hangs forever on slow servers | Custom http.Client with Timeout: 10*time.Second max for APIs |
| **Director function** (ReverseProxy) | Hop-by-hop headers removed AFTER Director returns, breaks header modification | Rewrite function (modern pattern, headers removed BEFORE Rewrite) |
| **Community Ollama clients** (unless needed) | Less stable than official SDK | github.com/ollama/ollama/api (used by CLI itself) |

## Stack Patterns by Variant

**If building for maximum throughput (1000+ req/s):**
- Use zerolog instead of slog (zero allocations)
- Custom http.Client with connection pooling tuned: MaxIdleConns, MaxIdleConnsPerHost
- Consider connection reuse for provider backends
- Because high-throughput scenarios benefit from allocation reduction

**If prioritizing simplicity and stdlib-only:**
- Use net/http/httputil.ReverseProxy with Rewrite function
- Use log/slog for logging
- Use encoding/json for config (skip viper)
- Because fewer dependencies = easier deployment, smaller binary

**If SSE streaming is critical:**
- Use http.Flusher interface directly with stdlib
- Set headers: Content-Type: text/event-stream, Cache-Control: no-cache, X-Accel-Buffering: no
- Flush after each SSE event write
- Because LLM proxies (Claude, ChatGPT) need exact SSE event ordering

**If building TUI dashboard:**
- Use Bubble Tea v1.3.10 (stable)
- Use Bubbles for components (spinners, tables, viewports)
- Use Lip Gloss for styling
- Because this is the de facto TUI stack in Go ecosystem

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| gRPC v1.78.0 | Go 1.23+ | Requires one of two latest major Go releases |
| aws-sdk-go-v2 v1.33.0+ | Go 1.23+ | Minimum version requirement |
| Bubble Tea v1.3.10 | Go 1.18+ | v2 available but ecosystem in transition |
| log/slog | Go 1.21+ | Introduced in Go 1.21, stable in 1.22+ |
| net/http/httputil Rewrite | Go 1.20+ | Rewrite function added in Go 1.20 |

## Critical Implementation Notes

### HTTP Reverse Proxy Pattern

Use modern Rewrite function, NOT Director:

```go
proxy := &httputil.ReverseProxy{
    Rewrite: func(r *httputil.ProxyRequest) {
        r.SetURL(targetURL)           // Set backend target
        r.SetXForwarded()              // Add X-Forwarded-* headers
        r.Out.Header.Set("key", "val") // Modify outbound headers
    },
}
```

### SSE Streaming Pattern

Must flush after each event for real-time LLM streaming:

```go
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache, no-transform")
w.Header().Set("X-Accel-Buffering", "no")
w.Header().Set("Connection", "keep-alive")

flusher := w.(http.Flusher)

fmt.Fprintf(w, "event: message_start\ndata: {...}\n\n")
flusher.Flush() // CRITICAL: flush after each event
```

### HTTP Client Timeout Pattern

Always set timeouts to prevent goroutine leaks:

```go
client := &http.Client{
    Timeout: 10 * time.Second, // Max 10s for API calls
}

// Or with per-request context timeout:
ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
defer cancel()
req = req.WithContext(ctx)
```

### Context Propagation

Use context for cancellation and timeouts:

```go
func (p *Provider) Transform(ctx context.Context, req *http.Request) error {
    select {
    case <-ctx.Done():
        return ctx.Err() // Handle cancellation
    default:
        // Continue processing
    }
}
```

## Confidence Assessment

| Area | Confidence | Rationale |
|------|------------|-----------|
| **Core Stack (stdlib)** | HIGH | Verified with pkg.go.dev (net/http/httputil Rewrite function, slog introduction in Go 1.21) |
| **gRPC** | HIGH | Verified v1.78.0 release (Dec 23, 2025) and Go 1.23+ requirement via pkg.go.dev |
| **Bubble Tea** | HIGH | Verified v1.3.10 and v2 availability via pkg.go.dev, widespread production use |
| **Provider SDKs** | HIGH | AWS SDK v2 v1.33.0 confirmed, Azure azopenai v0.8.0 (June 2025), Google genai SDK migration (June 2025) |
| **SSE Libraries** | MEDIUM | Multiple options exist (r3labs, tmaxmax, jetify), community preference varies; stdlib approach recommended for control |
| **Config Management** | HIGH | Viper is de facto standard (12K+ GitHub stars), fsnotify widely used (12,768 packages) |
| **Logging** | HIGH | slog in stdlib since Go 1.21, zerolog/zap benchmarks confirmed for high-throughput use cases |

## Sources

### Official Documentation
- [net/http/httputil ReverseProxy](https://pkg.go.dev/net/http/httputil) - Verified Rewrite function pattern, Director deprecation context
- [log/slog package](https://pkg.go.dev/log/slog) - Verified Go 1.21 introduction, TextHandler/JSONHandler
- [gRPC Go v1.78.0](https://pkg.go.dev/google.golang.org/grpc) - Verified version, Go 1.23+ requirement
- [Bubble Tea v1.3.10](https://pkg.go.dev/github.com/charmbracelet/bubbletea) - Verified version, v2 availability

### Web Search Sources (2025)
- [Go HTTP reverse proxy best practices 2025](https://go.dev/src/net/http/httputil/reverseproxy.go) - Rewrite function recommendation
- [Go SSE server-sent events streaming library 2025](https://github.com/tmaxmax/go-sse) - LLM streaming support confirmation
- [Go gRPC protobuf service management 2025](https://grpc.io/docs/languages/go/quickstart/) - buf generate workflow
- [Bubble Tea TUI framework Go 2025](https://github.com/charmbracelet/bubbletea) - Production use cases, architecture
- [AWS Bedrock Go SDK Anthropic Claude 2025](https://docs.anthropic.com/en/api/claude-on-amazon-bedrock) - BedrockRuntime client pattern
- [Go configuration management viper yaml 2025](https://github.com/spf13/viper) - Multi-format support, environment variables
- [Go structured logging slog zerolog 2025 best practices](https://betterstack.com/community/guides/logging/logging-in-go/) - slog vs zerolog performance comparison
- [Go Prometheus metrics instrumentation 2025](https://prometheus.io/docs/guides/go-application/) - promhttp.Handler() pattern
- [AWS SDK Go v2 2025 current version](https://github.com/aws/aws-sdk-go-v2/releases) - v1.33.0 release (Jan 15, 2025)
- [Google Cloud Go SDK Vertex AI 2025](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/sdks/overview) - genai SDK migration (June 24, 2025)
- [Azure OpenAI Go SDK 2025](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai) - azopenai v0.8.0 (June 2025)
- [Ollama Go client library 2025](https://pkg.go.dev/github.com/ollama/ollama/api) - Official API client confirmation
- [fsnotify file watcher Go 2025](https://github.com/fsnotify/fsnotify) - Cross-platform support, 12,768 dependents
- [Go HTTP client timeout best practices context 2025](https://betterstack.com/community/guides/scaling-go/golang-timeouts/) - Context-based timeout patterns

---
*Stack research for: Multi-provider LLM HTTP Proxy*
*Researched: 2026-01-20*
