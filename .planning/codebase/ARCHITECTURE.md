# Architecture

**Analysis Date:** 2026-01-22 (Updated)
**Previous Analysis:** 2026-01-20

## Package Dependency Graph (Current Implementation)

```
                              +---------------+
                              |  cmd/cc-relay |
                              |   (CLI/Main)  |
                              +-------+-------+
                                      |
                    +-----------------+-----------------+
                    |                 |                 |
                    v                 v                 v
            +-------+-------+ +-------+-------+ +-------+-------+
            | internal/proxy| |internal/config| |internal/keypool|
            |  (HTTP/SSE)   | | (YAML loader) | | (Multi-key)   |
            +-------+-------+ +-------+-------+ +-------+-------+
                    |                 |                 |
        +-----------+-----+           |                 |
        |           |     |           v                 v
        v           v     v    +------+------+   +------+------+
+-------+--+ +------+--+ +--+--+internal/cache| |internal/rate |
|internal/ | |internal/ | |    | (Ristretto/ | |    limit     |
|   auth   | |providers | |    |   Olric)    | |(Token bucket)|
+----------+ +---------+ |    +-------------+ +-------------+
                         |
                    +----+----+
                    |internal/|
                    | version |
                    +---------+
```

**Circular Dependency Status:** None detected.

## Pattern Overview

**Overall:** Multi-layer HTTP reverse proxy with pluggable routing strategies and provider transformers.

**Key Characteristics:**
- **Adapter Pattern**: Provider implementations transform between Anthropic API format and provider-specific formats
- **Strategy Pattern**: Routing strategies (simple-shuffle, round-robin, failover, cost-based, etc.) are interchangeable implementations
- **Circuit Breaker**: Health tracking with CLOSED/OPEN/HALF-OPEN states to manage provider failures
- **Managed Key Pools**: Per-provider, per-key rate limit tracking (RPM/TPM) with distributed request scheduling
- **gRPC Management Layer**: Daemon/client separation enabling TUI and CLI tools to manage the proxy

## Layers

**Client Layer:**
- Purpose: Claude Code client connecting via standard Anthropic API
- Location: External (Claude Code at `http://localhost:8787`)
- Communicates with: HTTP Proxy Server
- Expectation: Exact Anthropic Messages API format compliance

**HTTP Proxy Layer:**
- Purpose: Accept incoming Claude Code requests and route to backends
- Location: `internal/proxy/`
- Contains: HTTP server, SSE streaming, middleware (logging, auth, metrics)
- Depends on: Router, Provider transformers, Health tracker
- Used by: Client applications (Claude Code)
- Critical: Must preserve exact Anthropic API format, tool_use_id atomicity, SSE event sequence order

**Routing & Selection Layer:**
- Purpose: Choose backend provider + API key for each request
- Location: `internal/router/`
- Contains: Strategy implementations, key pool tracking, rate limit enforcement (RPM/TPM)
- Depends on: Configuration, Health tracker
- Used by: HTTP Proxy Layer
- Pattern: Strategy interface with implementations (shuffle, round-robin, least-busy, cost-based, latency-based, failover, model-based)

**Provider Transformation Layer:**
- Purpose: Transform requests/responses between Anthropic format and provider-specific formats
- Location: `internal/providers/`
- Contains: ProviderTransformer interface, implementations for each provider (anthropic.go, zai.go, ollama.go, bedrock.go, azure.go, vertex.go)
- Depends on: Configuration (auth keys, model mappings)
- Used by: HTTP Proxy Layer before/after backend calls
- Key Abstractions: Request transformation, response parsing, authentication, health checking

**Health & Observability Layer:**
- Purpose: Track provider health, implement circuit breaker, detect and recover from failures
- Location: `internal/health/`
- Contains: Circuit breaker state machine, failure tracking, recovery probing
- Depends on: Configuration (thresholds)
- Used by: Router (for strategy decisions), Proxy (for request dispatch)
- Triggers: Rate limit errors (429), server errors (5xx), timeouts

**Configuration Layer:**
- Purpose: Load, validate, and hot-reload configuration
- Location: `internal/config/`
- Contains: Config structs, YAML/TOML parsing, environment variable expansion, file watching
- Depends on: fsnotify (file watcher)
- Used by: All layers (server, routing, providers, health)

**gRPC Management Layer:**
- Purpose: Expose management and monitoring APIs for TUI/CLI/WebUI clients
- Location: `internal/grpc/`
- Contains: gRPC service implementations (stats streaming, provider management, key management, config updates, routing control)
- Depends on: Core routing/proxy components
- Used by: External clients (TUI, CLI, WebUI)
- Definition: `relay.proto` with RelayManager service

**TUI Layer:**
- Purpose: Terminal-based management interface for operators
- Location: `ui/tui/`
- Contains: Bubble Tea application with real-time UI components
- Depends on: gRPC Management API
- Elm Architecture: Model (state) → View (render) → Update (handle events)
- Displays: Provider health, per-key usage, request logs, real-time stats

**CLI Entry Point:**
- Purpose: Command-line interface to start daemon and manage proxy
- Location: `cmd/cc-relay/` (planned)
- Subcommands: `serve`, `tui`, `status`, `config`, `provider`, `key`

## Data Flow

**Incoming Request Flow:**

1. Claude Code sends `POST /v1/messages` to `http://localhost:8787`
2. HTTP Proxy Server (`internal/proxy/`) receives request, validates auth
3. Middleware logs request (if configured)
4. Router (`internal/router/`) selects strategy and picks provider + key
5. Router checks health tracker to avoid circuit-open backends
6. Router checks rate limits on selected key (RPM/TPM)
7. Provider Transformer (`internal/providers/`) adapts request:
   - Map model name via configuration
   - Add provider-specific auth (Bedrock: SigV4, Vertex: OAuth, etc.)
   - Transform request body/headers if needed
8. Provider Transformer makes backend HTTP call
9. Provider Transformer parses response, validates format
10. HTTP Proxy Server sends back SSE stream with event sequence: message_start → content_block_start → content_block_delta → content_block_stop → message_delta → message_stop
11. Each SSE event is flushed immediately via `http.Flusher`
12. Router records completion, updates key usage counters
13. Health tracker records success or failure

**Error/Failover Flow:**

1. Backend returns 429 (rate limit), 5xx (error), or request times out
2. Health tracker increments failure counter for that provider+key
3. If failures exceed threshold, circuit breaker enters OPEN state
4. If strategy is failover, Router tries next provider in fallback chain
5. If strategy is simple-shuffle/round-robin, unhealthy providers are deprioritized
6. After cooldown period, circuit breaker enters HALF-OPEN, probes health
7. If probe succeeds, circuit breaker returns to CLOSED

**Management API Flow:**

1. TUI/CLI connects to gRPC Management API
2. TUI subscribes to `StreamStats` for real-time updates
3. TUI calls `ListProviders` to display provider list
4. Operator clicks "disable provider" → TUI calls `DisableProvider(provider_name)`
5. gRPC handler updates runtime state (provider marked disabled)
6. Router excludes disabled providers from selection
7. TUI refreshes stats stream to show updated state

**Configuration Hot-Reload Flow:**

1. Operator edits `~/.config/cc-relay/config.yaml`
2. File watcher (fsnotify) detects change
3. Config loader re-reads and validates file
4. If valid: Core components reload providers, keys, routing settings
5. If invalid: Log error, keep previous config, alert operator via TUI
6. Existing in-flight requests complete under old config
7. New requests use updated config

**State Management:**

- Per-key usage (RPM/TPM) stored in-memory in Router
- Provider health state (CLOSED/OPEN/HALF-OPEN) stored in Health tracker
- Configuration cached in memory, reloaded on file change
- Request logs held in circular buffer for TUI display
- Stats aggregated from all components and streamed via gRPC

## Key Abstractions

**ProviderTransformer Interface:**
- Purpose: Encapsulate provider-specific API transformations
- Examples: `internal/providers/anthropic.go`, `internal/providers/bedrock.go`, `internal/providers/vertex.go`
- Pattern: Each provider implements TransformRequest, TransformResponse, Authenticate, HealthCheck methods
- Benefit: New providers can be added without modifying proxy layer

**RoutingStrategy Interface:**
- Purpose: Encapsulate request routing logic
- Examples: `internal/router/strategies/shuffle.go`, `internal/router/strategies/failover.go`
- Pattern: Each strategy implements SelectProvider(req, keyPool, health) method
- Benefit: Strategies can be swapped at runtime via gRPC API

**KeyPool:**
- Purpose: Track per-key rate limits and usage
- Location: `internal/router/keypool.go`
- Pattern: Each provider has a KeyPool with array of keys; each key tracks RPM/TPM usage with sliding window
- Benefit: Distributes requests across multiple keys, maximizes throughput while respecting limits

**CircuitBreaker:**
- Purpose: Implement fault tolerance and recovery
- Location: `internal/health/circuit.go`
- Pattern: Three states (CLOSED → OPEN → HALF-OPEN → CLOSED)
- Benefit: Prevents cascading failures, automatic recovery probing

## Entry Points

**HTTP Proxy Server (`internal/proxy/server.go`):**
- Location: `http://127.0.0.1:8787` (configurable)
- Endpoint: `POST /v1/messages`
- Headers handled: `x-api-key`, `anthropic-version`, `content-type`
- Response format: Server-Sent Events (SSE) with exact Anthropic sequence
- Responsibilities: Request validation, middleware chain, response streaming, error handling
- Called by: Claude Code client

**gRPC Management Server (`internal/grpc/server.go`):**
- Location: `127.0.0.1:50051` (configurable)
- Service: RelayManager (defined in `relay.proto`)
- RPCs: StreamStats, ListProviders, AddKey, RemoveKey, SetRoutingStrategy, etc.
- Responsibilities: Stats aggregation, provider/key management, config hot-reload
- Called by: TUI, CLI, WebUI clients

**CLI Entry Point (`cmd/cc-relay/main.go`):**
- Subcommands: `serve`, `tui`, `status`, `config`, `provider`, `key`
- `serve`: Start HTTP proxy + gRPC server daemon
- `tui`: Launch TUI (connects to running daemon)
- `status`: Query daemon status
- `config reload`: Trigger config hot-reload
- Responsibilities: Command parsing, daemon startup/shutdown, signal handling

**Router Selection (`internal/router/router.go`):**
- Method: SelectProvider(request) → (provider, key)
- Input: Anthropic request, current stats, health state
- Output: Chosen provider and API key
- Responsibilities: Apply strategy, respect rate limits, check health
- Called by: HTTP Proxy on each incoming request

## Error Handling

**Strategy:** Multi-layered with fallback chains

**Patterns:**

- **Rate Limit (429):** Health tracker records, circuit opens if threshold exceeded, strategy deprioritizes provider
- **Server Error (5xx):** Treated as transient failure, circuit tracks, may trigger failover
- **Timeout:** Increases failure count, circuit tracks, failover activates if strategy supports it
- **Invalid Request:** Proxy validates against Anthropic schema before routing, returns 400 to client
- **No Healthy Provider:** If all providers unhealthy, proxy returns 503 Service Unavailable
- **Configuration Error:** Config loader validates on startup and reload, alerts via logs/TUI
- **Provider Not Found:** Router returns error, proxy returns 500

## Cross-Cutting Concerns

**Logging:**
- Approach: Structured logging (planned: `log/slog` or `zerolog`)
- Logs: Request/response pairs, provider selections, errors, config reloads
- Format: JSON (production) or text (development)
- Output: File + stdout

**Validation:**
- Approach: Schema validation for Anthropic API requests
- Input validation: Check `model`, `messages`, `max_tokens` before routing
- Rate limit validation: Enforce RPM/TPM per key
- Config validation: Validate YAML schema, check provider settings

**Authentication:**
- Approach: Provider-specific authentication via ProviderTransformer
- Anthropic: `x-api-key` header passed through
- Bedrock: AWS SigV4 signing
- Vertex: Google OAuth bearer token
- Z.AI: Custom auth token mapping
- Azure: x-api-key header (mapped to deployment)

**Metrics:**
- Planned: Prometheus endpoint with request counts, latencies, error rates
- Per-provider: requests/sec, errors/sec, avg latency
- Per-key: usage (RPM/TPM)
- System: queue depth, goroutine count

**Request Tracing:**
- Request ID propagation through gRPC and HTTP headers
- Traces visible in TUI request log stream
- Useful for debugging routing decisions

## Singleton vs Request-Scoped Services

### Singletons (Application Lifetime)
| Service | Location | Initialization |
|---------|----------|----------------|
| Logger | proxy/logger.go | serve.go startup |
| KeyPool | keypool/pool.go | serve.go startup |
| Cache | cache/factory.go | (future) serve.go |
| Provider instances | providers/*.go | serve.go startup |
| HTTP Server | proxy/server.go | serve.go startup |

### Request-Scoped
| Component | Location | Per-Request |
|-----------|----------|-------------|
| Request context | proxy/handler.go | New per request |
| Request ID | proxy/middleware.go | Generated/extracted |
| Selected API key | keypool/pool.go | Selected per request |
| Response writer wrapper | proxy/middleware.go | Created per request |

## Concurrency Patterns

### Thread-Safe Components
| Component | Protection | Pattern |
|-----------|------------|---------|
| KeyPool | sync.RWMutex | Read-heavy optimization |
| KeyMetadata | sync.RWMutex | Per-key locking |
| RateLimiter | golang.org/x/time/rate | Atomic operations |
| Cache (Ristretto) | Internal sync | Built-in thread safety |
| Cache (Olric) | sync.RWMutex + atomic.Bool | Double-check locking |

### Graceful Shutdown
1. Signal handler (SIGINT/SIGTERM)
2. 30-second timeout context
3. HTTP server Shutdown()
4. Cache Close() (Olric leave timeout)

## Test Coverage Summary

| Package | Coverage | Status |
|---------|----------|--------|
| cmd/cc-relay | 13.6% | **Needs improvement** |
| internal/auth | ~60% | Adequate |
| internal/cache | 77.3% | Good, minor gaps |
| internal/config | ~80% | Good |
| internal/keypool | ~85% | Good |
| internal/providers | ~70% | Adequate |
| internal/proxy | ~75% | Good |
| internal/ratelimit | ~90% | Excellent |

## Refactoring Targets

### High Cognitive Complexity Functions
| Function | Location | Issue |
|----------|----------|-------|
| runServe | cmd/cc-relay/serve.go | Multiple responsibilities (marked nolint) |
| buildOlricConfig | internal/cache/olric.go | Many conditionals |

### Potential God Objects
| Struct | Location | Responsibilities |
|--------|----------|------------------|
| Handler | proxy/handler.go | Request handling, key selection, error formatting |
| KeyPool | keypool/pool.go | Key management, selection, rate limiting |

### Code Smells
| Issue | Location | Suggestion |
|-------|----------|------------|
| Duplicated findConfigFile | serve.go, status.go, config.go | Extract to shared helper |
| Magic strings | Various | Define constants for header names |
| Error wrapping inconsistency | Various | Standardize error patterns |

### Samber Library Opportunities
| Current Pattern | Samber Replacement | Benefit |
|-----------------|-------------------|---------|
| Manual slice filtering | lo.Filter | Cleaner, tested |
| Manual map operations | lo.Map, lo.Reduce | Functional style |
| Error handling boilerplate | mo.Result/mo.Option | Monadic composition |
| Service initialization | do.Provide/do.Invoke | DI container |
| Readonly collections | ro.Slice/ro.Map | Immutability |

## External Dependencies

### Production
- `github.com/spf13/cobra` - CLI framework
- `github.com/rs/zerolog` - Structured logging
- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/google/uuid` - Request ID generation
- `golang.org/x/time/rate` - Token bucket rate limiting
- `github.com/dgraph-io/ristretto/v2` - In-memory cache
- `github.com/olric-data/olric` - Distributed cache
- `golang.org/x/net/http2` - HTTP/2 support

### Samber Libraries (Planned)
- `github.com/samber/lo` - Functional programming utilities
- `github.com/samber/do/v2` - Dependency injection
- `github.com/samber/mo` - Monadic error handling
- `github.com/samber/ro` - Readonly collections

---

*Architecture analysis: 2026-01-22 (updated from 2026-01-20)*
