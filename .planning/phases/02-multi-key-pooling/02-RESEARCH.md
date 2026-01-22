# Phase 2: Multi-Key Pooling - Research

**Researched:** 2026-01-21
**Domain:** Rate limiting, concurrent pool management, key selection strategies
**Confidence:** HIGH

## Summary

Multi-key pooling requires three core subsystems: (1) a rate limiter to track RPM/TPM/daily limits per key using sliding window algorithm, (2) a key selector that picks the best available key based on configured strategy (least-loaded, round-robin, random, weighted), and (3) a pool coordinator that manages concurrent access and handles exhaustion with failover.

The standard Go approach uses `golang.org/x/time/rate` for token bucket rate limiting, `sync.RWMutex` for read-heavy pool access, and interface-based adapters (matching the existing cache pattern) for pluggable strategies. Anthropic provides rate limit headers (`anthropic-ratelimit-*-limit/remaining/reset`) in RFC3339 format that should update pool state dynamically, with 429 responses including `retry-after` headers for cooldown coordination.

**Primary recommendation:** Use token bucket (golang.org/x/time/rate) for per-key rate limiting, RWMutex-protected key pool with pluggable selector interface, and dynamic limit learning from response headers to avoid configuration burden.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/time/rate | Latest | Token bucket rate limiter | Official Go extended library, production-ready, concurrent-safe |
| sync.RWMutex | stdlib | Pool synchronization | Read-heavy workload optimization (multiple readers, single writer) |
| gopkg.in/yaml.v3 | Latest | Config parsing with env vars | Already used in Phase 1, supports ${VAR} expansion |
| net/http | stdlib | RFC3339 timestamp parsing | Standard for `anthropic-ratelimit-*-reset` header parsing |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/fsnotify/fsnotify | v1.7+ | File watching for hot reload | Already available, config rotation requirement (AUTH-05) |
| time.Ticker | stdlib | Cleanup/maintenance tasks | Periodic reset of sliding window state |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| golang.org/x/time/rate | Custom sliding window | More control but 10x implementation complexity, no battle-testing |
| RWMutex | sync.Map | Worse performance for write-heavy scenarios (per benchmarks), no type safety |
| Token bucket | Leaky bucket | More complex backpressure handling, no standard library implementation |
| Interface adapter | Direct implementation | Less extensible, harder to test, doesn't match cache pattern |

**Installation:**
```bash
go get golang.org/x/time/rate
go get github.com/fsnotify/fsnotify
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── ratelimit/           # Rate limiting subsystem
│   ├── limiter.go       # Interface: RateLimiter
│   ├── token_bucket.go  # Implementation using x/time/rate
│   ├── sliding_window.go # Optional: sliding window implementation
│   └── factory.go       # Factory: NewRateLimiter(strategy)
├── keypool/             # Key pool management
│   ├── pool.go          # KeyPool: manages keys, enforces limits
│   ├── selector.go      # Interface: KeySelector
│   ├── least_loaded.go  # Selector: pick key with most capacity
│   ├── round_robin.go   # Selector: cycle through keys
│   ├── random.go        # Selector: random selection
│   ├── weighted.go      # Selector: weighted distribution
│   └── key_metadata.go  # KeyMetadata: tracks limits, usage, health
└── config/
    └── config.go        # Extended: KeyConfig, rate limits
```

### Pattern 1: Adapter Interface (Matches Cache System)
**What:** Pluggable strategy pattern with factory function
**When to use:** When multiple implementations exist (4 key selectors, 2 rate limiters)
**Example:**
```go
// Source: Existing cache pattern (internal/cache/cache.go)
// RateLimiter interface (adapter pattern)
type RateLimiter interface {
    Allow(ctx context.Context) (bool, error)
    Wait(ctx context.Context) error
    SetLimit(rpm, tpm int)
    GetUsage() (requestsUsed, tokensUsed int)
}

// Factory function
func NewRateLimiter(strategy string, rpm, tpm int) (RateLimiter, error) {
    switch strategy {
    case "token_bucket":
        return newTokenBucketLimiter(rpm, tpm)
    case "sliding_window":
        return newSlidingWindowLimiter(rpm, tpm)
    default:
        return nil, fmt.Errorf("unknown strategy: %s", strategy)
    }
}

// KeySelector interface (adapter pattern)
type KeySelector interface {
    Select(keys []*KeyMetadata) (*KeyMetadata, error)
    Name() string
}

// KeyPool coordinates selector + limiters
type KeyPool struct {
    mu       sync.RWMutex
    keys     []*KeyMetadata
    selector KeySelector
}

func (p *KeyPool) GetKey(ctx context.Context) (string, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()

    // Selector picks key (strategy-dependent)
    keyMeta, err := p.selector.Select(p.keys)
    if err != nil {
        return "", err
    }

    // Rate limiter enforces limits
    if !keyMeta.limiter.Allow(ctx) {
        return "", ErrRateLimitExceeded
    }

    return keyMeta.APIKey, nil
}
```

### Pattern 2: Token Bucket Rate Limiting
**What:** golang.org/x/time/rate.Limiter tracks requests/tokens per minute
**When to use:** Per-key rate limiting (POOL-02, POOL-03)
**Example:**
```go
// Source: https://pkg.go.dev/golang.org/x/time/rate
import "golang.org/x/time/rate"

type TokenBucketLimiter struct {
    requestLimiter *rate.Limiter // RPM tracking
    tokenLimiter   *rate.Limiter // TPM tracking
}

func newTokenBucketLimiter(rpm, tpm int) *TokenBucketLimiter {
    // Convert per-minute to per-second rate
    // Burst = limit (allow full minute's worth instantly)
    return &TokenBucketLimiter{
        requestLimiter: rate.NewLimiter(rate.Limit(rpm/60.0), rpm),
        tokenLimiter:   rate.NewLimiter(rate.Limit(tpm/60.0), tpm),
    }
}

func (l *TokenBucketLimiter) Allow(ctx context.Context) (bool, error) {
    // Check both request and token limits
    if !l.requestLimiter.Allow() {
        return false, nil
    }
    // Note: Token count estimation happens before request
    // Update token limiter after response with actual count
    return true, nil
}

func (l *TokenBucketLimiter) ConsumeTokens(n int) error {
    // Called after request completes with actual token count
    return l.tokenLimiter.WaitN(context.Background(), n)
}
```

### Pattern 3: Dynamic Limit Learning from Headers
**What:** Update rate limits from `anthropic-ratelimit-*` response headers
**When to use:** Avoid hardcoding limits, adapt to tier changes (POOL-04)
**Example:**
```go
// Source: https://platform.claude.com/docs/en/api/rate-limits
// Header format:
// anthropic-ratelimit-requests-limit: 50
// anthropic-ratelimit-requests-remaining: 42
// anthropic-ratelimit-requests-reset: 2026-01-21T19:42:00Z
// anthropic-ratelimit-input-tokens-limit: 30000
// anthropic-ratelimit-input-tokens-remaining: 27000
// anthropic-ratelimit-input-tokens-reset: 2026-01-21T19:42:00Z

func (p *KeyPool) UpdateFromHeaders(keyID string, headers http.Header) error {
    p.mu.Lock()
    defer p.mu.Unlock()

    keyMeta := p.findKey(keyID)
    if keyMeta == nil {
        return ErrKeyNotFound
    }

    // Parse limits (use these as source of truth)
    if limit := headers.Get("anthropic-ratelimit-requests-limit"); limit != "" {
        rpm, _ := strconv.Atoi(limit)
        keyMeta.RPMLimit = rpm
    }

    if limit := headers.Get("anthropic-ratelimit-input-tokens-limit"); limit != "" {
        itpm, _ := strconv.Atoi(limit)
        keyMeta.ITPMLimit = itpm
    }

    if limit := headers.Get("anthropic-ratelimit-output-tokens-limit"); limit != "" {
        otpm, _ := strconv.Atoi(limit)
        keyMeta.OTPMLimit = otpm
    }

    // Parse remaining capacity
    if remaining := headers.Get("anthropic-ratelimit-requests-remaining"); remaining != "" {
        rpm, _ := strconv.Atoi(remaining)
        keyMeta.RPMRemaining = rpm
    }

    // Parse reset time (RFC3339 format)
    if reset := headers.Get("anthropic-ratelimit-requests-reset"); reset != "" {
        t, _ := time.Parse(time.RFC3339, reset)
        keyMeta.RPMResetAt = t
    }

    // Update rate limiter with new limits
    keyMeta.limiter.SetLimit(keyMeta.RPMLimit, keyMeta.ITPMLimit+keyMeta.OTPMLimit)

    return nil
}
```

### Pattern 4: Least-Loaded Key Selection
**What:** Pick key with most remaining capacity (RPM + TPM weighted)
**When to use:** Default strategy for fairest distribution
**Example:**
```go
type LeastLoadedSelector struct{}

func (s *LeastLoadedSelector) Select(keys []*KeyMetadata) (*KeyMetadata, error) {
    var bestKey *KeyMetadata
    var bestScore float64

    for _, k := range keys {
        if !k.IsHealthy() {
            continue // Skip unhealthy keys (circuit breaker)
        }

        // Score = weighted average of remaining capacity
        // Higher score = more capacity available
        rpmScore := float64(k.RPMRemaining) / float64(k.RPMLimit)
        tpmScore := float64(k.TPMRemaining) / float64(k.TPMLimit)
        score := (rpmScore + tpmScore) / 2.0

        if bestKey == nil || score > bestScore {
            bestKey = k
            bestScore = score
        }
    }

    if bestKey == nil {
        return nil, ErrAllKeysExhausted
    }

    return bestKey, nil
}
```

### Pattern 5: Config Hot Reload with fsnotify
**What:** Watch config file, reload keys on write event
**When to use:** Credential rotation without restart (AUTH-05)
**Example:**
```go
// Source: https://github.com/fsnotify/fsnotify
import "github.com/fsnotify/fsnotify"

func (p *KeyPool) WatchConfig(ctx context.Context, path string) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    defer watcher.Close()

    if err := watcher.Add(path); err != nil {
        return err
    }

    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                if err := p.ReloadConfig(path); err != nil {
                    // Log error but continue watching
                    log.Error().Err(err).Msg("config reload failed")
                }
            }
        case err := <-watcher.Errors:
            log.Error().Err(err).Msg("fsnotify error")
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (p *KeyPool) ReloadConfig(path string) error {
    newConfig, err := loadConfig(path)
    if err != nil {
        return err
    }

    p.mu.Lock()
    defer p.mu.Unlock()

    // Update keys atomically
    p.keys = buildKeyMetadata(newConfig.Providers)
    return nil
}
```

### Anti-Patterns to Avoid
- **Direct x/time/rate usage in handlers:** Wrap in RateLimiter interface for testability and strategy swapping
- **Global rate limiter:** Must be per-key to enable pooling
- **Ignoring response headers:** Config limits become stale, miss tier upgrades
- **sync.Map for key pool:** Worse write performance, loses type safety
- **Fixed window rate limiting:** Allows burst at window boundaries (2x limit spike)

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Rate limiting algorithm | Custom sliding window with timestamps | golang.org/x/time/rate.Limiter | Token bucket is simpler, better tested, concurrent-safe, stdlib-backed |
| Weighted round-robin | Custom weight tracking | github.com/smallnest/weighted (SW algorithm) | Nginx's smooth algorithm handles edge cases (unequal weights, dynamic updates) |
| File watching | Polling with time.Ticker | github.com/fsnotify/fsnotify | Cross-platform, efficient (inotify/kqueue), already available |
| RFC3339 parsing | Manual string parsing | time.Parse(time.RFC3339, s) | Handles timezones, validation, edge cases |
| Mutex for read-heavy | sync.Mutex | sync.RWMutex | Key selection is 90% reads (check capacity), 10% writes (update from headers) |
| Token counting | Regex/heuristics | Anthropic's /v1/messages/count_tokens API | Free to use, accurate, separate rate limits |

**Key insight:** Rate limiting has subtle edge cases (burst handling, time drift, concurrency races). Battle-tested libraries prevent production incidents.

## Common Pitfalls

### Pitfall 1: Token Bucket Misunderstanding
**What goes wrong:** Setting burst = rate causes request spikes to fail
**Why it happens:** Token bucket has two parameters: rate (refill speed) and burst (bucket size). Setting burst too low means legitimate bursts get rejected.
**How to avoid:** Set burst = limit (e.g., 50 RPM → burst: 50). This allows consuming the full minute's capacity instantly, then refills gradually.
**Warning signs:** 429s when overall rate is under limit, requests fail in clusters

### Pitfall 2: Ignoring 429 retry-after Header
**What goes wrong:** Immediately retrying a 429 response hammers the rate-limited key
**Why it happens:** Anthropic returns `retry-after` header (seconds) to indicate when capacity replenishes
**How to avoid:** Extract retry-after from provider 429 response, mark key unhealthy until reset time
**Warning signs:** Cascading 429s, provider key suspension

### Pitfall 3: Race Condition in Key Selection
**What goes wrong:** Multiple goroutines select same "least loaded" key simultaneously, all hit rate limit
**Why it happens:** RLock allows concurrent reads, but key selection + limiter check isn't atomic
**How to avoid:** Use RWMutex correctly: RLock for capacity check, release before calling limiter (which has own mutex)
**Warning signs:** Burst traffic causes all requests to fail on same key

### Pitfall 4: Not Accounting for Cache Tokens
**What goes wrong:** Overestimating token usage because cache_read_input_tokens count toward ITPM on older models
**Why it happens:** Anthropic's docs state "For most Claude models, only uncached input tokens count towards your ITPM rate limits" but older models (†) include cached tokens
**How to avoid:** Track model version, adjust TPM calculation: `tpm = input_tokens + cache_creation_input_tokens` (exclude cache_read for new models)
**Warning signs:** Hitting TPM limit faster than expected, even with high cache hit rate

### Pitfall 5: Fixed Window Boundary Problem
**What goes wrong:** Users make 2x requests at window boundaries (59s and 00s of next minute)
**Why it happens:** Fixed windows reset at exact intervals, allowing burst at edge
**How to avoid:** Use sliding window or token bucket (golang.org/x/time/rate uses token bucket)
**Warning signs:** Periodic spikes in rate limit errors at minute boundaries

### Pitfall 6: Config Reload Race During Request
**What goes wrong:** Key removed from config mid-request, handler panics with nil pointer
**Why it happens:** Reload overwrites p.keys while handler holds reference to old KeyMetadata
**How to avoid:** Use copy-on-write: build new keys slice, atomic pointer swap with sync/atomic
**Warning signs:** Panics during config reload, intermittent nil pointer errors

### Pitfall 7: Not Validating Response Header Values
**What goes wrong:** Malformed rate limit headers cause panics or infinite limits
**Why it happens:** Trusting external provider headers without validation (negative values, missing fields, invalid RFC3339)
**How to avoid:** Validate all parsed values: `if rpm > 0 && rpm < 1_000_000 { keyMeta.RPMLimit = rpm }`
**Warning signs:** Panics on strconv.Atoi, time.Parse errors, keys with 0 or negative limits

## Code Examples

Verified patterns from official sources:

### Rate Limiter with golang.org/x/time/rate
```go
// Source: https://pkg.go.dev/golang.org/x/time/rate
import "golang.org/x/time/rate"

// Per-key rate limiter
type KeyMetadata struct {
    APIKey         string
    RPMLimit       int
    TPMLimit       int
    RPMRemaining   int
    TPMRemaining   int
    RPMResetAt     time.Time
    TPMResetAt     time.Time
    requestLimiter *rate.Limiter
    tokenLimiter   *rate.Limiter
    mu             sync.Mutex
}

func NewKeyMetadata(apiKey string, rpm, tpm int) *KeyMetadata {
    // rate.NewLimiter(rate, burst)
    // rate: tokens per second (convert from per-minute)
    // burst: max tokens available instantly (set to limit)
    return &KeyMetadata{
        APIKey:         apiKey,
        RPMLimit:       rpm,
        TPMLimit:       tpm,
        RPMRemaining:   rpm,
        TPMRemaining:   tpm,
        requestLimiter: rate.NewLimiter(rate.Limit(rpm/60.0), rpm),
        tokenLimiter:   rate.NewLimiter(rate.Limit(tpm/60.0), tpm),
    }
}

func (k *KeyMetadata) Allow(ctx context.Context) bool {
    k.mu.Lock()
    defer k.mu.Unlock()

    // Check request rate limit
    if !k.requestLimiter.Allow() {
        return false
    }

    // Note: Token limit checked after response with actual count
    // For now, optimistically allow request
    return true
}

func (k *KeyMetadata) ConsumeTokens(ctx context.Context, count int) error {
    k.mu.Lock()
    defer k.mu.Unlock()

    // WaitN blocks until tokens available or context cancelled
    return k.tokenLimiter.WaitN(ctx, count)
}
```

### Weighted Round-Robin with Smooth Algorithm
```go
// Source: https://github.com/smallnest/weighted (Nginx algorithm)
// Smooth weighted round-robin ensures fair distribution even with unequal weights

type WeightedKey struct {
    key            *KeyMetadata
    weight         int // Configured weight
    currentWeight  int // Dynamic tracking
    effectiveWeight int // Adjusted for failures
}

type WeightedSelector struct {
    keys []*WeightedKey
    mu   sync.Mutex
}

func (s *WeightedSelector) Select(keys []*KeyMetadata) (*KeyMetadata, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    var best *WeightedKey
    totalWeight := 0

    for _, wk := range s.keys {
        if !wk.key.IsHealthy() {
            continue
        }

        // Increase current weight by effective weight
        wk.currentWeight += wk.effectiveWeight
        totalWeight += wk.effectiveWeight

        // Select key with highest current weight
        if best == nil || wk.currentWeight > best.currentWeight {
            best = wk
        }
    }

    if best == nil {
        return nil, ErrAllKeysExhausted
    }

    // Reduce selected key's weight by total (ensures rotation)
    best.currentWeight -= totalWeight

    return best.key, nil
}
```

### Anthropic Response Header Parsing
```go
// Source: https://platform.claude.com/docs/en/api/rate-limits
// Response headers returned by Anthropic API

func ParseRateLimitHeaders(headers http.Header) (*RateLimitInfo, error) {
    info := &RateLimitInfo{}

    // Requests per minute
    if val := headers.Get("anthropic-ratelimit-requests-limit"); val != "" {
        limit, err := strconv.Atoi(val)
        if err != nil || limit <= 0 {
            return nil, fmt.Errorf("invalid requests-limit: %s", val)
        }
        info.RPMLimit = limit
    }

    if val := headers.Get("anthropic-ratelimit-requests-remaining"); val != "" {
        remaining, _ := strconv.Atoi(val)
        info.RPMRemaining = remaining
    }

    if val := headers.Get("anthropic-ratelimit-requests-reset"); val != "" {
        resetTime, err := time.Parse(time.RFC3339, val)
        if err != nil {
            return nil, fmt.Errorf("invalid reset time: %w", err)
        }
        info.RPMResetAt = resetTime
    }

    // Input tokens per minute
    if val := headers.Get("anthropic-ratelimit-input-tokens-limit"); val != "" {
        limit, _ := strconv.Atoi(val)
        info.ITPMLimit = limit
    }

    if val := headers.Get("anthropic-ratelimit-input-tokens-remaining"); val != "" {
        remaining, _ := strconv.Atoi(val)
        info.ITPMRemaining = remaining
    }

    // Output tokens per minute
    if val := headers.Get("anthropic-ratelimit-output-tokens-limit"); val != "" {
        limit, _ := strconv.Atoi(val)
        info.OTPMLimit = limit
    }

    if val := headers.Get("anthropic-ratelimit-output-tokens-remaining"); val != "" {
        remaining, _ := strconv.Atoi(val)
        info.OTPMRemaining = remaining
    }

    return info, nil
}
```

### Environment Variable Expansion in YAML
```go
// Source: https://mtyurt.net/post/go-using-environment-variables-in-configuration-files.html
import (
    "os"
    "gopkg.in/yaml.v3"
)

func LoadConfigWithEnvExpansion(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    // Expand ${VAR} and $VAR patterns
    expanded := os.ExpandEnv(string(data))

    var cfg Config
    if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}

// Config example:
// providers:
//   - name: anthropic
//     keys:
//       - key: ${ANTHROPIC_API_KEY_1}
//         rpm_limit: 50
//       - key: ${ANTHROPIC_API_KEY_2}
//         rpm_limit: 50
```

### 429 Response with Retry-After
```go
// Source: https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/429
// RFC 6585 - 429 Too Many Requests

func Write429Response(w http.ResponseWriter, retryAfter time.Duration) {
    w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusTooManyRequests)

    json.NewEncoder(w).Encode(map[string]interface{}{
        "error": map[string]interface{}{
            "type": "rate_limit_error",
            "message": "Rate limit exceeded. All API keys are currently at capacity.",
        },
    })
}

// When all keys exhausted, calculate retry-after from earliest reset
func (p *KeyPool) GetEarliestResetTime() time.Duration {
    p.mu.RLock()
    defer p.mu.RUnlock()

    var earliest time.Time
    for _, k := range p.keys {
        if earliest.IsZero() || k.RPMResetAt.Before(earliest) {
            earliest = k.RPMResetAt
        }
    }

    if earliest.IsZero() {
        return 60 * time.Second // Default: 1 minute
    }

    return time.Until(earliest)
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Fixed window rate limiting | Token bucket (x/time/rate) | 2019 (Go 1.13) | No boundary burst problem, smoother traffic distribution |
| sync.Mutex for all access | sync.RWMutex for read-heavy | Always available | 3-5x better read concurrency for capacity checks |
| Hardcoded rate limits | Dynamic learning from headers | Anthropic API v1 | Auto-adapts to tier upgrades, no config updates needed |
| Manual timestamp tracking | RFC3339 parsing (time.Parse) | Stdlib always | Handles timezones, leap seconds, validation |
| Polling config file | fsnotify file watching | fsnotify v1.0 (2014) | <1ms latency vs 1s+ polling, lower CPU |
| Combined TPM limit | Separate ITPM/OTPM limits | 2024 (Anthropic) | Accurate tracking, cache-aware (uncached tokens only for new models) |

**Deprecated/outdated:**
- Fixed window rate limiting: Allows 2x burst at boundaries, use token bucket instead
- sync.Map for rate limiters: Poor write performance (confirmed 2024-2026 benchmarks), use RWMutex+map
- Global rate limiter: Defeats pooling purpose, must be per-key
- Ignoring retry-after header: RFC 6585 standard since 2012, Anthropic includes it in 429s

## Open Questions

Things that couldn't be fully resolved:

1. **Token Count Estimation Accuracy**
   - What we know: Anthropic provides /v1/messages/count_tokens API (free, separate rate limits)
   - What's unclear: Whether proxy should pre-count every request or use heuristic (4 chars ≈ 1 token)
   - Recommendation: Start with heuristic for speed, add optional pre-counting for accuracy (config flag)

2. **Daily/Monthly Quota Tracking**
   - What we know: Config supports daily limits, Anthropic has spend limits per tier
   - What's unclear: Best persistence strategy (memory vs disk) for multi-day tracking
   - Recommendation: Store in cache backend (Olric for HA, Ristretto for single), reset at UTC midnight

3. **Circuit Breaker Integration**
   - What we know: Keys should be marked unhealthy after repeated 429s/500s
   - What's unclear: Where circuit breaker lives (per-key? per-provider?)
   - Recommendation: Per-key state in KeyMetadata, with exponential backoff (1min → 5min → 15min)

4. **Weighted Strategy Weight Format**
   - What we know: Weights enable priority keys (e.g., weight: 3 vs weight: 1)
   - What's unclear: Integer weights vs percentage (70/30 split)
   - Recommendation: Integer weights (simpler, matches Nginx), document as relative priority

## Sources

### Primary (HIGH confidence)
- [Anthropic API Rate Limits Documentation](https://platform.claude.com/docs/en/api/rate-limits) - Official response headers, limits per tier, cache-aware ITPM
- [golang.org/x/time/rate Package](https://pkg.go.dev/golang.org/x/time/rate) - Token bucket implementation, official Go extended library
- [Go sync Package](https://pkg.go.dev/sync) - RWMutex semantics, official stdlib
- [RFC 6585 - 429 Too Many Requests](https://datatracker.ietf.org/doc/html/rfc6585) - Retry-After header specification
- [fsnotify GitHub](https://github.com/fsnotify/fsnotify) - Cross-platform file watching, v1.7+ current

### Secondary (MEDIUM confidence)
- [How to Implement Rate Limiting in Go Without External Services](https://oneuptime.com/blog/post/2026-01-07-go-rate-limiting/view) - 2026 guide, sliding window + token bucket comparison
- [Go: Performance of RwMutex vs Mutex Across Multiple Scenarios](https://leapcell.io/blog/golang-performance-rwmutex-vs-mutex) - 2024-2026 benchmarks showing RWMutex advantages
- [Dependency Injection in Go: Patterns & Best Practices](https://www.glukhov.org/post/2025/12/dependency-injection-in-go/) - Dec 2025, interface adapter pattern
- [github.com/smallnest/weighted](https://github.com/smallnest/weighted) - Nginx smooth weighted round-robin algorithm
- [Calculating Token Count for Claude API Using Go](https://www.pixelstech.net/article/1735013847-calculating-token-count-for-claude-api-using-go:-a-step-by-step-guide) - Dec 2024, /count_tokens API usage

### Secondary (MEDIUM confidence - Cross-verified)
- [Go sync.Map: The Right Tool for the Right Job](https://victoriametrics.com/blog/go-sync-map/) - 2024, explains why sync.Map performs worse for write-heavy scenarios
- [How to rate limit HTTP requests in Go – Alex Edwards](https://www.alexedwards.net/blog/how-to-rate-limit-http-requests) - Per-client rate limiting pattern
- [Environment Variables in Go Config Files](https://mtyurt.net/post/go-using-environment-variables-in-configuration-files.html) - os.ExpandEnv pattern
- [MDN: Retry-After Header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Retry-After) - HTTP header format (seconds or date)
- [MDN: 429 Too Many Requests](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Status/429) - HTTP status code specification

### Tertiary (LOW confidence - WebSearch only)
- [github.com/RussellLuo/slidingwindow](https://github.com/RussellLuo/slidingwindow) - Alternative sliding window implementation, not verified for production use
- [github.com/sony/gobreaker](https://github.com/sony/gobreaker) - Circuit breaker library, not tested with rate limiters
- [Circuit Breaker & Rate Limiting in Golang Microservices](https://medium.com/pickme-engineering-blog/circuit-breaker-rate-limiting-in-golang-microservices-a-practical-guide-1320e0cfe901) - Feb 2025 Medium article, not official source

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - golang.org/x/time/rate is official extended library, RWMutex is stdlib, fsnotify is battle-tested
- Architecture: HIGH - Adapter pattern verified from existing cache system, token bucket confirmed in official docs
- Pitfalls: MEDIUM - Some from experience reports (WebSearch), others inferred from API docs
- Token counting: MEDIUM - Anthropic docs confirmed but pre-counting strategy needs validation
- Circuit breaker: LOW - Integration pattern not officially documented, needs design validation

**Research date:** 2026-01-21
**Valid until:** 2026-02-21 (30 days - stable domain, but Anthropic may update rate limit headers)

**Research notes:**
- User context (02-CONTEXT.md) specified: pluggable interface pattern (like cache), least-loaded default strategy, failover when exhausted, config + headers for limits
- All user decisions incorporated: 4 selection strategies (least-loaded, round-robin, random, weighted), sliding window with header updates, failover before 429
- Key insight: Existing cache pattern (Cache interface + factory) provides excellent template for RateLimiter and KeySelector interfaces
