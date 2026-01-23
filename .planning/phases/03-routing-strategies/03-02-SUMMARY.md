---
phase: 03-routing-strategies
plan: 02
subsystem: routing
tags: [round-robin, shuffle, router, load-balancing]
dependency-graph:
  requires: ["03-01"]
  provides: ["RoundRobinRouter", "ShuffleRouter", "NewRouter factory"]
  affects: ["03-03", "03-04", "03-05"]
tech-stack:
  added: []
  patterns: ["atomic counter for thread-safety", "sync.Mutex for state protection", "Fisher-Yates shuffle"]
file-tracking:
  key-files:
    created:
      - internal/router/round_robin.go
      - internal/router/round_robin_test.go
      - internal/router/shuffle.go
      - internal/router/shuffle_test.go
    modified:
      - internal/router/router.go
      - internal/router/router_test.go
decisions:
  - id: "D03-02-01"
    title: "Atomic vs Mutex for RoundRobin"
    choice: "atomic.AddUint64 for thread-safety"
    rationale: "Mirrors keypool pattern, lower overhead than mutex for simple counter"
  - id: "D03-02-02"
    title: "Shuffle approach"
    choice: "Fisher-Yates via lo/mutable.Shuffle"
    rationale: "Standard unbiased shuffle algorithm, consistent with samber/lo usage"
metrics:
  duration: 11min
  completed: 2026-01-23
---

# Phase 03 Plan 02: RoundRobin and Shuffle Router Strategies Summary

**One-liner:** Implemented RoundRobinRouter with atomic counter and ShuffleRouter with Fisher-Yates "dealing cards" pattern.

## What Was Built

### RoundRobinRouter (internal/router/round_robin.go)
- Uses `atomic.AddUint64` for thread-safe index increment without mutex overhead
- Filters unhealthy providers before selection
- Distributes requests evenly in sequential order
- Returns `ErrNoProviders` for empty slice
- Returns `ErrAllProvidersUnhealthy` when all providers fail health checks

### ShuffleRouter (internal/router/shuffle.go)
- Uses `sync.Mutex` for state protection (shuffled order, position, lastLen)
- Uses `lo/mutable.Shuffle` for Fisher-Yates randomization
- "Dealing cards" pattern: each provider gets exactly one request before any gets seconds
- Reshuffles when: first call, provider count changes, or deck exhausted

### NewRouter Factory Update (internal/router/router.go)
- `NewRouter("round_robin", timeout)` returns `*RoundRobinRouter`
- `NewRouter("shuffle", timeout)` returns `*ShuffleRouter`
- `NewRouter("weighted_round_robin", timeout)` returns `*WeightedRoundRobinRouter` (pre-existing)
- `NewRouter("failover", timeout)` returns "not yet implemented" error (for 03-05)

## Key Implementation Details

### RoundRobinRouter Pattern
```go
// Get next index atomically
nextIndex := atomic.AddUint64(&r.index, 1) - 1
healthyLen := uint64(len(healthy))
//nolint:gosec // Safe: modulo ensures result is within int range
idx := int(nextIndex % healthyLen)
return healthy[idx], nil
```

### ShuffleRouter Pattern
```go
needsReshuffle := len(r.shuffledOrder) == 0 || // first time
    len(healthy) != r.lastLen ||               // provider count changed
    r.position >= len(r.shuffledOrder)         // exhausted

if needsReshuffle {
    r.reshuffle(len(healthy))
}

idx := r.shuffledOrder[r.position]
r.position++
return healthy[idx], nil
```

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | b74c201 | feat(03-02): implement RoundRobinRouter strategy |
| 2 | ecfc020 | feat(03-02): implement ShuffleRouter strategy |
| 3 | c39615b | feat(03-02): update NewRouter factory for round_robin and shuffle |

## Test Coverage

- **RoundRobinRouter**: 9 tests including concurrent safety with race detector
  - Empty providers, all unhealthy, even distribution, sequential order
  - Skips unhealthy, concurrent safety, nil IsHealthy handling

- **ShuffleRouter**: 12 tests including concurrent safety with race detector
  - Empty providers, all unhealthy, dealing cards pattern
  - Reshuffles when exhausted, reshuffles on count change
  - Skips unhealthy, concurrent safety, even distribution over many rounds

- **NewRouter Factory**: 5 tests
  - Creates correct router types for each strategy
  - Returns error for unknown and unimplemented strategies

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed pre-existing lint errors in triggers.go**
- **Found during:** Task 1 commit
- **Issue:** triggers.go had package comment detachment and missing string constant
- **Fix:** Moved comment after package statement, added TriggerStatusCode/TriggerTimeout/TriggerConnection constants
- **Files modified:** internal/router/triggers.go

**2. [Rule 3 - Blocking] Fixed pre-existing missing import in weighted_round_robin_test.go**
- **Found during:** Task 1 build
- **Issue:** Missing `errors` import
- **Fix:** Added `errors` import
- **Files modified:** internal/router/weighted_round_robin_test.go

**3. [Rule 3 - Blocking] Fixed pre-existing malformed test function in triggers_test.go**
- **Found during:** Task 1 build
- **Issue:** TestShouldFailover_StatusCostruct had broken function definition (missing parentheses)
- **Fix:** Auto-fixed by system (detected and corrected)
- **Files modified:** internal/router/triggers_test.go

## Next Phase Readiness

**Blockers:** None

**Ready for:**
- 03-03: WeightedRoundRobinRouter (already implemented, needs integration)
- 03-04: Priority-based routing
- 03-05: FailoverRouter with circuit breaker integration

## Success Criteria Verification

- [x] RoundRobinRouter distributes requests sequentially with atomic counter
- [x] ShuffleRouter shuffles like dealing cards with Fisher-Yates
- [x] Both strategies skip unhealthy providers
- [x] Both strategies are thread-safe (race detector passes)
- [x] NewRouter creates correct strategy instances
- [x] All tests pass including race detector
