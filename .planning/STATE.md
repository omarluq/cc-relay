# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-20)

**Core value:** Access all models from all three providers (Anthropic, Z.AI, Ollama) in Claude Code and switch between them seamlessly.
**Current focus:** Phase 1 - Core Proxy (MVP)

## Current Position

Phase: 1 of 11 (Core Proxy)
Plan: 5 of 5 in current phase
Status: Phase complete
Last activity: 2026-01-21 — Completed 01-05-PLAN.md (Integration Testing)

Progress: [████░░░░░░] 45% (5/11 plans)

## Performance Metrics

**Velocity:**
- Total plans completed: 5
- Average duration: 8 min
- Total execution time: 0.72 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01 (Core Proxy) | 5 | 43 min | 9 min |

**Recent Trend:**
- Last 5 plans: 01-01 (11min), 01-02 (8min), 01-03 (4min), 01-04 (8min), 01-05 (15min)
- Trend: Stable with variance (simple tasks 4-8min, complex tasks 11-15min)

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

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Session Continuity

Last session: 2026-01-21
Stopped at: Completed 01-05-PLAN.md (Integration Testing) - **PHASE 1 COMPLETE**
Resume file: None

**Phase 1 (Core Proxy) Milestone:**
- All 5 plans completed successfully
- Full end-to-end proxy working with real Anthropic API
- Ready to begin Phase 2 (Multi-key pooling & rate limiting)
