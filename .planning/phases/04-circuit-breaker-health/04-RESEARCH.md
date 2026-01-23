# Phase 4: Circuit Breaker & Health - Research

**Researched:** 2026-01-23
**Domain:** Circuit breaker pattern, health tracking, failure detection and recovery
**Confidence:** HIGH

## Summary

This research investigates how to implement circuit breaker health tracking for cc-relay's provider system. The circuit breaker pattern is a well-established resilience pattern that prevents cascading failures by detecting unhealthy providers and temporarily bypassing them.

The Go ecosystem has mature circuit breaker libraries, with **sony/gobreaker v2** being the industry standard. It provides a complete state machine implementation (CLOSED/OPEN/HALF-OPEN), configurable failure thresholds, and a `TwoStepCircuitBreaker` variant ideal for HTTP proxy scenarios where request initiation and response handling are decoupled.

The key integration point is the existing `ProviderInfo.IsHealthy func() bool` closure in the router package, which currently returns `true` as a stub. Phase 4 will implement this closure using gobreaker's state machine, enabling the existing `FilterHealthy()` function to automatically exclude providers with open circuits.

**Primary recommendation:** Use `sony/gobreaker/v2` with `TwoStepCircuitBreaker` for the health tracking state machine. Implement one circuit breaker per provider (not per key). Use synthetic health checks during OPEN state for faster recovery than waiting for full cooldown.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| [sony/gobreaker](https://github.com/sony/gobreaker) | v2.4.0 | Circuit breaker state machine | Industry standard, generics support, 60s default timeout, rolling window counters |
| Go stdlib sync | 1.24+ | Concurrency (sync.RWMutex) | Built-in, zero dependencies |
| Go stdlib time | 1.24+ | Timers for health check scheduling | Built-in ticker for periodic checks |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| zerolog | v1.34+ | Structured logging for state changes | Already in codebase, use for WARN on open, INFO on close |
| samber/lo | v1.52+ | Functional helpers | Already in codebase, use for provider filtering |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| sony/gobreaker | mercari/go-circuitbreaker | Better context cancellation handling, but less ecosystem adoption |
| sony/gobreaker | Custom implementation | Full control, but state machine edge cases are tricky |
| TwoStepCircuitBreaker | CircuitBreaker.Execute() | Execute() wraps function, but proxy needs decoupled request/response |

**Installation:**
```bash
go get github.com/sony/gobreaker/v2@v2.4.0
```

## Architecture Patterns

### Recommended Project Structure
```
internal/health/
    circuit.go          # CircuitBreaker wrapper with TwoStepCircuitBreaker
    tracker.go          # HealthTracker managing per-provider circuits
    checker.go          # Health check implementations per provider type
    config.go           # CircuitBreakerConfig struct
    errors.go           # Health-related errors
    tracker_test.go     # Unit tests
```

### Pattern 1: TwoStepCircuitBreaker for HTTP Proxy

**What:** Use gobreaker's `TwoStepCircuitBreaker` which separates permission-check from result-reporting
**When to use:** HTTP proxy scenarios where request initiation and response handling are in different code paths
**Example:**
```go
// Source: https://pkg.go.dev/github.com/sony/gobreaker/v2

import "github.com/sony/gobreaker/v2"

// Check if request is allowed
done, err := breaker.Allow()
if err != nil {
    // Circuit is OPEN - return error or try another provider
    return ErrCircuitOpen
}

// Forward request to provider
resp, err := client.Do(req)

// Report outcome to circuit breaker
done(err) // nil = success, non-nil = failure (depending on IsSuccessful)
```

### Pattern 2: Per-Provider Circuit Breakers via HealthTracker

**What:** Single `HealthTracker` struct manages circuit breakers for all providers
**When to use:** Multi-provider routing where each provider needs independent health tracking
**Example:**
```go
// Source: Custom pattern based on gobreaker documentation

type HealthTracker struct {
    circuits map[string]*gobreaker.TwoStepCircuitBreaker[struct{}]
    config   CircuitBreakerConfig
    mu       sync.RWMutex
}

// Returns an IsHealthy closure for a specific provider
func (t *HealthTracker) IsHealthyFunc(providerName string) func() bool {
    return func() bool {
        t.mu.RLock()
        cb, ok := t.circuits[providerName]
        t.mu.RUnlock()
        if !ok {
            return true // No circuit = healthy
        }
        return cb.State() != gobreaker.StateOpen
    }
}
```

### Pattern 3: ReadyToTrip with Consecutive Failures

**What:** Use gobreaker's `ReadyToTrip` callback to implement configurable failure threshold
**When to use:** When failure threshold should be configurable (not hardcoded to 5)
**Example:**
```go
// Source: https://pkg.go.dev/github.com/sony/gobreaker/v2

settings := gobreaker.Settings{
    Name:        providerName,
    MaxRequests: 3,      // Half-open allows 3 probes
    Timeout:     30 * time.Second, // Open->HalfOpen after 30s

    ReadyToTrip: func(counts gobreaker.Counts) bool {
        // Open circuit after configured consecutive failures
        return counts.ConsecutiveFailures >= uint32(config.FailureThreshold)
    },

    IsSuccessful: func(err error) bool {
        // Don't count context cancellation as failure
        return err == nil || errors.Is(err, context.Canceled)
    },
}
```

### Pattern 4: Synthetic Health Checks During OPEN State

**What:** Run periodic lightweight health checks when circuit is OPEN to detect recovery faster
**When to use:** When faster recovery is more important than protecting the provider from probe traffic
**Example:**
```go
// Source: Design pattern from CONTEXT.md decisions

func (t *HealthTracker) startHealthChecker(ctx context.Context, providerName string) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            cb := t.getCircuit(providerName)
            if cb.State() == gobreaker.StateOpen {
                // Run synthetic health check
                if err := t.checkHealth(ctx, providerName); err == nil {
                    // Successful health check - circuit will transition
                    // to HALF-OPEN on next Allow() call after Timeout
                }
            }
        }
    }
}
```

### Pattern 5: Debug Headers for State Visibility

**What:** Add `X-CC-Relay-Health` response header showing circuit state
**When to use:** When `routing.debug=true` in config
**Example:**
```go
// Add alongside existing X-CC-Relay-Provider, X-CC-Relay-Strategy headers

if h.routingDebug {
    state := tracker.GetState(providerName)
    w.Header().Set("X-CC-Relay-Health", state.String()) // "closed", "open", "half-open"
}
```

### Anti-Patterns to Avoid

- **Per-key circuit breakers:** Provider health is provider-level, not key-level. A bad key doesn't mean provider is down. Only track at provider level.
- **Circuit breaker wrapping entire handler:** Use `TwoStepCircuitBreaker.Allow()` at the routing layer, not wrapping the entire request flow.
- **Counting 4xx as failures:** Per CONTEXT.md, client errors (except 429) indicate bad requests, not provider health issues.
- **Instant recovery without probing:** The 3-probe rule in HALF-OPEN provides confidence the provider actually recovered.
- **Global circuit state:** Each provider needs independent health tracking. Provider A failing should not affect Provider B.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| State machine (CLOSED/OPEN/HALF-OPEN) | Custom state machine | `gobreaker.TwoStepCircuitBreaker` | Edge cases: concurrent transitions, timer races, counter resets |
| Consecutive failure counter | Manual counter with mutex | `gobreaker.Counts.ConsecutiveFailures` | Thread-safe, resets automatically on state change |
| Timeout-based OPEN->HALF-OPEN | Custom timer goroutine | gobreaker's built-in Timeout | Already handles race conditions, generation tracking |
| Rolling window failure rate | Sliding window implementation | gobreaker's BucketPeriod | Bucket-based rolling window built-in |

**Key insight:** Circuit breaker state machines have subtle concurrency bugs. gobreaker has been production-tested at Sony and is the go-to library in the Go ecosystem. The `TwoStepCircuitBreaker` variant specifically handles the HTTP proxy use case where you need to check permission before sending and report results after receiving.

## Common Pitfalls

### Pitfall 1: Opening Circuit on Context Cancellation
**What goes wrong:** Client disconnects (context canceled), circuit counts as failure, opens circuit
**Why it happens:** Default `IsSuccessful` treats all errors as failures
**How to avoid:** Configure `IsSuccessful` to exclude context.Canceled:
```go
IsSuccessful: func(err error) bool {
    return err == nil || errors.Is(err, context.Canceled)
}
```
**Warning signs:** Circuit opens when clients navigate away, refresh, or hit back button

### Pitfall 2: Counting 4xx Client Errors as Failures
**What goes wrong:** Bad requests (400, 401, 403, 404, 422) open circuit
**Why it happens:** HTTP client returns no error on 4xx, but downstream health check might count status
**How to avoid:** Per CONTEXT.md, only count 5xx + 429 + timeouts + connection errors. Pass status code to failure evaluation:
```go
// Report to circuit breaker with status-aware error
if statusCode >= 500 || statusCode == 429 {
    done(fmt.Errorf("server error: %d", statusCode))
} else {
    done(nil) // 4xx is not a provider health problem
}
```
**Warning signs:** Circuit opens when users send malformed requests

### Pitfall 3: Overly Aggressive Failure Threshold
**What goes wrong:** Single transient error (network blip) opens circuit
**Why it happens:** Threshold set too low (e.g., 1-2 failures)
**How to avoid:** Use CONTEXT.md default of 5 consecutive failures. Industry guidance suggests 5-10.
**Warning signs:** Circuit constantly flipping between CLOSED and OPEN

### Pitfall 4: Too Short Cooldown Period
**What goes wrong:** Circuit opens, immediately probes, provider still recovering, opens again
**Why it happens:** Timeout too short for provider to actually recover
**How to avoid:** Use CONTEXT.md default of 30 seconds. Consider provider's typical recovery time.
**Warning signs:** Rapid OPEN->HALF-OPEN->OPEN cycling in logs

### Pitfall 5: No Fallback During OPEN State
**What goes wrong:** Circuit open = all requests fail, user gets errors
**Why it happens:** Router doesn't skip unhealthy providers
**How to avoid:** Integration already exists: `FilterHealthy()` in router uses `IsHealthy` closure. Ensure circuit state flows to this closure correctly.
**Warning signs:** All requests fail when single provider is down despite multi-provider config

### Pitfall 6: Health Check Thundering Herd
**What goes wrong:** All instances probe recovering provider simultaneously
**Why it happens:** Synchronized periodic checks across instances
**How to avoid:** Add jitter to health check intervals:
```go
jitter := time.Duration(rand.Int63n(int64(2 * time.Second)))
ticker := time.NewTicker(10*time.Second + jitter)
```
**Warning signs:** Provider hit with burst of health checks every N seconds

## Code Examples

Verified patterns from official sources:

### Creating TwoStepCircuitBreaker with Custom Settings
```go
// Source: https://pkg.go.dev/github.com/sony/gobreaker/v2

import (
    "time"
    "github.com/sony/gobreaker/v2"
)

func NewProviderCircuitBreaker(name string, cfg CircuitBreakerConfig) *gobreaker.TwoStepCircuitBreaker[struct{}] {
    return gobreaker.NewTwoStepCircuitBreaker[struct{}](gobreaker.Settings{
        Name:        name,
        MaxRequests: uint32(cfg.HalfOpenProbes), // 3 probes in half-open
        Timeout:     cfg.OpenDuration,            // 30s before half-open

        ReadyToTrip: func(counts gobreaker.Counts) bool {
            return counts.ConsecutiveFailures >= uint32(cfg.FailureThreshold)
        },

        OnStateChange: func(name string, from, to gobreaker.State) {
            log.Warn().
                Str("provider", name).
                Str("from", from.String()).
                Str("to", to.String()).
                Msg("circuit breaker state change")
        },

        IsSuccessful: func(err error) bool {
            // Don't count context cancellation as failure
            return err == nil || errors.Is(err, context.Canceled)
        },
    })
}
```

### Using TwoStepCircuitBreaker in Request Flow
```go
// Source: https://pkg.go.dev/github.com/sony/gobreaker/v2

func (h *Handler) forwardWithCircuitBreaker(
    ctx context.Context,
    providerName string,
    req *http.Request,
) (*http.Response, error) {
    cb := h.tracker.GetCircuit(providerName)

    // Step 1: Check if request is allowed
    done, err := cb.Allow()
    if err != nil {
        // Circuit is OPEN - provider is unhealthy
        return nil, fmt.Errorf("circuit open for %s: %w", providerName, err)
    }

    // Step 2: Forward request
    resp, err := h.client.Do(req)

    // Step 3: Report outcome
    if err != nil {
        done(err) // Network error = failure
        return nil, err
    }

    // Step 4: Evaluate HTTP status
    if resp.StatusCode >= 500 || resp.StatusCode == 429 {
        done(fmt.Errorf("server error: %d", resp.StatusCode))
    } else {
        done(nil) // 2xx, 3xx, 4xx (except 429) = success
    }

    return resp, nil
}
```

### IsHealthy Closure Integration with Router
```go
// Source: Existing cc-relay pattern from internal/router/router.go

// In provider setup code:
providers := make([]router.ProviderInfo, 0, len(configs))
for _, cfg := range configs {
    provider := createProvider(cfg)

    providers = append(providers, router.ProviderInfo{
        Provider:  provider,
        IsHealthy: tracker.IsHealthyFunc(cfg.Name), // Circuit breaker closure
        Weight:    cfg.Weight,
        Priority:  cfg.Priority,
    })
}

// In HealthTracker:
func (t *HealthTracker) IsHealthyFunc(providerName string) func() bool {
    return func() bool {
        cb := t.getCircuit(providerName)
        if cb == nil {
            return true // No circuit breaker = assume healthy
        }
        // OPEN = unhealthy, CLOSED/HALF-OPEN = healthy (allow probes)
        return cb.State() != gobreaker.StateOpen
    }
}
```

### Configuration Struct for Circuit Breaker
```go
// Config structure to add to internal/config/config.go

// HealthConfig defines health tracking and circuit breaker settings.
type HealthConfig struct {
    // CircuitBreaker configures the circuit breaker state machine.
    CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`

    // HealthCheck configures periodic health checking.
    HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// CircuitBreakerConfig defines circuit breaker behavior.
type CircuitBreakerConfig struct {
    // FailureThreshold is consecutive failures before opening circuit.
    // Default: 5
    FailureThreshold int `yaml:"failure_threshold"`

    // OpenDurationMS is milliseconds to wait before half-open state.
    // Default: 30000 (30 seconds)
    OpenDurationMS int `yaml:"open_duration_ms"`

    // HalfOpenProbes is number of probes allowed in half-open state.
    // Default: 3
    HalfOpenProbes int `yaml:"half_open_probes"`
}

// HealthCheckConfig defines periodic health check behavior.
type HealthCheckConfig struct {
    // IntervalMS is milliseconds between health checks during OPEN state.
    // Default: 10000 (10 seconds)
    IntervalMS int `yaml:"interval_ms"`

    // Enabled enables synthetic health checks.
    // Default: true
    Enabled bool `yaml:"enabled"`
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| hystrix-go | sony/gobreaker | 2020+ | Netflix moved to Resilience4j; hystrix-go unmaintained |
| Non-generic circuit breaker | gobreaker/v2 with generics | 2024 | Type-safe return values, cleaner API |
| Fixed window counters | Rolling window with BucketPeriod | gobreaker 2.x | More accurate failure rate, smoother transitions |
| CircuitBreaker.Execute() | TwoStepCircuitBreaker.Allow() | gobreaker 2.x | Better fit for proxy/middleware patterns |

**Deprecated/outdated:**
- **hystrix-go:** Netflix stopped hystrix development; use gobreaker or resilience4j patterns instead
- **go-kit/kit/circuitbreaker:** Wrapper around gobreaker; use gobreaker directly for more control
- **afex/hystrix-go:** Unmaintained since 2020

## Open Questions

Things that couldn't be fully resolved:

1. **Provider-specific health check endpoints**
   - What we know: Some providers (Anthropic, OpenAI) have dedicated health/models endpoints; Ollama has `/api/tags`
   - What's unclear: Exact health check implementation for each supported provider (Z.AI, Bedrock, Azure, Vertex)
   - Recommendation: Start with a minimal API call (like listing models) as synthetic health check; refine per-provider in implementation

2. **Distributed circuit breaker state**
   - What we know: gobreaker v2.1+ has `DistributedCircuitBreaker` with `SharedDataStore` interface
   - What's unclear: Whether cc-relay needs distributed state (multiple proxy instances)
   - Recommendation: Start with local per-instance circuit breakers (simpler); add distributed state if needed later via SharedDataStore

3. **Counter reset behavior**
   - What we know: gobreaker resets counters on state change; CONTEXT.md mentions "reset on success vs sliding window"
   - What's unclear: Whether consecutive failure count should reset on a single success in CLOSED state
   - Recommendation: Use gobreaker's default behavior (consecutive failures reset on success); this is simpler and matches the CONTEXT.md preference

## Sources

### Primary (HIGH confidence)
- [sony/gobreaker v2 pkg.go.dev](https://pkg.go.dev/github.com/sony/gobreaker/v2) - Complete API documentation, Settings struct, State constants
- [sony/gobreaker GitHub](https://github.com/sony/gobreaker) - README, examples, v2 features
- [microservices.io Circuit Breaker Pattern](https://microservices.io/patterns/reliability/circuit-breaker.html) - Pattern definition, state machine

### Secondary (MEDIUM confidence)
- [OneUptime Go Circuit Breaker Tutorial (2026-01-07)](https://oneuptime.com/blog/post/2026-01-07-go-circuit-breaker/view) - Best practices, monitoring integration
- [Circuit Breaker Anti-patterns (moldstud.com)](https://moldstud.com/articles/p-top-common-pitfalls-when-implementing-circuit-breaker-pattern-in-microservices) - Common pitfalls, industry statistics
- [mercari/go-circuitbreaker GitHub](https://github.com/mercari/go-circuitbreaker) - Context-aware alternative, error classification patterns

### Tertiary (LOW confidence)
- [Medium GoTurkiye Circuit Breaker](https://medium.com/goturkiye/circuit-breaker-implementation-in-golang-efdfa40e49dc) - Custom implementation example
- WebSearch results for health check patterns - Various approaches, not specific to this use case

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - gobreaker is the established Go circuit breaker library with extensive documentation
- Architecture: HIGH - TwoStepCircuitBreaker pattern well-documented for HTTP proxy use cases
- Pitfalls: MEDIUM - Based on multiple sources agreeing, but some specifics unverified
- Health check specifics: LOW - Provider-specific health endpoints need verification during implementation

**Research date:** 2026-01-23
**Valid until:** 2026-02-23 (30 days - gobreaker is stable library with infrequent changes)
