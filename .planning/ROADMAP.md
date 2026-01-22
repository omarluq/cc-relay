# Roadmap: cc-relay

## Overview

cc-relay evolves from a basic single-provider proxy (Phase 1) to a production-ready multi-provider gateway with intelligent routing, health tracking, and visual management (Phases 2-11). Each phase delivers a working, verifiable capability while maintaining exact Anthropic API compatibility. The journey starts with core proxy functionality to validate Claude Code integration, adds multi-key pooling and reliability for production use, extends to multiple providers (Z.AI, Ollama, cloud providers), implements advanced routing strategies for cost/latency optimization, and culminates in comprehensive observability and management interfaces. Every requirement maps to exactly one phase, ensuring complete coverage of the 77 v1 requirements.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Core Proxy (MVP)** - Establish working proxy with exact Anthropic API compatibility
- [x] **Phase 1.1: Embedded HA Cache Clustering** - Enable cc-relay to form HA clusters with embedded Olric (INSERTED)
- [x] **Phase 1.2: Cache Documentation** - Comprehensive cache system documentation (keys, strategies, adapters, extensibility) (INSERTED)
- [x] **Phase 1.3: Site Documentation Update** - Update all site docs in all languages (INSERTED)
- [ ] **Phase 2: Multi-Key Pooling** - Add rate limit pooling across multiple API keys per provider
- [ ] **Phase 3: Routing Strategies** - Implement pluggable routing algorithms (round-robin, shuffle, failover)
- [ ] **Phase 4: Circuit Breaker & Health** - Add health tracking and automatic failover with state machine
- [ ] **Phase 5: Additional Providers** - Support Z.AI and Ollama providers
- [ ] **Phase 6: Cloud Providers** - Add AWS Bedrock, Azure Foundry, and Vertex AI support
- [ ] **Phase 7: Configuration Management** - Hot-reload, validation, and multi-format support
- [ ] **Phase 8: Observability** - Structured logging and Prometheus metrics
- [ ] **Phase 9: gRPC Management API** - Real-time stats streaming and provider management
- [ ] **Phase 10: TUI Dashboard** - Interactive Bubble Tea interface for monitoring
- [ ] **Phase 11: CLI Commands** - Complete command-line interface

## Phase Details

### Phase 1: Core Proxy (MVP)
**Goal**: Establish working proxy that accepts Claude Code requests, routes to Anthropic, preserves tool_use_id, handles SSE streaming correctly, and validates API keys
**Depends on**: Nothing (first phase)
**Requirements**: API-01, API-02, API-03, API-05, API-06, API-07, SSE-01, SSE-02, SSE-03, SSE-04, SSE-05, SSE-06, PROV-01, AUTH-01, AUTH-02, AUTH-03
**Success Criteria** (what must be TRUE):
  1. Claude Code can send requests to proxy and receive responses in Anthropic API format
  2. SSE streaming works with real-time event delivery (no buffering delays visible to user)
  3. Parallel tool calls preserve tool_use_id correctly (no orphan tool_result errors)
  4. Invalid API keys return 401 errors before hitting backend providers
  5. Extended thinking content blocks stream correctly without errors
**Plans**: 9 plans in 6 waves

Plans:
- [x] 01-01-PLAN.md - Foundation: Config loading and Provider interface
- [x] 01-02-PLAN.md - HTTP Server and Auth middleware
- [x] 01-03-PLAN.md - Proxy handler with SSE streaming
- [x] 01-04-PLAN.md - CLI integration and route wiring
- [x] 01-05-PLAN.md - Integration testing and verification
- [x] 01-06-PLAN.md - Structured logging with zerolog
- [x] 01-07-PLAN.md - CLI Subcommands (serve, status, config validate, version)
- [ ] 01-08-PLAN.md - Claude Code subscription token support
- [ ] 01-09-PLAN.md - Enhanced debug logging (request/response details, TLS metrics)

### Phase 1.1: Embedded HA Cache Clustering (INSERTED)
**Goal**: Enable cc-relay instances to form HA clusters with embedded Olric for shared cache state, automatic node discovery, and data replication
**Depends on**: Phase 1 (cache adapter foundation already implemented)
**Requirements**: CACHE-HA-01, CACHE-HA-02, CACHE-HA-03
**Success Criteria** (what must be TRUE):
  1. Multiple cc-relay instances can discover each other and form a cache cluster
  2. Cache data is replicated across nodes (ReplicaCount >= 2)
  3. Node failure does not cause data loss (surviving nodes have replicas)
  4. New nodes can join an existing cluster dynamically
  5. Remote client mode preserved for external cache (Redis future support)
**Plans**: 4 plans in 3 waves

Plans:
- [x] 01.1-01-PLAN.md - Extend OlricConfig with HA settings (environment, replication, quorum)
- [x] 01.1-02-PLAN.md - Apply HA config in embedded node creation (buildOlricConfig helper)
- [x] 01.1-03-PLAN.md - Cluster membership helpers and graceful shutdown verification
- [x] 01.1-04-PLAN.md - Integration tests for multi-node clustering

### Phase 1.2: Cache Documentation (INSERTED)
**Goal**: Create comprehensive documentation for the cache system covering keys, busting strategies, adapters, and extensibility
**Depends on**: Phase 1.1 (cache system fully implemented)
**Requirements**: DOC-CACHE-01
**Success Criteria** (what must be TRUE):
  1. Cache key naming conventions documented with examples
  2. Cache busting strategies documented (TTL, manual invalidation, cluster events)
  3. Cache adapter interface documented with implementation guide
  4. How to extend cache with new backends (Redis, Memcached) documented
  5. HA clustering configuration documented with examples
  6. Troubleshooting guide for common cache issues
**Plans**: 1 plan in 1 wave

Plans:
- [x] 01.2-01-PLAN.md - Comprehensive cache documentation (keys, strategies, adapters, HA clustering, troubleshooting)

### Phase 1.3: Site Documentation Update (INSERTED)
**Goal**: Update all site documentation in all supported languages to reflect current implementation
**Depends on**: Phase 1.2 (cache docs complete)
**Requirements**: DOC-SITE-01
**Success Criteria** (what must be TRUE):
  1. English documentation updated with all new features
  2. All other language translations updated (i18n)
  3. Cache documentation included in site docs
  4. Configuration examples updated for HA clustering
**Plans**: 6 plans in 2 waves

Plans:
- [x] 01.3-01-PLAN.md - Update English caching.md with HA clustering guide
- [x] 01.3-02-PLAN.md - Add cache configuration section to English configuration.md
- [x] 01.3-03-PLAN.md - Translate caching.md updates to German and Spanish
- [x] 01.3-04-PLAN.md - Translate caching.md updates to Japanese, Chinese, and Korean
- [x] 01.3-05-PLAN.md - Translate configuration.md cache section to German and Spanish
- [x] 01.3-06-PLAN.md - Translate configuration.md cache section to Japanese, Chinese, and Korean

### Phase 2: Multi-Key Pooling
**Goal**: Enable multiple API keys per provider with rate limit tracking (RPM/TPM) and intelligent key selection
**Depends on**: Phase 1
**Requirements**: POOL-01, POOL-02, POOL-03, POOL-04, POOL-05, POOL-06, AUTH-04, AUTH-05
**Success Criteria** (what must be TRUE):
  1. Proxy accepts configuration with multiple keys per provider
  2. Requests distribute across available keys based on rate limit capacity
  3. Proxy returns 429 when all keys are at capacity (not 5xx)
  4. Key rotation happens without service downtime or request failures
**Plans**: 5 plans in 3 waves

Plans:
- [ ] 02-01-PLAN.md - Rate limiter interface and token bucket implementation
- [ ] 02-02-PLAN.md - Key metadata and key selector interface with strategies
- [ ] 02-03-PLAN.md - Key pool coordination (pool.go)
- [ ] 02-04-PLAN.md - Config extension for multi-key pooling
- [ ] 02-05-PLAN.md - Integration with proxy handler and 429 handling

### Phase 3: Routing Strategies
**Goal**: Implement pluggable routing strategies (round-robin, shuffle, failover) selected via configuration
**Depends on**: Phase 2
**Requirements**: ROUT-01, ROUT-02, ROUT-03, ROUT-07
**Success Criteria** (what must be TRUE):
  1. User can select routing strategy in config file (round-robin/shuffle/failover)
  2. Round-robin distributes requests evenly across providers in sequence
  3. Shuffle randomizes provider selection for balanced load
  4. Failover tries primary provider first, falls back to secondary on failure
**Plans**: TBD

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD

### Phase 4: Circuit Breaker & Health
**Goal**: Add health tracking per provider with circuit breaker state machine (CLOSED/OPEN/HALF-OPEN) for automatic failure recovery
**Depends on**: Phase 3
**Requirements**: PROV-07, PROV-08, ROUT-08, CIRC-01, CIRC-02, CIRC-03, CIRC-04, CIRC-05, CIRC-06, CIRC-07
**Success Criteria** (what must be TRUE):
  1. Circuit breaker opens after threshold failures (e.g., 5 consecutive 5xx errors)
  2. Unhealthy providers are automatically bypassed in routing decisions
  3. Circuit breaker transitions to half-open after cooldown and probes provider health
  4. Successfully recovered providers return to rotation automatically
  5. Client errors (4xx) do not trigger circuit breaker (only server errors count)
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD

### Phase 5: Additional Providers
**Goal**: Support Z.AI (Anthropic-compatible) and Ollama (local models) providers
**Depends on**: Phase 4
**Requirements**: PROV-02, PROV-03
**Success Criteria** (what must be TRUE):
  1. User can configure Z.AI provider with API key and it routes requests correctly
  2. Z.AI model name mappings work (GLM-4.7 appears as model option)
  3. User can configure Ollama provider pointing to local endpoint
  4. Ollama provider handles requests without prompt caching or PDF support
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD

### Phase 6: Cloud Providers
**Goal**: Add AWS Bedrock (SigV4 signing), Azure Foundry (x-api-key auth), and Google Vertex AI (OAuth tokens) support
**Depends on**: Phase 5
**Requirements**: API-04, PROV-04, PROV-05, PROV-06
**Success Criteria** (what must be TRUE):
  1. User can configure AWS Bedrock provider with inference profile ARNs
  2. Bedrock requests use SigV4 signing and anthropic_version: "bedrock-2023-05-31"
  3. User can configure Azure Foundry provider with deployment names as model IDs
  4. User can configure Vertex AI provider and it generates/refreshes OAuth tokens automatically
  5. Model IDs transform correctly per provider (model in URL path for Bedrock/Vertex)
**Plans**: TBD

Plans:
- [ ] 06-01: TBD
- [ ] 06-02: TBD
- [ ] 06-03: TBD

### Phase 7: Configuration Management
**Goal**: Enable hot-reload when config changes, support multiple formats (YAML/TOML), validate on load, expand environment variables
**Depends on**: Phase 6
**Requirements**: CONF-01, CONF-02, CONF-03, CONF-04, CONF-05, CONF-06
**Success Criteria** (what must be TRUE):
  1. User can write YAML config file and proxy loads it successfully
  2. User can write TOML config file and proxy loads it successfully
  3. Environment variables in config (${VAR_NAME}) expand to actual values
  4. Invalid configuration causes startup failure with clear error message
  5. Changing config file triggers automatic reload without restarting proxy
  6. Config reload happens without dropping in-flight requests
**Plans**: TBD

Plans:
- [ ] 07-01: TBD
- [ ] 07-02: TBD

### Phase 8: Observability
**Goal**: Add structured JSON logging with request IDs, latency tracking per provider, and Prometheus metrics endpoint
**Depends on**: Phase 7
**Requirements**: OBS-01, OBS-02, OBS-03, OBS-04, OBS-05, OBS-06, OBS-07, MOD-01, MOD-02, MOD-03, MOD-04
**Success Criteria** (what must be TRUE):
  1. All requests log to stdout/file in structured JSON format with request_id
  2. Logs include provider name, model, latency, status code for every request
  3. Prometheus /metrics endpoint exposes request counters per provider
  4. Prometheus /metrics endpoint exposes latency histograms per provider
  5. Prometheus /metrics endpoint exposes provider health status as gauge (1=healthy, 0=unhealthy)
  6. GET /v1/models endpoint lists all models from all configured providers
**Plans**: TBD

Plans:
- [ ] 08-01: TBD
- [ ] 08-02: TBD

### Phase 9: gRPC Management API
**Goal**: Expose gRPC service for real-time stats streaming, provider enable/disable, key management, config reload
**Depends on**: Phase 8
**Requirements**: GRPC-01, GRPC-02, GRPC-03, GRPC-04, GRPC-05, GRPC-06
**Success Criteria** (what must be TRUE):
  1. gRPC client can connect to management API and receive stats stream
  2. Stats stream includes requests/sec, latency, and health status per provider
  3. gRPC client can disable a provider and routing stops sending requests to it
  4. gRPC client can enable a disabled provider and routing resumes
  5. gRPC client can trigger config reload via RPC call
  6. gRPC client can retrieve current config via RPC call
**Plans**: TBD

Plans:
- [ ] 09-01: TBD
- [ ] 09-02: TBD

### Phase 10: TUI Dashboard
**Goal**: Build Bubble Tea terminal UI that connects to proxy via gRPC and displays real-time stats, provider health, routing strategy
**Depends on**: Phase 9
**Requirements**: TUI-01, TUI-02, TUI-03, TUI-04, TUI-05, TUI-06, TUI-07
**Success Criteria** (what must be TRUE):
  1. User can launch TUI and it connects to running proxy daemon via gRPC
  2. TUI displays request rate and latency per provider with live updates
  3. TUI shows provider health status with visual indicators (green/yellow/red)
  4. TUI displays active routing strategy name
  5. TUI shows key pool status (available capacity per key)
  6. User can interactively disable/enable providers via TUI keyboard shortcuts
  7. User can trigger config reload from TUI
**Plans**: TBD

Plans:
- [ ] 10-01: TBD
- [ ] 10-02: TBD

### Phase 11: CLI Commands
**Goal**: Implement complete CLI with serve, status, tui, config reload commands and proper flag handling
**Depends on**: Phase 10
**Requirements**: CLI-01, CLI-02, CLI-03, CLI-04, CLI-05, CLI-06, ROUT-04, ROUT-05, ROUT-06
**Success Criteria** (what must be TRUE):
  1. User can run `cc-relay serve` to start proxy daemon
  2. User can run `cc-relay status` to check if proxy is healthy
  3. User can run `cc-relay tui` to launch TUI (connects to existing daemon)
  4. User can run `cc-relay config reload` to trigger hot-reload
  5. User can pass `--config /path/to/config.yaml` to use custom config file
  6. User can pass `--tui` flag to `serve` command to start daemon and TUI together
  7. Cost-based routing selects cheapest provider for given model
  8. Latency-based routing selects fastest provider based on historical latency
  9. Model-based routing routes by model name pattern (claude-* -> Anthropic, glm-* -> Z.AI)
**Plans**: TBD

Plans:
- [ ] 11-01: TBD
- [ ] 11-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 1.1 -> 1.2 -> 1.3 -> 2 -> 3 -> 4 -> 5 -> 6 -> 7 -> 8 -> 9 -> 10 -> 11

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Core Proxy (MVP) | 8/9 | In progress | - |
| 1.1 Embedded HA Cache (INSERTED) | 4/4 | Complete | 2026-01-21 |
| 1.2 Cache Documentation (INSERTED) | 1/1 | Complete | 2026-01-21 |
| 1.3 Site Docs Update (INSERTED) | 6/6 | Complete | 2026-01-21 |
| 2. Multi-Key Pooling | 0/5 | Not started | - |
| 3. Routing Strategies | 0/TBD | Not started | - |
| 4. Circuit Breaker & Health | 0/TBD | Not started | - |
| 5. Additional Providers | 0/TBD | Not started | - |
| 6. Cloud Providers | 0/TBD | Not started | - |
| 7. Configuration Management | 0/TBD | Not started | - |
| 8. Observability | 0/TBD | Not started | - |
| 9. gRPC Management API | 0/TBD | Not started | - |
| 10. TUI Dashboard | 0/TBD | Not started | - |
| 11. CLI Commands | 0/TBD | Not started | - |
