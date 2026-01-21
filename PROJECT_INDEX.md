# Project Index: cc-relay

**Generated:** 2026-01-20
**Status:** Pre-implementation (Specification Phase)
**Language:** Go 1.24.7
**Type:** HTTP Proxy / CLI Tool

---

## 📋 Overview

cc-relay is a multi-provider proxy for Claude Code that enables simultaneous use of multiple Anthropic-compatible API endpoints. It sits between Claude Code and multiple LLM providers (Anthropic, Z.AI, Ollama, AWS Bedrock, Azure Foundry, Vertex AI), providing rate limit pooling, cost optimization, automatic failover, and intelligent routing.

**Key Value Propositions:**

- Pool rate limits across multiple API keys
- Save costs by routing tasks to cheaper providers
- Automatic failover between providers
- Mix cloud providers with local models

---

## 📁 Project Structure

```
cc-relay/
├── .claude/
│   ├── CLAUDE.md              # Claude Code development guide
│   └── settings.local.json    # Local settings
├── .github/
│   └── workflows/
│       └── test.yml           # CI/CD pipeline
├── README.md                  # User-facing documentation
├── SPEC.md                    # Technical specification (614 lines)
├── llms.txt                   # LLM-friendly project context (155 lines)
├── relay.proto                # gRPC service definitions (355 lines)
├── example.yaml               # Example configuration (216 lines)
├── go.mod                     # Go module definition
└── PROJECT_INDEX.md           # This file
```

**Note:** No source code exists yet. This is the specification/design phase before implementation.

---

## 🚀 Entry Points

### Planned Entry Points (Not Yet Implemented)

Based on SPEC.md architecture:

- **CLI Entry:** `cmd/cc-relay/main.go`
  - Subcommands: `serve`, `tui`, `status`, `config`, `provider`, `key`

- **HTTP Server:** `internal/proxy/server.go`
  - Endpoint: `POST /v1/messages` (Anthropic API compatible)
  - Streaming: SSE support via `internal/proxy/sse.go`

- **gRPC Server:** `internal/grpc/server.go`
  - Management API for TUI/CLI/WebUI
  - Defined in `relay.proto`

- **TUI Application:** `ui/tui/app.go`
  - Built with Bubble Tea framework
  - Connects to daemon via gRPC

---

## 📦 Core Modules (Planned)

### 1. Proxy Layer (`internal/proxy/`)

**Purpose:** HTTP reverse proxy implementing Anthropic Messages API

**Key Components:**

- `server.go` - Main HTTP server
- `sse.go` - Server-Sent Events streaming handler
- `middleware.go` - Logging, metrics, auth validation

**Critical Requirements:**

- Exact Anthropic API format compatibility
- Preserve `tool_use_id` for parallel tool calls
- Maintain SSE event sequence order
- Support extended thinking blocks

### 2. Router (`internal/router/`)

**Purpose:** Request routing and API key selection

**Strategies:**

- `simple-shuffle` - Weighted random (default)
- `round-robin` - Sequential distribution
- `least-busy` - Fewest in-flight requests
- `cost-based` - Prefer cheaper providers
- `latency-based` - Prefer faster backends
- `failover` - Primary with fallback chain
- `model-based` - Route by model prefix

**Key Components:**

- `router.go` - Routing interface
- `strategies/*.go` - Strategy implementations
- `keypool.go` - API key pool with rate limit tracking

### 3. Providers (`internal/providers/`)

**Purpose:** Provider-specific API transformations

**Interface:**

```go
type ProviderTransformer interface {
    TransformRequest(req *AnthropicRequest) (*http.Request, error)
    TransformResponse(resp *http.Response) (*AnthropicResponse, error)
    Authenticate(req *http.Request) error
    HealthCheck(ctx context.Context) error
}
```

**Implementations:**

- `anthropic.go` - Direct Anthropic API (native)
- `zai.go` - Z.AI / Zhipu GLM (Anthropic-compatible)
- `ollama.go` - Local Ollama (limited features)
- `bedrock.go` - AWS Bedrock (model in URL, SigV4 auth)
- `azure.go` - Azure AI Foundry (x-api-key header)
- `vertex.go` - Google Vertex AI (model in URL, OAuth)

### 4. Health Tracking (`internal/health/`)

**Purpose:** Circuit breaker and provider health monitoring

**Components:**

- `tracker.go` - Health tracking per backend
- `circuit.go` - Circuit breaker (CLOSED/OPEN/HALF-OPEN)

**Triggers:**

- Rate limit errors (429)
- Timeouts
- Server errors (5xx)
- Configurable failure thresholds

### 5. Configuration (`internal/config/`)

**Purpose:** YAML/TOML config loading and hot-reload

**Components:**

- `config.go` - Configuration structs
- `loader.go` - Parse and validate config
- `watcher.go` - File watcher for hot-reload (fsnotify)

### 6. gRPC Management API (`internal/grpc/`)

**Purpose:** Real-time stats and management interface

**Service Definition:** See `relay.proto`

**Key RPCs:**

- `StreamStats` - Real-time statistics stream
- `ListProviders` / `EnableProvider` / `DisableProvider`
- `ListKeys` / `AddKey` / `RemoveKey` / `GetKeyUsage`
- `GetConfig` / `UpdateConfig` / `ReloadConfig`
- `GetRoutingStrategy` / `SetRoutingStrategy`
- `StreamRequests` - Request log stream

### 7. TUI (`ui/tui/`)

**Purpose:** Terminal user interface for management

**Framework:** Bubble Tea (Elm architecture)

**Features:**

- Real-time provider stats
- Per-key rate limit tracking
- Request logs
- Provider enable/disable
- Health status visualization

---

## 🔧 Configuration

### Primary Config File

- **Location:** `~/.config/cc-relay/config.yaml`
- **Format:** YAML (primary) or TOML
- **Example:** `example.yaml` (216 lines, comprehensive)

### Key Sections

| Section | Purpose |
|---------|---------|
| `server` | Listen address, timeout, max concurrent |
| `routing` | Strategy selection, failover chain |
| `providers` | Array of provider configs with keys, rate limits, model mappings |
| `grpc` | Management API settings |
| `logging` | Level, format (json/text), file output |
| `metrics` | Prometheus endpoint config |
| `health` | Check intervals, circuit breaker thresholds |

### Environment Variables

- Support `${VAR_NAME}` expansion in config
- Commonly used: `ANTHROPIC_API_KEY`, `ZAI_API_KEY`, `AWS_BEARER_TOKEN_BEDROCK`, `AZURE_API_KEY`, `GOOGLE_APPLICATION_CREDENTIALS`

---

## 📚 Documentation

### For Users

- **README.md** (173 lines) - Quick start, features, usage
- **example.yaml** (216 lines) - Comprehensive config reference with comments
- **llms.txt** (155 lines) - LLM-optimized project context

### For Developers

- **SPEC.md** (614 lines) - Complete technical specification
  - Architecture diagrams
  - Provider-specific requirements
  - API compatibility details
  - Development phases
  - Component architecture

- **.claude/CLAUDE.md** - Claude Code development guide
  - Build, test, run commands
  - Architecture overview
  - Critical API compatibility notes
  - Provider transformation requirements

### API Definitions

- **relay.proto** (355 lines) - gRPC service definitions
  - Stats messages
  - Provider management
  - Key management
  - Configuration
  - Routing
  - Request logs

---

## 🏗️ Development Phases

The project follows a phased roadmap:

### Phase 1: MVP (v0.1.0)

- [ ] Basic HTTP proxy server
- [ ] Anthropic provider (single key)
- [ ] Z.AI provider
- [ ] Ollama provider
- [ ] Simple-shuffle routing
- [ ] YAML configuration
- [ ] Basic CLI (`cc-relay serve`)

### Phase 2: Multi-Key & Routing (v0.2.0)

- [ ] Multi-key pooling per provider
- [ ] Rate limit tracking (RPM/TPM)
- [ ] Round-robin strategy
- [ ] Failover strategy with circuit breaker
- [ ] Health checking
- [ ] Hot-reload configuration

### Phase 3: Cloud Providers (v0.3.0)

- [ ] AWS Bedrock provider
- [ ] Azure Foundry provider
- [ ] Vertex AI provider

### Phase 4: Management Interface (v0.4.0)

- [ ] gRPC management API
- [ ] Bubble Tea TUI
- [ ] Real-time stats streaming
- [ ] Key/provider management commands

### Phase 5: Advanced Features (v0.5.0)

- [ ] Cost-based routing
- [ ] Latency-based routing
- [ ] Model-based routing
- [ ] Prometheus metrics
- [ ] Request logging/audit trail

### Phase 6: WebUI (v0.6.0)

- [ ] grpc-web proxy
- [ ] React/Svelte WebUI
- [ ] Dashboard with live stats

---

## 🔗 Key Dependencies (Planned)

| Dependency | Purpose |
|------------|---------|
| Go 1.24.7 | Base language |
| `net/http`, `net/http/httputil` | HTTP proxy server |
| `google.golang.org/grpc` | gRPC management API |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/fsnotify/fsnotify` | Config file watching |
| `gopkg.in/yaml.v3` | YAML parsing |
| AWS SDK | Bedrock SigV4 signing |
| Google Auth Library | Vertex AI OAuth |
| Prometheus client | Metrics export |

---

## 🧪 Testing Strategy

### Planned Test Coverage

**Unit Tests:**

- Provider transformers (request/response)
- Routing strategies
- Key pool selection logic
- Circuit breaker state transitions
- Config parsing and validation

**Integration Tests:**

- End-to-end proxy flow
- SSE streaming correctness
- Multi-provider failover
- Rate limit enforcement
- gRPC API functionality

**Test Providers:**

- Mock providers for unit tests
- Local Ollama for integration tests
- Z.AI test endpoint (if available)

### Test Commands

```bash
go test ./...                        # All tests
go test -v ./internal/proxy          # Specific package
go test -race ./...                  # Race detection
go test -cover ./...                 # Coverage
go test -bench=. ./internal/router   # Benchmarks
```

---

## 📝 Quick Start (Planned)

### Installation

```bash
go install github.com/omarish/cc-relay@latest
```

### Configuration

```bash
mkdir -p ~/.config/cc-relay
cp example.yaml ~/.config/cc-relay/config.yaml
# Edit config.yaml with your API keys
```

### Running

```bash
# Start daemon
cc-relay serve

# In another terminal, configure Claude Code
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_API_KEY="managed-by-cc-relay"

# Use Claude Code normally
claude
```

### Management

```bash
cc-relay tui              # Launch TUI
cc-relay status           # Show status
cc-relay config reload    # Reload config
cc-relay provider list    # List providers
```

---

## 🎯 Critical Implementation Notes

### API Compatibility

- **Must** implement `/v1/messages` exactly matching Anthropic API
- **Must** handle SSE with correct event sequence
- **Must** preserve `tool_use_id` for Claude Code's parallel tool calls
- **Must** support extended thinking blocks

### Provider Transformations

| Provider | Key Requirement |
|----------|----------------|
| Bedrock | Model in URL, `anthropic_version: bedrock-2023-05-31`, SigV4 |
| Vertex AI | Model in URL, `anthropic_version: vertex-2023-10-16`, OAuth |
| Azure | Use `x-api-key` header, deployment names as model IDs |
| Ollama | No prompt caching, no PDF support, base64 images only |

### SSE Streaming

```
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
X-Accel-Buffering: no
Connection: keep-alive
```

Use `http.Flusher` to flush each event immediately.

---

## 🔍 Similar Projects

- **LiteLLM** (Python) - Multi-LLM proxy, 8ms P95 latency
- **claude-code-router** (TypeScript) - Route-based provider selection
- **Bifrost** (Go) - High-performance LLM gateway, 11μs overhead
- **OpenRouter** (Service) - Cloud-scale routing

---

## 📊 Token Efficiency

**This index saves tokens in future sessions:**

| Metric | Value |
|--------|-------|
| Full documentation read | ~3,500 tokens |
| This index | ~1,200 tokens |
| **Savings per session** | **~2,300 tokens (66%)** |
| **10 sessions** | **23,000 tokens saved** |

---

**Last Updated:** 2026-01-20
**Index Version:** 1.0
**Project Phase:** Specification & Design (Pre-implementation)
