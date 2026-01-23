---
phase: 03-routing-strategies
plan: 01
subsystem: routing
tags: [router, interface, config, foundation]
dependency-graph:
  requires: [02-multi-key-pool]
  provides: [ProviderRouter-interface, RoutingConfig, FilterHealthy]
  affects: [03-02-failover, 03-03-round-robin, 03-04-weighted, 03-05-shuffle]
tech-stack:
  added: []
  patterns: [interface-abstraction, factory-function, sentinel-errors]
key-files:
  created:
    - internal/router/router.go
    - internal/router/router_test.go
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
decisions:
  - key: default-routing-strategy
    value: failover
    rationale: Safest default - tries providers in priority order until one succeeds
  - key: health-check-closure
    value: IsHealthy func() bool
    rationale: Allows lazy health integration with Phase 4 health tracking
  - key: mirror-keypool-pattern
    value: ProviderRouter mirrors KeySelector
    rationale: Consistent codebase patterns for maintainability
metrics:
  duration: 7m 34s
  completed: 2026-01-23
---

# Phase 03 Plan 01: ProviderRouter Interface Foundation Summary

**One-liner:** ProviderRouter interface with failover default and mo.Option timeout helper

## What Was Done

### Task 1: Create ProviderRouter interface and types
**Commit:** e26e7d0

Created `internal/router/router.go` (116 lines) with:

1. **Package documentation** explaining provider-level routing vs key selection
2. **ProviderRouter interface** with Select and Name methods (mirrors KeySelector)
3. **ProviderInfo struct** wrapping providers with routing metadata:
   - Provider: providers.Provider
   - Weight: int (for weighted strategies)
   - Priority: int (for failover ordering)
   - IsHealthy: func() bool (closure for health tracking)
4. **Strategy constants**:
   - StrategyRoundRobin = "round_robin"
   - StrategyWeightedRoundRobin = "weighted_round_robin"
   - StrategyShuffle = "shuffle"
   - StrategyFailover = "failover"
5. **Sentinel errors**:
   - ErrNoProviders
   - ErrAllProvidersUnhealthy
6. **FilterHealthy helper** using lo.Filter
7. **NewRouter factory** (stub returning not-implemented for now)

### Task 2: Add RoutingConfig to config package
**Commit:** 4abe9a8

Modified `internal/config/config.go` to add:

1. **RoutingConfig struct**:
   - Strategy: string (routing algorithm)
   - FailoverTimeout: int (milliseconds for failover attempts)
   - Debug: bool (enable routing debug headers)
2. **Config.Routing field** in main Config struct
3. **GetEffectiveStrategy()** returning "failover" as default
4. **GetFailoverTimeoutOption()** using mo.Option pattern
5. **IsDebugEnabled()** helper method
6. Fixed field alignment per govet linter

## Deviations from Plan

None - plan executed exactly as written.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Default strategy | "failover" | Safest default - tries providers in priority order |
| Health check design | Closure func() bool | Allows lazy integration with Phase 4 health tracking |
| Interface pattern | Mirror KeySelector | Consistent patterns across routing layers |
| Timeout handling | mo.Option[time.Duration] | Matches existing config patterns (GetTimeoutOption, etc.) |

## Files Changed

| File | Lines | Change |
|------|-------|--------|
| internal/router/router.go | +116 | New file - interface, types, helpers |
| internal/router/router_test.go | +182 | New file - comprehensive tests |
| internal/config/config.go | +45 | RoutingConfig struct and helpers |
| internal/config/config_test.go | +162 | Tests for RoutingConfig methods |

## Test Coverage

- internal/router: All tests pass (7 test functions)
- internal/config: All tests pass (16 test functions including new Routing tests)

## Next Phase Readiness

### Blocking Issues
None.

### Ready For
- 03-02: Failover strategy implementation (can start immediately)
- 03-03: Round-robin strategy implementation
- 03-04: Weighted round-robin strategy implementation
- 03-05: Shuffle strategy implementation

### Integration Points
- ProviderInfo.IsHealthy closure ready for Phase 4 health tracking integration
- NewRouter factory ready to return real implementations as plans complete
- RoutingConfig.GetFailoverTimeoutOption() ready for failover implementation
