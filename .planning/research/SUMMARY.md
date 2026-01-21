# Project Research Summary

**Project:** cc-relay
**Domain:** Multi-provider LLM HTTP Proxy
**Researched:** 2026-01-20
**Confidence:** HIGH

## Executive Summary

cc-relay is a multi-provider LLM proxy designed specifically for Claude Code, routing requests across Anthropic, Z.AI, Ollama, and cloud providers (Bedrock, Azure, Vertex). Based on comprehensive research, experts build these systems as layered HTTP gateways with routing intelligence, using Go's native HTTP/2 and SSE streaming capabilities for performance, provider adapter patterns for extensibility, and circuit breaker patterns for reliability. The architecture follows a proven pattern: API Gateway → Router → Provider Adapters → Backend Providers.

The recommended approach prioritizes exact Anthropic API compatibility in Phase 1 (MVP), establishing a working proxy with basic routing before adding complexity. This foundation enables Claude Code to work unchanged while the proxy transparently manages multi-key pooling (Phase 2), cloud provider support (Phase 3), and advanced routing strategies (Phase 5). The stack centers on Go 1.23+ with stdlib components (net/http/httputil.ReverseProxy, log/slog), gRPC for management APIs, and Bubble Tea for the TUI dashboard.

Critical risks center on SSE streaming buffering (breaks real-time UX), tool_use_id preservation (breaks parallel agent operations), and authentication gaps (enables API key theft). Mitigation requires immediate SSE flushing with proper headers (X-Accel-Buffering: no), preserving all JSON fields during transformations, and robust API key validation from day one. Provider-specific quirks (Bedrock inference profiles, Vertex OAuth tokens, Ollama limitations) must be handled in dedicated adapter implementations. Following the phased roadmap prevents premature optimization while maintaining a working system at each step.

## Key Findings

### Recommended Stack

Go 1.23+ provides the ideal foundation for LLM proxy development, offering excellent HTTP/2 support, native SSE streaming capabilities, and strong concurrency primitives. The stdlib's net/http/httputil.ReverseProxy (with modern Rewrite function pattern) handles proxy concerns with battle-tested code, while log/slog provides structured logging without external dependencies. This stack prioritizes simplicity and performance, avoiding the allocation overhead and deployment complexity of Python-based alternatives like LiteLLM.

**Core technologies:**
- **Go 1.23+**: Primary language — Excellent HTTP/2 support, native concurrency for SSE streaming, strong stdlib, statically typed for API transformations
- **net/http/httputil**: HTTP reverse proxy — Built-in ReverseProxy with Rewrite function, handles hop-by-hop headers, connection pooling, X-Forwarded headers automatically
- **log/slog**: Structured logging — Standard library solution (Go 1.21+), TextHandler for dev, JSONHandler for prod, integrates with context for request tracing
- **gRPC v1.78.0**: Management API — Industry standard for service-to-service communication, supports streaming stats, bidirectional communication for TUI updates
- **Bubble Tea v1.3.10**: Terminal UI — Elm Architecture (functional, testable), battle-tested in production, excellent for real-time dashboards
- **fsnotify v1.8.0+**: Config hot-reload — Cross-platform file watching, enables zero-downtime configuration updates
- **spf13/viper**: Configuration management — Multi-format support (YAML/TOML/JSON), environment variable expansion, de facto standard
- **Prometheus client_golang v1.20+**: Metrics — Standard for Go observability, Counter/Gauge/Histogram types, promhttp.Handler() for /metrics

**Provider SDKs:**
- **aws-sdk-go-v2 v1.33.0+**: AWS Bedrock — Official SDK v2 (v1 EOL), BedrockRuntime client, SigV4 signing built-in
- **google.golang.org/genai**: Vertex AI — NEW preferred SDK (June 2025), replaces deprecated cloud.google.com/go/vertexai
- **Azure/azure-sdk-for-go/sdk/ai/azopenai v0.8.0+**: Azure OpenAI — Official SDK, supports Azure-specific features
- **ollama/ollama/api**: Ollama — Official client used by CLI itself, fully typed, respects OLLAMA_HOST env var

**Critical version requirements:**
- Go 1.21+ for log/slog
- Go 1.20+ for net/http/httputil Rewrite function
- Go 1.23+ required by gRPC v1.78.0 and aws-sdk-go-v2 v1.33.0

### Expected Features

Research shows clear feature tiers based on competitive analysis (LiteLLM, Portkey, claude-code-router) and domain expertise. Users expect flawless API compatibility and SSE streaming as table stakes — missing these makes the product feel broken. Multi-key pooling and automatic failover provide competitive advantage by maximizing throughput and reliability. Advanced features like semantic caching and multi-tenancy are explicitly anti-features that add complexity without value for the target use case (single developer/small team).

**Must have (table stakes):**
- **Multi-provider routing** — Core value proposition, users expect to route to multiple backends
- **API compatibility** — Existing clients (Claude Code) must work without modification, exact `/v1/messages` endpoint match
- **SSE streaming support** — LLM responses streamed for real-time UX, exact event sequence (message_start → content_block_delta → message_stop)
- **Authentication** — API key authentication via `x-api-key` header to control access
- **Configuration file** — Declarative YAML/TOML config, environment variable expansion, validation
- **Request/response logging** — Structured JSON logging, request IDs, latency tracking for debugging
- **Graceful shutdown** — Context cancellation, drain period, signal handling to avoid dropping in-flight requests

**Should have (competitive):**
- **Multi-key rate limit pooling** — Maximize throughput by distributing across API keys with per-key RPM/TPM tracking
- **Automatic failover with circuit breaker** — High availability, route around failing providers with state machine (CLOSED/OPEN/HALF-OPEN)
- **Cost-based routing** — Save money routing simple tasks to cheaper providers (Z.AI, Ollama)
- **Latency-based routing** — Performance-critical apps prefer faster backends dynamically
- **Model-based routing** — Route by model name prefix (claude-* → Anthropic, glm-* → Z.AI, local → Ollama)
- **Hot-reload configuration** — Change config without downtime using fsnotify file watcher
- **Real-time TUI** — Visual monitoring via Bubble Tea with gRPC stats streaming
- **Prometheus metrics** — Ops teams expect metrics for alerting (/metrics endpoint, per-provider counters)

**Defer (v2+):**
- **Semantic caching** — High complexity, storage requirement, conflicts with streaming UX (can't cache until response complete)
- **Request queueing** — Adds latency, users can implement client-side queueing
- **WebUI** — TUI works for target users (developers), web adds deployment complexity
- **Prompt transformation/rewriting** — Anti-feature: introduces unpredictable behavior, breaks tool use, violates user intent
- **Built-in rate limiting (throttling)** — Anti-feature: proxy should pool limits not create new ones, adds latency
- **Multi-tenancy** — Anti-feature: adds auth complexity, database requirement, different use case (single-tenant per deployment)

### Architecture Approach

Multi-provider LLM proxies follow a proven layered gateway architecture: API Gateway (handles incoming requests, auth, SSE) → Routing Layer (selects provider + key based on strategy) → Provider Adapter Layer (transforms requests for specific provider APIs) → Backend Providers. This separation of concerns enables independent evolution of routing strategies and provider integrations. The key architectural patterns are: (1) Reverse Proxy with Transformation Pipeline using net/http/httputil.ReverseProxy as base, (2) Circuit Breaker with State Machine to prevent cascading failures, (3) API Key Pool with Rate Limit Tracking using sliding windows, and (4) SSE Streaming with Immediate Flush to avoid buffering latency.

**Major components:**
1. **HTTP Proxy Server** (`internal/proxy/`) — API Gateway layer accepting `/v1/messages` requests, validating auth, handling SSE streaming with proper headers (X-Accel-Buffering: no), implementing middleware for logging/metrics
2. **Router** (`internal/router/`) — Routing layer implementing pluggable strategies (shuffle, round-robin, failover, cost-based, latency-based, model-based), selecting backend provider + API key for each request, consulting health tracker before selection
3. **Provider Adapters** (`internal/providers/`) — Provider interface with TransformRequest/TransformResponse/Authenticate/HealthCheck methods, separate adapter per provider (anthropic.go, zai.go, ollama.go, bedrock.go, azure.go, vertex.go) handling API-specific transformations
4. **Key Pool Manager** (`internal/router/keypool.go`) — Tracks per-key rate limits (RPM/TPM) using sliding windows, distributes load across multiple API keys for same provider, returns ErrAllKeysExhausted when all keys at limit
5. **Health Tracker** (`internal/health/`) — Circuit breaker state machine per provider (CLOSED → OPEN → HALF-OPEN), tracks failures (5xx, timeouts, connection errors), automatic recovery probing after cooldown, exempts client errors (4xx) from circuit breaker logic
6. **Configuration Loader** (`internal/config/`) — YAML/TOML parsing with environment variable expansion, hot-reload via fsnotify watcher, config validation on load with clear error messages
7. **gRPC Management API** (`internal/grpc/`) — Service defined in proto/relay.proto, exposes stats streaming, provider/key management, config updates, consumed by TUI/CLI clients

**Data flow (streaming):**
Claude Code → API Gateway (validate, set SSE headers) → Router (select provider + key) → Health Tracker (check circuit breaker) → Provider Adapter (transform request, maintain streaming connection) → Backend Provider (stream SSE events) → Provider Adapter (transform events to Anthropic format) → API Gateway (flush immediately via http.Flusher) → Claude Code

### Critical Pitfalls

Research reveals 10 critical pitfalls with proven mitigation strategies. The top 5 must be addressed in Phase 1 (MVP) as they break core functionality, while remaining pitfalls align with specific phases (multi-key pooling, cloud providers, metrics).

1. **SSE Streaming Buffering** — Proxy buffers SSE events instead of flushing immediately, causing Claude Code to hang. Set required headers (Content-Type: text/event-stream, Cache-Control: no-cache, X-Accel-Buffering: no), use http.Flusher interface to flush after each event, test with real Claude Code not curl. Phase 1 (MVP).

2. **Tool Use ID Preservation Failure** — Proxy fails to preserve tool_use_id when handling parallel tool calls, causing "orphan tool_result blocks" errors. Use map[string]interface{} instead of struct marshaling to preserve all JSON fields, handle multiple tool_use blocks atomically, test with parallel tool calls (Read + Bash + Grep simultaneously). Phase 1 (MVP).

3. **Weak Authentication and Public Exposure** — Proxy deployed without robust auth becomes access broker for attackers. Between Oct 2025-Jan 2026, 91,000+ attack sessions targeted misconfigured LLM proxies. Never deploy with authentication disabled, implement proper API key validation, add per-key rate limiting, log auth failures with alerting. Phase 1 (MVP).

4. **Rate Limit Bypass via Key Pool Mismanagement** — Rate limiting bypassed because proxy identifies requests by IP instead of API key. Track rate limits per incoming API key AND per backend provider key, don't use IP as primary rate limit key, implement token bucket or sliding window (not fixed window), return Retry-After headers on 429. Phase 2 (Multi-key pooling).

5. **Circuit Breaker Anti-Patterns** — Circuit breaker treats all failures equally, opening on recoverable errors (400 Bad Request). Only count server errors (5xx, timeouts, connection failures) as circuit breaker failures, use dedicated health check endpoints in half-open state, implement per-provider circuit breakers, set appropriate thresholds (5 consecutive failures not 1). Phase 2 (Health tracking).

**Additional critical pitfalls:**
- **Cost Attribution Blindness** — Can't determine who/what caused costs. Capture metadata (API key, model, provider, tokens, cost) on every request, log to structured JSON, export Prometheus metrics with labels. Phase 2 (Metrics).
- **Provider-Specific Compatibility Ignored** — Bedrock requires inference profiles, Ollama doesn't support prompt caching, Azure uses different auth headers. Create provider compatibility matrix, implement provider-specific validation, test each provider independently. Phase 3 (Cloud providers).
- **Missing Header Forwarding** — Proxy doesn't forward anthropic-beta headers, silently disabling features like extended thinking. Forward ALL anthropic-* headers by default, subscribe to API changelog, test with beta features enabled. Phase 1 (MVP).
- **Credential Rotation Causing Downtime** — Rotating API keys causes 401 errors during rotation window. Support multiple active keys per provider during rotation, implement graceful config reload, follow zero-downtime rotation procedure. Phase 2 (Multi-key pooling).
- **AWS Bedrock Inference Profile Confusion** — Using direct model IDs causes "on-demand throughput not supported" errors. Document Bedrock-specific setup prominently, validate model IDs against expected pattern, provide clear error messages. Phase 3 (Cloud providers).

## Implications for Roadmap

Based on research findings, the roadmap should follow a dependency-driven progression that maintains a working proxy at each phase while incrementally adding reliability and provider support. The architecture research reveals clear component boundaries that map to natural phase groupings. Feature research shows MVP requires only API compatibility + basic routing, with advanced features deferred until post-validation. Pitfall research identifies which phases carry highest risk and need extra attention.

### Phase 1: Core Proxy (MVP)

**Rationale:** Establish working proxy with exact Anthropic API compatibility before adding routing complexity. This validates the core value proposition (Claude Code works unchanged) and de-risks SSE streaming implementation, which research shows is the highest-impact pitfall if done incorrectly.

**Delivers:** Working proxy that accepts Claude Code requests, routes to single Anthropic key, preserves tool_use_id, handles SSE streaming correctly, validates API keys.

**Addresses features:**
- API compatibility (table stakes)
- SSE streaming support (table stakes)
- Authentication (table stakes)
- Configuration file (table stakes)
- Request/response logging (table stakes)
- Graceful shutdown (table stakes)

**Avoids pitfalls:**
- SSE Streaming Buffering (Critical #1)
- Tool Use ID Preservation Failure (Critical #2)
- Weak Authentication (Critical #3)
- Missing Header Forwarding (Critical #8)

**Implementation order:**
1. HTTP Server (`internal/proxy/server.go`) — Basic /v1/messages endpoint
2. Provider Interface (`internal/providers/provider.go`) — Define interface
3. Anthropic Provider (`internal/providers/anthropic.go`) — Passthrough implementation
4. Config Loader (`internal/config/`) — YAML parsing, validation
5. SSE Handler (`internal/proxy/sse.go`) — Streaming with immediate flush
6. Auth Middleware (`internal/proxy/middleware.go`) — API key validation

**Testing focus:** Real Claude Code integration, parallel tool calls, extended thinking blocks, SSE streaming latency

### Phase 2: Multi-Key Pooling and Reliability

**Rationale:** Once basic proxy works, users will immediately hit rate limits on single keys and request failover capability. This phase adds production-readiness without changing the core proxy logic.

**Delivers:** Multi-key rate limit pooling with RPM/TPM tracking, automatic failover between providers, circuit breaker for degraded backends, hot-reload configuration.

**Uses stack elements:**
- fsnotify for config file watching
- Context for graceful shutdown
- log/slog for structured failure logging

**Implements architecture components:**
- Router Interface (`internal/router/router.go`)
- Simple Shuffle Strategy (`internal/router/strategies/shuffle.go`)
- Round-Robin Strategy (`internal/router/strategies/roundrobin.go`)
- Failover Strategy (`internal/router/strategies/failover.go`)
- Key Pool Manager (`internal/router/keypool.go`)
- Circuit Breaker (`internal/health/circuit.go`)
- Health Tracker (`internal/health/tracker.go`)

**Addresses features:**
- Multi-key rate limit pooling (competitive advantage)
- Automatic failover with circuit breaker (competitive advantage)
- Hot-reload configuration (competitive advantage)

**Avoids pitfalls:**
- Rate Limit Bypass (Critical #4)
- Circuit Breaker Anti-Patterns (Critical #5)
- Credential Rotation Downtime (Critical #9)

**Testing focus:** Rate limit sliding windows, circuit breaker state transitions, failover under provider outage, zero-downtime config reload

### Phase 3: Additional Providers

**Rationale:** With routing and reliability established, adding provider adapters is low-risk since the abstraction is proven. Start with compatible providers (Z.AI, Ollama) before tackling cloud providers with complex auth.

**Delivers:** Support for Z.AI (Anthropic-compatible), Ollama (local models), establishing provider adapter pattern before cloud complexity.

**Uses stack elements:**
- Provider Interface from Phase 1
- Routing strategies from Phase 2

**Implements architecture components:**
- Z.AI Provider (`internal/providers/zai.go`) — Model mapping (GLM-4.7 → claude-sonnet-4.5)
- Ollama Provider (`internal/providers/ollama.go`) — Local endpoint, no auth, limitations documented

**Addresses features:**
- Multi-provider routing (table stakes, extended beyond Anthropic)
- Cost-based routing (begins to differentiate Z.AI vs Anthropic pricing)

**Avoids pitfalls:**
- Provider-Specific Compatibility Ignored (Critical #7)

**Testing focus:** Provider-specific quirks (Ollama no prompt caching, Z.AI model mapping), integration tests per provider

### Phase 4: Cloud Providers (Bedrock, Azure, Vertex)

**Rationale:** Enterprise users need cloud provider support, but these require complex auth (SigV4 signing, OAuth tokens) and have provider-specific quirks (Bedrock inference profiles). Defer until adapter pattern is proven with simpler providers.

**Delivers:** AWS Bedrock support with SigV4 signing, Azure AI Foundry with x-api-key auth, Google Vertex AI with OAuth tokens.

**Uses stack elements:**
- aws-sdk-go-v2 v1.33.0+ for Bedrock BedrockRuntime client
- google.golang.org/genai for Vertex AI (new SDK)
- Azure/azure-sdk-for-go/sdk/ai/azopenai v0.8.0+ for Azure

**Implements architecture components:**
- Bedrock Provider (`internal/providers/bedrock.go`) — SigV4 signing, inference profile validation
- Azure Provider (`internal/providers/azure.go`) — x-api-key auth, deployment names as model IDs
- Vertex Provider (`internal/providers/vertex.go`) — OAuth token generation, model in URL path

**Addresses features:**
- Enterprise cloud provider support (competitive advantage)

**Avoids pitfalls:**
- Provider-Specific Compatibility Ignored (Critical #7)
- Bedrock Inference Profile Confusion (Critical #10)

**Testing focus:** Provider-specific auth (SigV4, OAuth), model ID validation, inference profile handling, token refresh logic

### Phase 5: Advanced Routing Strategies

**Rationale:** With all providers working, optimize routing based on cost, latency, and model names. These are nice-to-have optimizations that don't block core functionality.

**Delivers:** Cost-based routing (route simple tasks to cheaper providers), latency-based routing (route to fastest backend), model-based routing (pattern matching on model field).

**Uses stack elements:**
- Router Interface from Phase 2
- Provider adapters from Phases 3-4

**Implements architecture components:**
- Cost-Based Strategy (`internal/router/strategies/costbased.go`) — Cost mapping per provider/model
- Latency-Based Strategy (`internal/router/strategies/latency.go`) — Exponential moving average
- Model-Based Strategy (`internal/router/strategies/modelbased.go`) — Pattern matching (claude-* → Anthropic)

**Addresses features:**
- Cost-based routing (competitive advantage)
- Latency-based routing (competitive advantage)
- Model-based routing (competitive advantage)

**Testing focus:** Cost calculation accuracy, latency tracking with EMA, model pattern matching edge cases

### Phase 6: Management Interface (gRPC + TUI)

**Rationale:** Once proxy is feature-complete, add management interface for visibility and control. TUI is differentiator vs web-based alternatives (LiteLLM, Portkey).

**Delivers:** gRPC management API for stats streaming, provider/key management, config updates. Bubble Tea TUI for real-time monitoring.

**Uses stack elements:**
- gRPC v1.78.0 for management API
- Bubble Tea v1.3.10 for TUI
- Prometheus client_golang for metrics export

**Implements architecture components:**
- gRPC Server (`internal/grpc/server.go`) — Stats streaming, management RPCs
- TUI (`ui/tui/`) — Bubble Tea interface, gRPC client, real-time stats display
- Prometheus Metrics (`internal/metrics/`) — /metrics endpoint, per-provider counters

**Addresses features:**
- Real-time TUI (competitive advantage)
- Prometheus metrics (competitive advantage)

**Avoids pitfalls:**
- Cost Attribution Blindness (Critical #6)

**Testing focus:** gRPC streaming stats, TUI responsiveness under load, Prometheus metric accuracy

### Phase 7: WebUI (Optional)

**Rationale:** TUI covers developer use case. WebUI is optional enhancement for teams preferring browser-based interfaces.

**Delivers:** Web-based management interface using grpc-web to connect to existing gRPC API.

**Uses stack elements:**
- grpc-web for browser-to-gRPC bridge
- Existing gRPC API from Phase 6

**Addresses features:**
- WebUI (deferred, low priority)

### Phase Ordering Rationale

- **Phases 1-2 establish foundation:** Working proxy → Production reliability. Cannot add providers without routing, cannot add routing without working proxy.
- **Phases 3-4 add providers incrementally:** Simple providers (Z.AI, Ollama) validate adapter pattern before complex cloud providers (Bedrock, Vertex, Azure) with auth requirements.
- **Phase 5 optimizes routing:** Advanced strategies require all providers working to be useful (can't do cost-based routing without multiple providers to choose from).
- **Phase 6-7 add observability:** Management interfaces are most useful when proxy has all features (complete stats, all providers, all strategies).

**Dependency chain:**
- Phase 2 requires Phase 1 (routing requires working proxy)
- Phase 3 requires Phase 2 (provider adapters use routing interface)
- Phase 4 requires Phase 3 (cloud providers use proven adapter pattern)
- Phase 5 requires Phase 4 (advanced routing needs all providers available)
- Phase 6 requires Phase 5 (TUI shows stats for all routing strategies)
- Phase 7 requires Phase 6 (WebUI uses gRPC API)

**Risk mitigation:**
- Phases 1-2 address all Critical pitfalls #1-5 before adding complexity
- Each phase delivers working system (no "big bang" integration)
- Provider adapters isolated (one failing provider doesn't break others)
- Routing strategies pluggable (can swap algorithms at runtime)

### Research Flags

Phases likely needing deeper research during planning:

- **Phase 4 (Cloud Providers):** AWS Bedrock inference profiles, Vertex AI OAuth token lifecycle, Azure deployment name mapping. Bedrock docs are confusing around inference profiles vs model IDs. Budget extra time for Bedrock integration tests.

- **Phase 6 (gRPC + TUI):** Bubble Tea component composition for real-time stats display, gRPC streaming best practices for stats. TUI architecture needs careful planning to avoid spaghetti code.

Phases with standard patterns (skip research-phase):

- **Phase 1 (Core Proxy):** net/http/httputil.ReverseProxy pattern is well-documented, SSE streaming is standard Go pattern, auth middleware is straightforward. Implementation is well-trodden.

- **Phase 2 (Multi-Key Pooling):** Circuit breaker pattern has excellent resources, rate limiting with sliding windows is established pattern, config hot-reload with fsnotify is common Go pattern.

- **Phase 3 (Additional Providers):** Z.AI is Anthropic-compatible (minimal transformation), Ollama has official Go SDK with clear docs.

- **Phase 5 (Advanced Routing):** Cost-based and latency-based routing are well-understood patterns in load balancing literature.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Verified with pkg.go.dev (net/http/httputil Rewrite function, slog in Go 1.21), gRPC v1.78.0 release confirmed, AWS SDK v2 v1.33.0 verified, Google genai SDK migration date confirmed (June 2025) |
| Features | HIGH | Competitive analysis across LiteLLM, Portkey, claude-code-router with multiple sources, feature prioritization validated against domain expertise and user expectations |
| Architecture | HIGH | Verified with official docs, recent 2026 sources, Go-specific patterns (ReverseProxy, SSE, circuit breaker), reference implementations analyzed (LiteLLM, Bifrost) |
| Pitfalls | HIGH | Based on real-world issues from 2025-2026 incidents (91,000+ attack sessions, Bedrock inference profile confusion, SSE buffering across platforms), verified with official docs and engineering blogs |

**Overall confidence:** HIGH

All four research areas reached HIGH confidence through convergent evidence from official documentation, recent (2025-2026) web sources, and reference implementations. Stack recommendations verified against pkg.go.dev release notes and version compatibility matrices. Feature analysis cross-referenced three major competitors (LiteLLM, Portkey, claude-code-router) plus domain research. Architecture patterns validated with official Go documentation and production systems (LiteLLM at 8ms P95, Bifrost at 11μs overhead). Pitfalls confirmed with real-world incidents and vendor-specific documentation (AWS, Google, Azure).

### Gaps to Address

While overall confidence is high, the following areas need validation during implementation:

- **Bedrock inference profile mapping:** Research shows confusion between direct model IDs vs inference profiles, but optimal validation strategy unclear. During Phase 4, test with actual Bedrock account to understand error messages and build clear validation. Document common mistakes prominently.

- **Vertex AI token refresh timing:** Research confirms OAuth tokens expire after 1 hour, but optimal refresh strategy (proactive vs reactive, margin before expiry) needs testing. During Phase 4, implement token refresh logic and monitor token lifetime in practice.

- **SSE buffering platform-specific behavior:** Research confirms X-Accel-Buffering: no required for nginx/Cloudflare, but behavior across platforms (Vercel, Azure App Service, AWS ALB) may vary. During Phase 1, test with multiple reverse proxy configurations to validate headers.

- **Circuit breaker threshold tuning:** Research provides pattern (5 consecutive failures, cooldown period), but optimal thresholds depend on provider SLAs and request patterns. During Phase 2, start with conservative thresholds (5 failures, 60s cooldown) and tune based on metrics.

- **Cost mapping accuracy:** Research shows cost-based routing requires provider pricing, but providers don't expose pricing APIs consistently. During Phase 5, implement cost mapping via config file (manual updates) with clear documentation that pricing may drift.

## Sources

### Primary (HIGH confidence)

**Official Documentation:**
- [net/http/httputil ReverseProxy](https://pkg.go.dev/net/http/httputil) — Verified Rewrite function pattern, Director deprecation context, FlushInterval for SSE
- [log/slog package](https://pkg.go.dev/log/slog) — Verified Go 1.21 introduction, TextHandler/JSONHandler usage
- [gRPC Go v1.78.0](https://pkg.go.dev/google.golang.org/grpc) — Verified version, Go 1.23+ requirement
- [Bubble Tea v1.3.10](https://pkg.go.dev/github.com/charmbracelet/bubbletea) — Verified version, v2 availability
- [AWS SDK Go v2 Releases](https://github.com/aws/aws-sdk-go-v2/releases) — v1.33.0 release (Jan 15, 2025)
- [Google Cloud Vertex AI SDKs](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/sdks/overview) — genai SDK migration (June 24, 2025)
- [Azure OpenAI Go SDK](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai) — azopenai v0.8.0 (June 2025)
- [Anthropic Streaming Messages](https://docs.anthropic.com/en/api/messages-streaming) — SSE event sequence specification
- [Anthropic Claude on Amazon Bedrock](https://docs.anthropic.com/en/api/claude-on-amazon-bedrock) — BedrockRuntime client pattern
- [Anthropic Claude Code with Bedrock/Vertex/Proxies](https://docs.anthropic.com/en/docs/claude-code/bedrock-vertex-proxies) — Provider-specific requirements

### Secondary (MEDIUM confidence)

**Architecture & Patterns:**
- [How API Gateways Proxy LLM Requests - API7.ai](https://api7.ai/learning-center/api-gateway-guide/api-gateway-proxy-llm-requests) — Gateway architecture patterns
- [Multi-provider LLM orchestration in production: A 2026 Guide - DEV](https://dev.to/ash_dubai/multi-provider-llm-orchestration-in-production-a-2026-guide-1g10) — Production patterns
- [Building an SSE Proxy in Go - Medium](https://medium.com/@sercan.celenk/building-an-sse-proxy-in-go-streaming-and-forwarding-server-sent-events-1c951d3acd70) — SSE implementation
- [Circuit Breaker Patterns in Go Microservices - DEV](https://dev.to/serifcolakel/circuit-breaker-patterns-in-go-microservices-n3) — Circuit breaker implementation
- [Circuit Breaker Pattern - Azure Architecture Center](https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker) — Microsoft guidance

**Competitive Analysis:**
- [LiteLLM GitHub](https://github.com/BerriAI/litellm) — Multi-LLM proxy features, 8ms P95 latency
- [Bifrost by Maxim AI](https://www.getmaxim.ai/blog/bifrost-a-drop-in-llm-proxy-40x-faster-than-litellm/) — Go implementation, 11μs overhead
- [Portkey Alternatives](https://portkey.ai/alternatives/litellm-alternatives) — Feature comparison
- [TrueFoundry LLM Proxy Guide](https://www.truefoundry.com/blog/llm-proxy) — Gateway patterns
- [TrueFoundry Portkey vs LiteLLM](https://www.truefoundry.com/blog/portkey-vs-litellm) — Detailed comparison

**Security & Pitfalls:**
- [Hackers scan misconfigured proxies for paid LLM services](https://anavem.com/cybersecurity/hackers-scan-misconfigured-proxies-paid-llm-services) — 91,000+ attack sessions
- [Fixing Slow SSE Streaming in Next.js and Vercel](https://medium.com/@oyetoketoby80/fixing-slow-sse-server-sent-events-streaming-in-next-js-and-vercel-99f42fbdb996) — Buffering issues
- [Using Server Sent Events with Cloudflare Proxy](https://community.cloudflare.com/t/using-server-sent-events-sse-with-cloudflare-proxy/656279) — X-Accel-Buffering
- [API Error: 400 due to tool use concurrency](https://github.com/badrisnarayanan/antigravity-claude-proxy/issues/91) — tool_use_id preservation
- [Configuring Claude Code with AWS Bedrock (And My Mistakes)](https://aws.plainenglish.io/configuring-claude-code-extension-with-aws-bedrock-and-how-you-can-avoid-my-mistakes-090dbed5215b) — Bedrock pitfalls

**Cost & Observability:**
- [Monitor LiteLLM AI proxy with Datadog](https://www.datadoghq.com/blog/monitor-litellm-with-datadog/) — Metrics patterns
- [LLM cost attribution for GenAI apps](https://portkey.ai/blog/llm-cost-attribution-for-genai-apps/) — Cost tracking
- [Optimize AI proxies with Datadog](https://www.datadoghq.com/blog/optimize-ai-proxies-with-datadog/) — Observability requirements

### Tertiary (LOW confidence)

**Emerging Patterns (needs validation):**
- [Prompt caching - ngrok blog](https://ngrok.com/blog/prompt-caching/) — Semantic caching patterns (not implemented in Phase 1)
- [prompt-cache GitHub](https://github.com/messkan/prompt-cache) — Go semantic caching reference (future consideration)

---
*Research completed: 2026-01-20*
*Ready for roadmap: yes*
