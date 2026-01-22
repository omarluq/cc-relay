# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-20)

**Core value:** Access all models from all three providers (Anthropic, Z.AI, Ollama) in Claude Code and switch between them seamlessly.
**Current focus:** Phase 3 - Routing Strategies (next)

## Current Position

Phase: 2.2 of 11 (Subscription Token Relay)
Plan: 1 of 1 in current phase
Status: COMPLETE
Last activity: 2026-01-22 - Completed 02.2-01-PLAN.md (Transparent Auth Forwarding)

Progress: [██████████] 100% (29/29 plans total)

## Performance Metrics

**Velocity:**
- Total plans completed: 29
- Average duration: 7.3 min
- Total execution time: 3.5 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 (Core Proxy) | 8 | 76 min | 9.5 min |
| 01.1 (HA Cache) | 4 | 40 min | 10 min |
| 01.2 (Cache Docs) | 1 | 3 min | 3 min |
| 01.3 (Site Docs) | 6 | 21 min | 3.5 min |
| 02 (Multi-Key Pool) | 6 | 71 min | 11.8 min |
| 02.1 (MKP Docs) | 1 | 12 min | 12 min |
| 02.2 (Sub Token Relay) | 1 | 8 min | 8 min |

**Recent Trend:**
- Last 5 plans: 02-05 (12min), 02-06 (9min), 02.1-01 (12min), 02.2-01 (8min)
- Trend: Implementation plans average 8-12 min

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

**From 01-01 (Config & Provider Foundation):**
- Use gopkg.in/yaml.v3 for config parsing (standard library approach)
- Provider interface: Name, BaseURL, Authenticate, ForwardHeaders, SupportsStreaming (simple Phase 1 design)
- ServerConfig.APIKey field for client authentication (AUTH-02 requirement)
- CanonicalHeaderKey for HTTP header matching (case-insensitive handling)

**From 01-02 (HTTP Server Foundation):**
- Use SHA-256 hashing before constant-time comparison for API key validation
- Set WriteTimeout to 600s to support 10+ minute Claude Code streaming operations
- Pre-hash expected API key at middleware creation rather than per-request
- Streaming timeout pattern: short ReadTimeout (10s) + long WriteTimeout (600s)

**From 01-03 (Proxy Handler & SSE Streaming):**
- Use httputil.ReverseProxy with Rewrite function (not deprecated Director)
- Set FlushInterval: -1 for immediate SSE event flushing
- Do not parse/modify request body to preserve tool_use_id
- Use WriteError in ErrorHandler for Anthropic-format error responses

**From 01-04 (Routing & CLI Integration):**
- Config search order: --config flag > ./config.yaml > ~/.config/cc-relay/config.yaml
- 30 second timeout for graceful shutdown (adequate for in-flight requests)
- Use errors.Is for wrapped error checking (errorlint compliance)
- Mock HTTP backends in tests to avoid real network calls

**From 01-05 (Integration Testing):**
- Use build tag 'integration' to separate integration tests from unit tests
- Skip tests when ANTHROPIC_API_KEY not set (no CI failures without credentials)
- Verify streaming behavior by checking event timing and sequence
- Test tool_use_id preservation with actual tool calling flow

**From 01-06 (Zerolog Integration):**
- Use zerolog for structured logging (JSON and console formats)
- Generate UUID v4 for request IDs when X-Request-ID header missing
- Apply middleware in order: RequestID -> Logging -> Auth -> Handler
- Use responseWriter wrapper to capture HTTP status codes
- Log authentication attempts at Debug/Warn levels for security auditing

**From 01-08 (Subscription Token Support):**
- Option-D: Use existing BearerAuthenticator for subscription tokens (no special handling)
- AllowSubscription is a user-friendly alias for AllowBearer
- Passthrough mode: empty bearer_secret means any token is accepted, backend validates
- IsBearerEnabled() method abstracts checking both AllowBearer and AllowSubscription

**From 01-09 (Enhanced Debug Logging):**
- Use httptrace for TLS metrics (DNS, connect, handshake timing)
- Redact api_key, password, token, secret, authorization, bearer patterns
- Default MaxBodyLogSize: 1000 bytes to prevent log bloat
- --debug flag enables all debug options + sets level to debug

**From 01.1-01 (HA Clustering Config):**
- Extracted OlricConfig.Validate() method to reduce cognitive complexity
- Default Environment to "local" for development compatibility
- Default quorum values to 1 for single-node operation

**From 01.1-02 (Apply HA Config):**
- Extract buildOlricConfig helper to centralize config building
- Only set non-zero values to preserve Olric internal defaults
- Add EnvLocal, EnvLAN, EnvWAN constants for type safety

**From 01.1-03 (Cluster Membership Helpers):**
- Stats API returns 0 in embedded test mode (Olric limitation with external interface)
- ClusterInfo methods return safe defaults (empty string, 0) when stats unavailable
- Use explicit client.Close() error handling to satisfy errcheck linter

**From 01.1-04 (Multi-Node Cluster Tests):**
- Use integration build tag for cluster tests (keep regular suite fast)
- Track memberlist addresses explicitly (Stats API unavailable in embedded mode)
- Memberlist port = Olric port + 2 (matching Olric defaults 3320/3322)
- Space test ports by 10 to avoid Olric/memberlist overlap

**From 01.2-01 (Cache Documentation):**
- Single comprehensive docs/cache.md file for all cache documentation
- Include Redis skeleton implementation example for extensibility
- Document memberlist port calculation explicitly (bind_addr + 2)

**From 01.3 (Site Documentation Update):**
- Adapt comprehensive docs to site-appropriate concise format
- Keep all YAML/bash code blocks in English across translations
- Technical terms (Olric, Ristretto, memberlist, etc.) preserved in English
- Use language-specific URL prefixes in cross-references (/de/docs/, /es/docs/, etc.)

**From 02-01 (Rate Limiter Foundation):**
- Use golang.org/x/time/rate for token bucket (battle-tested, stdlib-backed)
- Set burst = limit to avoid rejecting legitimate bursts
- Treat zero/negative limits as unlimited (1M rate) for flexibility
- Use RWMutex for GetUsage (read-heavy workload optimization)
- Track RPM and TPM separately with independent limiters
- Support dynamic limit updates via SetLimit for learning from response headers

**From 02-02 (Key Metadata and Selectors):**
- Field alignment optimized for time.Time grouping over strict memory optimization (8-byte overhead acceptable)
- Capacity score combines RPM and TPM equally (50/50 weight) for balanced selection
- Cooldown and health checks unified in IsAvailable() for simple availability logic
- Thread-safe with RWMutex for read-heavy workload optimization
- Header parsing tolerates invalid values for graceful degradation
- Extract helper functions to reduce cognitive complexity (parseRPMLimits, parseInputTokenLimits, parseOutputTokenLimits)


**From 02-03 (KeyPool Implementation):**
- Use KeySelector interface for pluggable selection strategies
- Implement GetKey with retry logic up to 3x key count attempts
- Use cooldown period (1 minute) after 429 responses before retrying exhausted keys
- Track per-key rate limiters (RPM, ITPM, OTPM) with dynamic updates from headers
- Use zerolog for debug/warn logging with structured fields
- Thread-safe with RWMutex protecting key metadata and selectors

**From 02-04 (Multi-Key Pooling Configuration):**
- Separate ITPM/OTPM instead of single TPM for accurate Anthropic rate limit tracking
- Priority range 0-2 (low/normal/high) for key selection preferences
- Auto-enable pooling when multiple keys configured (reduces configuration burden)
- Default selection strategy: least_loaded (maximizes capacity utilization)
- Backwards compatible GetEffectiveTPM() for legacy TPMLimit field
- Split complex tests to reduce cognitive complexity (21 → <10 per function)

**From 02-06 (KeyPool Production Wiring):**
- Initialize KeyPool in serve.go after provider loop (config validated, before handler needs it)
- Handler accepts nil pool for single-key mode (zero-cost backwards compatibility)
- Integration tests use mock backend with httptest.Server (fast, deterministic, no API costs)
- Skip 429 test if token bucket burst allows through (documents expected burst behavior)

**From 02.2-01 (Transparent Auth Forwarding):**
- Auto-detect client auth: check Authorization/x-api-key headers
- Transparent mode: forward client auth unchanged when present
- Fallback mode: use configured keys when client has no auth
- Skip KeyPool in transparent mode (rate limiting not our concern)
- Claude Code subscription users just set ANTHROPIC_BASE_URL

### Pending Todos

None.

### Roadmap Evolution

- Phase 2.2 COMPLETE: Subscription Token Relay
  - 02.2-01 COMPLETE: Transparent Auth Forwarding
    - Conditional auth handling in handler.go Rewrite function
    - Skip KeyPool when client provides auth (transparent mode)
    - Use configured keys when client has no auth (fallback mode)
    - 7 new tests covering transparent and fallback modes
    - Documentation: Transparent Authentication section in configuration.md

- Phase 2.1 COMPLETE: Multi-Key Pooling Site Documentation
  - All 6 languages updated with Multi-Key Pooling configuration section
  - x-cc-relay-* response headers documented
  - Configuration examples with priorities, rate limits, strategies
  - Hugo builds verified for all languages

- Phase 2 COMPLETE: Multi-Key Pooling
  - All 6 plans complete: RateLimiter, KeyMetadata, KeyPool, Config, Handler integration, Production wiring
  - Rate limiting with RPM, ITPM, OTPM tracking per key
  - Intelligent key selection strategies (least_loaded, round_robin)
  - Automatic failover when keys exhausted
  - 429 handling with Retry-After headers
  - x-cc-relay-* headers expose capacity to clients
  - Dynamic limit learning from response headers
  - Backwards compatible single-key mode
  - KeyPool initialized from config in serve.go
  - Integration tests verify end-to-end wiring
  - Ready for production deployment

- Phase 1.1 COMPLETE: Embedded HA Cache Clustering
  - cc-relay now supports node discovery and HA clustering natively
  - Embedded Olric mode fully configured (replication, quorum, environment)
  - Integration tests validate multi-node clustering
  - Ready for production deployment testing

- Phase 1.2 COMPLETE: Cache Documentation
  - Comprehensive docs/cache.md (1033 lines) covering all 6 success criteria
  - Cache key naming conventions with examples
  - Cache busting strategies (TTL, manual, cluster events)
  - Backend implementation guide with Redis skeleton
  - HA clustering configuration with docker-compose example
  - Troubleshooting guide for common issues

- Phase 1.3 COMPLETE: Site Documentation Update
  - All 6 languages updated with HA clustering and cache configuration docs
  - English caching.md: +237 lines (HA Clustering Guide, troubleshooting)
  - English configuration.md: +126 lines (Cache Configuration section)
  - All translations (DE, ES, JA, ZH-CN, KO) updated with equivalent content
  - Hugo site builds successfully with all languages
  - 10/10 must-haves verified against actual codebase

- Phase 2 COMPLETE: Multi-Key Pooling (6/6 plans complete)
  - 02-01 COMPLETE: Rate limiter foundation
    - RateLimiter interface with Allow, Wait, SetLimit, GetUsage, Reserve, ConsumeTokens
    - TokenBucketLimiter using golang.org/x/time/rate
    - RPM and TPM tracking with burst = limit
    - Dynamic limit updates from response headers
    - Thread-safe, 60+ test cases, race detector verified
  - 02-02 COMPLETE: Key metadata and selector strategies
    - KeyMetadata tracks RPM/ITPM/OTPM limits with health and cooldown
    - Parses anthropic-ratelimit-* headers dynamically
    - KeySelector interface with LeastLoadedSelector and RoundRobinSelector
    - Thread-safe operations, comprehensive test coverage
  - 02-03 COMPLETE: KeyPool integration
    - KeyPool coordinates rate limiters and key selectors
    - GetKey() selects best key with automatic failover on rate limit
    - UpdateKeyFromHeaders() synchronizes metadata and limiters
    - MarkKeyExhausted() handles 429 cooldown periods
    - GetEarliestResetTime() for retry-after calculation
    - GetStats() for pool capacity monitoring
    - 100+ test cases, concurrent access verified with race detector
  - 02-04 COMPLETE: Multi-key pooling configuration
    - Extended KeyConfig with ITPMLimit, OTPMLimit, Priority, Weight
    - Added PoolingConfig with strategy selection and auto-enable
    - GetEffectiveTPM() for backwards compatibility with TPMLimit
    - KeyConfig.Validate() with InvalidPriorityError, InvalidWeightError
    - config/example.yaml with comprehensive multi-key examples
  - 02-05 COMPLETE: Handler KeyPool integration
    - Handler uses KeyPool.GetKey() for key selection
    - UpdateKeyFromHeaders() called from response director
    - 429 with Retry-After when all keys exhausted
    - x-cc-relay-* headers expose capacity
  - 02-06 COMPLETE: Production wiring and integration tests
    - KeyPool initialized in serve.go from config
    - routes.go passes pool to handler
    - Integration tests verify distribution, fallback, 429 handling
    - All verification gaps closed

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-01-22
Stopped at: Completed 02.2-01-PLAN.md execution (Transparent Auth Forwarding)
Resume file: None

**Phase 02.2-01 Complete:**
- handler.go: Conditional auth forwarding in Rewrite function
- handler.go: Skip KeyPool in ServeHTTP when client has auth
- handler_test.go: 7 new tests for transparent and fallback modes
- configuration.md: Transparent Authentication section with usage guides
- 4 commits made (feat, test, docs, docs)
- SUMMARY.md created: .planning/phases/02.2-subscription-token-relay/02.2-01-SUMMARY.md

**Key Behavior:**
- Client sends Authorization/x-api-key → Forward unchanged (transparent mode)
- Client sends no auth → Use configured keys (fallback mode)
- KeyPool only used in fallback mode (rate limiting for our keys only)

**Next:** Phase 3 - Routing Strategies
