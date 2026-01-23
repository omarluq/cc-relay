---
phase: 04-circuit-breaker-health
plan: 02
title: "Circuit Breaker State Machine Implementation"
completed: 2026-01-23
duration: 11 min
subsystem: health
tags: ["circuit-breaker", "gobreaker", "health-tracking", "resilience"]

dependency_graph:
  requires:
    - "04-01 (health config and errors)"
  provides:
    - "CircuitBreaker wrapper with gobreaker TwoStepCircuitBreaker"
    - "Tracker for per-provider circuit management"
    - "IsHealthyFunc closure for router integration"
  affects:
    - "04-03 (handler integration)"
    - "04-04 (health checker probing)"

tech_stack:
  added:
    - "github.com/sony/gobreaker/v2 v2.4.0"
  patterns:
    - "TwoStepCircuitBreaker for decoupled request/response"
    - "Lazy initialization with double-checked locking"
    - "Closure-based health checks for router integration"

files:
  created:
    - internal/health/circuit.go
    - internal/health/circuit_test.go
    - internal/health/tracker.go
    - internal/health/tracker_test.go
  modified: []

decisions:
  - id: tracker-naming
    choice: "Renamed HealthTracker to Tracker (avoid health.HealthTracker stutter)"
    rationale: "Go naming convention - package name provides context"
  - id: state-reexport
    choice: "Re-export gobreaker.State as health.State with constants"
    rationale: "Allows external code to use state constants without gobreaker import"
  - id: lazy-circuit-init
    choice: "Lazy initialization with double-checked locking"
    rationale: "Avoids upfront cost, circuits created on first request to provider"

metrics:
  test_coverage: 93%
  tests_added: 21
  lines_of_code: 300 (approx)
---

# Phase 04 Plan 02: Circuit Breaker State Machine Implementation Summary

Implemented circuit breaker state machine using sony/gobreaker TwoStepCircuitBreaker with per-provider health tracking via the Tracker struct.

## One-Liner

Circuit breaker wrapper around gobreaker.TwoStepCircuitBreaker with lazy per-provider Tracker and IsHealthyFunc closure for router integration.

## What Was Built

### CircuitBreaker (circuit.go)
- Wraps `gobreaker.TwoStepCircuitBreaker[struct{}]` with simplified interface
- State constants: `StateClosed`, `StateOpen`, `StateHalfOpen`
- `NewCircuitBreaker(name, cfg, logger)` - creates breaker with configured thresholds
- `Allow()` - returns done callback, ErrCircuitOpen if open
- `State()` - returns current circuit state
- `ReportSuccess()` / `ReportFailure(err)` - convenience methods
- `ShouldCountAsFailure(statusCode, err)` - helper for HTTP evaluation
  - 5xx and 429 count as failures
  - 4xx (except 429) do NOT count as failures
  - context.Canceled does NOT count as failure

### Tracker (tracker.go)
- Manages per-provider circuit breakers with lazy initialization
- Thread-safe with sync.RWMutex (double-checked locking pattern)
- `NewTracker(cfg, logger)` - creates tracker
- `GetOrCreateCircuit(providerName)` - lazy circuit creation
- `IsHealthyFunc(providerName)` - returns `func() bool` for ProviderInfo.IsHealthy
- `GetState(providerName)` - returns circuit state (StateClosed for unknown)
- `RecordSuccess/RecordFailure` - outcome reporting with debug logging
- `AllStates()` - snapshot for monitoring

## Key Integration Points

```go
// Router integration - the IsHealthyFunc closure
tracker := health.NewTracker(cfg.Health.CircuitBreaker, &logger)

providers := []router.ProviderInfo{
    {
        Provider:  anthropicProvider,
        IsHealthy: tracker.IsHealthyFunc("anthropic"), // Closure wired here
        Priority:  1,
    },
}

// Handler integration - recording outcomes
tracker.RecordSuccess("anthropic")
// or
if health.ShouldCountAsFailure(resp.StatusCode, err) {
    tracker.RecordFailure("anthropic", err)
} else {
    tracker.RecordSuccess("anthropic")
}
```

## State Transitions

```
CLOSED ──[threshold failures]──> OPEN
   │                               │
   └──[success]──┐                 │
                 │   [timeout]     ▼
                 │        ┌───────HALF-OPEN
                 └────────┤           │
                          │ [probes succeed]
                          └───────────┘
                          │ [probe fails]
                          └──────> OPEN
```

## Deviations from Plan

### [Rule 3 - Blocking] Plan 01 prerequisite not executed
- **Found during:** Task start
- **Issue:** Plan 02 depends on config.go, errors.go, and gobreaker which hadn't been set up
- **Fix:** Verified plan 01 was already committed in previous session (commits bbdb2c6, 3179373)
- **Impact:** No additional work needed, foundation already present

## Verification Results

```bash
go build ./internal/health/...          # PASS
go test -v -race ./internal/health/...  # PASS (all 21 tests)
go test -cover ./internal/health/...    # 93% coverage
```

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 3c3dfc5 | CircuitBreaker wrapper with gobreaker TwoStepCircuitBreaker |
| 2 | 393656f | Tracker managing per-provider circuit breakers |

## Next Phase Readiness

**Ready for 04-03:** Handler Integration
- CircuitBreaker.Allow() ready for use in proxy handler
- ShouldCountAsFailure() helper for HTTP response evaluation
- Tracker.RecordSuccess/RecordFailure for outcome reporting

**Ready for 04-04:** Health Checker
- Tracker provides GetState() and AllStates() for monitoring
- CircuitBreaker.State() available for health check logic
