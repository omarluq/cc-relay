# Feature Research

**Domain:** Multi-provider LLM proxy
**Researched:** 2026-01-20
**Confidence:** HIGH

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete or broken.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Multi-provider routing** | Core value proposition — users expect to route to multiple backends | MEDIUM | Router with provider selection logic, requires provider abstraction |
| **API compatibility** | Users expect existing clients (Claude Code) to work without modification | HIGH | Must match Anthropic API exactly: `/v1/messages`, SSE streaming, tool_use_id preservation |
| **SSE streaming support** | LLM responses are streamed for real-time UX | HIGH | Must maintain exact event sequence (message_start → content_block_start → delta → stop), proper headers, flushing |
| **Authentication** | Users expect API key authentication to control access | LOW | Accept Anthropic-style `x-api-key` header, validate against config |
| **Basic error handling** | Proxies must surface errors from backends clearly | LOW | Pass through provider errors, add context for routing failures |
| **Configuration file** | Users expect declarative config, not code changes | MEDIUM | YAML/TOML parsing, environment variable expansion, validation |
| **Provider health checks** | Users expect degraded providers to be detected automatically | MEDIUM | Periodic health pings, status tracking per provider |
| **Request/response logging** | Debugging requires visibility into proxy behavior | LOW | Structured logging (JSON), request IDs, latency tracking |
| **Graceful shutdown** | Proxies shouldn't drop in-flight requests on restart | LOW | Context cancellation, drain period, signal handling |
| **Basic CLI** | Users expect `serve`, `status` commands at minimum | LOW | Cobra/flag-based CLI with subcommands |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required for basic function, but create value.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Multi-key rate limit pooling** | Maximize throughput by distributing across API keys from same provider | HIGH | Per-key RPM/TPM tracking, sliding windows, intelligent selection |
| **Automatic failover with circuit breaker** | High availability — route around failing providers automatically | HIGH | Circuit breaker states (CLOSED/OPEN/HALF-OPEN), failure thresholds, recovery probing |
| **Cost-based routing** | Save money by routing simple tasks to cheaper providers (Z.AI, Ollama) | MEDIUM | Cost mapping per provider/model, request complexity heuristics |
| **Latency-based routing** | Performance-critical apps prefer faster backends dynamically | MEDIUM | Track response times per provider, exponential moving average, route to fastest |
| **Model-based routing** | Route by model name prefix (`claude-*` → Anthropic, `glm-*` → Z.AI, local models → Ollama) | LOW | Pattern matching on model field, enables multi-model workflows |
| **Hot-reload configuration** | Change config without downtime or dropped requests | MEDIUM | File watcher (fsnotify), atomic config swap, graceful transition |
| **Real-time TUI** | Visual monitoring of proxy state, provider health, rate limits | HIGH | Bubble Tea app, gRPC client, live stats streaming, interactive controls |
| **Semantic caching** | Cache responses for identical prompts to reduce cost/latency | HIGH | Embedding-based similarity, TTL, cache invalidation, storage backend |
| **Request queueing** | Handle bursts gracefully instead of rejecting | MEDIUM | Buffered queue, backpressure, fair distribution across providers |
| **Provider weights/preferences** | Users can prefer certain providers (cost/quality/speed tradeoffs) | LOW | Weight field in config, weighted random selection in router |
| **Model mapping** | Map generic model names to provider-specific IDs | LOW | Config-driven mapping (e.g., `claude-sonnet-4-5` → `GLM-4.7` for Z.AI) |
| **Prometheus metrics** | Ops teams expect metrics for alerting and dashboards | MEDIUM | Prometheus client, expose /metrics endpoint, per-provider counters |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems or complexity without value.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Prompt transformation/rewriting** | "Improve prompts for cheaper models" | Introduces unpredictable behavior, breaks tool use, violates user intent | Let users control prompts, provide routing based on model capability instead |
| **Response caching by exact match** | "Save costs on repeated requests" | LLM responses are non-deterministic, exact match rarely hits, storage bloat | Use semantic caching with embeddings (differentiator) or client-side caching |
| **Built-in rate limiting (throttling requests)** | "Prevent abuse" | Proxy should pool limits, not create new ones — adds latency and complexity | Track provider limits, fail fast with 429 when all keys exhausted |
| **Request modification/injection** | "Add system prompts automatically" | Violates transparency, breaks debugging, creates hidden behavior | Users should control full request, proxy should be transparent |
| **Local model hosting** | "Bundle Ollama into proxy" | Scope creep — proxy is router, not model server | Support Ollama as provider (already planned), let users run Ollama separately |
| **Multi-tenancy with user auth** | "Share proxy across teams with quotas" | Adds auth complexity, database requirement, different use case | Single-tenant per deployment, users can run multiple instances |
| **Response validation/filtering** | "Block toxic responses" | Provider responsibility, proxy shouldn't modify responses | Pass through responses unchanged, let users add guardrails at app level |
| **Model fine-tuning integration** | "Train custom models" | Completely different product category | Focus on routing to existing models, not model management |

## Feature Dependencies

```
[API Compatibility]
    └──requires──> [SSE Streaming]
    └──requires──> [Provider Transformers]
                       └──requires──> [Provider Interface]

[Multi-Key Pooling]
    └──requires──> [Rate Limit Tracking]
    └──requires──> [Key Pool Manager]

[Automatic Failover]
    └──requires──> [Health Tracking]
    └──requires──> [Circuit Breaker]
                       └──requires──> [Provider Stats]

[Cost-Based Routing] ──requires──> [Router Interface]
[Latency-Based Routing] ──requires──> [Router Interface]
[Model-Based Routing] ──requires──> [Router Interface]

[TUI] ──requires──> [gRPC API]
          └──requires──> [Stats Streaming]

[Hot-Reload] ──requires──> [Configuration Loader]

[Prometheus Metrics] ──enhances──> [Request Logging]
                      ──enhances──> [Provider Stats]

[Semantic Caching] ──conflicts──> [Streaming Responses]
    (Can't cache until response complete, adds latency to first-time requests)
```

### Dependency Notes

- **API Compatibility requires SSE Streaming:** Claude Code expects streaming responses, non-negotiable for compatibility
- **API Compatibility requires Provider Transformers:** Each provider (Bedrock, Vertex, Azure) has different auth and format requirements
- **Multi-Key Pooling requires Rate Limit Tracking:** Can't pool without knowing which keys have capacity
- **Automatic Failover requires Circuit Breaker:** Need state machine to avoid retry storms on failed providers
- **TUI requires gRPC API:** TUI is a client to the daemon's management API
- **All routing strategies require Router Interface:** Common abstraction enables swapping strategies
- **Semantic Caching conflicts with Streaming:** Can't return cached response until you've seen the full request, creates first-time latency penalty

## MVP Definition

### Launch With (v0.1.0 - MVP)

Minimum viable product — what's needed to validate the core value proposition.

- [x] **API compatibility** — Claude Code must work unchanged (endpoint, SSE, tool_use_id)
- [x] **Multi-provider routing** — Route to Anthropic, Z.AI, Ollama at minimum
- [x] **Basic routing strategy** — simple-shuffle (weighted random) is sufficient for MVP
- [x] **Configuration file** — YAML config with provider definitions, API keys
- [x] **Provider transformers** — Handle Anthropic (native), Z.AI (compatible), Ollama (limited)
- [x] **Basic error handling** — Surface provider errors, log routing decisions
- [x] **CLI serve command** — `cc-relay serve` to start daemon
- [x] **Request logging** — Structured logs for debugging

**Validation criteria:** Can I use Claude Code with cc-relay and seamlessly switch between Anthropic, Z.AI, and Ollama? If yes, MVP succeeds.

### Add After Validation (v0.2.0 - v0.3.0)

Features to add once core routing works and users validate the approach.

- [ ] **Multi-key pooling** — Trigger: Users hit rate limits on single keys
- [ ] **Rate limit tracking** — Trigger: Pooling requires knowing key capacity
- [ ] **Failover strategy** — Trigger: Users experience provider outages
- [ ] **Circuit breaker** — Trigger: Failover without this creates retry storms
- [ ] **Health checks** — Trigger: Need to detect degraded providers proactively
- [ ] **Hot-reload config** — Trigger: Users complain about downtime during config changes
- [ ] **Cloud providers (Bedrock, Azure, Vertex)** — Trigger: Users request enterprise provider support
- [ ] **Round-robin strategy** — Trigger: Users want predictable distribution

### Future Consideration (v0.4.0+)

Features to defer until product-market fit is established.

- [ ] **Cost-based routing** — Why defer: Requires cost mapping and complexity heuristics, niche use case
- [ ] **Latency-based routing** — Why defer: Requires latency tracking and EMA, optimization for power users
- [ ] **Model-based routing** — Why defer: Simple to add but unclear if users need it (can use multiple proxies instead)
- [ ] **TUI** — Why defer: Nice-to-have, config files + logs work for MVP
- [ ] **gRPC management API** — Why defer: Only needed if TUI or WebUI added
- [ ] **Prometheus metrics** — Why defer: Logs sufficient for early users, add when ops teams adopt
- [ ] **Semantic caching** — Why defer: High complexity, storage requirement, conflicts with streaming UX
- [ ] **Request queueing** — Why defer: Adds latency, users can implement client-side queueing
- [ ] **WebUI** — Why defer: TUI works for target users (developers), web adds deployment complexity

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| API compatibility | HIGH | HIGH | P1 (MVP) |
| Multi-provider routing | HIGH | MEDIUM | P1 (MVP) |
| SSE streaming | HIGH | HIGH | P1 (MVP) |
| Configuration file | HIGH | LOW | P1 (MVP) |
| Provider transformers | HIGH | HIGH | P1 (MVP) |
| Basic CLI | HIGH | LOW | P1 (MVP) |
| Request logging | MEDIUM | LOW | P1 (MVP) |
| Multi-key pooling | HIGH | HIGH | P2 (v0.2.0) |
| Rate limit tracking | HIGH | MEDIUM | P2 (v0.2.0) |
| Automatic failover | HIGH | HIGH | P2 (v0.2.0) |
| Circuit breaker | HIGH | MEDIUM | P2 (v0.2.0) |
| Health checks | MEDIUM | MEDIUM | P2 (v0.2.0) |
| Hot-reload config | MEDIUM | MEDIUM | P2 (v0.2.0) |
| Cloud providers (Bedrock/Azure/Vertex) | MEDIUM | HIGH | P2 (v0.3.0) |
| TUI | MEDIUM | HIGH | P3 (v0.4.0) |
| gRPC management API | LOW | HIGH | P3 (v0.4.0) |
| Cost-based routing | MEDIUM | MEDIUM | P3 (v0.5.0) |
| Latency-based routing | LOW | MEDIUM | P3 (v0.5.0) |
| Prometheus metrics | LOW | MEDIUM | P3 (v0.5.0) |
| Semantic caching | LOW | HIGH | P3 (future) |
| WebUI | LOW | HIGH | P3 (v0.6.0) |

**Priority key:**
- **P1 (MVP):** Must have for launch — validates core value proposition
- **P2 (Post-MVP):** Should have after validation — adds reliability and enterprise features
- **P3 (Future):** Nice to have — optimization and management features for mature product

## Competitor Feature Analysis

| Feature | LiteLLM (Python) | Portkey (Cloud) | claude-code-router (TS) | Our Approach (cc-relay) |
|---------|------------------|-----------------|-------------------------|-------------------------|
| **Multi-provider support** | 100+ providers via unified API | 1600+ models | Route-based selection (Anthropic/Z.AI/Ollama) | 6 providers (Anthropic/Z.AI/Ollama/Bedrock/Azure/Vertex), focus on Anthropic-compatible |
| **Rate limiting** | Virtual keys, per-project quotas | Centralized rate limiting | No | Multi-key pooling with per-key RPM/TPM tracking |
| **Routing strategies** | Retry/fallback, load balancing, cost tracking | Configurable routing with retries and fallback | URL-based routing | simple-shuffle, round-robin, failover, cost-based, latency-based, model-based |
| **Observability** | Callbacks to Lunary, MLflow, Langfuse | Built-in observability core, prompt management | Basic logging | Structured logs (MVP), Prometheus metrics (future), gRPC stats API (future) |
| **Caching** | Yes (per-project) | Yes (automatic caching) | No | Not MVP, semantic caching considered for future |
| **Management UI** | Admin dashboard | Enterprise web UI | React UI | TUI (Bubble Tea) for v0.4.0, WebUI for v0.6.0 |
| **Deployment** | Self-hosted or cloud | Cloud-hosted SaaS | Self-hosted | Self-hosted binary (Go), no cloud dependency |
| **Guardrails** | Yes, configurable | Real-time safety filters | No | No (anti-feature — let users handle at app level) |
| **Authentication** | Virtual keys, JWT auth (enterprise) | SSO, virtual key management | API key passthrough | API key validation (MVP), no multi-tenancy |
| **Performance** | 8ms P95 latency at 1k RPS | N/A (cloud service) | Not specified | Target: <5ms overhead (Go's ReverseProxy is fast) |
| **License/Cost** | Open source (free) + Enterprise (paid) | Enterprise SaaS (paid) | Open source (MIT) | Open source (MIT), no enterprise version planned |

### Our Competitive Positioning

**vs LiteLLM:**
- **Narrower scope:** Focus on Anthropic-compatible providers only (not 100+ LLMs)
- **Simpler:** No virtual keys, no multi-tenancy, no guardrails — just routing
- **Lower latency:** Go vs Python, less middleware overhead
- **Better for Claude Code:** Purpose-built for Claude Code's specific API requirements (tool_use_id, extended thinking)

**vs Portkey:**
- **Self-hosted:** No cloud dependency, no SaaS lock-in
- **Developer-focused:** TUI and CLI instead of web dashboards
- **Lower complexity:** No prompt management, no compliance controls
- **Open source:** No paid tier, no enterprise upsell

**vs claude-code-router:**
- **More providers:** Supports cloud providers (Bedrock, Azure, Vertex) not just Z.AI
- **Smarter routing:** Multiple strategies (cost, latency, failover) not just URL-based
- **Production-ready:** Circuit breaker, health checks, rate limit pooling
- **Better observability:** gRPC API, TUI, Prometheus metrics (planned)

## Sources

**Competitive Analysis:**
- [LiteLLM GitHub](https://github.com/BerriAI/litellm) - Multi-LLM proxy features and architecture
- [Portkey Alternatives](https://portkey.ai/alternatives/litellm-alternatives) - Feature comparison
- [TrueFoundry LLM Proxy Guide](https://www.truefoundry.com/blog/llm-proxy) - Gateway patterns and features
- [TrueFoundry Portkey vs LiteLLM](https://www.truefoundry.com/blog/portkey-vs-litellm) - Detailed comparison
- [Maxim AI: Top 5 LLM Gateways](https://www.getmaxim.ai/articles/list-of-top-5-llm-gateways-in-2025/) - Industry landscape
- [OpenRouter Review 2025](https://skywork.ai/blog/openrouter-review-2025-unified-ai-model-api-pricing-privacy/) - Routing and failover patterns

**Security & Pitfalls:**
- [BleepingComputer: Misconfigured Proxies](https://www.bleepingcomputer.com/news/security/hackers-target-misconfigured-proxies-to-access-paid-llm-services/) - Security vulnerabilities
- [Datadog: Monitoring AI Proxies](https://www.datadoghq.com/blog/optimize-ai-proxies-with-datadog/) - Observability requirements
- [Medium: AI Gateways Primer](https://medium.com/@adnanmasood/primer-on-ai-gateways-llm-proxies-routers-definition-usage-and-purpose-9b714d544f8c) - Architecture patterns

**Observability & Caching:**
- [Portkey: LLM Observability Guide](https://portkey.ai/blog/the-complete-guide-to-llm-observability/) - Observability requirements
- [LakeFFS: LLM Observability Tools 2026](https://lakefs.io/blog/llm-observability-tools/) - Tool comparison
- [ngrok: Prompt Caching](https://ngrok.com/blog/prompt-caching/) - Semantic caching patterns
- [GitHub: prompt-cache](https://github.com/messkan/prompt-cache) - Go-based semantic caching implementation

**Developer Tools:**
- [Ollama Tutorial 2026](https://dev.to/proflead/complete-ollama-tutorial-2026-llms-via-cli-cloud-python-3m97) - Local model management
- [Copilot Proxy](https://dev.to/hankchiutw/copilot-proxy-your-free-llm-api-for-local-development-3c07) - Developer proxy patterns

---
*Feature research for: Multi-provider LLM proxy*
*Researched: 2026-01-20*
