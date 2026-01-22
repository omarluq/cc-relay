# Phase 2 Plan 4: Multi-Key Pooling Configuration Summary

**One-liner:** Extended KeyConfig with separate ITPM/OTPM limits, priority, weight, and PoolingConfig for provider-level strategy selection

## What Was Built

### Configuration Extensions

**KeyConfig Enhancements:**
- `ITPMLimit` - Input tokens per minute limit (separate from output)
- `OTPMLimit` - Output tokens per minute limit (better accuracy)
- `Priority` (0-2) - Selection priority: 0=low, 1=normal, 2=high
- `Weight` (0+) - For weighted selection strategy
- `TPMLimit` (deprecated) - Maintained for backwards compatibility via GetEffectiveTPM()

**PoolingConfig (new):**
- `Strategy` - Selection strategy: least_loaded, round_robin, random, weighted
- `Enabled` - Explicit pooling control (auto-enables with multiple keys)

**Methods Added:**
- `KeyConfig.GetEffectiveTPM()` - Backwards compatibility for legacy TPMLimit
- `KeyConfig.Validate()` - Validates key, priority range, weight non-negative
- `ProviderConfig.GetEffectiveStrategy()` - Returns strategy with default fallback
- `ProviderConfig.IsPoolingEnabled()` - Auto-enables pooling with multiple keys

**Error Types:**
- `InvalidPriorityError` - Priority must be 0-2
- `InvalidWeightError` - Weight must be >= 0

### Tests

**Test Coverage:**
- `TestKeyConfig_GetEffectiveTPM` - 6 scenarios (ITPM/OTPM, legacy, fallback)
- `TestKeyConfig_Validate_ValidCases` - 6 valid configurations
- `TestKeyConfig_Validate_ErrorCases` - 4 error validation scenarios
- `TestProviderConfig_GetEffectiveStrategy` - 5 strategy selection scenarios
- `TestProviderConfig_IsPoolingEnabled` - 7 pooling enable/disable scenarios

Split from single test to reduce cognitive complexity (21 → <10 each function).

### Example Configuration

Created `config/example.yaml` with:
- Multi-key pooling example (3 keys with different priorities)
- Single-key example (pooling disabled)
- Rate limit configuration patterns (explicit vs learn-from-headers)
- Strategy selection examples
- Comprehensive inline documentation
- Environment variable expansion examples
- Z.AI and Ollama provider examples

## Key Decisions

| Decision | Rationale | Impact |
|----------|-----------|--------|
| Separate ITPM/OTPM instead of single TPM | Anthropic rate limits are separate, allows more accurate tracking | Better capacity utilization |
| GetEffectiveTPM() for backwards compatibility | Existing code using TPMLimit continues to work | Smooth migration path |
| Priority range 0-2 | Simple enough for most use cases, room for expansion | Easy to understand |
| Default strategy: least_loaded | Maximizes capacity utilization | Better throughput |
| Auto-enable pooling with multiple keys | Reduces configuration burden | Fewer surprises |
| Split Validate tests | Reduce cognitive complexity from 21 to <10 | Passes linter, maintainable |

## Implementation Notes

### Rate Limit Learning

Keys can operate without explicit rate limits (0 = unlimited/learn). The keypool will learn limits dynamically from anthropic-ratelimit-* response headers.

### Priority Semantics

- **High (2)**: Used first when multiple keys have capacity
- **Normal (1)**: Standard priority, used when high-priority keys exhausted
- **Low (0)**: Backup keys, used only when others unavailable

### Strategy Selection

- **least_loaded** (default): Picks key with most available capacity (RPM+TPM combined)
- **round_robin**: Rotates through keys evenly
- **random**: Random selection
- **weighted**: Distributes based on Weight field

### Configuration Patterns

**Production (explicit limits):**
```yaml
keys:
  - key: ${KEY_1}
    rpm_limit: 50
    itpm_limit: 30000
    otpm_limit: 10000
    priority: 2
```

**Development (learn from headers):**
```yaml
keys:
  - key: ${KEY_1}
    priority: 1  # rpm_limit, itpm_limit, otpm_limit default to 0
```

## Verification

All success criteria met:

1. ✅ KeyConfig has rpm_limit, itpm_limit, otpm_limit, priority, weight fields
2. ✅ KeyConfig.Validate() rejects invalid configurations
3. ✅ ProviderConfig.GetEffectiveStrategy() returns strategy with default
4. ✅ Backwards compatible (tpm_limit still works via GetEffectiveTPM)
5. ✅ Example config demonstrates multi-key pooling clearly
6. ✅ All config tests pass

```bash
go build ./internal/config/...  # ✅ Compiles
go test -v ./internal/config/...  # ✅ All tests pass
python3 -c "import yaml; yaml.safe_load(open('config/example.yaml'))"  # ✅ Valid YAML
golangci-lint run ./internal/config/...  # ✅ 0 issues
```

## Files Modified

- `internal/config/config.go` - Extended KeyConfig, added PoolingConfig, validation methods
- `internal/config/config_test.go` - Split Validate test, added 5 new test functions
- `internal/keypool/pool_test.go` - Fixed unused parameter (unrelated lint issue)
- `config/example.yaml` (created) - Comprehensive multi-key configuration examples

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Error type naming convention**
- **Found during:** Task 1 commit attempt
- **Issue:** ErrInvalidPriority/ErrInvalidWeight didn't match errname linter (XxxError format)
- **Fix:** Renamed to InvalidPriorityError/InvalidWeightError
- **Commit:** Part of first task commit

**2. [Rule 2 - Missing Critical] Range value copy optimization**
- **Found during:** Task 1 commit attempt
- **Issue:** serve.go copied 136-byte ProviderConfig structs in range loop
- **Fix:** Changed `for _, p := range` to `for i := range` with pointer access
- **Files modified:** cmd/cc-relay/serve.go
- **Commit:** Part of first task commit

**3. [Rule 2 - Missing Critical] Cognitive complexity reduction**
- **Found during:** Task 2 commit attempt
- **Issue:** TestKeyConfig_Validate had cognitive complexity 21 (limit: 20)
- **Fix:** Split into TestKeyConfig_Validate_ValidCases and TestKeyConfig_Validate_ErrorCases
- **Commit:** Part of test commit

**4. [Rule 1 - Bug] Pointer receiver in test calls**
- **Found during:** Task 2 testing
- **Issue:** Calling Validate() on value instead of pointer (method has pointer receiver)
- **Fix:** Changed `KeyConfig{...}.Validate()` to `(&KeyConfig{...}).Validate()`
- **Commit:** Part of test commit

**5. [Rule 3 - Blocking] Unrelated lint issue in keypool tests**
- **Found during:** Task 2/3 commit attempt
- **Issue:** newTestHeaders rpm parameter always receives 50 (unparam lint error)
- **Fix:** Removed rpm parameter, hardcoded "50" in function
- **Files modified:** internal/keypool/pool_test.go
- **Commit:** Part of final commit

## Next Phase Readiness

**Ready for:** 02-05 (Example.yaml multi-provider configuration completion)

**Dependencies met:**
- Config structs support all multi-key fields
- Validation ensures configuration correctness
- Example demonstrates intended usage patterns

**No blockers** - Configuration layer complete for key pooling implementation.

## Metadata

```yaml
phase: 02-multi-key-pooling
plan: 04
type: execute
subsystem: configuration
tags: [config, multi-key, rate-limits, validation, pooling]
dependencies:
  requires: [02-02]  # Key metadata and selectors
  provides: [multi-key-config, pooling-config, rate-limit-fields]
  affects: [02-05]  # Will use this config structure
tech-stack:
  added: []
  patterns: [backwards-compatibility, auto-enable, priority-levels]
key-files:
  created:
    - config/example.yaml
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - internal/keypool/pool_test.go
    - cmd/cc-relay/serve.go
decisions:
  - id: ITPM-OTPM-SPLIT
    decision: "Separate ITPM/OTPM instead of single TPM"
    rationale: "Anthropic tracks input/output separately, allows accurate capacity calculation"
  - id: PRIORITY-RANGE
    decision: "Priority range 0-2 (low/normal/high)"
    rationale: "Simple enough for most use cases, room for expansion if needed"
  - id: AUTO-ENABLE-POOLING
    decision: "Auto-enable pooling when multiple keys configured"
    rationale: "Reduces configuration burden, matches expected behavior"
  - id: DEFAULT-LEAST-LOADED
    decision: "Default strategy: least_loaded"
    rationale: "Maximizes capacity utilization across keys"
metrics:
  duration: 9  # minutes
  completed: "2026-01-22"
  lines_added: 350
  lines_modified: 45
  tests_added: 25
```
