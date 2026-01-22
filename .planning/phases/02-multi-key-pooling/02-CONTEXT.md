# Phase 2: Multi-Key Pooling - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Enable multiple API keys per provider with rate limit tracking (RPM/TPM/daily) and intelligent key selection. Keys are pooled to maximize throughput and avoid rate limits. When keys are exhausted, failover to other providers.

</domain>

<decisions>
## Implementation Decisions

### Key Selection Strategy
- **Pluggable interface/adapter pattern** — same architecture as cache system
- Four strategies for initial release:
  1. **Least-loaded** — pick key with most remaining capacity (default)
  2. **Round-robin** — cycle through keys in order
  3. **Random** — random selection from available keys
  4. **Weighted** — distribute based on configured weights
- Default strategy: **least-loaded** when not specified
- Optional **priority flag per key** (high/normal/low) — higher priority keys preferred

### Rate Limit Tracking
- **Pluggable rate limiter interface** — extensible for future strategies
- Track three metrics per key:
  1. RPM (requests per minute)
  2. TPM (tokens per minute)
  3. Daily/monthly quotas
- Initial implementation: **sliding window**
- Limit source: **config + response headers** — start with config values, update dynamically from `x-ratelimit-*` headers
- State persistence: **persist as hint, update from headers** — avoids burst on restart while staying fresh

### Exhaustion Behavior
- **Failover is a core system capability** — baked in, not optional
- When all keys for a provider are exhausted → **try next provider** in fallback chain
- When ALL providers exhausted → **return 429 with retry-after header** (based on earliest key recovery)
- Expose capacity via:
  - **Custom response headers** (`x-cc-relay-*`) on every response
  - **/status endpoint** for detailed capacity info
- When key hits 429 from provider → **honor provider's retry-after** for cooldown

### Configuration Design
- **Integrate with existing config structure** — extend current `keys` array under provider
- Rate limits **optional** — default to unlimited, learn actual limits from headers
- Key selection strategy: **per-provider with global default**
- Hot reload via **fsnotify file watch** — changes apply without restart

### Claude's Discretion
- Weighted strategy weight format (integer vs percentage)
- Exact header names for `x-cc-relay-*` capacity headers
- /status endpoint response format
- Rate limiter internal data structures

</decisions>

<specifics>
## Specific Ideas

- "Similar to the caching style we should probably have an interface and adapter style"
- Failover/fallback should feel like a first-class system feature, not an afterthought
- Config should require minimal setup — keys work without specifying limits

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-multi-key-pooling*
*Context gathered: 2026-01-21*
