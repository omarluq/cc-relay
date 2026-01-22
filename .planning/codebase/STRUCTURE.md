# Codebase Structure

**Analysis Date:** 2026-01-22 (Updated)
**Previous Analysis:** 2026-01-20

## Current Directory Layout (Implemented)

```
cc-relay/
├── .claude/                           # Claude Code development guidance
│   ├── CLAUDE.md                     # Development commands, architecture overview
│   ├── INDEX.md                      # Skills and commands index
│   ├── skills/                       # Reusable skills
│   ├── commands/                     # Custom commands
│   └── agents/                       # Agent configurations
├── .github/
│   └── workflows/
│       └── ci.yml                    # CI/CD pipeline (test, lint, build)
├── .planning/
│   ├── codebase/                     # Architecture documentation
│   │   ├── ARCHITECTURE.md           # Pattern, layers, data flow
│   │   ├── STRUCTURE.md              # This file
│   │   └── TESTING.md                # Test strategy
│   ├── phases/                       # Phase-based planning
│   │   ├── 01-core-proxy/            # Phase 1: Core proxy
│   │   ├── 01.1-embedded-ha-cache/   # Phase 1.1: HA cache
│   │   ├── 02-multi-key-pooling/     # Phase 2: Multi-key pooling
│   │   └── 02.3-codebase-refactor/   # Phase 2.3: Samber refactor
│   ├── STATE.md                      # Current project state
│   ├── ROADMAP.md                    # Project roadmap
│   └── PROJECT.md                    # Project overview
├── cmd/
│   └── cc-relay/
│       ├── main.go                   # CLI entry point
│       ├── serve.go                  # serve command (server startup)
│       ├── config.go                 # config validate command
│       ├── config_init.go            # config init command
│       ├── config_cc.go              # config cc subcommands
│       ├── config_cc_init.go         # config cc init command
│       ├── config_cc_remove.go       # config cc remove command
│       ├── status.go                 # status command
│       └── version.go                # version command
├── internal/
│   ├── auth/
│   │   ├── auth.go                   # Authenticator interface
│   │   ├── apikey.go                 # API key authenticator
│   │   ├── oauth.go                  # OAuth/Bearer authenticator
│   │   └── chain.go                  # Chain authenticator
│   ├── cache/
│   │   ├── cache.go                  # Cache interface
│   │   ├── config.go                 # Cache configuration
│   │   ├── errors.go                 # Cache errors
│   │   ├── factory.go                # Cache factory (New function)
│   │   ├── logging.go                # Cache logging
│   │   ├── noop.go                   # No-op cache
│   │   ├── olric.go                  # Olric distributed cache
│   │   ├── ristretto.go              # Ristretto local cache
│   │   └── testutil.go               # Test utilities
│   ├── config/
│   │   ├── config.go                 # Config structs
│   │   └── loader.go                 # YAML loading with env expansion
│   ├── keypool/
│   │   ├── pool.go                   # KeyPool implementation
│   │   ├── key.go                    # KeyMetadata, header parsing
│   │   ├── selector.go               # KeySelector interface
│   │   ├── least_loaded.go           # Least loaded selector
│   │   └── round_robin.go            # Round robin selector
│   ├── pkg/
│   │   └── functional/
│   │       └── functional.go         # Samber library imports
│   ├── providers/
│   │   ├── provider.go               # Provider interface
│   │   ├── base.go                   # Base provider
│   │   ├── anthropic.go              # Anthropic provider
│   │   └── zai.go                    # Z.AI provider
│   ├── proxy/
│   │   ├── handler.go                # Main request handler
│   │   ├── routes.go                 # Route setup
│   │   ├── middleware.go             # Auth, logging middleware
│   │   ├── server.go                 # HTTP server
│   │   ├── sse.go                    # SSE event handling
│   │   ├── models.go                 # Model listing handler
│   │   ├── providers_handler.go      # Providers endpoint
│   │   ├── logger.go                 # Zerolog configuration
│   │   ├── debug.go                  # Debug options
│   │   └── errors.go                 # Error handling
│   ├── ratelimit/
│   │   ├── limiter.go                # RateLimiter interface
│   │   └── token_bucket.go           # Token bucket implementation
│   └── version/
│       └── version.go                # Build version info
├── config/
│   └── example.yaml                  # Example configuration
├── docs/
│   ├── cache.md                      # Cache documentation
│   └── configuration.md              # Configuration docs
├── docs-site/                        # Hugo documentation site
│   └── content/                      # Multi-language content
│       ├── en/                       # English
│       ├── de/                       # German
│       ├── es/                       # Spanish
│       ├── ja/                       # Japanese
│       ├── ko/                       # Korean
│       └── zh-cn/                    # Chinese (Simplified)
├── scripts/
│   └── setup-tools.sh                # Development tools setup
├── Taskfile.yml                      # Task runner configuration
├── go.mod                            # Go module definition
├── go.sum                            # Go module checksums
├── lefthook.yml                      # Git hooks (pre-commit, pre-push)
├── .golangci.yml                     # Linter configuration
├── .air.toml                         # Live reload configuration
└── README.md                         # Project README
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

## Key Files Per Package

### cmd/cc-relay (CLI Commands)
| File | Lines | Purpose |
|------|-------|---------|
| main.go | 37 | Entry point, root command |
| serve.go | 235 | Server startup, signal handling |
| config.go | 105 | Config validate command |
| config_init.go | ~150 | Generate default config |
| config_cc_init.go | 85 | Claude Code integration |
| config_cc_remove.go | 98 | Remove CC integration |
| status.go | 90 | Health check command |

### internal/proxy (HTTP Server)
| File | Lines | Purpose |
|------|-------|---------|
| handler.go | ~140 | Main request handler |
| routes.go | ~80 | Route setup |
| middleware.go | ~200 | Auth, logging middleware |
| server.go | ~100 | HTTP server config |
| sse.go | ~100 | SSE event handling |
| logger.go | ~150 | Zerolog configuration |
| debug.go | ~100 | Debug options |

### internal/keypool (Multi-Key Management)
| File | Lines | Purpose |
|------|-------|---------|
| pool.go | ~350 | KeyPool implementation |
| key.go | ~300 | KeyMetadata, header parsing |
| selector.go | ~50 | KeySelector interface |
| least_loaded.go | ~100 | Least loaded selector |
| round_robin.go | ~80 | Round robin selector |

### internal/cache (Caching)
| File | Lines | Purpose |
|------|-------|---------|
| olric.go | 670 | Distributed cache |
| ristretto.go | 280 | Local cache |
| factory.go | 71 | Cache factory |
| config.go | ~200 | Configuration types |

## Public API Surface Per Package

### internal/proxy
- `SetupRoutes(cfg, provider, key) http.Handler`
- `SetupRoutesWithProviders(cfg, provider, key, pool, providers) http.Handler`
- `NewServer(addr, handler, http2) *http.Server`
- `NewLogger(cfg) (zerolog.Logger, error)`

### internal/keypool
- `NewKeyPool(name, cfg) (*KeyPool, error)`
- `pool.GetKey(ctx) (string, error)`
- `pool.UpdateKeyFromHeaders(key, headers)`
- `pool.MarkKeyExhausted(key)`
- `pool.GetStats() PoolStats`

### internal/cache
- `New(ctx, cfg) (Cache, error)`
- `Cache` interface: Get, Set, SetWithTTL, Delete, Exists, Close
- `StatsProvider` interface: Stats()
- `Pinger` interface: Ping(ctx)
- `ClusterInfo` interface: MemberlistAddr, ClusterMembers, IsEmbedded

### internal/config
- `Load(path) (*Config, error)`
- `Config` struct with Server, Providers, Logging, Cache fields

### internal/auth
- `Authenticator` interface: Authenticate(r) error
- `NewAPIKeyAuthenticator(key) Authenticator`
- `NewChainAuthenticator(auths...) Authenticator`

---

*Structure analysis: 2026-01-22 (updated from 2026-01-20)*
