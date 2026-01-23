# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-20)

**Core value:** Access all models from all three providers (Anthropic, Z.AI, Ollama) in Claude Code and switch between them seamlessly.
**Current focus:** Phase 2.3 - Codebase Refactor with Samber Libraries (IN PROGRESS)

## Current Position

Phase: 2.3 of 11 (Codebase Refactor with Samber Libraries)
Plan: 9 of 12 in current phase
Status: In progress
Last activity: 2026-01-22 - Completed 02.3-08-PLAN.md (Refactoring Agents and Pattern Skills)

Progress: [████░░░░░░] 39/46 plans total (Phase 2.3: 9/12)

## Performance Metrics

**Velocity:**
- Total plans completed: 39
- Average duration: 7.6 min
- Total execution time: 5.3 hours

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
| 02.3 (Samber Refactor) | 9 | 99 min | 11.0 min |

**Recent Trend:**
- Last 5 plans: 02.3-05 (5min), 02.3-06 (10min), 02.3-07a (28min), 02.3-07b (11min), 02.3-08 (18min)
- Trend: Documentation phase completing pattern capture

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

**From 02.3-08 (Refactoring Agents and Pattern Skills):**
- Separate library skills from pattern skills (library = API, pattern = when/how)
- Agents reference both library and pattern skills for complete guidance
- Pattern skills reference agents for automation
- Include cc-relay file paths in examples (makes examples verifiable)
- Include anti-patterns section in all files (prevent common mistakes)
- streams.md includes future use cases (ro not yet used in cc-relay)

**From 02.3-07b (DI Container Serve.go Integration):**
- Eager config validation in NewContainer (fail fast on startup errors)
- runWithGracefulShutdown helper for signal handling with DI cleanup
- serve.go reduced from ~130 to ~70 lines of main logic
- Coverage: serve 85.2%, di 90.4%

**From 02.3-07a (DI Container Foundation):**
- Wrapper service types for type safety (ConfigService, CacheService, etc.)
- Lazy initialization for all services (created on first request)
- ShutdownerWithError interface for graceful cleanup (CacheService, ServerService)
- Named value for config path (ConfigPathKey constant)
- Coverage: 91.2% for di package

**From 02.3-06 (Mo Monads Integration):**
- Adapted plan: Config uses zero-value semantics, not pointer fields; added Option helpers instead of struct changes
- mo.Option helpers for config: GetTimeoutOption(), GetMaxConcurrentOption(), GetRPMLimitOption(), etc.
- mo.Result methods for auth: ValidateResult() on all authenticators with ValidationError type
- mo.Result methods for keypool: GetKeyResult() with KeySelection struct, UpdateKeyFromHeadersResult()
- Coverage maintained: config 90%, auth 100%, keypool 93.6%

**From 02.3-05 (Proxy/Config lo Refactoring):**
- Config package has no production loops to refactor (all 11 loops in test files)
- Proxy package: lo.Map (nested), lo.FlatMap, lo.ForEach+lo.Entries, lo.Reduce, lo.SliceToMap+lo.FilterMap
- Remaining production loops (6 total in cmd/, keypool/) are appropriately imperative
- Coverage maintained: proxy 83.4%, config 86.5%

**From 02.3-04 (Providers/Auth lo Refactoring):**
- lo.ForEach + lo.Entries for map iteration (http.Header)
- lo.Map for slice transformation (model IDs to Model structs)
- lo.Reduce for chain validation with short-circuit on first valid result
- Coverage maintained: providers 91.2%, auth 100%

**From 02.3-03 (Keypool lo Refactoring):**
- Keep initialization loop with side effects (logging, populating maps) as imperative loop
- Keep round_robin.go loop as-is (index-based wraparound semantics not suitable for lo)
- lo.Filter allocation acceptable for code clarity in LeastLoadedSelector
- lo.MaxBy comparison: returns true if 'a' should replace 'b' as new max
- Fixed IsAvailable() mutex ordering bug during refactor

**From 02.3-02 (Install Samber Libraries):**
- Created internal/pkg/functional package to anchor samber imports (prevents go mod tidy removal)
- gopter added for property-based testing (RESEARCH.md recommendation)
- samber/ro v0.2.0 included with cautious usage guidance (pre-1.0 stability)

**From 02.3-01 (Codebase Architecture):**
- ARCHITECTURE.md created documenting component architecture
- Identified 76 for-range loops as refactoring targets for samber/lo
- Test coverage baseline established (avg 81%, cmd at 13.6%)

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

### Known Gaps

- **Phase 2.1 Translation Gap**: Multi-key pooling docs only in English. DE, ES, JA, KO, ZH-CN missing. Fix later.

### Roadmap Evolution

- Phase 2.3 IN PROGRESS: Codebase Refactor with Samber Libraries
  - 02.3-01 COMPLETE: Codebase architecture mapping
    - ARCHITECTURE.md documenting component structure
    - Dependency graph visualized
    - Test coverage baseline established
  - 02.3-02 COMPLETE: Install samber libraries and create skills
    - samber/lo v1.52.0, do/v2 v2.0.0, mo v1.16.0, ro v0.2.0 installed
    - gopter v0.2.11 for property-based testing
    - 4 skill files created (1857 lines total)
    - internal/pkg/functional package anchors imports
  - 02.3-03 COMPLETE: Refactor keypool with samber/lo
    - pool.go: lo.Filter, lo.FilterMap+lo.MinBy, lo.Reduce
    - least_loaded.go: lo.Filter + lo.MaxBy
    - Benchmarks created: GetStats 0 allocs, LeastLoadedSelector 1 alloc
    - Test coverage maintained at 93.3%
  - 02.3-04 COMPLETE: Refactor providers/auth with samber/lo
    - providers/base.go: lo.ForEach+lo.Entries, lo.Map
    - auth/chain.go: lo.Reduce for chain validation
    - Coverage maintained: providers 91.2%, auth 100%
  - 02.3-05 COMPLETE: Refactor proxy/config with samber/lo
    - proxy: lo.Map (nested), lo.FlatMap, lo.ForEach+lo.Entries, lo.Reduce, lo.SliceToMap+lo.FilterMap
    - config: No production loops to refactor (all in test files)
    - Coverage maintained: proxy 83.4%, config 86.5%
  - 02.3-06 COMPLETE: Mo Monads Integration
    - mo.Option helpers for config nullable semantics
    - mo.Result methods for auth chain (ValidateResult, ValidationError)
    - mo.Result methods for keypool (GetKeyResult, KeySelection)
    - Coverage maintained: config 90%, auth 100%, keypool 93.6%
  - 02.3-07a COMPLETE: DI Container Foundation
    - Container wrapper with generic Invoke/MustInvoke helpers
    - Service wrappers: ConfigService, CacheService, ProviderMapService, etc.
    - Graceful shutdown with ShutdownerWithError interface
    - Coverage: 91.2%
  - 02.3-07b COMPLETE: DI Container Serve.go Integration
    - serve.go refactored to use di.NewContainer()
    - runWithGracefulShutdown helper extracted
    - Eager config validation (fail fast)
    - Coverage: serve 85.2%, di 90.4%
  - 02.3-08 COMPLETE: Refactoring Agents and Pattern Skills
    - 3 refactoring agents: loop-to-lo, error-to-result, inject-di (904 lines)
    - 4 pattern skills: di-patterns, error-handling, collections, streams (1834 lines)
    - All with cc-relay code examples and cross-references
  - 02.3-09 NEXT: Test coverage improvement (cmd package)
  - 02.3-10: Performance benchmarking
  - 02.3-11: Final verification and documentation
  - 02.3-12: Phase completion and handoff

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

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-01-22
Stopped at: Completed 02.3-08-PLAN.md (Refactoring Agents and Pattern Skills)
Resume file: None

**Phase 02.3-08 Complete:**
- .claude/agents/loop-to-lo.md: Convert for-range loops to lo functions (216 lines)
- .claude/agents/error-to-result.md: Convert (value, error) to mo.Result (323 lines)
- .claude/agents/inject-di.md: Wire services into DI container (365 lines)
- .claude/skills/di-patterns.md: DI patterns with cc-relay examples (424 lines)
- .claude/skills/error-handling.md: Result monad patterns (418 lines)
- .claude/skills/collections.md: lo function selection guide (463 lines)
- .claude/skills/streams.md: ro reactive patterns (529 lines)
- 2 commits made: 811ea1e, a8e2bab
- SUMMARY.md created: .planning/phases/02.3-codebase-refactor-samber-libs/02.3-08-SUMMARY.md

**Phase 02.3-07b Complete:**
- cmd/cc-relay/serve.go: Replaced manual wiring with di.NewContainer(), extracted runWithGracefulShutdown()
- cmd/cc-relay/di/container.go: Added eager config validation in NewContainer
- cmd/cc-relay/serve_test.go: Added DI integration tests, graceful shutdown tests
- cmd/cc-relay/di/container_test.go: Updated for eager config validation
- 2 commits made: d9a63fb, 49c9e7c
- Coverage: serve 85.2%, di 90.4%
- SUMMARY.md created: .planning/phases/02.3-codebase-refactor-samber-libs/02.3-07b-SUMMARY.md

**Phase 02.3-07a Complete:**
- cmd/cc-relay/di/container.go: Container wrapper, Invoke/MustInvoke generics, Shutdown methods
- cmd/cc-relay/di/providers.go: Service wrappers (ConfigService, CacheService, etc.), RegisterSingletons
- cmd/cc-relay/di/container_test.go: Container creation, invoke, shutdown, health check tests
- cmd/cc-relay/di/providers_test.go: Provider function tests, dependency order tests
- 2 commits made: c6a4485, 02c666d
- Coverage: 91.2%
- SUMMARY.md created: .planning/phases/02.3-codebase-refactor-samber-libs/02.3-07a-SUMMARY.md

**lo Patterns Established:**
| Pattern | Usage | Example |
|---------|-------|---------|
| lo.Filter | Filter collections | `lo.Filter(keys, func(k *Key, _ int) bool { return k.IsAvailable() })` |
| lo.Reduce | Aggregate values | `lo.Reduce(keys, reducer, initialValue)` |
| lo.MaxBy | Find maximum | comparison returns true if 'a' should replace 'b' |
| lo.MinBy | Find minimum | comparison returns true if 'a' < 'b' |
| lo.FilterMap | Filter + transform | Combined operation in single pass |
| lo.ForEach | Side-effect iteration | `lo.ForEach(items, func(item T, _ int) { ... })` |
| lo.Entries | Map to slice | `lo.Entries(map[K]V)` returns `[]lo.Entry[K,V]` |
| lo.Map | Transform slice | `lo.Map(items, func(item T, _ int) U { return ... })` |
| lo.FlatMap | Flatten nested slices | `lo.FlatMap(providers, func(p Provider, _ int) []Model { return p.ListModels() })` |
| lo.SliceToMap | Slice to map | `lo.SliceToMap(entries, func(e Entry) (K, V) { return e.Key, e.Value })` |

**Samber Libraries Installed:**
| Library | Version | Purpose |
|---------|---------|---------|
| samber/lo | v1.52.0 | Functional collection utilities |
| samber/do/v2 | v2.0.0 | Dependency injection |
| samber/mo | v1.16.0 | Monads (Option, Result) |
| samber/ro | v0.2.0 | Reactive streams (pre-1.0) |
| gopter | v0.2.11 | Property-based testing |

**mo Patterns Established:**
| Pattern | Usage | Example |
|---------|-------|---------|
| mo.Option | Nullable semantics | `cfg.GetTimeoutOption().OrElse(defaultTimeout)` |
| mo.Result | Error composability | `auth.ValidateResult(req).Map(transform).Get()` |
| ValidationError | Typed auth errors | `auth.NewValidationError(authType, message)` |
| KeySelection | Bundled key data | `KeySelection{KeyID: id, APIKey: key}` |
| Result.Map | Transform success | `result.Map(func(v T) (T, error) { ... })` |
| Result.OrElse | Default on error | `result.OrElse(defaultValue)` |

**do Patterns Established:**
| Pattern | Usage | Example |
|---------|-------|---------|
| do.New() | Create root container | `injector := do.New()` |
| do.Provide | Register lazy service | `do.Provide(i, NewConfig)` |
| do.ProvideValue | Register pre-built value | `do.ProvideValue(i, cfg)` |
| do.ProvideNamedValue | Register named value | `do.ProvideNamedValue(i, "key", value)` |
| do.Invoke | Resolve service | `svc, err := do.Invoke[*ConfigService](i)` |
| do.MustInvoke | Resolve or panic | `svc := do.MustInvoke[*ConfigService](i)` |
| do.InvokeNamed | Resolve by name | `val := do.MustInvokeNamed[string](i, "key")` |
| ShutdownerWithError | Graceful cleanup | `func (s *Svc) Shutdown() error { ... }` |

**Agents Created:**
| Agent | Purpose | Lines |
|-------|---------|-------|
| loop-to-lo | Convert for-range loops to lo functions | 216 |
| error-to-result | Convert (value, error) to mo.Result | 323 |
| inject-di | Wire services into DI container | 365 |

**Pattern Skills Created:**
| Skill | Purpose | Lines |
|-------|---------|-------|
| di-patterns | DI patterns (singleton, transient, request-scoped) | 424 |
| error-handling | Result monad and Railway-Oriented Programming | 418 |
| collections | lo function selection and patterns | 463 |
| streams | ro reactive stream patterns | 529 |

**Next:** 02.3-09 - Test Coverage Improvement (cmd package)
