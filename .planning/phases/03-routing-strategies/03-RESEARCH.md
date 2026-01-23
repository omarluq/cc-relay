# Phase 3: Routing Strategies - Research

**Researched:** 2026-01-23
**Domain:** Go routing strategy implementation (round-robin, weighted-round-robin, shuffle, failover)
**Confidence:** HIGH

## Summary

Phase 3 implements pluggable provider-level routing strategies that select which provider receives each incoming request. This is distinct from the existing key-level selection (KeySelector) which chooses which API key within a provider to use. The codebase already has a mature KeySelector pattern in `internal/keypool/` that can inform the design.

The routing strategies require implementing: round-robin (sequential distribution), weighted-round-robin (proportional distribution), shuffle (shuffled queue dealing), and failover (primary with fallback chain + smart parallel retry). The failover strategy has the most complexity due to the "first success wins" parallel retry pattern and extensible trigger system.

The CONTEXT.md locks several decisions: config location `routing: { strategy: "..." }`, default strategy `failover`, health checks via `IsHealthy()` interface (Phase 4), and debug-only response headers.

**Primary recommendation:** Create a `ProviderRouter` interface mirroring the existing `KeySelector` pattern. Implement strategies as separate files (round_robin.go, weighted_round_robin.go, shuffle.go, failover.go). Use `errgroup.WithContext` + channel for "first success wins" parallel retry. Use `lo.Shuffle` for the shuffled queue approach.

## Standard Stack

The established libraries/tools for this phase:

### Core (Already in Codebase)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| samber/lo | v1.x | Shuffle, Filter, Map for provider lists | Already used for functional patterns |
| samber/mo | v1.x | mo.Option for optional config, mo.Result for routing results | Already used for monadic patterns |
| samber/do | v2.0 | DI container for router injection | Already used for service wiring |
| sync/atomic | stdlib | Thread-safe counter for round-robin index | Standard Go concurrency primitive |
| golang.org/x/sync/errgroup | Latest | Parallel retry with context cancellation | Standard Go extended library |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| time | stdlib | Timeout handling for failover | Failover timeout configuration |
| context | stdlib | Request cancellation, deadline propagation | All routing operations |
| math/rand/v2 | Go 1.22+ | Seed for shuffle if needed | Alternative to lo.Shuffle |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| lo.Shuffle | math/rand/v2 + custom | lo.Shuffle uses Fisher-Yates, battle-tested |
| errgroup for parallel | Manual goroutines + WaitGroup | errgroup provides context cancellation |
| sync/atomic counter | sync.Mutex | Atomic is faster for single counter |
| smallnest/weighted | Custom implementation | Library is not goroutine-safe, simpler to hand-roll |

**Installation:**
```bash
# No new dependencies - all already in go.mod
go get golang.org/x/sync/errgroup  # If not present
```

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── router/                        # NEW: Provider-level routing
│   ├── router.go                  # ProviderRouter interface + NewRouter factory
│   ├── round_robin.go             # RoundRobinRouter implementation
│   ├── weighted_round_robin.go    # WeightedRoundRobinRouter implementation
│   ├── shuffle.go                 # ShuffleRouter implementation
│   ├── failover.go                # FailoverRouter implementation + parallel retry
│   ├── triggers.go                # Extensible failover trigger system
│   ├── router_test.go             # Unit tests
│   └── router_property_test.go    # Property-based tests
├── keypool/                       # EXISTING: Key-level selection
│   ├── selector.go                # KeySelector interface (inform design)
│   ├── round_robin.go             # Example pattern to follow
│   └── least_loaded.go
├── config/
│   └── config.go                  # Add RoutingConfig struct
└── proxy/
    └── handler.go                 # Integrate ProviderRouter
```

### Pattern 1: Strategy Interface (Mirror KeySelector)

**What:** Define ProviderRouter interface similar to existing KeySelector
**When to use:** All routing strategy implementations

```go
// Source: Informed by internal/keypool/selector.go pattern
package router

import (
    "context"
    "github.com/omarluq/cc-relay/internal/providers"
    "github.com/samber/mo"
)

// ProviderRouter selects which provider to route a request to.
// Implementations are interchangeable routing strategies.
type ProviderRouter interface {
    // Select chooses a provider from the available list.
    // Returns the selected provider or error if none available.
    Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error)

    // Name returns the strategy name for logging/config.
    Name() string
}

// ProviderInfo wraps a provider with routing metadata.
type ProviderInfo struct {
    Provider  providers.Provider
    Weight    int  // For weighted-round-robin
    Priority  int  // For failover ordering
    IsHealthy func() bool  // Health check (Phase 4 integration point)
}

// Common errors
var (
    ErrNoProviders         = errors.New("router: no providers configured")
    ErrAllProvidersUnhealthy = errors.New("router: all providers unhealthy")
)
```

### Pattern 2: Round-Robin with Atomic Counter

**What:** Sequential distribution using atomic counter (like existing keypool/round_robin.go)
**When to use:** Even distribution across providers

```go
// Source: Informed by internal/keypool/round_robin.go
package router

import (
    "context"
    "sync/atomic"
    "github.com/samber/lo"
)

type RoundRobinRouter struct {
    index uint64
}

func NewRoundRobinRouter() *RoundRobinRouter {
    return &RoundRobinRouter{}
}

func (r *RoundRobinRouter) Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error) {
    if len(providers) == 0 {
        return ProviderInfo{}, ErrNoProviders
    }

    // Filter to healthy providers
    healthy := lo.Filter(providers, func(p ProviderInfo, _ int) bool {
        return p.IsHealthy == nil || p.IsHealthy()
    })

    if len(healthy) == 0 {
        return ProviderInfo{}, ErrAllProvidersUnhealthy
    }

    // Atomic increment and modulo
    nextIdx := atomic.AddUint64(&r.index, 1) - 1
    //nolint:gosec // Safe: modulo ensures result within int range
    idx := int(nextIdx % uint64(len(healthy)))

    return healthy[idx], nil
}

func (r *RoundRobinRouter) Name() string {
    return "round_robin"
}
```

### Pattern 3: Weighted Round-Robin (Smooth Algorithm)

**What:** Proportional distribution based on weights (Nginx smooth algorithm)
**When to use:** Providers with different capacities (A:3, B:2, C:1)

```go
// Source: Nginx smooth weighted round-robin algorithm
// Reference: https://pkg.go.dev/github.com/smallnest/weighted
package router

import (
    "context"
    "sync"
    "github.com/samber/lo"
)

type WeightedRoundRobinRouter struct {
    mu             sync.Mutex
    currentWeights []int  // Current weight state
}

func NewWeightedRoundRobinRouter() *WeightedRoundRobinRouter {
    return &WeightedRoundRobinRouter{}
}

func (r *WeightedRoundRobinRouter) Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error) {
    if len(providers) == 0 {
        return ProviderInfo{}, ErrNoProviders
    }

    // Filter healthy providers
    healthy := lo.Filter(providers, func(p ProviderInfo, _ int) bool {
        return p.IsHealthy == nil || p.IsHealthy()
    })

    if len(healthy) == 0 {
        return ProviderInfo{}, ErrAllProvidersUnhealthy
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    // Initialize or resize current weights
    if len(r.currentWeights) != len(healthy) {
        r.currentWeights = make([]int, len(healthy))
    }

    // Calculate total weight
    totalWeight := lo.SumBy(healthy, func(p ProviderInfo) int {
        if p.Weight <= 0 {
            return 1  // Default weight
        }
        return p.Weight
    })

    // Smooth weighted round-robin:
    // 1. Add configured weight to current weight
    // 2. Select provider with highest current weight
    // 3. Subtract total weight from selected
    bestIdx := 0
    bestWeight := r.currentWeights[0]

    for i, p := range healthy {
        weight := p.Weight
        if weight <= 0 {
            weight = 1
        }
        r.currentWeights[i] += weight

        if r.currentWeights[i] > bestWeight {
            bestWeight = r.currentWeights[i]
            bestIdx = i
        }
    }

    r.currentWeights[bestIdx] -= totalWeight
    return healthy[bestIdx], nil
}

func (r *WeightedRoundRobinRouter) Name() string {
    return "weighted_round_robin"
}
```

### Pattern 4: Shuffle Router (Dealing Cards)

**What:** Shuffled queue - shuffle once, cycle through, reshuffle when exhausted
**When to use:** Randomized but fair distribution (like dealing cards)

```go
// Source: CONTEXT.md specification - "shuffled queue like dealing cards"
package router

import (
    "context"
    "sync"
    lom "github.com/samber/lo/mutable"
    "github.com/samber/lo"
)

type ShuffleRouter struct {
    mu            sync.Mutex
    shuffledOrder []int  // Indices into provider list
    position      int    // Current position in shuffled order
    lastLen       int    // Track if provider list changed
}

func NewShuffleRouter() *ShuffleRouter {
    return &ShuffleRouter{}
}

func (r *ShuffleRouter) Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error) {
    if len(providers) == 0 {
        return ProviderInfo{}, ErrNoProviders
    }

    // Filter healthy providers
    healthy := lo.Filter(providers, func(p ProviderInfo, _ int) bool {
        return p.IsHealthy == nil || p.IsHealthy()
    })

    if len(healthy) == 0 {
        return ProviderInfo{}, ErrAllProvidersUnhealthy
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    // Reshuffle if: first time, provider count changed, or exhausted
    if r.shuffledOrder == nil || len(healthy) != r.lastLen || r.position >= len(r.shuffledOrder) {
        r.shuffledOrder = make([]int, len(healthy))
        for i := range r.shuffledOrder {
            r.shuffledOrder[i] = i
        }
        lom.Shuffle(r.shuffledOrder)  // Fisher-Yates shuffle
        r.position = 0
        r.lastLen = len(healthy)
    }

    // Deal next card
    idx := r.shuffledOrder[r.position]
    r.position++

    return healthy[idx], nil
}

func (r *ShuffleRouter) Name() string {
    return "shuffle"
}
```

### Pattern 5: Failover Router with Parallel Retry

**What:** Primary provider first, smart parallel retry on failure
**When to use:** When reliability is critical and you want fast failover

```go
// Source: CONTEXT.md specification - smart parallel retry
// Reference: golang.org/x/sync/errgroup
package router

import (
    "context"
    "errors"
    "time"
    "golang.org/x/sync/errgroup"
    "github.com/samber/lo"
)

// FailoverTrigger defines conditions that trigger failover.
type FailoverTrigger interface {
    ShouldFailover(err error, statusCode int) bool
    Name() string
}

type FailoverRouter struct {
    triggers []FailoverTrigger
    timeout  time.Duration  // From config, default 5s
}

func NewFailoverRouter(timeout time.Duration, triggers ...FailoverTrigger) *FailoverRouter {
    if len(triggers) == 0 {
        triggers = DefaultTriggers()
    }
    if timeout == 0 {
        timeout = 5 * time.Second
    }
    return &FailoverRouter{
        triggers: triggers,
        timeout:  timeout,
    }
}

// RoutingResult contains the result of a routing attempt.
type RoutingResult struct {
    Provider ProviderInfo
    Err      error
}

// SelectWithRetry implements smart parallel retry:
// 1. Try primary provider
// 2. If fails with trigger condition, start timeout
// 3. Continue retrying primary WHILE ALSO trying fallback
// 4. First success wins, cancel others
func (r *FailoverRouter) SelectWithRetry(
    ctx context.Context,
    providers []ProviderInfo,
    tryProvider func(context.Context, ProviderInfo) error,
) (ProviderInfo, error) {
    // Sort by priority (higher = first)
    sorted := lo.Filter(providers, func(p ProviderInfo, _ int) bool {
        return p.IsHealthy == nil || p.IsHealthy()
    })

    if len(sorted) == 0 {
        return ProviderInfo{}, ErrAllProvidersUnhealthy
    }

    // Sort by priority descending
    lo.Slice(sorted, func(i, j int) bool {
        return sorted[i].Priority > sorted[j].Priority
    })

    // Simple case: only one provider
    if len(sorted) == 1 {
        return sorted[0], tryProvider(ctx, sorted[0])
    }

    // Try primary first
    primary := sorted[0]
    err := tryProvider(ctx, primary)

    if err == nil {
        return primary, nil  // Primary succeeded
    }

    // Check if we should failover
    if !r.shouldFailover(err) {
        return primary, err  // Don't failover for this error type
    }

    // Start parallel retry: continue primary + try fallbacks
    return r.parallelRace(ctx, sorted, tryProvider)
}

func (r *FailoverRouter) parallelRace(
    ctx context.Context,
    providers []ProviderInfo,
    tryProvider func(context.Context, ProviderInfo) error,
) (ProviderInfo, error) {
    // Create cancellable context
    raceCtx, cancel := context.WithTimeout(ctx, r.timeout)
    defer cancel()

    // Result channel - first success wins
    resultCh := make(chan RoutingResult, len(providers))

    // Launch all attempts in parallel
    var wg sync.WaitGroup
    for _, p := range providers {
        wg.Add(1)
        go func(provider ProviderInfo) {
            defer wg.Done()

            err := tryProvider(raceCtx, provider)
            select {
            case resultCh <- RoutingResult{Provider: provider, Err: err}:
            case <-raceCtx.Done():
            }
        }(p)
    }

    // Close result channel when all done
    go func() {
        wg.Wait()
        close(resultCh)
    }()

    // Wait for first success or all failures
    var lastErr error
    for result := range resultCh {
        if result.Err == nil {
            cancel()  // Cancel other attempts
            return result.Provider, nil
        }
        lastErr = result.Err
    }

    return ProviderInfo{}, lastErr
}

func (r *FailoverRouter) shouldFailover(err error) bool {
    for _, trigger := range r.triggers {
        if trigger.ShouldFailover(err, extractStatusCode(err)) {
            return true
        }
    }
    return false
}

func (r *FailoverRouter) Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error) {
    // For simple selection (no retry logic), just return highest priority healthy
    healthy := lo.Filter(providers, func(p ProviderInfo, _ int) bool {
        return p.IsHealthy == nil || p.IsHealthy()
    })

    if len(healthy) == 0 {
        return ProviderInfo{}, ErrAllProvidersUnhealthy
    }

    // Return highest priority
    best := lo.MaxBy(healthy, func(a, b ProviderInfo) bool {
        return a.Priority > b.Priority
    })

    return best, nil
}

func (r *FailoverRouter) Name() string {
    return "failover"
}
```

### Pattern 6: Extensible Failover Triggers

**What:** Pluggable conditions for triggering failover
**When to use:** Different failure modes require different handling

```go
// Source: CONTEXT.md specification - extensible trigger system
package router

import (
    "errors"
    "net"
    "net/http"
)

// Default triggers: 5xx, 429, timeout, connection errors
func DefaultTriggers() []FailoverTrigger {
    return []FailoverTrigger{
        &StatusCodeTrigger{codes: []int{429, 500, 502, 503, 504}},
        &TimeoutTrigger{},
        &ConnectionTrigger{},
    }
}

// StatusCodeTrigger triggers failover on specific HTTP status codes.
type StatusCodeTrigger struct {
    codes []int
}

func (t *StatusCodeTrigger) ShouldFailover(err error, statusCode int) bool {
    for _, code := range t.codes {
        if statusCode == code {
            return true
        }
    }
    return false
}

func (t *StatusCodeTrigger) Name() string {
    return "status_code"
}

// TimeoutTrigger triggers failover on context deadline exceeded.
type TimeoutTrigger struct{}

func (t *TimeoutTrigger) ShouldFailover(err error, _ int) bool {
    return errors.Is(err, context.DeadlineExceeded)
}

func (t *TimeoutTrigger) Name() string {
    return "timeout"
}

// ConnectionTrigger triggers failover on network connection errors.
type ConnectionTrigger struct{}

func (t *ConnectionTrigger) ShouldFailover(err error, _ int) bool {
    var netErr net.Error
    return errors.As(err, &netErr)
}

func (t *ConnectionTrigger) Name() string {
    return "connection"
}
```

### Pattern 7: Configuration Integration

**What:** Add routing config nested under `routing:`
**When to use:** Config loading

```go
// Add to internal/config/config.go
type RoutingConfig struct {
    Strategy        string `yaml:"strategy"`         // round_robin, weighted_round_robin, shuffle, failover
    FailoverTimeout int    `yaml:"failover_timeout"` // Milliseconds, default 5000
    Debug           bool   `yaml:"debug"`            // Enable debug headers
}

// GetEffectiveStrategy returns strategy with default fallback
func (r *RoutingConfig) GetEffectiveStrategy() string {
    if r.Strategy == "" {
        return "failover"  // Default per CONTEXT.md
    }
    return r.Strategy
}

// GetFailoverTimeout returns timeout as time.Duration
func (r *RoutingConfig) GetFailoverTimeoutOption() mo.Option[time.Duration] {
    if r.FailoverTimeout <= 0 {
        return mo.None[time.Duration]()
    }
    return mo.Some(time.Duration(r.FailoverTimeout) * time.Millisecond)
}
```

### Anti-Patterns to Avoid

- **Mixing provider and key selection:** Keep ProviderRouter separate from KeySelector. Provider routing happens first, then key selection within the chosen provider.

- **Blocking on all providers:** In failover, don't wait for all providers sequentially. Use parallel racing with first-success-wins.

- **Ignoring context cancellation:** All Select methods must respect ctx.Done() to support request cancellation.

- **Stateful shuffle without reset:** The shuffle queue must reset when provider list changes (new provider added/removed).

- **Hardcoded triggers:** Make failover triggers extensible so new conditions can be added (per CONTEXT.md).

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Shuffle algorithm | Manual array randomization | lo/mutable.Shuffle | Uses Fisher-Yates, proven correct |
| Parallel racing | Manual goroutines + channels | errgroup.WithContext + channel | Proper cancellation, error propagation |
| Thread-safe counter | sync.Mutex for simple int | sync/atomic | Lock-free, faster for single counter |
| Provider filtering | Manual for-loops | lo.Filter | Cleaner, tested, consistent with codebase |
| Max by priority | Manual iteration | lo.MaxBy | Handles empty slices, cleaner |

**Key insight:** The codebase already uses samber/lo extensively. Leverage lo.Filter, lo.MaxBy, lo.SumBy for provider list operations. Use lo/mutable.Shuffle for Fisher-Yates shuffle.

## Common Pitfalls

### Pitfall 1: Race Condition in Weighted Round-Robin State

**What goes wrong:** Multiple requests modify currentWeights array concurrently, causing incorrect distribution.

**Why it happens:** The smooth weighted algorithm requires reading AND writing state atomically.

**How to avoid:**
1. Use sync.Mutex to protect the entire select operation
2. Initialize state on first use, not constructor
3. Handle provider list changes (resize currentWeights)

**Warning signs:**
- Distribution doesn't match configured weights over time
- Occasional panics from index out of bounds
- Provider always selected even with low weight

### Pitfall 2: Shuffle Queue Not Resetting

**What goes wrong:** Provider added/removed but shuffle continues with old order.

**Why it happens:** Only checking position >= len, not checking if provider list changed.

**How to avoid:**
1. Track lastLen and compare on each Select
2. Reset shuffledOrder when len(providers) changes
3. Consider using provider IDs for change detection (not just count)

**Warning signs:**
- New provider never receives requests
- Removed provider still in rotation (index out of bounds)

### Pitfall 3: Parallel Retry Resource Leak

**What goes wrong:** Goroutines continue after first success or timeout.

**Why it happens:** Not properly cancelling context when winner found.

**How to avoid:**
1. Use context.WithCancel and call cancel() when first success arrives
2. Check ctx.Done() in goroutines before sending to channel
3. Use buffered channel sized for all providers

**Warning signs:**
- Goroutine count grows over time
- Backend receives requests after client cancelled
- Memory usage increases under load

### Pitfall 4: Trigger System Not Extensible

**What goes wrong:** Hardcoded conditions, can't add new triggers without code change.

**Why it happens:** Using switch/case instead of interface.

**How to avoid:**
1. Define FailoverTrigger interface
2. DefaultTriggers() returns slice of triggers
3. Allow custom triggers via config or DI

**Warning signs:**
- Need to modify failover.go to add new condition
- Can't configure triggers per deployment

### Pitfall 5: Health Check Coupling

**What goes wrong:** Router directly calls health check, creating tight coupling.

**Why it happens:** Passing health tracker instead of IsHealthy() function.

**How to avoid:**
1. ProviderInfo includes IsHealthy func() bool
2. Router just calls the function, doesn't know implementation
3. Phase 4 can change health implementation without router changes

**Warning signs:**
- Import cycle between router and health packages
- Need to mock entire health tracker in tests

## Code Examples

Verified patterns from official sources:

### Router Factory

```go
// Source: Pattern from internal/keypool/selector.go NewSelector
package router

import "fmt"

const (
    StrategyRoundRobin         = "round_robin"
    StrategyWeightedRoundRobin = "weighted_round_robin"
    StrategyShuffle            = "shuffle"
    StrategyFailover           = "failover"
)

// NewRouter creates a ProviderRouter based on strategy name.
func NewRouter(strategy string, timeout time.Duration) (ProviderRouter, error) {
    switch strategy {
    case StrategyRoundRobin:
        return NewRoundRobinRouter(), nil
    case StrategyWeightedRoundRobin:
        return NewWeightedRoundRobinRouter(), nil
    case StrategyShuffle:
        return NewShuffleRouter(), nil
    case StrategyFailover, "":  // Default
        return NewFailoverRouter(timeout), nil
    default:
        return nil, fmt.Errorf("router: unknown strategy %q", strategy)
    }
}
```

### DI Container Registration

```go
// Source: Pattern from cmd/cc-relay/di/providers.go
package di

import (
    "github.com/samber/do/v2"
    "github.com/omarluq/cc-relay/internal/router"
)

// RouterService wraps the provider router for DI.
type RouterService struct {
    Router router.ProviderRouter
}

// NewRouter creates the router based on config.
func NewRouter(i do.Injector) (*RouterService, error) {
    cfgSvc := do.MustInvoke[*ConfigService](i)
    routingCfg := cfgSvc.Config.Routing

    timeout := routingCfg.GetFailoverTimeoutOption().OrElse(5 * time.Second)

    r, err := router.NewRouter(routingCfg.GetEffectiveStrategy(), timeout)
    if err != nil {
        return nil, fmt.Errorf("failed to create router: %w", err)
    }

    return &RouterService{Router: r}, nil
}
```

### Debug Headers (Conditional)

```go
// Source: CONTEXT.md - debug mode only
package proxy

func (h *Handler) addDebugHeaders(w http.ResponseWriter, provider ProviderInfo, strategy string) {
    if !h.debugOpts.IsEnabled() {
        return  // No headers in production
    }

    w.Header().Set("X-CC-Relay-Strategy", strategy)
    w.Header().Set("X-CC-Relay-Provider", provider.Provider.Name())
}
```

### Status Endpoint Integration

```go
// Source: CONTEXT.md - status endpoint includes routing info
type StatusResponse struct {
    // ... existing fields
    Routing RoutingStatus `json:"routing"`
}

type RoutingStatus struct {
    Strategy      string            `json:"strategy"`
    Providers     []ProviderStatus  `json:"providers"`
    FailoverChain []string          `json:"failover_chain,omitempty"`
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Global round-robin counter with Mutex | sync/atomic counter | Go 1.x (always) | Lock-free performance |
| Manual shuffle with rand | lo/mutable.Shuffle (Fisher-Yates) | 2022 (lo v1) | Proven algorithm |
| Sequential failover | Parallel racing with errgroup | Go 1.16+ (errgroup) | Faster failover |
| Hardcoded failover conditions | Pluggable trigger interface | Best practice | Extensibility |
| Single retry per provider | Smart parallel retry | Modern pattern | First success wins |

**Deprecated/outdated:**
- **math/rand.Seed():** Deprecated in Go 1.20, use math/rand/v2 or let runtime seed
- **sync.WaitGroup for racing:** Use errgroup.WithContext for proper cancellation
- **Blocking sequential failover:** Wastes time when parallel retry is faster

## Open Questions

Things that couldn't be fully resolved:

### 1. Per-Provider Override Strategy

**What we know:**
- CONTEXT.md mentions "Global default + per-provider override supported"
- Need to decide where override is configured

**What's unclear:**
- Config syntax: `providers: [{name: x, routing: {...}}]` or separate section?
- How to handle override for failover (override the fallback chain?)

**Recommendation:**
- Start with global strategy only (simpler)
- Add per-provider override in future if needed
- Document as future enhancement

### 2. Health Check Interface

**What we know:**
- ProviderInfo has `IsHealthy func() bool`
- Phase 4 implements actual health tracking

**What's unclear:**
- Who creates the IsHealthy function?
- How to inject health tracker into provider info?

**Recommendation:**
- Use closure pattern: health tracker creates IsHealthy functions
- Pass closure when building ProviderInfo list
- Stub with `func() bool { return true }` until Phase 4

### 3. Parallel Retry Cancellation Timing

**What we know:**
- First success should cancel others
- Context cancellation propagates

**What's unclear:**
- Should we wait for cancelled goroutines to clean up?
- What if backend already received request?

**Recommendation:**
- Cancel immediately on first success
- Don't wait for cleanup (goroutines will exit on next ctx.Done check)
- Backend requests may complete but response is ignored

## Sources

### Primary (HIGH confidence)

**Go Standard Library:**
- [sync/atomic](https://pkg.go.dev/sync/atomic) - Atomic operations
- [context](https://pkg.go.dev/context) - Cancellation patterns
- [golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) - Parallel operations

**Samber Libraries:**
- [lo.Shuffle](https://pkg.go.dev/github.com/samber/lo#Shuffle) - Fisher-Yates shuffle
- [lo.Filter](https://pkg.go.dev/github.com/samber/lo#Filter) - Slice filtering
- [lo.MaxBy](https://pkg.go.dev/github.com/samber/lo#MaxBy) - Maximum by comparator

**Codebase Patterns:**
- `internal/keypool/selector.go` - KeySelector interface pattern
- `internal/keypool/round_robin.go` - Atomic counter pattern
- `cmd/cc-relay/di/providers.go` - DI registration pattern

### Secondary (MEDIUM confidence)

**Load Balancing Algorithms:**
- [Building a simple load balancer in Go](https://dev.to/vivekalhat/building-a-simple-load-balancer-in-go-70d) - Round-robin patterns
- [smallnest/weighted](https://pkg.go.dev/github.com/smallnest/weighted) - Weighted round-robin (Nginx algorithm reference)
- [Building Resilient Go Services](https://dev.to/serifcolakel/building-resilient-go-services-context-graceful-shutdown-and-retrytimeout-patterns-21g3) - Context and retry patterns

**Concurrency Patterns:**
- [How to Use errgroup](https://oneuptime.com/blog/post/2026-01-07-go-errgroup/view) - errgroup patterns
- [Error Group Go Patterns](https://go-patterns.dev/parallel-computing/errgroup) - Parallel operations

### Tertiary (LOW confidence)

**Community Discussions:**
- [gRPC weighted round robin](https://pkg.go.dev/google.golang.org/grpc/balancer/weightedroundrobin) - Experimental, not recommended for production

## Metadata

**Confidence breakdown:**
- Standard stack: **HIGH** - All from existing codebase or Go stdlib
- Architecture patterns: **HIGH** - Based on existing keypool patterns in codebase
- Pitfalls: **MEDIUM** - Based on general Go concurrency experience
- Failover parallel retry: **MEDIUM** - Pattern is sound but edge cases need testing

**Research date:** 2026-01-23
**Valid until:** 2026-02-23 (30 days - stable patterns, Go stdlib)

**Key uncertainties requiring validation:**
1. Per-provider override syntax and implementation
2. Health check integration point (Phase 4 dependency)
3. Parallel retry cancellation behavior under load
4. Weighted round-robin state management with dynamic provider list
