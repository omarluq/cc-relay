# Codebase Concerns

**Analysis Date:** 2026-01-20

## Pre-Implementation Status

This project is in **specification phase** with no implementation code yet. The concerns below identify architectural risks, design decisions that need careful execution, and potential pitfalls discovered during specification.

---

## Critical API Compatibility Risks

**SSE Streaming Event Order:**
- **Risk:** Anthropic's SSE event sequence is strict. Out-of-order events break Claude Code's parallel tool call handling
- **Files:** `internal/proxy/sse.go` (planned)
- **Current mitigation:** Specification documents exact event order in SPEC.md
- **Implementation requirements:**
  - Must use `http.Flusher` to flush each event immediately (not batch)
  - Cannot reorder events received from providers
  - Must handle ping events correctly
  - Test with real Claude Code to verify

**Tool Use ID Preservation:**
- **Risk:** If `tool_use_id` is modified/lost during transformation, Claude Code's tool tracking breaks silently
- **Files:** `internal/providers/*.go` (all provider implementations), `internal/proxy/server.go`
- **Current mitigation:** CLAUDE.md explicitly notes "preserve `tool_use_id` for Claude Code's parallel tool calls"
- **Implementation requirements:**
  - Every provider transformer must pass through `tool_use_id` unchanged
  - Extended thinking blocks and multiple tool blocks must remain atomic
  - Add unit tests specifically for `tool_use_id` preservation across all providers

**Provider Model Mapping Inconsistencies:**
- **Risk:** Model names vary across providers (Anthropic uses `claude-sonnet-4-5-20250929`, Z.AI uses `GLM-4.7`, Bedrock uses `anthropic.claude-sonnet-4-5-20250929-v1:0`)
- **Files:** `example.yaml`, `internal/providers/*.go`, `internal/router/router.go`
- **Current mitigation:** Model mapping config in YAML
- **Implementation requirements:**
  - Map at routing layer before calling provider
  - Handle unmapped models gracefully (fail fast with clear error)
  - Track which models each provider supports
  - Add validation in config loader to catch missing mappings

---

## Multi-Provider Request Transformation Complexity

**Provider-Specific Auth Requirements:**
- **Risk:** Each provider has different auth methods. Incorrect auth doesn't fail until request hits provider backend
- **Files:** `internal/providers/bedrock.go`, `internal/providers/azure.go`, `internal/providers/vertex.go`
- **Transformations needed:**
  - **Bedrock:** AWS SigV4 signing (requires AWS SDK), Bearer token generation
  - **Azure:** `x-api-key` header placement, Entra ID token flow
  - **Vertex:** Google OAuth token, specific header format
- **Implementation requirements:**
  - Extract auth logic to separate methods per provider
  - Use dependency injection for auth clients (AWS SDK, Google Auth lib)
  - Add comprehensive error messages for auth failures
  - Test auth against sandbox/test endpoints before production use

**Request Body Transformation Pitfalls:**
- **Risk:** Some providers require model in URL instead of body (Bedrock, Vertex). Incorrect transformation produces wrong API calls
- **Files:** `internal/providers/bedrock.go`, `internal/providers/vertex.go`, `internal/proxy/server.go`
- **Current mitigation:** SPEC.md documents provider-specific requirements
- **Implementation requirements:**
  - Bedrock: Move model to URL path format `bedrock-runtime.{region}.amazonaws.com/model/...`
  - Vertex: Move model to URL path format `.../models/{model}:rawPredict`
  - Add integration tests with mock providers for each transformation

**Response Transformation Edge Cases:**
- **Risk:** Streaming responses may be partially consumed. If transformation fails mid-stream, client gets broken stream
- **Files:** `internal/proxy/server.go`, `internal/providers/*.go` (TransformResponse)
- **Implementation requirements:**
  - Never buffer entire response in memory (use streaming throughout)
  - Handle provider errors that appear mid-stream (not in HTTP status code)
  - Add recovery mechanisms for stream corruption
  - Test with intentional provider failures

---

## Rate Limiting and Key Pool Management

**RPM/TPM Tracking Complexity:**
- **Risk:** Rate limits reset on provider-specific schedules. Incorrect tracking allows quota violations or resource starvation
- **Files:** `internal/router/keypool.go`, `internal/router/strategies/*.go`
- **Current mitigation:** Configuration defines RPM/TPM limits
- **Implementation requirements:**
  - Track usage per key with explicit reset timestamps
  - Different providers have different reset periods (Anthropic: minute boundaries)
  - Implement token counting: tokens may be estimated pre-request or confirmed post-response
  - Handle over-limit errors (429) with backoff
  - Add metrics for key pool saturation

**Key Selection Under Load:**
- **Risk:** Router selects key when all are near limit. Key hits limit mid-request → 429 → retry chaos
- **Files:** `internal/router/strategies/*.go` (shuffle, round-robin, failover, cost-based, latency-based, model-based)
- **Implementation requirements:**
  - Select key with lowest usage ratio, not just any available key
  - Implement predictive selection: account for request size before committing key
  - Graceful degradation when all keys limited
  - Add circuit breaker per key (not just per provider)

**Multi-Key Failover Logic:**
- **Risk:** If primary key fails, should failover to alternate key on same provider or to different provider?
- **Current mitigation:** Not explicitly defined in spec
- **Implementation requirements:**
  - Define clear failover hierarchy (primary key → alternate keys on provider → fallback provider)
  - Retry with alternate key if 429 or auth error
  - Don't retry non-transient errors (invalid model, malformed request)

---

## Routing Strategy Correctness

**Simple-Shuffle Weighted Selection:**
- **Risk:** "Weighted random based on available capacity" is vague. Wrong implementation causes request concentration on few keys
- **Files:** `internal/router/strategies/shuffle.go`
- **Implementation requirements:**
  - Define "available capacity" precisely: (rpm_limit - rpm_used) × (tpm_limit - tpm_used)
  - Use weighted random selection (e.g., per key: weight = available_capacity / total_capacity)
  - Re-evaluate weights per request (dynamic, not static)
  - Test distribution with histogram to verify actual randomness

**Least-Busy Strategy Fairness:**
- **Risk:** Always selecting provider with fewest in-flight requests can starve slower backends
- **Files:** `internal/router/strategies/leastbusy.go`
- **Implementation requirements:**
  - Balance in-flight count with latency estimate (account for slow backends)
  - Prevent permanent starvation of slower providers
  - Add stickiness: don't immediately switch if in-flight count is close

**Cost-Based Routing Thresholds:**
- **Risk:** Token threshold for cost-based routing may be too low/high. Ineffective cost optimization
- **Files:** `internal/router/strategies/costbased.go`, `example.yaml`
- **Current configuration:** `threshold_tokens: 1000` (default) - needs tuning
- **Implementation requirements:**
  - Default threshold should be tunable per deployment
  - Consider latency trade-offs: cheaper provider might add 500ms latency
  - Only use cost-based routing when response time requirements permit
  - Add metrics to track cost vs. latency trade-offs

**Model-Based Routing Correctness:**
- **Risk:** Prefix matching might incorrectly route similar models (e.g., `claude-*` pattern matches both sonnet and haiku)
- **Files:** `internal/router/strategies/modelbased.go`
- **Implementation requirements:**
  - Use exact model version matching, not prefix matching
  - Fall back gracefully if model not supported by selected provider
  - Log routing decision for debugging
  - Add provider-specific model availability checks

---

## Circuit Breaker Implementation

**State Transition Correctness:**
- **Risk:** Incorrect CLOSED → OPEN → HALF_OPEN → CLOSED transitions can cause request blackholes
- **Files:** `internal/health/circuit.go`
- **Implementation requirements:**
  - Define failure triggers clearly: 429, 5xx, timeout
  - Cooldown period (60s default) before HALF_OPEN probe
  - HALF_OPEN: allow exactly one probe request, not bulk requests
  - Track consecutive failures per key, reset counter on success
  - Add metrics for state transitions (helps debugging)

**False Positive Circuit Opens:**
- **Risk:** Single 429 shouldn't open circuit. Current spec says "3 rate limit errors" but concurrency complicates counting
- **Files:** `internal/health/circuit.go`, configuration in `example.yaml`
- **Current settings:**
  ```yaml
  triggers:
    rate_limit_errors: 3      # 429 responses
    timeout_errors: 2         # Request timeouts
    server_errors: 3          # 5xx responses
  ```
- **Implementation requirements:**
  - Count failures within time window (e.g., last 60 seconds), not absolute count
  - 429s are rate limits, not provider failures - should not open circuit immediately
  - Separate circuit breaker per key (key-level) and per provider (provider-level)
  - Add backoff jitter for probes to avoid thundering herd

---

## Configuration Hot-Reload Risks

**Race Conditions During Reload:**
- **Risk:** In-flight requests using old config while new config is applied
- **Files:** `internal/config/watcher.go`, `internal/proxy/server.go`
- **Implementation requirements:**
  - Use atomic config updates (not field-by-field mutations)
  - In-flight requests should use config snapshot captured at request start
  - Track active requests per config version, drain before removing old keys
  - Add config version tracking to logs for debugging

**Removing API Keys Mid-Stream:**
- **Risk:** If active key is removed in config reload, in-flight request loses auth
- **Files:** `internal/config/watcher.go`, `internal/router/keypool.go`
- **Implementation requirements:**
  - Drain active requests before removing keys
  - Add grace period: give in-flight requests 5 seconds before removing key
  - Log when keys are removed and how many active requests used them
  - Fail new requests immediately if provider disabled

**Invalid Config Reload:**
- **Risk:** Invalid config reload blocks daemon startup. Hot reload can't recover
- **Files:** `internal/config/loader.go`
- **Implementation requirements:**
  - Validate config before applying
  - On validation failure, keep previous config (don't partially apply)
  - Add rollback mechanism for failed reloads
  - Maintain config version history (last 5 configs)

---

## Concurrency and Resource Management

**Unbounded Request Queuing:**
- **Risk:** If `max_concurrent: 0` (unlimited), daemon can be OOMed by burst of long requests
- **Files:** `internal/proxy/server.go`, `example.yaml`
- **Current configuration:** `max_concurrent: 0` (unlimited)
- **Implementation requirements:**
  - Default to sensible limit (e.g., 1000)
  - Add semaphore to enforce max_concurrent
  - Return 503 when queue full, not 500
  - Track queue depth in metrics

**Connection Pool Exhaustion:**
- **Risk:** HTTP client connection pools can be exhausted if backends are slow. New requests hang
- **Files:** `internal/proxy/server.go`, `internal/providers/*.go`
- **Implementation requirements:**
  - Use properly configured http.Client with timeout and connection limits
  - Set `MaxConnsPerHost` to prevent per-backend exhaustion
  - Add context timeout propagation (from request to backend call)
  - Close idle connections with keep-alive timeout

**Goroutine Leaks in Streaming:**
- **Risk:** If client disconnects mid-stream, goroutines handling stream might leak
- **Files:** `internal/proxy/sse.go`, `internal/providers/*.go`
- **Implementation requirements:**
  - Always use context.WithCancel for streaming operations
  - Hook client disconnect detection (http.ResponseWriter Flusher or context done)
  - Use defer to cleanup goroutines
  - Test with client disconnections to catch leaks

---

## Security Concerns

**API Key Exposure in Logs:**
- **Risk:** If API keys appear in error logs, they're exposed to anyone with log access
- **Files:** All provider implementations, `internal/proxy/middleware.go`
- **Implementation requirements:**
  - Never log full API keys
  - Use masked identifiers (e.g., "sk-ant-...x7f2")
  - Scrub request/response bodies from logs (contains tokens)
  - Add log sanitizer middleware

**gRPC API Authentication:**
- **Risk:** gRPC management API (relay.proto) has no authentication defined. Anyone can call to disable providers, modify keys
- **Files:** `internal/grpc/server.go`, `relay.proto`
- **Current mitigation:** None documented
- **Implementation requirements:**
  - Add mTLS requirement for gRPC connections
  - Or add token-based auth (bearer token in metadata)
  - Restrict TUI/WebUI to localhost only
  - Document security model clearly

**Configuration File Permissions:**
- **Risk:** Config file contains API keys. If world-readable, keys are exposed
- **Files:** `~/.config/cc-relay/config.yaml`
- **Implementation requirements:**
  - Validate config file permissions on startup (should be 0600)
  - Refuse to start if permissions are insecure
  - Add warning if keys in env vars have insecure permissions

---

## Performance and Scaling Limits

**Streaming Response Latency:**
- **Risk:** If proxy buffers responses, TTFB (time to first byte) increases significantly
- **Files:** `internal/proxy/sse.go`, `internal/providers/*.go`
- **Implementation requirements:**
  - Stream responses directly without buffering
  - Flush each SSE event immediately
  - Minimize transformation latency (don't parse entire response)
  - Benchmark TTFB against direct connection

**Provider Latency Amplification:**
- **Risk:** Proxy adds latency to every request. If proxy adds 500ms, users perceive 500ms slower responses
- **Files:** `internal/proxy/server.go`, `internal/providers/*.go`
- **Implementation requirements:**
  - Profile latency breakdown: routing (ms), auth (ms), transformation (ms)
  - Target <50ms proxy overhead
  - Use efficient libraries (avoid reflection, avoid JSON marshaling in hot path)
  - Add latency metrics to identify bottlenecks

**Memory Usage During Streaming:**
- **Risk:** If streaming requests buffer in memory, large context windows OOM proxy
- **Files:** `internal/proxy/sse.go`, `internal/providers/*.go`
- **Implementation requirements:**
  - Stream request bodies, don't buffer
  - Stream response bodies, don't buffer
  - Test with 100K+ token requests to verify no memory accumulation

---

## Testing Gaps

**SSE Event Correctness:**
- **Gap:** No defined test for exact SSE event sequence matching Anthropic's format
- **Files:** Tests for `internal/proxy/sse.go` (planned)
- **Required coverage:**
  - message_start → content_block_start → ... → message_stop sequence
  - Ping event insertion
  - Multiple content blocks
  - Tool use blocks with proper structure

**Provider Transformer Unit Tests:**
- **Gap:** Each provider (Bedrock, Azure, Vertex, Ollama, Z.AI) needs transformation tests
- **Files:** Tests for `internal/providers/*.go` (planned)
- **Required coverage:**
  - Request transformation (auth headers, URL format, body format)
  - Response transformation (status codes, headers, body format)
  - Error cases (invalid auth, rate limits, timeouts)
  - Tool use ID preservation
  - Model mapping correctness

**Circuit Breaker State Transitions:**
- **Gap:** No defined tests for CLOSED → OPEN → HALF_OPEN → CLOSED transitions
- **Files:** Tests for `internal/health/circuit.go` (planned)
- **Required coverage:**
  - Failure threshold triggering (3 failures → open)
  - Cooldown period enforcement
  - HALF_OPEN probe behavior
  - Recovery after successful probe
  - Concurrent failure handling

**Rate Limit Key Pool Selection:**
- **Gap:** No defined tests for key selection under load and rate limit edge cases
- **Files:** Tests for `internal/router/keypool.go` (planned)
- **Required coverage:**
  - Weighted selection distribution
  - Selection when all keys limited
  - Rate limit counter reset timing
  - Token counting accuracy

**Hot-Reload Race Conditions:**
- **Gap:** No defined tests for config reload with in-flight requests
- **Files:** Tests for `internal/config/watcher.go`, integration tests
- **Required coverage:**
  - Reload while requests in-flight
  - Key removal while requests active
  - Provider disabling while requests active
  - Invalid config load doesn't corrupt state

---

## Design Decisions Needing Validation

**Single Proxy Instance per Machine:**
- **Assumption:** One cc-relay daemon per machine, Claude Code connects to localhost:8787
- **Risk:** If this assumption changes (e.g., multiple Claude Code instances), load balancing becomes complex
- **Mitigation:** Document this clearly, make easy to run multiple instances on different ports

**gRPC for Management API:**
- **Assumption:** TUI uses gRPC to talk to daemon
- **Risk:** gRPC complexity for TUI. Alternative: Unix socket + JSON RPC might be simpler
- **Mitigation:** Prototype TUI early to validate gRPC choice

**Exact Anthropic API Compatibility:**
- **Assumption:** Proxy must be 100% compatible with Anthropic API to work with Claude Code
- **Risk:** Any divergence breaks Claude Code
- **Mitigation:** Start with single provider (Anthropic direct), only add others after validation

---

## Known Implementation Challenges

**Provider Diversity Complexity:**
- **Challenge:** Each provider has different auth, model naming, feature support
- **Scope:** 6 providers × 4 methods each (TransformRequest/Response, Auth, HealthCheck) = 24 implementations
- **Mitigation:** Build provider interface carefully, use comprehensive tests

**SSE Streaming Correctness Under Load:**
- **Challenge:** SSE events must be in exact order even under concurrent requests and network jitter
- **Scope:** Proxy must be deterministic while handling 100+ concurrent streams
- **Mitigation:** Use buffer per stream (not global), test concurrency early

**gRPC Streaming Stats:**
- **Challenge:** StreamStats must provide real-time updates without blocking requests
- **Scope:** Request side effects stats collection for downstream gRPC clients
- **Mitigation:** Use non-blocking stats aggregation (atomic counters, not mutexes)

---

## Scaling Concerns (Future Phases)

**Phase 5+ Multi-Tenant Risk:**
- **Risk:** If multi-tenancy is added later, current single-daemon-per-machine architecture won't scale
- **Mitigation:** Avoid hardcoding localhost:8787. Design for easy multi-instance setup

**Phase 6 WebUI Security:**
- **Risk:** Browser-based WebUI over grpc-web introduces CORS, authentication, HTTPS complexity
- **Mitigation:** Design grpc-web integration carefully, enforce authentication in WebUI phase

---

*Concerns audit: 2026-01-20*
