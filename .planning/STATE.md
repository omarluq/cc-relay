# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-20)

**Core value:** Access all models from all three providers (Anthropic, Z.AI, Ollama) in Claude Code and switch between them seamlessly.
**Current focus:** Phase 1.3 In Progress - Site Documentation Update

## Current Position

Phase: 1.3 of 11 (Site Documentation Update)
Plan: 7 of N in current phase (COMPLETE)
Status: Plan 01.3-04 complete
Last activity: 2026-01-21 - Completed 01.3-04-PLAN.md (HA Caching Translation JA/ZH-CN/KO)

Progress: [████████░░] (19 plans completed)

## Performance Metrics

**Velocity:**
- Total plans completed: 19
- Average duration: 7.5 min
- Total execution time: 2.28 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 (Core Proxy) | 8 | 76 min | 9.5 min |
| 01.1 (HA Cache) | 4 | 40 min | 10 min |
| 01.2 (Cache Docs) | 1 | 3 min | 3 min |
| 01.3 (Site Docs) | 6 | 21 min | 3.5 min |

**Recent Trend:**
- Last 5 plans: 01.3-01 (2min), 01.3-05 (3min), 01.3-03 (5min), 01.3-06 (4min), 01.3-04 (3min)
- Trend: Translation plans executing quickly

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

**From 01.3-01 (English Site Caching HA Documentation):**
- Content adapted from docs/cache.md with site-appropriate conciseness
- HA Clustering Guide placed after "Disabled Mode" section for logical flow
- HA troubleshooting added to existing Troubleshooting section

**From 01.3-02 (English Configuration Cache Documentation):**
- Place cache section after logging, before example configurations
- Include both detailed section and complete reference YAML block
- Cross-reference to docs/cache/ for detailed documentation

**From 01.3-05 (Configuration Cache Translation DE/ES):**
- Use German technical terminology with ASCII-safe umlauts (ue/ae/oe)
- Use Spanish technical terminology without accents in headings
- Maintain consistent structure with English source

**From 01.3-04 (HA Caching Translation JA/ZH-CN/KO):**
- Use natural language equivalents for section headings
- Consistent terminology within each language
- All code blocks preserved in English

### Pending Todos

None.

### Roadmap Evolution

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

- Phase 1.3 IN PROGRESS: Site Documentation Update
  - Plan 01 complete: English caching.md updated with HA Clustering Guide
  - Plan 02 complete: English configuration.md updated with cache section
  - Plan 03 complete: German and Spanish caching.md translated
  - Plan 04 complete: Japanese, Chinese, Korean caching.md translated
  - Plan 05 complete: German and Spanish configuration.md translated
  - Plan 06 complete: Japanese, Chinese, Korean configuration.md translated
  - Remaining: Check if any additional plans needed

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-01-21
Stopped at: Plan 01.3-04 complete, ready for next plan
Resume file: None

**Phase 1.3 Progress:**
- Plan 01 (English Caching HA): Complete
- Plan 02 (English Config): Complete
- Plan 03 (DE/ES Caching HA Translation): Complete
- Plan 04 (JA/ZH-CN/KO Caching HA Translation): Complete
- Plan 05 (DE/ES Config Translation): Complete
- Plan 06 (JA/ZH-CN/KO Config Translation): Complete

**Next Steps:**
- Check if Phase 1.3 is complete or if additional plans remain
