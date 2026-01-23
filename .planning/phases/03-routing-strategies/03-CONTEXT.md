# Phase 3: Routing Strategies - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Implement pluggable routing strategies (round-robin, shuffle, failover) that users can select via configuration to control how requests distribute across providers and keys. Strategies check provider health but don't implement health tracking (that's Phase 4).

</domain>

<decisions>
## Implementation Decisions

### Strategy Selection
- Config location: Nested under `routing: { strategy: "...", ... }` (not top-level)
- Default strategy: `failover` when not specified
- Scope: Global default + per-provider override supported
- Key routing: Keys within a provider use the same strategy as the provider

### Failover Behavior
- Triggers: 5xx errors, timeouts, 429 rate limits, provider-side errors
- Extensible: Build trigger system so new conditions can be added easily
- Smart parallel retry: After initial failure + timeout, continue retrying primary while ALSO trying fallback provider
- First success wins: Cancel other in-flight requests when one succeeds
- Priority restoration: After recovery, subsequent requests resume priority order naturally
- Timeout: Configurable `failover_timeout` in config, default 5 seconds

### Load Distribution
- `round-robin`: Strict sequential (A → B → C → A → B → C)
- `weighted-round-robin`: Weighted distribution (A:3, B:2, C:1 → A gets 3x traffic)
- `shuffle`: Shuffled queue approach (shuffle order, cycle through, reshuffle) - NOT pure random
- Health awareness: All strategies skip providers marked unhealthy (via `IsHealthy()` check)
- Phase 4 dependency: Routing checks health status but doesn't implement tracking

### Strategy Visibility
- Response headers: Only in debug mode (--debug flag or config option), default no headers
- Headers when enabled: `x-cc-relay-strategy`, `x-cc-relay-provider`
- Logging (Info level): Provider name, key ID, status code
- Logging (Debug level): Full trace - strategy, selection reason, alternatives skipped, failover attempts
- Inspection: Both CLI command (`cc-relay config show routing`) AND status endpoint (`/status` includes routing info)

### Claude's Discretion
- Exact implementation of shuffled queue algorithm
- How to structure the extensible failover trigger system
- Parallel retry cancellation mechanism
- Status endpoint response format

</decisions>

<specifics>
## Specific Ideas

- "If we ping Anthropic 3 times and it's failing, continue retry but also switch to Z.AI so user workflow isn't stopped"
- Conversation transfer on failover should be seamless (shouldn't be too complex since it's just HTTP proxying)
- Shuffled queue like dealing cards - everyone gets one before anyone gets seconds

</specifics>

<deferred>
## Deferred Ideas

- **Health tracking and circuit breaker** → Phase 4
  - Marking providers healthy/unhealthy
  - Background health check pinging to detect recovery
  - Circuit breaker state machine (CLOSED/OPEN/HALF-OPEN)
  - Automatic recovery probing

</deferred>

---

*Phase: 03-routing-strategies*
*Context gathered: 2026-01-23*
