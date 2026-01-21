# Architecture Research: Multi-Provider LLM Proxy

**Domain:** HTTP Reverse Proxy / LLM Gateway
**Researched:** 2026-01-20
**Confidence:** HIGH

## Standard Architecture

### System Overview

Multi-provider LLM proxies follow a layered gateway architecture with routing intelligence:

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENT LAYER                            │
│               (Claude Code, OpenAI SDK clients)                 │
└────────────────────────┬────────────────────────────────────────┘
                         │ Standardized API (e.g., Anthropic format)
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                      API GATEWAY LAYER                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────┐  │
│  │ Auth/      │  │ Request    │  │ Streaming  │  │ Metrics  │  │
│  │ Validation │  │ Transform  │  │ SSE/HTTP2  │  │ Logging  │  │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘  └─────┬────┘  │
│        └────────────────┴────────────────┴──────────────┘       │
├─────────────────────────────────────────────────────────────────┤
│                      ROUTING LAYER                              │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────┐  │
│  │ Routing Strategy │  │ Key Pool Manager │  │ Health       │  │
│  │ (shuffle, cost,  │  │ (RPM/TPM limits) │  │ Tracker      │  │
│  │  latency, etc.)  │  │                  │  │              │  │
│  └────────┬─────────┘  └────────┬─────────┘  └──────┬───────┘  │
│           └────────────┬─────────┴────────────────────┘         │
│                        │ Select: Provider + API Key             │
├────────────────────────┴────────────────────────────────────────┤
│                   PROVIDER ADAPTER LAYER                        │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ Provider │  │ Provider │  │ Provider │  │ Provider │        │
│  │ Interface│  │ Interface│  │ Interface│  │ Interface│        │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘        │
│       │             │             │             │              │
├───────┴─────────────┴─────────────┴─────────────┴──────────────┤
│                  BACKEND PROVIDER LAYER                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │Anthropic │  │  Z.AI    │  │  Ollama  │  │ Bedrock  │        │
│  │ (native) │  │  (compat)│  │  (local) │  │ (cloud)  │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└─────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **API Gateway** | Accept client requests, validate auth, normalize format | HTTP server (Go: `net/http`, Python: FastAPI) |
| **Request Transform** | Convert standardized API to provider-specific format | Middleware/decorator pattern |
| **Streaming Handler** | Handle SSE/HTTP2 streaming with proper buffering | Go: `http.Flusher`, Python: async generators |
| **Router** | Select backend provider + API key based on strategy | Strategy pattern with pluggable implementations |
| **Key Pool Manager** | Track per-key rate limits (RPM/TPM), distribute load | In-memory state with TTL windows |
| **Health Tracker** | Monitor provider health, circuit breaker | State machine (CLOSED/OPEN/HALF-OPEN) |
| **Provider Adapter** | Transform requests/responses for specific provider API | Adapter pattern, one per provider |
| **Metrics/Logging** | Observability, cost tracking, audit trail | Prometheus, structured logging |

## Recommended Project Structure

### Go Implementation (Recommended for Performance)

```
cc-relay/
├── cmd/
│   └── cc-relay/
│       └── main.go              # CLI entry point, command routing
├── internal/
│   ├── proxy/                   # API Gateway Layer
│   │   ├── server.go            # HTTP server setup
│   │   ├── handler.go           # Request handler (/v1/messages)
│   │   ├── sse.go               # SSE streaming handler
│   │   ├── middleware.go        # Auth, logging, metrics middleware
│   │   └── transform.go         # Request/response normalization
│   ├── router/                  # Routing Layer
│   │   ├── router.go            # Routing interface
│   │   ├── strategies/          # Strategy implementations
│   │   │   ├── shuffle.go       # Random weighted selection
│   │   │   ├── roundrobin.go    # Sequential distribution
│   │   │   ├── leastbusy.go     # Least in-flight requests
│   │   │   ├── costbased.go     # Cost optimization routing
│   │   │   ├── latency.go       # Latency-based selection
│   │   │   └── failover.go      # Primary → fallback chain
│   │   └── keypool.go           # API key pool management
│   ├── providers/               # Provider Adapter Layer
│   │   ├── provider.go          # Provider interface definition
│   │   ├── anthropic.go         # Direct Anthropic API
│   │   ├── zai.go               # Z.AI / Zhipu GLM
│   │   ├── ollama.go            # Local Ollama
│   │   ├── bedrock.go           # AWS Bedrock (SigV4 auth)
│   │   ├── azure.go             # Azure AI Foundry
│   │   └── vertex.go            # Google Vertex AI (OAuth)
│   ├── health/                  # Health Tracking
│   │   ├── tracker.go           # Per-backend health tracking
│   │   └── circuit.go           # Circuit breaker implementation
│   ├── config/                  # Configuration
│   │   ├── config.go            # Config structs
│   │   ├── loader.go            # YAML/TOML parsing
│   │   └── watcher.go           # Hot-reload (fsnotify)
│   └── grpc/                    # Management API (optional)
│       ├── server.go            # gRPC server
│       └── handlers.go          # gRPC service implementations
├── ui/                          # Optional management interfaces
│   ├── tui/                     # Terminal UI (Bubble Tea)
│   └── web/                     # Web UI (grpc-web)
└── proto/
    └── relay.proto              # gRPC service definitions
```

### Structure Rationale

- **`internal/proxy/`:** Isolates HTTP server concerns (SSE, middleware, auth). This is the "front door" that clients connect to.
- **`internal/router/`:** Encapsulates routing logic. Strategy pattern allows swapping algorithms at runtime without changing other layers.
- **`internal/providers/`:** Each provider is a separate adapter implementing the same interface. Easy to add new providers without modifying routing or proxy layers.
- **`internal/health/`:** Circuit breaker is cross-cutting but separate from routing. Routing consults health before selecting a backend.
- **`internal/config/`:** Centralized configuration with hot-reload. Separates config parsing from business logic.

## Architectural Patterns

### Pattern 1: Reverse Proxy with Transformation Pipeline

**What:** Use Go's `net/http/httputil.ReverseProxy` as the base, but inject custom request/response transformation logic for each provider.

**When to use:** When backends are mostly HTTP-compatible but differ in auth, headers, or request body format.

**Trade-offs:**

- **Pros:** Leverages battle-tested proxy code, handles connection pooling, HTTP/2, etc.
- **Cons:** Transformation overhead on hot path, streaming requires careful buffering configuration.

**Example (Go):**

```go
// Simplified provider transformer interface
type ProviderTransformer interface {
    TransformRequest(req *AnthropicRequest) (*http.Request, error)
    TransformResponse(resp *http.Response) (*AnthropicResponse, error)
    Authenticate(req *http.Request) error
}

// In proxy handler
func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Parse incoming Anthropic-format request
    var anthropicReq AnthropicRequest
    json.NewDecoder(r.Body).Decode(&anthropicReq)

    // 2. Router selects provider + key
    provider, key := p.router.SelectBackend(anthropicReq)

    // 3. Transform request for selected provider
    backendReq, _ := provider.TransformRequest(&anthropicReq)
    provider.Authenticate(backendReq, key)

    // 4. Forward to backend (with SSE handling if streaming)
    p.forwardToBackend(w, backendReq, provider)
}
```

### Pattern 2: Circuit Breaker with State Machine

**What:** Implement circuit breaker as a finite state machine (CLOSED → OPEN → HALF-OPEN) that wraps provider calls.

**When to use:** Always, for production LLM proxies. Prevents cascading failures when a provider goes down.

**Trade-offs:**

- **Pros:** Automatic failure isolation, fast fail during outages, controlled recovery probing.
- **Cons:** Requires tuning thresholds (failure rate, timeout), adds latency on state checks.

**Example (Go):**

```go
type CircuitState int
const (
    CLOSED CircuitState = iota  // Normal operation
    OPEN                        // Blocking all requests
    HALF_OPEN                   // Testing recovery
)

type CircuitBreaker struct {
    state         CircuitState
    failures      int
    threshold     int
    timeout       time.Duration
    lastFailTime  time.Time
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    switch cb.state {
    case OPEN:
        if time.Since(cb.lastFailTime) > cb.timeout {
            cb.state = HALF_OPEN
        } else {
            return ErrCircuitOpen
        }
    case HALF_OPEN:
        // Allow one probe request
    }

    err := fn()
    if err != nil {
        cb.recordFailure()
        return err
    }
    cb.recordSuccess()
    return nil
}
```

### Pattern 3: API Key Pool with Rate Limit Tracking

**What:** Maintain an in-memory pool of API keys per provider, tracking RPM (requests per minute) and TPM (tokens per minute) for each key using sliding windows.

**When to use:** When rate limits are enforced per-key and you want to maximize throughput by distributing across multiple keys.

**Trade-offs:**

- **Pros:** Maximizes aggregate throughput, prevents hitting rate limits.
- **Cons:** Requires accurate tracking, potential race conditions in concurrent routing, keys may desync if limits change.

**Example (Go):**

```go
type KeyPool struct {
    keys     []*APIKey
    windows  map[string]*RateLimitWindow  // key ID → window
}

type RateLimitWindow struct {
    requestTimes []time.Time  // Timestamps of recent requests
    tokenCounts  []int         // Token counts of recent requests
    rpmLimit     int
    tpmLimit     int
}

func (kp *KeyPool) SelectKey() (*APIKey, error) {
    now := time.Now()
    for _, key := range kp.keys {
        window := kp.windows[key.ID]

        // Evict old entries from sliding window
        window.requestTimes = filterRecent(window.requestTimes, now.Add(-1*time.Minute))

        // Check if key is under limits
        if len(window.requestTimes) < window.rpmLimit {
            totalTokens := sumTokens(window.tokenCounts)
            if totalTokens < window.tpmLimit {
                return key, nil
            }
        }
    }
    return nil, ErrAllKeysExhausted
}
```

### Pattern 4: SSE Streaming with Immediate Flush

**What:** Configure HTTP reverse proxy to flush SSE events immediately, avoiding buffering that causes latency spikes.

**When to use:** Always for streaming LLM APIs. Buffering breaks the real-time user experience.

**Trade-offs:**

- **Pros:** Real-time event delivery, matches Anthropic API behavior exactly.
- **Cons:** Higher syscall overhead (more flushes), requires HTTP/1.1 or HTTP/2.

**Example (Go):**

```go
// Use httputil.ReverseProxy with FlushInterval
proxy := &httputil.ReverseProxy{
    Director: func(req *http.Request) {
        // Modify request for backend
    },
    FlushInterval: -1,  // Immediate flush (no buffering)
}

// In SSE handler
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache, no-transform")
    w.Header().Set("X-Accel-Buffering", "no")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
        return
    }

    // Forward events from backend
    for event := range eventStream {
        fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
        flusher.Flush()  // Immediate send
    }
}
```

## Data Flow

### Request Flow (Non-Streaming)

```
[Claude Code Client]
    ↓ POST /v1/messages (Anthropic format)
[API Gateway] → Auth Validation → Parse Request
    ↓
[Router] → Select Strategy (shuffle, cost, latency, etc.)
    ↓
[Key Pool Manager] → Find available key (check RPM/TPM limits)
    ↓
[Health Tracker] → Check provider health (circuit breaker)
    ↓
[Provider Adapter] → Transform request (Bedrock needs model in URL, etc.)
    ↓ HTTP request to backend
[Backend Provider API] → Process request
    ↓ HTTP response
[Provider Adapter] → Transform response (normalize to Anthropic format)
    ↓
[API Gateway] → Log metrics, update rate limit counters
    ↓ JSON response
[Claude Code Client]
```

### Streaming Flow (SSE)

```
[Claude Code Client]
    ↓ POST /v1/messages with "stream": true
[API Gateway] → Validate, set SSE headers
    ↓
[Router + Key Pool + Health Tracker] → Select backend + key
    ↓
[Provider Adapter] → Transform request, maintain streaming connection
    ↓ Streaming HTTP connection
[Backend Provider API] → Stream SSE events
    ↓ event: message_start, content_block_delta, ...
[Provider Adapter] → Transform events to Anthropic format
    ↓ Forward each event
[API Gateway] → Flush immediately (http.Flusher)
    ↓ SSE events
[Claude Code Client] → Process events in real-time
```

**Critical:** Event sequence must match Anthropic API exactly:

1. `message_start`
2. `content_block_start`
3. `content_block_delta` (multiple)
4. `content_block_stop`
5. `message_delta`
6. `message_stop`

### State Management

```
┌─────────────────────────────────────────────────────┐
│             In-Memory State (per instance)          │
├─────────────────────────────────────────────────────┤
│  ┌────────────────┐  ┌────────────────┐             │
│  │ Key Pool State │  │ Health Tracker │             │
│  │ - RPM counters │  │ - Circuit state│             │
│  │ - TPM counters │  │ - Failure count│             │
│  │ - Sliding win  │  │ - Last failure │             │
│  └────────────────┘  └────────────────┘             │
└─────────────────────────────────────────────────────┘
         ↓ (optional, for multi-instance)
┌─────────────────────────────────────────────────────┐
│            Shared State (Redis/etcd)                │
│  - Global rate limit tracking                       │
│  - Distributed circuit breaker coordination         │
└─────────────────────────────────────────────────────┘
```

**Note:** For single-instance deployments (typical for personal use), in-memory state is sufficient. Multi-instance requires shared state store.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| **Single user** | Single instance, in-memory state, simple-shuffle routing. No shared state needed. |
| **Team (10-50 users)** | Still single instance, optimize for connection pooling. Add Prometheus metrics. Consider failover routing. |
| **Organization (100+ users)** | Multiple instances behind load balancer. Redis for shared rate limit state. Horizontal scaling of proxy instances. |

### Scaling Priorities

1. **First bottleneck:** Rate limits on Anthropic API
   - **Fix:** Add more API keys, use key pooling

2. **Second bottleneck:** Proxy instance CPU (request transformation overhead)
   - **Fix:** Optimize transformations (avoid JSON re-parsing), add instance replicas behind load balancer

3. **Third bottleneck:** Streaming connection limits (file descriptor limits)
   - **Fix:** Increase OS limits (`ulimit`), use connection pooling to backends

**Performance Targets (from research):**

- **LiteLLM:** 8ms P95 latency (Python)
- **Bifrost:** 11μs overhead at 5,000 RPS (Go)
- **Target for cc-relay:** <5ms overhead for non-streaming, <50ms P95 for streaming start

## Anti-Patterns

### Anti-Pattern 1: Buffering SSE Events

**What people do:** Use default HTTP buffering settings, causing SSE events to accumulate before being sent.

**Why it's wrong:** Users see laggy responses, breaks real-time UX. Claude Code expects immediate streaming.

**Do this instead:** Set `FlushInterval: -1` on `httputil.ReverseProxy` and explicitly call `Flusher.Flush()` after each SSE event. Set `X-Accel-Buffering: no` header for nginx proxies.

### Anti-Pattern 2: Hardcoding Provider Transformations in Router

**What people do:** Put provider-specific logic (auth, URL construction) directly in the routing layer.

**Why it's wrong:** Violates separation of concerns. Makes adding new providers require changes to routing code.

**Do this instead:** Use the adapter pattern. Define a `Provider` interface with `TransformRequest`, `TransformResponse`, `Authenticate` methods. Router only selects which provider + key, adapter handles all transformations.

### Anti-Pattern 3: Ignoring Circuit Breaker Recovery

**What people do:** Implement circuit breaker with CLOSED/OPEN states, but forget HALF-OPEN recovery probing.

**Why it's wrong:** Once a provider fails and circuit opens, it never recovers automatically. Manual intervention required.

**Do this instead:** Add HALF-OPEN state that allows a single probe request after cooldown period. On success, transition back to CLOSED. On failure, return to OPEN.

### Anti-Pattern 4: Global Rate Limit Tracking Without Sliding Windows

**What people do:** Track rate limits as "requests in current minute" using fixed 60-second buckets.

**Why it's wrong:** Allows burst at bucket boundaries (e.g., 60 requests at 0:59, 60 more at 1:00 = 120 in 1 second).

**Do this instead:** Use sliding windows that track request timestamps and evict entries older than 60 seconds. Provides smooth rate limiting.

### Anti-Pattern 5: Trusting External Provider Error Messages Without Parsing

**What people do:** Forward provider error responses directly to client without inspection.

**Why it's wrong:** Provider-specific error formats leak to client. Some providers expose internal details (API keys in logs, internal IPs, etc.).

**Do this instead:** Parse provider errors, normalize to Anthropic error format. Sanitize error messages before forwarding.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| **Anthropic API** | Direct HTTP with `x-api-key` header | Native format, no transformation needed |
| **Z.AI** | HTTP with model mapping (GLM-4.7 → claude-sonnet-4-5) | Fully Anthropic-compatible |
| **Ollama** | Local HTTP, no auth | No prompt caching, images must be base64 |
| **AWS Bedrock** | HTTPS with SigV4 signing or Bearer Token | Model in URL path, `anthropic_version: bedrock-2023-05-31` |
| **Azure AI Foundry** | HTTPS with `x-api-key` or Entra ID token | Deployment names as model IDs |
| **Google Vertex AI** | HTTPS with OAuth bearer token | Model in URL path, `anthropic_version: vertex-2023-10-16` |
| **Prometheus** | Pull model, expose `/metrics` endpoint | For observability |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| **API Gateway ↔ Router** | Direct function calls | Synchronous, router returns selected backend |
| **Router ↔ Key Pool** | Direct function calls | Lock-free design preferred (use channels or atomic ops) |
| **Router ↔ Health Tracker** | Query before selection | Health tracker maintains circuit breaker state |
| **Provider Adapter ↔ Backend** | HTTP client (`net/http`) | Connection pooling, configurable timeouts |
| **TUI/CLI ↔ Daemon** | gRPC | Management API for stats, config updates |

## Build Order and Dependencies

Recommended implementation order to maintain working system at each step:

### Phase 1: Core Proxy (MVP)

1. **HTTP Server** (`internal/proxy/server.go`)
   - Basic `/v1/messages` endpoint
   - No routing yet, hardcode Anthropic provider

2. **Provider Interface** (`internal/providers/provider.go`)
   - Define interface

3. **Anthropic Provider** (`internal/providers/anthropic.go`)
   - Passthrough implementation (no transformation)

4. **Config Loader** (`internal/config/`)
   - YAML parsing, basic validation

**Dependency:** None (standalone proxy)
**Deliverable:** Working proxy for single Anthropic key

### Phase 2: Routing + Multi-Key

1. **Router Interface** (`internal/router/router.go`)
2. **Simple Shuffle Strategy** (`internal/router/strategies/shuffle.go`)
3. **Key Pool Manager** (`internal/router/keypool.go`)
   - In-memory rate limit tracking

**Dependency:** Phase 1 proxy
**Deliverable:** Multi-key pooling with random selection

### Phase 3: Health + Failover

1. **Circuit Breaker** (`internal/health/circuit.go`)
2. **Health Tracker** (`internal/health/tracker.go`)
3. **Failover Strategy** (`internal/router/strategies/failover.go`)

**Dependency:** Phase 2 routing
**Deliverable:** Automatic failover when provider fails

### Phase 4: Additional Providers

1. **Z.AI Provider** (`internal/providers/zai.go`)
   - Model mapping

2. **Ollama Provider** (`internal/providers/ollama.go`)
   - Local endpoint, no auth

**Dependency:** Phase 1 provider interface
**Deliverable:** Multi-provider support

### Phase 5: Advanced Routing

1. **Cost-Based Strategy** (`internal/router/strategies/costbased.go`)
2. **Latency-Based Strategy** (`internal/router/strategies/latency.go`)
3. **Model-Based Strategy** (`internal/router/strategies/modelbased.go`)

**Dependency:** Phase 2 routing framework
**Deliverable:** Intelligent routing strategies

### Phase 6: Cloud Providers

1. **AWS Bedrock** (`internal/providers/bedrock.go`)
   - SigV4 signing

2. **Azure AI Foundry** (`internal/providers/azure.go`)
3. **Google Vertex AI** (`internal/providers/vertex.go`)
   - OAuth token generation

**Dependency:** Phase 1 provider interface
**Deliverable:** Enterprise cloud provider support

### Phase 7: Management Interface

1. **gRPC Server** (`internal/grpc/`)
   - Stats streaming, provider/key management

2. **TUI** (`ui/tui/`)
   - Bubble Tea interface

**Dependency:** All prior phases (needs stats from router, health tracker)
**Deliverable:** Real-time management interface

## Sources

**LLM Gateway Architecture:**

- [How API Gateways Proxy LLM Requests - API7.ai](https://api7.ai/learning-center/api-gateway-guide/api-gateway-proxy-llm-requests)
- [Multi-provider LLM orchestration in production: A 2026 Guide - DEV](https://dev.to/ash_dubai/multi-provider-llm-orchestration-in-production-a-2026-guide-1g10)
- [Building the AI Control Plane - Medium](https://medium.com/@adnanmasood/primer-on-ai-gateways-llm-proxies-routers-definition-usage-and-purpose-9b714d544f8c)
- [LLM Orchestration in 2026 - AIMultiple](https://research.aimultiple.com/llm-orchestration/)

**Go Reverse Proxy & SSE:**

- [Building an SSE Proxy in Go - Medium](https://medium.com/@sercan.celenk/building-an-sse-proxy-in-go-streaming-and-forwarding-server-sent-events-1c951d3acd70)
- [Go httputil.ReverseProxy SSE issues - GitHub](https://github.com/golang/go/issues/27816)
- [Server-Sent Events: A Comprehensive Guide - Medium](https://medium.com/@moali314/server-sent-events-a-comprehensive-guide-e4b15d147576)

**Circuit Breaker Pattern:**

- [Circuit Breaker Patterns in Go Microservices - DEV](https://dev.to/serifcolakel/circuit-breaker-patterns-in-go-microservices-n3)
- [How to Implement Circuit Breakers in Go with sony/gobreaker - OneUptime](https://oneuptime.com/blog/post/2026-01-07-go-circuit-breaker/view)
- [Circuit Breaker Pattern in Microservices - GeeksforGeeks](https://www.geeksforgeeks.org/system-design/what-is-circuit-breaker-pattern-in-microservices/)

**Rate Limiting:**

- [Building a Lightweight Go API Gateway - Leapcell](https://leapcell.io/blog/building-a-lightweight-go-api-gateway-for-authentication-rate-limiting-and-routing)
- [How to Implement Rate Limiting in Go Without External Services - OneUptime](https://oneuptime.com/blog/post/2026-01-07-go-rate-limiting/view)

**API Key Management:**

- [Building a Resilient API Key Pool System - DEV](https://dev.to/diandiancya/building-a-resilient-api-key-pool-system-with-health-checks-and-multi-tier-degradation-3ba)

**Anthropic API:**

- [Streaming Messages - Claude Docs](https://docs.anthropic.com/en/api/messages-streaming)

**Reference Implementations:**

- [LiteLLM - GitHub](https://github.com/BerriAI/litellm) (Python, 8ms P95)
- [Bifrost by Maxim AI](https://www.getmaxim.ai/blog/bifrost-a-drop-in-llm-proxy-40x-faster-than-litellm/) (Go, 11μs overhead)

---
*Architecture research for: Multi-Provider LLM Proxy (cc-relay)*
*Researched: 2026-01-20*
*Confidence: HIGH (verified with official docs, recent 2026 sources, Go-specific patterns)*
