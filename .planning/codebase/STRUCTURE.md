# Codebase Structure

**Analysis Date:** 2026-01-20

## Directory Layout

```
cc-relay/
├── .claude/                           # Claude Code development guidance
│   ├── CLAUDE.md                     # Development commands, architecture overview
│   └── settings.local.json           # Local IDE settings
├── .github/
│   └── workflows/
│       └── test.yml                  # CI/CD pipeline (run tests on PR)
├── .planning/
│   └── codebase/                     # Generated architecture docs (this location)
│       ├── ARCHITECTURE.md           # Pattern, layers, data flow
│       └── STRUCTURE.md              # This file
├── cmd/
│   └── cc-relay/
│       └── main.go                   # CLI entry point (planned: not yet implemented)
├── internal/
│   ├── proxy/
│   │   ├── server.go                # HTTP proxy server (POST /v1/messages)
│   │   ├── middleware.go            # Logging, auth, metrics middleware
│   │   └── sse.go                   # Server-Sent Events streaming handler
│   ├── router/
│   │   ├── router.go                # Core routing interface & selector
│   │   ├── keypool.go               # API key pool with rate limit tracking
│   │   └── strategies/              # Routing strategy implementations
│   │       ├── shuffle.go           # Weighted random (default)
│   │       ├── round_robin.go       # Sequential distribution
│   │       ├── least_busy.go        # Fewest in-flight requests
│   │       ├── cost_based.go        # Prefer cheaper providers
│   │       ├── latency_based.go     # Track latencies, prefer faster
│   │       ├── failover.go          # Primary + fallback chain
│   │       └── model_based.go       # Route by model prefix
│   ├── providers/
│   │   ├── interface.go             # ProviderTransformer interface
│   │   ├── anthropic.go             # Anthropic direct API (native)
│   │   ├── zai.go                   # Z.AI / Zhipu GLM (Anthropic-compatible)
│   │   ├── ollama.go                # Local Ollama (limited features)
│   │   ├── bedrock.go               # AWS Bedrock (model in URL, SigV4)
│   │   ├── azure.go                 # Azure AI Foundry (x-api-key header)
│   │   └── vertex.go                # Google Vertex AI (model in URL, OAuth)
│   ├── health/
│   │   ├── tracker.go               # Health tracking per backend
│   │   └── circuit.go               # Circuit breaker (CLOSED/OPEN/HALF-OPEN)
│   ├── config/
│   │   ├── config.go                # Configuration structs
│   │   ├── loader.go                # YAML/TOML parsing & validation
│   │   └── watcher.go               # File watcher for hot-reload (fsnotify)
│   ├── grpc/
│   │   ├── server.go                # gRPC server implementation
│   │   └── handlers.go              # RPC handlers (stats, providers, keys, config)
│   ├── types/
│   │   └── types.go                 # Common types (Request, Response, etc.)
│   └── util/
│       └── util.go                  # Helper functions (logging, errors)
├── ui/
│   └── tui/
│       ├── app.go                   # Bubble Tea application (TUI main)
│       ├── models.go                # TUI state model
│       ├── views/
│       │   ├── dashboard.go         # Real-time stats view
│       │   ├── providers.go         # Provider list and management
│       │   ├── keys.go              # API key management view
│       │   └── logs.go              # Request log stream view
│       └── utils/
│           └── tui_utils.go         # TUI formatting helpers
├── proto/
│   └── relay.proto                   # gRPC service definitions (355 lines)
│       └── Generates: internal/grpc/relay.pb.go, relay_grpc.pb.go
├── config/
│   └── (not yet used in Phase 1)
├── scripts/
│   └── (not yet used in Phase 1)
├── .planning/
│   └── codebase/                    # This directory for generated docs
├── pkg/                              # Packages directory (empty, placeholder)
├── go.mod                            # Go module definition
├── example.yaml                      # Example configuration (216 lines)
├── relay.proto                       # Protobuf definitions (355 lines)
├── SPEC.md                           # Complete technical specification (614 lines)
├── PROJECT_INDEX.md                  # Project overview and index
├── README.md                         # User-facing documentation
├── llms.txt                          # LLM-optimized project context
└── lefthook.yml                      # Git hooks (pre-commit, pre-push)
```

## Directory Purposes

**`.claude/`**
- Purpose: Claude Code development guidance and IDE settings
- Contains: Development commands, architecture notes, local settings
- Key files: `CLAUDE.md` (referenced by Claude Code IDE)

**`.github/workflows/`**
- Purpose: CI/CD pipeline definitions
- Contains: GitHub Actions workflow for automated testing
- Key files: `test.yml` (runs `go test ./...` on PRs)

**`.planning/codebase/`**
- Purpose: Generated architecture documentation
- Contains: ARCHITECTURE.md, STRUCTURE.md, CONVENTIONS.md, TESTING.md, CONCERNS.md
- Generated by: GSD codebase mapper tools

**`cmd/cc-relay/`**
- Purpose: CLI binary entry point
- Contains: main.go with command parsing
- Subcommands: serve, tui, status, config, provider, key

**`internal/proxy/`**
- Purpose: HTTP reverse proxy layer
- Contains: Server listening on :8787, request routing, SSE streaming
- Key abstractions: HTTP middleware chain, request validation
- Tests: `internal/proxy/proxy_test.go`, `internal/proxy/sse_test.go`

**`internal/router/`**
- Purpose: Request routing and API key selection
- Contains: Router interface, strategy implementations, key pool tracking
- Strategies: shuffle, round-robin, least-busy, cost-based, latency-based, failover, model-based
- Per-key tracking: RPM/TPM usage, sliding window counters
- Tests: `internal/router/router_test.go`, `internal/router/strategies/*_test.go`

**`internal/providers/`**
- Purpose: Provider-specific API transformations
- Contains: ProviderTransformer interface and 6 implementations
- Each provider handles: request transformation, response parsing, authentication, health checking
- No generic HTTP calling — each provider wraps provider-specific client or makes direct HTTP calls
- Tests: `internal/providers/*_test.go` with mocked backends

**`internal/health/`**
- Purpose: Circuit breaker and health tracking
- Contains: Circuit breaker state machine, failure tracking per provider
- States: CLOSED (normal), OPEN (blocking), HALF-OPEN (probing)
- Triggers: 429s, 5xx, timeouts
- Tests: `internal/health/circuit_test.go`

**`internal/config/`**
- Purpose: Configuration loading and hot-reload
- Contains: Config parsing (YAML/TOML), validation, file watcher
- Environment variables: Support `${VAR_NAME}` expansion
- Hot-reload: Watched file changes trigger reload with fsnotify
- Tests: `internal/config/config_test.go`, `internal/config/loader_test.go`

**`internal/grpc/`**
- Purpose: gRPC management API implementation
- Contains: gRPC server, RPC handlers
- Service: RelayManager (stats streaming, provider/key management, config updates, routing control)
- Used by: TUI, CLI, optional WebUI
- Tests: `internal/grpc/server_test.go`

**`internal/types/`**
- Purpose: Common type definitions
- Contains: Anthropic API types, internal request/response types
- Reused by: Proxy, providers, router, gRPC handlers

**`internal/util/`**
- Purpose: Helper utilities
- Contains: Logging wrappers, error handling, formatting
- Reused by: All packages

**`ui/tui/`**
- Purpose: Terminal User Interface for daemon management
- Contains: Bubble Tea application with views and models
- Views: Dashboard (stats), Providers (enable/disable), Keys (manage), Logs (stream)
- Architecture: Elm-style (Model → View → Update)
- Tests: `ui/tui/app_test.go`

**`proto/`**
- Purpose: gRPC service definitions
- Contains: `relay.proto` with RelayManager service (stats, providers, keys, config, routing, requests)
- Generated: Invokes `protoc` to generate Go code in `internal/grpc/`

**`example.yaml`**
- Purpose: Reference configuration
- Contains: All configurable sections with comments
- Sections: server, routing, providers (Anthropic, Z.AI, Ollama, Bedrock, Azure, Vertex), grpc, logging, metrics, health

## Key File Locations

**Entry Points:**
- `cmd/cc-relay/main.go`: CLI binary (parses `serve`, `tui`, `status` subcommands)
- `internal/proxy/server.go`: HTTP server on :8787 (accepts Claude Code requests)
- `internal/grpc/server.go`: gRPC server on :50051 (accepts TUI/CLI management)
- `ui/tui/app.go`: TUI application (connects to gRPC daemon)

**Configuration:**
- `~/.config/cc-relay/config.yaml`: Runtime config (loaded at startup, watched for changes)
- `example.yaml`: Template configuration with all available options
- `internal/config/config.go`: Config struct definitions (maps to YAML schema)

**Core Logic:**
- `internal/router/router.go`: Main routing interface (SelectProvider)
- `internal/providers/interface.go`: ProviderTransformer interface definition
- `internal/health/circuit.go`: Circuit breaker state machine
- `internal/proxy/server.go`: HTTP request/response handling

**Testing:**
- `internal/proxy/proxy_test.go`: HTTP proxy handler tests
- `internal/router/router_test.go`: Routing strategy tests
- `internal/providers/*_test.go`: Provider transformer tests with mocked backends
- `internal/health/circuit_test.go`: Circuit breaker state transition tests

## Naming Conventions

**Files:**
- Lowercase with underscores: `proxy_handler.go`, `circuit_breaker.go`
- Test files: `*_test.go` suffix (co-located with code)
- Interface files: `interface.go` for abstract types
- Main entry: `main.go` in cmd packages

**Directories:**
- Lowercase, single word or hyphenated: `internal/`, `internal/grpc/`
- Plural for collections: `strategies/`, `handlers/`, `views/`
- Domain-based: `internal/proxy/`, `internal/router/`, `internal/providers/`

**Packages:**
- Lowercase: `proxy`, `router`, `providers`, `health`, `config`, `grpc`
- Match directory name
- No underscores in package names

**Functions:**
- Exported (public): PascalCase (e.g., `SelectProvider`, `TransformRequest`)
- Unexported (private): camelCase (e.g., `selectStrategy`, `transformAuth`)
- Interfaces: Usually suffixed with `-er` (e.g., `ProviderTransformer`, `Router`)

**Variables:**
- Unexported: camelCase (e.g., `keyPool`, `circuitBreaker`)
- Exported: PascalCase (e.g., `APIKey`, `HealthState`)
- Config: Match YAML keys: `rpm_limit`, `tpm_limit`, `model_mapping`

**Types:**
- Interfaces: End in `-er`, `-or`, or `-r`: `ProviderTransformer`, `Router`
- Structs: PascalCase: `Config`, `KeyPool`, `CircuitBreaker`
- Enums (const): PascalCase with prefix: `StateOpen`, `StateClosed`

## Where to Add New Code

**New Feature (e.g., new routing strategy):**
- Create: `internal/router/strategies/new_strategy.go`
- Implementation: Satisfy `Router` interface
- Tests: `internal/router/strategies/new_strategy_test.go`
- Register: Add to strategy map in `internal/router/router.go`

**New Provider (e.g., OpenAI):**
- Create: `internal/providers/openai.go`
- Implementation: Satisfy `ProviderTransformer` interface
- Tests: `internal/providers/openai_test.go` with mocked backend
- Registration: Add config parsing in `internal/config/loader.go`

**New TUI View (e.g., metrics dashboard):**
- Create: `ui/tui/views/metrics.go`
- Implementation: Bubble Tea component (Model, View, Update)
- Integration: Add to main app in `ui/tui/app.go`

**New gRPC RPC (e.g., GetMetrics):**
- Add to: `proto/relay.proto` (new RPC in RelayManager service)
- Generate: Run `protoc` or `buf generate`
- Implement: Add handler in `internal/grpc/handlers.go`
- Tests: `internal/grpc/server_test.go`

**Utilities:**
- Shared helpers: `internal/util/` (logging, formatting, error helpers)
- Common types: `internal/types/` (Anthropic API types)

**Middleware / Middleware-like:**
- Proxy middleware: `internal/proxy/middleware.go` (logging, metrics, auth)
- Each middleware is its own function that wraps `http.Handler`

## Special Directories

**`internal/`:**
- Purpose: Go standard - code not meant for external import
- Generated: Not committed, generated from `proto/relay.proto`
- Contains: All core business logic

**`cmd/`:**
- Purpose: Go standard - executable entry points
- Contains: CLI binary with subcommands
- Pattern: One subdirectory per executable

**`ui/`:**
- Purpose: User interface code (TUI, future WebUI)
- Generated: Not committed, built from source
- Architecture: Bubble Tea framework with Elm-style state management

**`proto/`:**
- Purpose: gRPC service definitions
- Generated: Generates code into `internal/grpc/` via protoc
- Pattern: Single `relay.proto` file with one service

**`pkg/`:**
- Purpose: Placeholder for future public packages
- Currently: Empty
- Usage: If code needs to be reusable/importable by other projects

---

*Structure analysis: 2026-01-20*
