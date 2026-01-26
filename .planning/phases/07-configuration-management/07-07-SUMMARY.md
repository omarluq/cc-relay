---
phase: 07-configuration-management
plan: 07-07
subsystem: config
tags: [bugfix, copilot-review, code-quality, goroutine-leak]
depends:
  requires: []
  provides: [copilot-fixes]
  affects: [07-08, 07-09, 07-10, 07-11, 07-12, 07-13]
tech-stack:
  added: []
  patterns: [fmt.Sprintf-for-index, context-cancellation-pattern]
key-files:
  created: []
  modified:
    - internal/config/validator.go
    - internal/config/validator_test.go
    - internal/config/watcher.go
decisions:
  - id: use-fmt-sprintf
    choice: fmt.Sprintf over string concatenation
    reason: Cleaner code, handles indices >= 10 correctly
  - id: context-cancel-pattern
    choice: Context cancellation in timer callback
    reason: Prevents goroutine leak when watcher closes before timer fires
metrics:
  duration: 5 min
  completed: 2026-01-26
---

# Phase 07 Plan 07: Fix Copilot Code Review Issues Summary

Fixed all critical code quality issues from GitHub Copilot PR #59 review.

**One-liner:** Fixed rune('0'+index) bug for indices >= 10, improved string construction with fmt.Sprintf, and prevented goroutine leak in watcher timer callback.

## Changes Made

### 1. Fixed `string(rune('0'+index))` Bug (Critical)

**Problem:** `string(rune('0'+index))` only works for indices 0-9. For index >= 10, it produces incorrect Unicode characters (e.g., index=10 produces ":" instead of "10").

**Files modified:**
- `internal/config/validator.go` - 3 occurrences in prefix functions
- `internal/config/validator_test.go` - 1 occurrence in test assertion

**Solution:** Replaced with `fmt.Sprintf("%d", index)` and `strconv.Itoa(i)` for correct string conversion at any index value.

### 2. Improved String Construction Performance

**Before:**
```go
return "provider[" + providerName + "].keys[" + string(rune('0'+index)) + "]." + field
```

**After:**
```go
return fmt.Sprintf("provider[%s].keys[%d].%s", providerName, index, field)
```

Single-operation string construction is cleaner and avoids multiple temporary string allocations.

### 3. Fixed Goroutine Leak in Watcher Timer Callback

**Problem:** When watcher context is canceled, `cleanupTimer` stops the timer. But if the timer has already fired, the goroutine from `time.AfterFunc` may still call `triggerReload()` after watcher cleanup.

**Solution:** Added context-based cancellation:
1. Added `ctx` and `cancel` fields to Watcher struct
2. Timer callback checks `w.ctx.Done()` before calling `triggerReload()`
3. `Close()` calls `w.cancel()` to signal pending callbacks to exit

```go
*timer = time.AfterFunc(w.debounceDelay, func() {
    select {
    case <-w.ctx.Done():
        return // Watcher is closed, don't trigger reload
    default:
    }
    timerMu.Lock()
    *pending = false
    timerMu.Unlock()
    w.triggerReload()
})
```

### 4. Fixed Struct Field Alignment

Used `fieldalignment` tool to optimize Watcher struct memory layout (reduced from 112 to 56 pointer bytes).

## Commits

| Hash | Type | Description |
|------|------|-------------|
| e5f0d23 | fix | Replace rune('0'+index) with fmt.Sprintf for index-to-string |
| b44a30e | test | Fix rune('0'+i) pattern in validator_test.go |
| b2605e6 | fix | Prevent goroutine leak in watcher timer callback |

## Verification

- [x] No `string(rune('0'+...))` patterns remain in config package production code
- [x] Prefix functions use `fmt.Sprintf` for string construction
- [x] Watcher timer callback checks context before triggering reload
- [x] All tests pass (go test ./internal/config/...)
- [x] Linter passes (golangci-lint run ./internal/config/...)

## Test Results

```
ok  github.com/omarluq/cc-relay/internal/config  0.564s
```

All 70+ config package tests pass.

## Deviations from Plan

None - plan executed exactly as written.

## Additional Findings

Found similar `rune('0'+...)` patterns in test files:
- `internal/cache/ro_cache_test.go` - Uses `%10` modulo, currently safe
- `internal/cache/olric_test.go` - Uses `%10` modulo, currently safe
- `internal/cache/ristretto_test.go` - Uses `%10` modulo, currently safe
- `internal/keypool/pool_bench_test.go` - Uses clever two-digit pattern, safe

These are test-only and use patterns that ensure values stay within 0-9 range. Not critical to fix but could be cleaned up in future to prevent confusion.

## Next Phase Readiness

All Copilot PR #59 issues resolved. Ready for TOML documentation plans (07-08 through 07-13).
