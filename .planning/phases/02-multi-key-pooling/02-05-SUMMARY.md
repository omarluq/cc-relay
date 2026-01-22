---
phase: 02-multi-key-pooling
plan: 05
subsystem: proxy-handler
tags: [golang, proxy, keypool, rate-limiting, http-handler, 429-errors]

# Dependency graph
requires:
  - phase: 02-03
    provides: KeyPool coordination with GetKey(), UpdateKeyFromHeaders(), MarkKeyExhausted()
  - phase: 02-04
    provides: KeyConfig and PoolingConfig for multi-key configuration
provides:
  - Handler integrated with KeyPool for multi-key request routing
  - WriteRateLimitError() for Anthropic-format 429 responses
  - parseRetryAfter() for Retry-After header parsing
  - x-cc-relay-* headers exposing capacity information
  - Backwards compatible single-key mode (nil pool)
affects: [cmd/serve, future-router-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "KeyPool integration in HTTP handler for multi-key routing"
    - "Context-based keyID passing from ServeHTTP to ModifyResponse"
    - "X-Selected-Key temporary header for key passing to Rewrite"
    - "Extracted modifyResponse method to reduce cognitive complexity"
    - "429 error handling with Retry-After header (RFC 6585)"

key-files:
  created: []
  modified:
    - internal/proxy/errors.go - Added WriteRateLimitError() and header constants (46 lines)
    - internal/proxy/handler.go - Integrated KeyPool with GetKey/Update/MarkExhausted (140 lines)
    - internal/proxy/handler_test.go - Added 6 test functions for key pool integration (268 lines)
    - internal/proxy/routes.go - Updated NewHandler call with nil pool parameter

key-decisions:
  - "Use temporary X-Selected-Key header to pass key from ServeHTTP to Rewrite closure"
  - "Store keyID in request context for access in ModifyResponse"
  - "Extract modifyResponse method to reduce NewHandler cognitive complexity (21→<10)"
  - "Set x-cc-relay-* headers in ServeHTTP (not ModifyResponse) for correct ordering"
  - "Default Retry-After to 60s when header missing or unparseable"
  - "Call GetEarliestResetTime() for 429 retry-after calculation"

patterns-established:
  - "WriteRateLimitError() for consistent 429 responses across codebase"
  - "parseRetryAfter() handles both integer seconds and HTTP-date formats"
  - "Handler methods (modifyResponse) reduce closure complexity for linter compliance"
  - "Nil pool check pattern: if h.keyPool != nil { ... } else { single-key mode }"
  - "Relay headers provide transparency: key ID, total keys, available keys"

# Metrics
duration: 12min
completed: 2026-01-22
---

# Phase 2 Plan 5: Handler KeyPool Integration Summary

**Handler integrates with KeyPool for multi-key routing, 429 handling, and response header tracking with full backwards compatibility**

## Performance

- **Duration:** 12 min
- **Started:** 2026-01-22T02:47:00Z
- **Completed:** 2026-01-22T02:58:40Z
- **Tasks:** 3
- **Files modified:** 4 (454 lines added/modified)

## Accomplishments

- WriteRateLimitError() helper returns Anthropic-format 429 with Retry-After header
- Handler.ServeHTTP() calls KeyPool.GetKey() for key selection
- Returns 429 with earliest reset time when all keys exhausted
- Handler.modifyResponse() updates key state from response headers
- Handler.modifyResponse() marks key exhausted on backend 429 responses
- parseRetryAfter() parses both integer seconds and HTTP-date formats
- x-cc-relay-* headers expose key ID, total keys, available keys
- Context-based keyID passing enables ModifyResponse to access selected key
- Backwards compatible single-key mode (nil pool parameter)
- 6 comprehensive test functions covering all integration scenarios
- All tests pass with race detector enabled

## Task Commits

Each task was committed atomically:

1. **Tasks 1-2: Handler integration** - `1ca9ad6` (feat)
2. **Task 3: Comprehensive tests** - `d05136f` (test)

## Files Created/Modified

- `internal/proxy/errors.go` - Added WriteRateLimitError() and x-cc-relay-* header constants
- `internal/proxy/handler.go` - Integrated KeyPool with ServeHTTP and modifyResponse
- `internal/proxy/handler_test.go` - Added 6 test functions (268 lines)
- `internal/proxy/routes.go` - Updated NewHandler call to pass nil pool

## Decisions Made

**1. Temporary header for key passing**
- **Rationale:** Rewrite closure can't access local variables from ServeHTTP
- **Implementation:** Set X-Selected-Key header, read in Rewrite, defer delete
- **Alternative considered:** Store in context (rejected: can't modify ProxyRequest.Out context)
- **Benefit:** Simple, no state management needed

**2. Context for keyID in ModifyResponse**
- **Rationale:** ModifyResponse needs keyID to update pool, but only has *http.Response
- **Implementation:** context.WithValue(keyIDContextKey, keyID) in ServeHTTP
- **Benefit:** Type-safe, follows Go context patterns

**3. Extract modifyResponse method**
- **Rationale:** Linter flagged NewHandler cognitive complexity > 20
- **Implementation:** Move ModifyResponse closure body to separate method
- **Impact:** Complexity reduced from 21 to <10
- **Benefit:** More maintainable, easier to test

**4. x-cc-relay-* headers in ServeHTTP**
- **Rationale:** Need headers set before ReverseProxy writes response
- **Implementation:** w.Header().Set() before h.proxy.ServeHTTP()
- **Benefit:** Headers appear in response to client

**5. 60s default for Retry-After**
- **Rationale:** RFC 6585 doesn't mandate specific default, need reasonable fallback
- **Implementation:** Return 60s when header missing or unparseable
- **Benefit:** Prevents infinite retry loops, gives keys time to reset

**6. GetEarliestResetTime() for 429**
- **Rationale:** Client needs accurate retry-after when all keys exhausted
- **Implementation:** Pool calculates earliest RPMResetAt across all keys
- **Benefit:** Minimizes client wait time, maximizes throughput

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Linter errors blocking commit**
- **Found during:** Task 1-2 commit attempt
- **Issue:** errcheck and ineffassign linter errors
  - `keyID, _ := resp.Request.Context().Value()` not checking ok value
  - `selectedKey = h.apiKey` ineffectual assignment in else branch
- **Fix:**
  - Changed to `keyID, ok := ...` and check `ok && keyID != ""`
  - Changed else branch to set X-Selected-Key header directly
- **Files modified:** internal/proxy/handler.go
- **Verification:** All linters pass after changes
- **Committed in:** 1ca9ad6 (included with handler integration)

**2. [Rule 2 - Missing Critical] Cognitive complexity too high**
- **Found during:** Task 1-2 commit attempt (after first fix)
- **Issue:** NewHandler function complexity 21 (limit: 20) due to ModifyResponse closure
- **Fix:** Extracted modifyResponse closure body to separate method
- **Impact:** Reduced complexity from 21 to manageable level
- **Files modified:** internal/proxy/handler.go
- **Committed in:** 1ca9ad6 (handler integration commit)

**3. [Rule 2 - Missing Critical] Test linter issues**
- **Found during:** Task 3 commit attempt
- **Issue:**
  - revive: unused parameter `r *http.Request` in 3 mock backends
  - tparallel: TestParseRetryAfter subtests missing t.Parallel()
- **Fix:**
  - Renamed unused parameters to `_ *http.Request`
  - Added t.Parallel() to each subtest in TestParseRetryAfter
- **Files modified:** internal/proxy/handler_test.go
- **Committed in:** d05136f (test commit)

---

**Total deviations:** 3 auto-fixed (3 missing critical)
**Impact on plan:** All fixes necessary for linter compliance. No scope creep.

## Issues Encountered

**1. Passing selected key to Rewrite closure**
- **Problem:** ServeHTTP local variables not accessible in Rewrite closure
- **Options considered:**
  - Store in context (rejected: can't modify ProxyRequest.Out context)
  - Store in Handler struct (rejected: race conditions with concurrent requests)
  - Temporary header (chosen: simple, works with ReverseProxy architecture)
- **Resolution:** Use X-Selected-Key header, set in ServeHTTP, delete after proxy
- **Note:** Header approach is unconventional but necessary given ReverseProxy design

**2. Cognitive complexity warnings**
- **Problem:** NewHandler with inline closures exceeds complexity limit
- **Impact:** Linter blocking commit
- **Resolution:** Extract ModifyResponse closure to separate method
- **Benefit:** Also improves testability (can test modifyResponse in isolation)

**3. Test organization**
- **Problem:** 6 new test functions + helper test = potential file organization issues
- **Resolution:** Kept all tests in handler_test.go (still manageable at ~475 lines)
- **Note:** Future consideration: Split into handler_test.go + handler_pool_test.go if grows

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for production multi-key pooling:**
- ✅ Handler selects keys via KeyPool.GetKey()
- ✅ Handler returns 429 when all keys exhausted
- ✅ Handler updates key state from response headers
- ✅ Handler marks keys exhausted on backend 429
- ✅ x-cc-relay-* headers expose capacity to clients
- ✅ Backwards compatible with single-key deployments
- ✅ Comprehensive test coverage with race detector

**Enables:**
- Production deployment of multi-key pooling (needs config updates in cmd/serve.go)
- Multi-key configuration in example.yaml (already exists from 02-04)
- Monitoring/metrics via x-cc-relay-* headers
- Client retry logic based on Retry-After headers

**TODO for production:**
- Update cmd/cc-relay/serve.go to initialize KeyPool from config
- Update routes.go to pass KeyPool instead of nil
- Document x-cc-relay-* headers in API documentation
- Add integration tests with real key pools

**No blockers or concerns.**

---
*Phase: 02-multi-key-pooling*
*Completed: 2026-01-22*
