# samber/ro - Reactive Streams

A guide to using samber/ro in cc-relay for reactive stream processing.

**Version:** v0.2.0 (pre-1.0 - use cautiously)
**Import:** `github.com/samber/ro`
**Docs:** https://ro.samber.dev/

## Important Notice

samber/ro is v0.2.0, which is pre-1.0 stability. This means:
- API may change between minor versions
- Use for non-critical paths first
- Monitor GitHub releases for breaking changes
- Consider abstraction layer if stability concerns arise

## Quick Reference

### Observable Creation

| Function | Purpose | Example |
|----------|---------|---------|
| `ro.Just(v)` | Single value | `ro.Just(42)` |
| `ro.FromSlice(s)` | From slice | `ro.FromSlice(items)` |
| `ro.FromChannel(ch)` | From channel | `ro.FromChannel(ch)` |
| `ro.Empty[T]()` | No values | `ro.Empty[int]()` |
| `ro.Never[T]()` | Never completes | `ro.Never[int]()` |
| `ro.Throw[T](err)` | Immediate error | `ro.Throw[int](err)` |

### Core Operators

| Operator | Purpose | Example |
|----------|---------|---------|
| `ro.Map` | Transform values | `ro.Map(o, transform)` |
| `ro.Filter` | Keep matching values | `ro.Filter(o, predicate)` |
| `ro.Pipe` | Chain operators | `ro.Pipe(o, op1, op2)` |
| `ro.Take` | First N values | `ro.Take(o, 10)` |
| `ro.Skip` | Skip N values | `ro.Skip(o, 5)` |
| `ro.Distinct` | Remove duplicates | `ro.Distinct(o)` |
| `ro.Catch` | Handle errors | `ro.Catch(o, handler)` |

### Subscription

```go
observable.Subscribe(ro.Observer[T]{
    OnNext:     func(v T) { /* handle value */ },
    OnError:    func(err error) { /* handle error */ },
    OnComplete: func() { /* stream ended */ },
})
```

## cc-relay Examples

### SSE Streaming Pipeline (Future)

```go
import (
    "github.com/samber/ro"
)

// Process SSE events from upstream provider
func streamSSE(w http.ResponseWriter, upstreamResp *http.Response) error {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")

    // Create observable from SSE reader
    events := ro.FromChannel(readSSEEvents(upstreamResp.Body))

    // Process stream
    stream := ro.Pipe(
        events,
        ro.Filter(func(e SSEEvent) bool {
            return e.Type != "" // Skip empty events
        }),
        ro.Map(func(e SSEEvent) []byte {
            return formatSSE(e)
        }),
    )

    // Subscribe and forward to client
    done := make(chan struct{})
    stream.Subscribe(ro.Observer[[]byte]{
        OnNext: func(data []byte) {
            w.Write(data)
            w.(http.Flusher).Flush()
        },
        OnError: func(err error) {
            log.Error().Err(err).Msg("SSE stream error")
            close(done)
        },
        OnComplete: func() {
            close(done)
        },
    })

    <-done
    return nil
}

// Helper: Convert io.Reader to event channel
func readSSEEvents(r io.Reader) <-chan SSEEvent {
    ch := make(chan SSEEvent)
    go func() {
        defer close(ch)
        scanner := bufio.NewScanner(r)
        for scanner.Scan() {
            if event := parseSSELine(scanner.Text()); event.Type != "" {
                ch <- event
            }
        }
    }()
    return ch
}
```

### Event Logging Stream

```go
import "github.com/samber/ro"

// Stream request events for logging
type RequestEvent struct {
    Timestamp time.Time
    RequestID string
    Status    int
    Duration  time.Duration
}

func setupEventLogging(events <-chan RequestEvent) {
    stream := ro.FromChannel(events)

    // Log errors separately
    errorStream := ro.Filter(stream, func(e RequestEvent) bool {
        return e.Status >= 500
    })

    // Log slow requests
    slowStream := ro.Filter(stream, func(e RequestEvent) bool {
        return e.Duration > 5*time.Second
    })

    // Subscribe to error stream
    errorStream.Subscribe(ro.Observer[RequestEvent]{
        OnNext: func(e RequestEvent) {
            log.Error().
                Str("request_id", e.RequestID).
                Int("status", e.Status).
                Msg("request failed")
        },
    })

    // Subscribe to slow stream
    slowStream.Subscribe(ro.Observer[RequestEvent]{
        OnNext: func(e RequestEvent) {
            log.Warn().
                Str("request_id", e.RequestID).
                Dur("duration", e.Duration).
                Msg("slow request")
        },
    })
}
```

### Batched Key Updates

```go
import "github.com/samber/ro"

// Batch key metadata updates
func batchKeyUpdates(updates <-chan KeyUpdate) {
    stream := ro.FromChannel(updates)

    // Batch updates every 100ms or 10 items
    batched := ro.BufferTime(stream, 100*time.Millisecond, 10)

    batched.Subscribe(ro.Observer[[]KeyUpdate]{
        OnNext: func(batch []KeyUpdate) {
            if len(batch) > 0 {
                applyKeyUpdates(batch)
            }
        },
    })
}
```

### Rate Limit Monitoring

```go
import "github.com/samber/ro"

// Monitor rate limit events
type RateLimitEvent struct {
    KeyID     string
    Remaining int
    ResetAt   time.Time
}

func monitorRateLimits(events <-chan RateLimitEvent) {
    stream := ro.FromChannel(events)

    // Alert when approaching limit
    lowCapacity := ro.Filter(stream, func(e RateLimitEvent) bool {
        return e.Remaining < 10 // Less than 10% capacity
    })

    lowCapacity.Subscribe(ro.Observer[RateLimitEvent]{
        OnNext: func(e RateLimitEvent) {
            log.Warn().
                Str("key_id", e.KeyID).
                Int("remaining", e.Remaining).
                Time("reset_at", e.ResetAt).
                Msg("rate limit approaching")
        },
    })
}
```

### Config Hot-Reload (Future)

```go
import (
    "github.com/samber/ro"
)

// Watch config file for changes
func watchConfig(path string, onReload func(*Config)) {
    changes := ro.FromChannel(watchFile(path))

    // Debounce rapid changes
    debounced := ro.Debounce(changes, 500*time.Millisecond)

    // Reload on each change
    reloaded := ro.Map(debounced, func(_ FileEvent) *Config {
        cfg, err := loadConfig(path)
        if err != nil {
            log.Error().Err(err).Msg("failed to reload config")
            return nil
        }
        return cfg
    })

    // Filter successful reloads
    valid := ro.Filter(reloaded, func(cfg *Config) bool {
        return cfg != nil
    })

    valid.Subscribe(ro.Observer[*Config]{
        OnNext: onReload,
    })
}
```

### Health Check Stream

```go
import "github.com/samber/ro"

// Periodic health checks with reactive stream
func healthCheckStream(providers []Provider, interval time.Duration) ro.Observable[HealthStatus] {
    ticker := time.NewTicker(interval)
    ch := make(chan HealthStatus)

    go func() {
        defer close(ch)
        for range ticker.C {
            for _, p := range providers {
                status := p.HealthCheck()
                ch <- status
            }
        }
    }()

    return ro.FromChannel(ch)
}

// Subscribe to unhealthy providers
func monitorHealth(providers []Provider) {
    stream := healthCheckStream(providers, 30*time.Second)

    unhealthy := ro.Filter(stream, func(s HealthStatus) bool {
        return !s.Healthy
    })

    unhealthy.Subscribe(ro.Observer[HealthStatus]{
        OnNext: func(s HealthStatus) {
            log.Warn().
                Str("provider", s.ProviderName).
                Str("reason", s.Reason).
                Msg("provider unhealthy")
        },
    })
}
```

## When to Use

**Use ro when:**
- Processing actual streams (SSE, websockets, file watching)
- Event-driven architectures
- Need operators like debounce, throttle, buffer
- Complex async coordination

**Good candidates in cc-relay:**
- SSE streaming pipeline (when implemented)
- Config hot-reload watching
- Health check monitoring
- Rate limit event processing
- Metrics aggregation

## When NOT to Use

**Avoid ro when:**
- Simple request/response (use normal handlers)
- Synchronous operations (overhead not justified)
- Small, bounded data (use lo instead)
- Critical hot paths (benchmark first)

**NOT recommended for:**
- Simple HTTP handlers (overkill)
- Single-item transformations
- Synchronous config loading
- Basic CRUD operations

## Comparison with Alternatives

| Approach | Use When | Example |
|----------|----------|---------|
| Channels | Simple producer/consumer | One goroutine feeds another |
| ro streams | Complex operators needed | Debounce, buffer, combine |
| lo collections | Bounded data | Transform slice in memory |
| goroutines | Fire-and-forget | Async logging, metrics |

## Performance Considerations

ro streams add overhead compared to raw channels:
- Observable wrapper allocation
- Operator chain overhead
- Subscription management

**Benchmark before using in:**
- High-throughput paths (>10k events/sec)
- Low-latency requirements (<1ms)
- Memory-constrained environments

## Error Handling

```go
import "github.com/samber/ro"

// Catch and recover from errors
stream := ro.Pipe(
    ro.FromChannel(events),
    ro.Map(transform),
    ro.Catch(func(err error) ro.Observable[T] {
        log.Error().Err(err).Msg("stream error")
        // Return fallback observable
        return ro.Just(fallbackValue)
    }),
)
```

## Common Patterns

### Timeout

```go
// Timeout if no value within duration
result := ro.Timeout(stream, 5*time.Second)
```

### Retry

```go
// Retry on error
result := ro.Retry(stream, 3) // 3 retries
```

### Combine Multiple Streams

```go
// Merge multiple streams
merged := ro.Merge(stream1, stream2, stream3)

// Combine latest from each
combined := ro.CombineLatest(stream1, stream2)
```

## Related Skills

- [samber-lo.md](samber-lo.md) - Functional collection utilities (for bounded data)
- [samber-mo.md](samber-mo.md) - Monads for error handling
- [samber-do.md](samber-do.md) - Dependency injection

## References

- [GitHub Repository](https://github.com/samber/ro)
- [Documentation](https://ro.samber.dev/)
- [API Reference](https://pkg.go.dev/github.com/samber/ro)
- [ReactiveX Specification](http://reactivex.io/)
