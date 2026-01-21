# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-20)

**Core value:** Access all models from all three providers (Anthropic, Z.AI, Ollama) in Claude Code and switch between them seamlessly.
**Current focus:** Phase 1 - Core Proxy (MVP)

## Current Position

Phase: 1 of 11 (Core Proxy)
Plan: 8 of 9 in current phase (Wave 6 extension)
Status: Phase complete (with extensions)
Last activity: 2026-01-21 - Completed 01-08-PLAN.md (Subscription Token Support)

Progress: [██████░░░░] 73% (8/11 plans)

## Performance Metrics

**Velocity:**
- Total plans completed: 8
- Average duration: 10 min
- Total execution time: 1.27 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 (Core Proxy) | 8 | 76 min | 9.5 min |

**Recent Trend:**
- Last 5 plans: 01-04 (8min), 01-05 (15min), 01-06 (17min), 01-09 (8min), 01-08 (8min)
- Trend: Consistent 8-10 min for focused tasks, 15+ min for integration tasks

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

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Session Continuity

Last session: 2026-01-21
Stopped at: Completed 01-08-PLAN.md (Subscription Token Support) - **PHASE 1 FULLY COMPLETE**
Resume file: None

**Phase 1 (Core Proxy) Milestone:**
- All 8 plans completed successfully (5 core + 3 Wave 5/6 extensions)
- Full end-to-end proxy working with real Anthropic API
- Production-ready structured logging with zerolog
- Request correlation and operational visibility
- Enhanced debug logging with TLS metrics and --debug flag
- Subscription token support for Claude Code Pro/Team users
- Ready to begin Phase 2 (Multi-key pooling & rate limiting)
