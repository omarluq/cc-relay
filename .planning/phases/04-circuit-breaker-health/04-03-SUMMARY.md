---
phase: 04
plan: 03
subsystem: health
tags: [health-checks, circuit-breaker, recovery, http, periodic]
dependency-graph:
  requires: ["04-01", "04-02"]
  provides: ["Checker", "ProviderHealthCheck", "HTTPHealthCheck", "NoOpHealthCheck"]
  affects: ["04-04"]
tech-stack:
  added: []
  patterns: ["periodic-monitoring", "crypto-jitter", "pluggable-interface"]
key-files:
  created:
    - internal/health/checker.go
    - internal/health/checker_test.go
  modified: []
decisions:
  - id: D04-03-01
    choice: "Renamed HealthChecker to Checker to avoid health.HealthChecker stuttering"
    rationale: "Go naming convention - type name used with package prefix"
  - id: D04-03-02
    choice: "crypto/rand for jitter instead of math/rand"
    rationale: "gosec linter requires cryptographically secure randomness"
  - id: D04-03-03
    choice: "ProviderHealthCheck as pluggable interface"
    rationale: "Allows HTTP, NoOp, or future provider-specific health checks"
metrics:
  duration: 7min
  completed: 2026-01-23
---

# Phase 04 Plan 03: Periodic Health Checking Summary

Periodic health checker that runs synthetic health checks during OPEN state to detect provider recovery faster than waiting for full cooldown.

## One-Liner

Periodic health Checker monitors OPEN circuits with pluggable ProviderHealthCheck interface, using crypto/rand jitter to prevent thundering herd.

## What Was Built

### ProviderHealthCheck Interface
```go
type ProviderHealthCheck interface {
    Check(ctx context.Context) error
    ProviderName() string
}
```

Pluggable interface for provider health validation. Two implementations provided:

1. **HTTPHealthCheck** - HTTP-based connectivity validation
   - Configurable URL, method, expected status
   - Default 5s timeout client
   - Returns error on non-2xx response

2. **NoOpHealthCheck** - Always returns healthy
   - For providers without health endpoints
   - Useful as fallback

### Checker (Health Checker)
```go
type Checker struct {
    tracker *Tracker
    config  CheckConfig
    checks  map[string]ProviderHealthCheck
    // ...
}
```

Periodic monitoring of provider health:
- **Start/Stop lifecycle** with WaitGroup for graceful shutdown
- **Jitter** using crypto/rand (0-2s) to prevent thundering herd
- **OPEN-only checking** - only probes providers with open circuits
- **Success recording** - calls tracker.RecordSuccess to accelerate recovery

### Factory Function
```go
func NewProviderHealthCheck(name, baseURL string, client *http.Client) ProviderHealthCheck
```

Creates appropriate health check based on provider configuration.

## Commits

| Hash | Type | Description |
|------|------|-------------|
| 75deddc | feat | ProviderHealthCheck interface and Checker implementation |
| 628de32 | test | Comprehensive health checker tests (95% coverage) |

## Key Files

| File | Purpose |
|------|---------|
| internal/health/checker.go | Checker and ProviderHealthCheck interface |
| internal/health/checker_test.go | 19 tests covering all behaviors |

## Test Coverage

- **Coverage:** 95% of statements
- **Tests:** 19 new tests for checker.go
- **Race detection:** All tests pass with -race flag

Key test scenarios:
- HTTP health check success/failure/timeout
- NoOp always healthy
- Only OPEN circuits checked
- Success recorded on healthy check
- Start/Stop lifecycle with jitter
- Disabled checker does not start
- Concurrent registration safety
- Crypto jitter in valid range

## Integration Points

### Uses from 04-01 (Config)
- `CheckConfig.GetInterval()` for check frequency
- `CheckConfig.IsEnabled()` for enable/disable

### Uses from 04-02 (Tracker)
- `Tracker.GetState()` to identify OPEN circuits
- `Tracker.RecordSuccess()` to accelerate recovery

### Provides for 04-04 (Handler Integration)
- `NewChecker()` constructor
- `RegisterProvider()` to add health checks
- `Start()` / `Stop()` lifecycle methods

## Deviations from Plan

### [Rule 1 - Bug] Lint fixes required

1. **Renamed HealthChecker to Checker** - revive linter flagged stuttering
2. **Used crypto/rand instead of math/rand** - gosec G404 required secure random
3. **Explicit error handling for resp.Body.Close()** - errcheck linter

All deviations were auto-fixed per deviation rules (lint compliance).

## Architecture Notes

### Thundering Herd Prevention
```go
jitter := cryptoRandDuration(2 * time.Second)
ticker := time.NewTicker(interval + jitter)
```

Each Checker instance gets random jitter at startup, spreading health check bursts across multiple relay instances.

### OPEN-Only Checking Logic
```go
state := h.tracker.GetState(name)
if state != StateOpen {
    continue  // Skip CLOSED and HALF-OPEN
}
```

- **CLOSED** circuits are healthy - no check needed
- **HALF-OPEN** circuits already have probe traffic - no synthetic check
- **OPEN** circuits need synthetic checks to detect recovery

### Recovery Acceleration
When health check succeeds for an OPEN circuit:
1. `RecordSuccess()` called on tracker
2. gobreaker receives success signal
3. Circuit may transition to HALF-OPEN sooner

## Next Phase Readiness

**Ready for 04-04:** Handler integration can now:
1. Create Checker with `NewChecker(tracker, cfg, logger)`
2. Register providers with `RegisterProvider(healthCheck)`
3. Start monitoring with `Start()`
4. Stop gracefully with `Stop()`

All interfaces and behaviors are tested and documented.
