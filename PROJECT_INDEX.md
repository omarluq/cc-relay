# Project Index: cc-relay

**Generated:** 2026-01-20
**Status:** Phase 1 Development (Core Proxy Implementation)
**Language:** Go 1.24.7
**Type:** HTTP Proxy / CLI Tool

---

## 📋 Overview

cc-relay is a multi-provider proxy for Claude Code that enables simultaneous use of multiple Anthropic-compatible API endpoints. It sits between Claude Code and multiple LLM providers (Anthropic, Z.AI, Ollama, AWS Bedrock, Azure Foundry, Vertex AI), providing rate limit pooling, cost optimization, automatic failover, and intelligent routing.

**Key Value Propositions:**

- Pool rate limits across multiple API keys
- Save costs by routing tasks to cheaper providers (Z.AI ~1/7 cost)
- Automatic failover between providers
- Mix cloud providers with local models
- TUI dashboard for real-time monitoring

---

## 📁 Project Structure

```
cc-relay/
├── .claude/                   # Claude Code configuration
│   ├── CLAUDE.md             # Development guide & instructions
│   ├── agents/               # GSD agent definitions
│   ├── commands/             # GSD slash commands
│   └── get-shit-done/        # GSD framework templates
├── .github/
│   └── workflows/
│       └── ci.yml            # CI/CD pipeline
├── .planning/                # Phase-based development planning
│   ├── PROJECT.md            # Project requirements
│   ├── ROADMAP.md            # 11-phase roadmap
│   ├── codebase/             # Codebase documentation
│   └── phases/               # Phase-specific plans
│       └── 01-core-proxy/    # Phase 1 plans (5 plans)
├── cmd/
│   └── cc-relay/
│       └── main.go           # CLI entry point
├── internal/
│   └── version/              # Version information
│       ├── version.go
│       └── version_test.go
├── scripts/
│   └── setup-tools.sh        # Development tools installer
├── bin/                      # Compiled binaries
├── README.md                 # User-facing documentation
├── SPEC.md                   # Technical specification (614 lines)
├── DEVELOPMENT.md            # Development setup guide
├── llms.txt                  # LLM-friendly context
├── relay.proto               # gRPC service definitions
├── example.yaml              # Example configuration
├── Taskfile.yml              # Task runner definitions
├── lefthook.yml              # Git hooks configuration
├── .air.toml                 # Live reload configuration
├── .golangci.yml             # Linter configuration
├── .mise.toml                # Tool version management
└── go.mod                    # Go module definition
```

---

## 🚀 Entry Points

### Current Entry Points

- **CLI Entry:** `cmd/cc-relay/main.go`
  - Currently minimal (prints "cc-relay")
  - Planned subcommands: `serve`, `tui`, `status`, `config`, `provider`, `key`

### Planned Entry Points (Phase 1+)

- **HTTP Server:** `internal/proxy/server.go` (TBD)
  - Endpoint: `POST /v1/messages` (Anthropic API compatible)
  - Streaming: SSE support via `internal/proxy/sse.go`

- **gRPC Server:** `internal/grpc/server.go` (Phase 4)
  - Management API for TUI/CLI/WebUI
  - Defined in `relay.proto`

- **TUI Application:** `ui/tui/app.go` (Phase 4)
  - Built with Bubble Tea framework
  - Connects to daemon via gRPC

---

## 📦 Core Modules

### Implemented

**1. Version Module** (`internal/version/`)
- Provides version information
- Test coverage included

### Planned Modules

**2. Proxy Layer** (`internal/proxy/`) - Phase 1
- HTTP reverse proxy implementing Anthropic Messages API
- SSE streaming handler
- Middleware for logging, metrics, auth
- Must preserve `tool_use_id` for parallel tool calls

**3. Router** (`internal/router/`) - Phases 2-3
- Request routing and API key selection
- Strategies: shuffle, round-robin, failover, cost-based, latency-based
- API key pool with rate limit tracking (RPM/TPM)

**4. Providers** (`internal/providers/`) - Phases 1, 5, 6
- Provider-specific API transformations
- Anthropic (Phase 1), Z.AI (Phase 5), Ollama (Phase 5)
- Bedrock, Azure, Vertex AI (Phase 6)

**5. Health Tracking** (`internal/health/`) - Phase 4
- Circuit breaker (CLOSED/OPEN/HALF-OPEN states)
- Provider health monitoring
- Automatic recovery probing

**6. Configuration** (`internal/config/`) - Phases 1, 7
- YAML/TOML config loading
- Environment variable expansion
- Hot-reload via fsnotify (Phase 7)

**7. gRPC Management API** (`internal/grpc/`) - Phase 9
- Real-time stats streaming
- Provider/key management
- Configuration updates

**8. TUI** (`ui/tui/`) - Phase 10
- Bubble Tea interface
- Real-time monitoring
- Interactive management

---

## 🔧 Configuration

### Default Location
`~/.config/cc-relay/config.yaml`

### Formats
- YAML (primary)
- TOML (alternative)

### Key Sections

| Section | Purpose | Phase |
|---------|---------|-------|
| `server` | Listen address, timeout, max concurrent | 1 |
| `routing` | Strategy selection, failover chain | 3 |
| `providers` | Provider configs, keys, rate limits | 1 |
| `grpc` | Management API settings | 9 |
| `logging` | Level, format, file output | 8 |
| `metrics` | Prometheus endpoint | 8 |
| `health` | Check intervals, circuit breaker | 4 |

### Environment Variables
Supports `${VAR_NAME}` expansion for API keys and credentials.

**Example:**
```yaml
providers:
  - name: anthropic
    type: anthropic
    keys:
      - key: "${ANTHROPIC_API_KEY}"
```

---

## 📚 Documentation

### User Documentation
- **README.md** (173 lines) - Quick start, features, setup
- **example.yaml** (216 lines) - Full config reference
- **QUICK_REFERENCE.md** - Command reference
- **SETUP_SUMMARY.md** - Setup guide

### Developer Documentation
- **SPEC.md** (614 lines) - Complete technical specification
- **DEVELOPMENT.md** - Development workflow guide
- **.claude/CLAUDE.md** - Claude Code development instructions
- **llms.txt** (155 lines) - LLM-optimized context
- **relay.proto** (355 lines) - gRPC API definitions

### Planning Documentation
- **.planning/PROJECT.md** - Core requirements
- **.planning/ROADMAP.md** - 11-phase development plan
- **.planning/phases/01-core-proxy/** - Phase 1 detailed plans (5 plans)

---

## 🏗️ Development Roadmap

### Current Status: Phase 1 - Core Proxy (MVP)

**11-Phase Roadmap:**

1. **✅ Phase 1: Core Proxy (In Progress)** - Basic proxy with Anthropic compatibility
2. **Phase 2: Multi-Key Pooling** - Rate limit pooling across multiple keys
3. **Phase 3: Routing Strategies** - Pluggable routing algorithms
4. **Phase 4: Circuit Breaker & Health** - Health tracking and failover
5. **Phase 5: Additional Providers** - Z.AI and Ollama support
6. **Phase 6: Cloud Providers** - Bedrock, Azure, Vertex AI
7. **Phase 7: Configuration Management** - Hot-reload and validation
8. **Phase 8: Observability** - Logging and metrics
9. **Phase 9: gRPC Management API** - Real-time stats and control
10. **Phase 10: TUI Dashboard** - Interactive monitoring interface
11. **Phase 11: CLI Commands** - Complete CLI tooling

### Phase 1 Details

**Goal:** Establish working proxy with Anthropic API compatibility

**Plans (5 total in 4 waves):**
- 01-01: Foundation - Config loading and Provider interface
- 01-02: HTTP Server and Auth middleware
- 01-03: Proxy handler with SSE streaming
- 01-04: CLI integration and route wiring
- 01-05: Integration testing and verification

**Success Criteria:**
1. Claude Code can send requests and receive responses
2. SSE streaming works with real-time delivery
3. Parallel tool calls preserve `tool_use_id`
4. Invalid API keys return 401
5. Extended thinking blocks stream correctly

---

## 🔗 Dependencies

### Current
```go
module github.com/omarluq/cc-relay
go 1.24.7
```

### Planned Dependencies

| Package | Purpose | Phase |
|---------|---------|-------|
| `net/http`, `net/http/httputil` | HTTP proxy server | 1 |
| `google.golang.org/grpc` | gRPC management API | 9 |
| `github.com/charmbracelet/bubbletea` | TUI framework | 10 |
| `github.com/fsnotify/fsnotify` | Config hot-reload | 7 |
| `gopkg.in/yaml.v3` | YAML parsing | 1 |
| AWS SDK | Bedrock SigV4 signing | 6 |
| Google Auth Library | Vertex AI OAuth | 6 |
| Prometheus client | Metrics export | 8 |

---

## 🧪 Testing

### Current Status
- `internal/version/version_test.go` - Version module tests

### Test Commands (via Taskfile)

```bash
task test              # Run all tests
task test-short        # Quick test run
task test-coverage     # Generate coverage report
task test-integration  # Integration tests
task bench             # Benchmarks
```

### Planned Test Coverage

**Unit Tests:**
- Provider transformers
- Routing strategies
- Key pool selection
- Circuit breaker states
- Config parsing

**Integration Tests:**
- End-to-end proxy flow
- SSE streaming correctness
- Multi-provider failover
- Rate limit enforcement
- gRPC API functionality

---

## 🛠️ Development Tools

### Installed Tools (via `scripts/setup-tools.sh`)

**Formatters:**
- `gofmt`, `goimports`, `gofumpt`

**Linters:**
- `golangci-lint` (meta-linter with 40+ linters)
- `go vet`

**Security:**
- `govulncheck`
- `gosec`

**Build & Development:**
- `task` (task runner)
- `air` (live reload)
- `lefthook` (git hooks)

**Other:**
- `buf` (protobuf tooling)
- `yamlfmt`, `yamllint`
- `markdownlint-cli`

### Development Workflow

```bash
# Setup tools (first time)
./scripts/setup-tools.sh
# or
task setup

# Development with live reload
task dev

# Build
task build

# Code quality
task fmt          # Auto-format
task lint         # Run linters
task ci           # Full CI checks

# Git hooks
task hooks-install
task hooks-run    # Manual run
```

### Git Hooks (Lefthook)

**Pre-commit:**
- Format code (gofmt, goimports, gofumpt)
- Lint (golangci-lint with auto-fix)
- YAML/Markdown formatting
- Quick tests

**Pre-push:**
- Full test suite
- Security scanning (govulncheck)
- Build verification

**Commit message:**
- Conventional Commits validation
- Valid types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `ci`, `build`

---

## 📝 Quick Start (Current)

### Build & Run

```bash
# Clone
git clone https://github.com/omarish/cc-relay
cd cc-relay

# Install tools
./scripts/setup-tools.sh

# Build
task build

# Run
./bin/cc-relay
```

### Development

```bash
# Live reload development
task dev

# Run CI checks locally
task ci

# Format and fix code
task fmt

# Run tests
task test
```

---

## 🎯 Critical Implementation Notes

### API Compatibility (Phase 1 Priority)

- **Must** implement `/v1/messages` exactly matching Anthropic API
- **Must** handle SSE with correct event sequence order
- **Must** preserve `tool_use_id` for Claude Code's parallel tool calls
- **Must** support extended thinking blocks

### SSE Streaming Requirements

```
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
X-Accel-Buffering: no
Connection: keep-alive
```

Use `http.Flusher` interface to flush each SSE event immediately.

### Provider Transformations

| Provider | Key Requirement | Phase |
|----------|----------------|-------|
| Anthropic | Native format | 1 |
| Z.AI | Fully compatible | 5 |
| Ollama | No caching, base64 images | 5 |
| Bedrock | Model in URL, SigV4 auth | 6 |
| Vertex AI | Model in URL, OAuth | 6 |
| Azure | `x-api-key` header | 6 |

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
| Full documentation read | ~6,500 tokens |
| This index | ~2,800 tokens |
| **Savings per session** | **~3,700 tokens (57%)** |
| **10 sessions** | **37,000 tokens saved** |

**Break-even:** 1 session (index creation ~2,000 tokens)

---

## 🔄 GSD Framework Integration

This project uses the Get Shit Done (GSD) framework for development:

- **Planning:** `.planning/` directory with PROJECT.md, ROADMAP.md
- **Phases:** 11 phases with detailed plans and success criteria
- **Commands:** GSD slash commands in `.claude/commands/gsd/`
- **Agents:** Specialized agents for planning, execution, verification
- **Workflow:** Discuss → Research → Plan → Execute → Verify

**Key GSD Commands:**
- `/gsd:progress` - Check current status
- `/gsd:execute-phase` - Execute phase plans
- `/gsd:plan-phase` - Create detailed phase plan
- `/gsd:verify-work` - Validate implementation

---

**Last Updated:** 2026-01-20
**Index Version:** 2.0
**Project Phase:** Phase 1 Development (Core Proxy Implementation)
**Current Wave:** Planning and foundation setup
