# Phase 4: Circuit Breaker & Health - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Health tracking per provider with circuit breaker state machine (CLOSED/OPEN/HALF-OPEN) for automatic failure detection and recovery. Integrates with Phase 3's router via the `ProviderInfo.IsHealthy` closure that currently returns `true` stub.

This phase makes health tracking real — routers will skip unhealthy providers automatically.

</domain>

<decisions>
## Implementation Decisions

### Failure Criteria
- **Triggering events:** 5xx status codes + 429 rate limits + timeouts + connection errors
- **Failure threshold:** Configurable via config field (default: 5 consecutive failures)
- **4xx handling:** Ignore client errors (except 429) — they don't indicate provider health problems

### State Transitions
- **OPEN duration:** Configurable cooldown (default: 30 seconds)
- **HALF-OPEN probes:** 3 requests to test recovery
- **Probe failure rule:** Majority — if 2 of 3 probes fail → back to OPEN
- **Recovery rule:** Strict — all 3 probes must succeed to transition to CLOSED
- **Asymmetric design:** Tolerant on failure detection, strict on recovery confirmation

### Health Exposure
- **Debug headers:** Add `X-CC-Relay-Health` header showing state (CLOSED/OPEN/HALF-OPEN) when `routing.debug=true`
- **Router integration:** Circuit breaker provides `IsHealthy func() bool` closure — router calls it via `FilterHealthy` (matches Phase 3 design)
- **Persistence:** None — all circuits start CLOSED on startup (simple, no state file)

### Recovery Behavior
- **Probe type:** Synthetic health checks (not user traffic) — protects users from failed requests
- **Health check design:** Provider-specific implementations (some providers have health endpoints, others need minimal API calls)
- **Traffic after recovery:** Immediate full traffic — no gradual ramp-up (sufficient confidence from 3 successful probes)
- **Check timing:** Periodic during OPEN state (every 10s) — faster recovery than waiting for full cooldown

### Claude's Discretion
- Counter reset behavior (reset on success vs sliding window — recommend reset on success for simplicity)
- Log levels for state transitions (recommend: WARN for circuit opening, INFO for closing)
- Specific health check implementations per provider (research during planning)

</decisions>

<specifics>
## Specific Ideas

- Circuit breaker integrates with existing router's `ProviderInfo.IsHealthy` closure from Phase 3
- Health headers follow same pattern as `X-CC-Relay-Strategy` and `X-CC-Relay-Provider` (debug-only)
- Provider-specific health checks: Anthropic may need minimal messages request, others may have dedicated endpoints
- Periodic checks during OPEN (every 10s) means faster recovery for transient issues

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-circuit-breaker-health*
*Context gathered: 2026-01-23*
