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
- [x] **Phase 2: Multi-Key Pooling** - Add rate limit pooling across multiple API keys per provider
- [x] **Phase 2.1: Multi-Key Pooling Site Documentation** - Update all site docs in all languages (INSERTED)
- [x] **Phase 2.2: Subscription Token Relay** - Implement transparent proxy for client auth forwarding (INSERTED)
- [x] **Phase 2.3: Codebase Refactor with Samber Libraries** - Map codebase, integrate samber/lo/do/ro/mo, fix tech debt, improve coverage (INSERTED)
- [x] **Phase 3: Routing Strategies** - Implement pluggable routing algorithms (round-robin, shuffle, failover)
- [x] **Phase 3.1: Routing Documentation** - Add routing docs to site-docs in all languages (INSERTED)
- [x] **Phase 4: Circuit Breaker & Health** - Add health tracking and automatic failover with state machine
- [x] **Phase 4.1: Health Checker Wiring** - Wire Checker lifecycle to fix integration gaps (INSERTED)
- [x] **Phase 4.2: Config File Cleanup** - Consolidate config files, ensure example.yaml is single source of truth (INSERTED)
- [x] **Phase 4.3: Health Configuration Documentation** - Add health/circuit-breaker docs to site-docs (INSERTED)
- [x] **Phase 5: Additional Providers** - Support Z.AI and Ollama providers
- [x] **Phase 6: Cloud Providers** - Add AWS Bedrock, Azure Foundry, and Vertex AI support with transformer architecture
- [x] **Phase 7: Configuration Management** - Hot-reload, validation, and multi-format support
- [x] **Phase 7.1: Fix CodeQL Weak Crypto Alerts** - Address GitHub CodeQL security alerts (INSERTED)
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
**Plans**: 6 plans (5 original + 1 gap closure)

Plans:
- [x] 02-01-PLAN.md - Rate limiter interface and token bucket implementation
- [x] 02-02-PLAN.md - Key metadata and key selector interface with strategies
- [x] 02-03-PLAN.md - Key pool coordination (pool.go)
- [x] 02-04-PLAN.md - Config extension for multi-key pooling
- [x] 02-05-PLAN.md - Integration with proxy handler and 429 handling
- [x] 02-06-PLAN.md - Gap closure: Wire KeyPool in serve.go and routes.go with integration tests

### Phase 2.1: Multi-Key Pooling Site Documentation (INSERTED)
**Goal**: Update all site documentation in all supported languages with multi-key pooling configuration and usage
**Depends on**: Phase 2 (multi-key pooling complete)
**Requirements**: DOC-SITE-02
**Success Criteria** (what must be TRUE):
  1. English documentation updated with multi-key pooling configuration
  2. All other language translations updated (DE, ES, JA, ZH-CN, KO)
  3. Configuration examples show multiple keys with priorities, rate limits
  4. x-cc-relay-* response headers documented
**Plans**: 1 plan in 1 wave

Plans:
- [x] 02.1-01-PLAN.md - Add Multi-Key Pooling documentation to all 6 language configuration.md files

### Phase 2.2: Subscription Token Relay (INSERTED)
**Goal**: Implement transparent proxy that forwards client Authorization headers to Anthropic unchanged. Auto-detects based on client headers - no new config fields needed.
**Depends on**: Phase 2.1 (multi-key pooling docs complete)
**Requirements**: AUTH-SUB-01, AUTH-SUB-02, DOC-AUTH-01
**Success Criteria** (what must be TRUE):
  1. Client Authorization header forwarded unchanged when present
  2. Client x-api-key header forwarded unchanged when present
  3. Fallback to configured provider keys when client has no auth
  4. KeyPool/rate limiting only applies when using proxy's own keys
  5. Works with existing ANTHROPIC_AUTH_TOKEN - user just changes URL
  6. Documentation explains auto-detection behavior
**Plans**: 1 plan in 1 wave

Plans:
- [x] 02.2-01-PLAN.md - Transparent auth forwarding in handler.go with tests and documentation

### Phase 2.3: Codebase Refactor with Samber Libraries (INSERTED)
**Goal**: Comprehensive codebase improvement using samber/lo, samber/do, samber/ro, samber/mo libraries. Map codebase, fix tech debt, improve test coverage, and modernize patterns.
**Depends on**: Phase 2.2 (transparent auth complete)
**Requirements**: QUALITY-01, QUALITY-02, QUALITY-03
**Success Criteria** (what must be TRUE):
  1. Codebase architecture mapped and documented
  2. samber/lo integrated for functional collection utilities (map, filter, reduce, etc.)
  3. samber/do integrated for dependency injection container
  4. samber/mo integrated for Option/Result monads (better error handling)
  5. samber/ro integrated with plugins for reactive streams
  6. Local .claude skills/agents created for samber library usage patterns
  7. Tech debt identified and resolved (code smells, bad patterns fixed)
  8. Test coverage improved (target: >80% on all packages)
  9. All existing tests pass after refactoring
  10. Linter strictness increased (gocognit threshold reduced)
  11. Property-based tests added for complex logic
**Plans**: 13 plans in 12 waves

Plans:
- [x] 02.3-01-PLAN.md - Test coverage baseline + codebase architecture mapping
- [x] 02.3-02-PLAN.md - Install samber libraries, create reference skills (lo, mo, do, ro)
- [x] 02.3-03-PLAN.md - Refactor keypool package with lo functional patterns
- [x] 02.3-04-PLAN.md - Refactor providers and auth packages with lo patterns
- [x] 02.3-05-PLAN.md - Refactor proxy and config packages with lo patterns
- [x] 02.3-06-PLAN.md - Integrate mo monads (Option for nullable, Result for errors)
- [x] 02.3-07a-PLAN.md - Create DI container foundation with samber/do
- [x] 02.3-07b-PLAN.md - Integrate DI container into serve.go
- [x] 02.3-08-PLAN.md - Create refactoring agents and pattern skills (including streams.md)
- [x] 02.3-09-PLAN.md - Tech debt audit and linter strictness increase
- [x] 02.3-10-PLAN.md - Property-based tests for keypool, ratelimit, auth
- [x] 02.3-11-PLAN.md - samber/ro foundation (core + plugins installation, stream utilities)
- [x] 02.3-12-PLAN.md - samber/ro integration (rate limiter, cache, SSE plugins)

### Phase 3: Routing Strategies
**Goal**: Implement pluggable routing strategies (round-robin, weighted-round-robin, shuffle, failover) selected via configuration
**Depends on**: Phase 2
**Requirements**: ROUT-01, ROUT-02, ROUT-03, ROUT-07
**Success Criteria** (what must be TRUE):
  1. User can select routing strategy in config file (round-robin/weighted-round-robin/shuffle/failover)
  2. Round-robin distributes requests evenly across providers in sequence
  3. Weighted-round-robin distributes proportionally to configured weights
  4. Shuffle randomizes provider selection like dealing cards
  5. Failover tries primary provider first, falls back to secondary on failure
**Plans**: 6 plans in 4 waves

Plans:
- [x] 03-01-PLAN.md - ProviderRouter interface foundation and RoutingConfig
- [x] 03-02-PLAN.md - RoundRobinRouter and ShuffleRouter implementations
- [x] 03-03-PLAN.md - WeightedRoundRobinRouter implementation
- [x] 03-04-PLAN.md - Extensible failover trigger system
- [x] 03-05-PLAN.md - FailoverRouter with parallel retry
- [x] 03-06-PLAN.md - DI integration and handler wiring

### Phase 3.1: Routing Documentation (INSERTED)
**Goal**: Add comprehensive routing documentation to site-docs in all supported languages (EN, DE, ES, JA, ZH-CN, KO)
**Depends on**: Phase 3
**Requirements**: DOC-ROUTE-01
**Success Criteria** (what must be TRUE):
  1. English routing.md created with complete routing strategy documentation
  2. All routing strategies explained (round-robin, weighted-round-robin, shuffle, failover)
  3. RoutingConfig YAML examples with all options documented
  4. Debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider) documented
  5. Failover triggers and timeout configuration documented
  6. All translations updated (DE, ES, JA, ZH-CN, KO)
  7. configuration.md updated with routing section in all languages
**Plans**: 3 plans in 2 waves

Plans:
- [x] 03.1-01-PLAN.md - English routing.md and configuration.md routing section
- [x] 03.1-02-PLAN.md - German and Spanish translations
- [x] 03.1-03-PLAN.md - Japanese, Chinese, and Korean translations

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
**Plans**: 4 plans in 3 waves

Plans:
- [x] 04-01-PLAN.md - Health config foundation (gobreaker install, config structs, errors)
- [x] 04-02-PLAN.md - Circuit breaker state machine with HealthTracker
- [x] 04-03-PLAN.md - Periodic health checker for OPEN state recovery
- [x] 04-04-PLAN.md - DI integration and handler wiring

### Phase 4.1: Health Checker Wiring (INSERTED)
**Goal**: Wire Checker.Start() and RegisterProvider() to make periodic health checks operational
**Depends on**: Phase 4
**Requirements**: None new (closes integration gaps from Phase 4 audit)
**Gap Closure**: Fixes 2 integration gaps and 1 broken E2E flow from v0.0.1-MILESTONE-AUDIT.md
**Success Criteria** (what must be TRUE):
  1. Checker.Start() called during application startup
  2. All configured providers registered with Checker via RegisterProvider()
  3. Periodic health checks run for providers with OPEN circuits
  4. Integration test verifies Checker lifecycle works end-to-end
**Plans**: 1 plan in 1 wave

Plans:
- [x] 04.1-01-PLAN.md - Wire Checker lifecycle (register providers, start checker, add tests)

### Phase 4.2: Config File Cleanup (INSERTED)
**Goal**: Consolidate config files by removing duplicate config.yaml from root, keeping example.yaml as the canonical reference
**Depends on**: Phase 4.1
**Requirements**: None new (closes config gap from extended audit)
**Gap Closure**: Fixes duplicate config files issue from v0.0.1-MILESTONE-AUDIT.md
**Success Criteria** (what must be TRUE):
  1. Only example.yaml exists in root as canonical reference
  2. config.yaml removed from root (development artifact)
  3. All code/doc references to config.yaml updated appropriately
  4. `cc-relay config init` still generates user config correctly
**Plans**: 1 plan in 1 wave

Plans:
- [x] 04.2-01-PLAN.md - Remove config.yaml and update documentation references

### Phase 4.3: Health Configuration Documentation (INSERTED)
**Goal**: Add comprehensive health/circuit-breaker configuration documentation to site-docs
**Depends on**: Phase 4.2
**Requirements**: DOC-HEALTH-01
**Gap Closure**: Fixes documentation gap from v0.0.1-MILESTONE-AUDIT.md
**Success Criteria** (what must be TRUE):
  1. English docs have dedicated health configuration section
  2. All health config options documented (check_interval_seconds, failure_threshold, recovery_timeout_seconds, triggers)
  3. Circuit breaker behavior documented (CLOSED/OPEN/HALF-OPEN states)
  4. All languages updated (DE, ES, JA, ZH-CN, KO)
**Plans**: 2 plans in 2 waves

Plans:
- [x] 04.3-01-PLAN.md - English health configuration documentation
- [x] 04.3-02-PLAN.md - Translate health docs to all languages

### Phase 5: Additional Providers
**Goal**: Support Z.AI (Anthropic-compatible) and Ollama (local models) providers
**Depends on**: Phase 4
**Requirements**: PROV-02, PROV-03
**Success Criteria** (what must be TRUE):
  1. User can configure Z.AI provider with API key and it routes requests correctly
  2. Z.AI model name mappings work (GLM-4.7 appears as model option)
  3. User can configure Ollama provider pointing to local endpoint
  4. Ollama provider handles requests without prompt caching or PDF support
**Plans**: 2 plans in 2 waves

Plans:
- [x] 05-01-PLAN.md - Ollama provider implementation and DI wiring
- [x] 05-02-PLAN.md - Integration tests and provider documentation

### Phase 6: Cloud Providers
**Goal**: Add AWS Bedrock, Azure Foundry, and Vertex AI support with transformer architecture for request/response modification
**Depends on**: Phase 5
**Requirements**: API-04, PROV-04, PROV-05, PROV-06
**Success Criteria** (what must be TRUE):
  1. Provider interface extended with TransformRequest/TransformResponse methods
  2. User can configure AWS Bedrock provider with SigV4 signing
  3. Bedrock requests use model-in-URL and anthropic_version: "bedrock-2023-05-31" in body
  4. User can configure Azure Foundry provider with x-api-key authentication
  5. User can configure Vertex AI provider with OAuth token refresh
  6. Vertex requests use model-in-URL and anthropic_version: "vertex-2023-10-16" in body
  7. Bedrock Event Stream responses converted to SSE format for Claude Code
  8. All cloud providers documented with setup instructions
**Plans**: 5 plans in 4 waves

Plans:
- [x] 06-01-PLAN.md - Provider interface extension and transformation utilities
- [x] 06-02-PLAN.md - Azure Foundry provider (x-api-key auth, standard format)
- [x] 06-03-PLAN.md - Vertex AI provider (OAuth, model-in-URL, body transform)
- [x] 06-04-PLAN.md - AWS Bedrock provider (SigV4, model-in-URL, Event Stream conversion)
- [x] 06-05-PLAN.md - DI wiring, integration tests, and documentation

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
**Plans**: 13 plans in 4 waves

Plans:
- [x] 07-01-PLAN.md - Install dependencies (fsnotify, go-toml/v2) and add TOML struct tags
- [x] 07-02-PLAN.md - Format detection in loader and comprehensive validation
- [x] 07-03-PLAN.md - Config file watcher with debounce
- [x] 07-04-PLAN.md - DI integration for hot-reload with atomic config swap
- [x] 07-05-PLAN.md - Gap closure: English docs with TOML support and hot-reload documentation
- [x] 07-06-PLAN.md - Gap closure: Translate TOML and hot-reload docs to all 5 languages
- [x] 07-07-PLAN.md - Gap closure: Fix Copilot PR review issues (rune conversion, goroutine leak)
- [x] 07-08-PLAN.md - Gap closure: Add TOML tabs to EN docs (caching, getting-started, health, providers, routing)
- [x] 07-09-PLAN.md - Gap closure: Add TOML tabs to DE docs
- [x] 07-10-PLAN.md - Gap closure: Add TOML tabs to ES docs
- [x] 07-11-PLAN.md - Gap closure: Add TOML tabs to JA docs
- [x] 07-12-PLAN.md - Gap closure: Add TOML tabs to KO docs
- [x] 07-13-PLAN.md - Gap closure: Add TOML tabs to ZH-CN docs

### Phase 7.1: Fix CodeQL Weak Crypto Alerts (INSERTED)
**Goal**: Address GitHub CodeQL security alerts for weak cryptographic hashing (CWE-327, CWE-328, CWE-916) in API key handling
**Depends on**: Phase 7
**Requirements**: SEC-01 (CodeQL compliance)
**Gap Closure**: Fixes 3 CodeQL alerts from GitHub code scanning
**Success Criteria** (what must be TRUE):
  1. CodeQL alert #1 (auth/apikey.go:22) resolved - SHA-256 for API key hashing
  2. CodeQL alert #2 (keypool/key.go:71) resolved - SHA-256 for key ID generation
  3. CodeQL alert #3 (proxy/middleware.go:23) resolved - SHA-256 for API key hashing
  4. All existing auth tests pass after refactoring
  5. Constant-time comparison maintained (timing attack prevention)
  6. No functional regressions in API key validation
**Plans**: 1 plan in 1 wave

Plans:
- [x] 07.1-01-PLAN.md - Fix weak crypto hashing alerts with security annotations

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
Phases execute in numeric order: 1 -> 1.1 -> 1.2 -> 1.3 -> 2 -> 2.1 -> 2.2 -> 2.3 -> 3 -> 3.1 -> 4 -> 4.1 -> 4.2 -> 4.3 -> 5 -> 6 -> 7 -> 7.1 -> 8 -> 9 -> 10 -> 11

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Core Proxy (MVP) | 8/9 | In progress | - |
| 1.1 Embedded HA Cache (INSERTED) | 4/4 | Complete | 2026-01-21 |
| 1.2 Cache Documentation (INSERTED) | 1/1 | Complete | 2026-01-21 |
| 1.3 Site Docs Update (INSERTED) | 6/6 | Complete | 2026-01-21 |
| 2. Multi-Key Pooling | 6/6 | Complete | 2026-01-22 |
| 2.1 Multi-Key Pooling Docs (INSERTED) | 1/1 | Complete | 2026-01-21 |
| 2.2 Subscription Token Relay (INSERTED) | 1/1 | Complete | 2026-01-22 |
| 2.3 Samber Libs Refactor (INSERTED) | 12/12 | Complete | 2026-01-23 |
| 3. Routing Strategies | 6/6 | Complete | 2026-01-23 |
| 3.1 Routing Documentation (INSERTED) | 3/3 | Complete | 2026-01-23 |
| 4. Circuit Breaker & Health | 4/4 | Complete | 2026-01-23 |
| 4.1 Health Checker Wiring (INSERTED) | 1/1 | Complete | 2026-01-23 |
| 4.2 Config File Cleanup (INSERTED) | 1/1 | Complete | 2026-01-23 |
| 4.3 Health Config Docs (INSERTED) | 2/2 | Complete | 2026-01-23 |
| 5. Additional Providers | 2/2 | Complete | 2026-01-23 |
| 6. Cloud Providers | 5/5 | Complete | 2026-01-24 |
| 7. Configuration Management | 13/13 | Complete | 2026-01-26 |
| 7.1 Fix CodeQL Weak Crypto (INSERTED) | 1/1 | Complete | 2026-01-26 |
| 8. Observability | 0/TBD | Not started | - |
| 9. gRPC Management API | 0/TBD | Not started | - |
| 10. TUI Dashboard | 0/TBD | Not started | - |
| 11. CLI Commands | 0/TBD | Not started | - |
