# Reactive Streams Patterns

Patterns for reactive stream processing with samber/ro in Go.

**Reference:** @.claude/skills/samber-ro.md for API details

## Stability Notice

samber/ro is v0.2.0 (pre-1.0). This means:
- API may change between minor versions
- Use for non-critical paths first
- Monitor GitHub releases for breaking changes
- Consider abstraction layer if stability is critical

## When to Use Streams

| Use Case | Streams? | Alternative |
|----------|----------|-------------|
| SSE/WebSocket data | Yes | Raw channels |
| Event-driven processing | Yes | Callbacks |
| Config hot-reload watching | Yes | fsnotify + callback |
| Request/response handling | No | Standard HTTP |
| Single-value operations | No | Direct functions |
| Small bounded data | No | `lo` functions |

### Decision Guide

**Use ro streams when:**
- Processing actual streams (SSE, WebSocket, file watching)
- Need operators like debounce, throttle, buffer
- Multiple consumers of same stream
- Complex async coordination

**Don't use streams when:**
- Simple request/response (standard HTTP handlers)
- Synchronous operations
- Small, bounded data (use `lo` instead)
- Single consumer of channel

## Pattern 1: Observable Creation

### From Channel (most common)

```go
import "github.com/samber/ro"

// Convert channel to observable
func streamFromChannel(ch <-chan Event) ro.Observable[Event] {
    return ro.FromChannel(ch)
}

// SSE event reader
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

// Usage
events := ro.FromChannel(readSSEEvents(response.Body))
```

### From Slice (bounded data)

```go
// Create observable from existing data
items := []Event{event1, event2, event3}
stream := ro.FromSlice(items)
```

### Single Value

```go
// Single value observable
single := ro.Just(config)

// Empty observable
empty := ro.Empty[Event]()

// Error observable
errStream := ro.Throw[Event](errors.New("connection failed"))
```

## Pattern 2: Operators

### Filter

```go
// Keep only error events
errorEvents := ro.Filter(events, func(e Event) bool {
    return e.Level == "error"
})

// Keep events for specific provider
providerEvents := ro.Filter(events, func(e RequestEvent) bool {
    return e.ProviderName == "anthropic"
})
```

### Map

```go
// Transform events
formatted := ro.Map(events, func(e Event) []byte {
    return formatSSE(e)
})

// Extract specific field
requestIDs := ro.Map(events, func(e RequestEvent) string {
    return e.RequestID
})
```

### Pipe (Chaining)

```go
// Chain multiple operators
stream := ro.Pipe(
    events,
    ro.Filter(func(e Event) bool { return e.Type != "" }),
    ro.Map(func(e Event) []byte { return formatSSE(e) }),
)
```

### Take / Skip

```go
// First 10 events
first10 := ro.Take(events, 10)

// Skip first 5 events
remaining := ro.Skip(events, 5)
```

### Distinct

```go
// Remove duplicate events (by equality)
unique := ro.Distinct(events)
```

## Pattern 3: Error Handling

### Catch

```go
// Handle errors and provide fallback
stream := ro.Pipe(
    events,
    ro.Map(transform),
    ro.Catch(func(err error) ro.Observable[Event] {
        log.Error().Err(err).Msg("stream error")
        return ro.Just(fallbackEvent)  // Continue with fallback
    }),
)
```

### Retry

```go
// Retry on error
resilient := ro.Retry(stream, 3)  // 3 retries
```

### Timeout

```go
// Timeout if no value
withTimeout := ro.Timeout(stream, 5*time.Second)
```

## Pattern 4: Subscription

### Basic Subscription

```go
stream.Subscribe(ro.Observer[Event]{
    OnNext: func(e Event) {
        // Handle each event
        processEvent(e)
    },
    OnError: func(err error) {
        // Handle errors
        log.Error().Err(err).Msg("stream error")
    },
    OnComplete: func() {
        // Stream ended
        log.Info().Msg("stream complete")
    },
})
```

### Blocking Until Complete

```go
done := make(chan struct{})

stream.Subscribe(ro.Observer[Event]{
    OnNext: func(e Event) {
        processEvent(e)
    },
    OnComplete: func() {
        close(done)
    },
})

<-done  // Wait for stream to complete
```

## Pattern 5: Combining Streams

### Merge

```go
// Combine multiple streams
merged := ro.Merge(stream1, stream2, stream3)
```

### CombineLatest

```go
// Combine latest values from each stream
combined := ro.CombineLatest(configStream, healthStream)
```

## Future cc-relay Use Cases

### SSE Streaming Pipeline

```go
// Process SSE events from upstream provider
func streamSSE(w http.ResponseWriter, upstreamResp *http.Response) error {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")

    events := ro.FromChannel(readSSEEvents(upstreamResp.Body))

    stream := ro.Pipe(
        events,
        ro.Filter(func(e SSEEvent) bool {
            return e.Type != ""  // Skip empty
        }),
        ro.Map(func(e SSEEvent) []byte {
            return formatSSE(e)
        }),
    )

    done := make(chan struct{})

    stream.Subscribe(ro.Observer[[]byte]{
        OnNext: func(data []byte) {
            w.Write(data)
            w.(http.Flusher).Flush()
        },
        OnError: func(err error) {
            log.Error().Err(err).Msg("SSE error")
            close(done)
        },
        OnComplete: func() {
            close(done)
        },
    })

    <-done
    return nil
}
```

### Config Hot-Reload

```go
// Watch config file for changes
func watchConfig(path string, onReload func(*Config)) {
    changes := ro.FromChannel(watchFile(path))

    // Debounce rapid changes
    debounced := ro.Debounce(changes, 500*time.Millisecond)

    // Reload and validate
    stream := ro.Pipe(
        debounced,
        ro.Map(func(_ FileEvent) *Config {
            cfg, err := loadConfig(path)
            if err != nil {
                log.Error().Err(err).Msg("config reload failed")
                return nil
            }
            return cfg
        }),
        ro.Filter(func(cfg *Config) bool {
            return cfg != nil
        }),
    )

    stream.Subscribe(ro.Observer[*Config]{
        OnNext: onReload,
    })
}
```

### Rate Limit Monitoring

```go
// Monitor rate limit events
func monitorRateLimits(events <-chan RateLimitEvent) {
    stream := ro.FromChannel(events)

    // Alert on low capacity
    lowCapacity := ro.Filter(stream, func(e RateLimitEvent) bool {
        return e.Remaining < 10
    })

    lowCapacity.Subscribe(ro.Observer[RateLimitEvent]{
        OnNext: func(e RateLimitEvent) {
            log.Warn().
                Str("key_id", e.KeyID).
                Int("remaining", e.Remaining).
                Msg("rate limit approaching")
        },
    })
}
```

### Request Event Logging

```go
// Stream request events for logging
func setupEventLogging(events <-chan RequestEvent) {
    stream := ro.FromChannel(events)

    // Log errors
    errorStream := ro.Filter(stream, func(e RequestEvent) bool {
        return e.Status >= 500
    })

    errorStream.Subscribe(ro.Observer[RequestEvent]{
        OnNext: func(e RequestEvent) {
            log.Error().
                Str("request_id", e.RequestID).
                Int("status", e.Status).
                Msg("request failed")
        },
    })

    // Log slow requests
    slowStream := ro.Filter(stream, func(e RequestEvent) bool {
        return e.Duration > 5*time.Second
    })

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

### Health Check Stream

```go
// Periodic health checks
func healthCheckStream(providers []Provider, interval time.Duration) ro.Observable[HealthStatus] {
    ticker := time.NewTicker(interval)
    ch := make(chan HealthStatus)

    go func() {
        defer close(ch)
        for range ticker.C {
            for _, p := range providers {
                ch <- p.HealthCheck()
            }
        }
    }()

    return ro.FromChannel(ch)
}

// Monitor unhealthy providers
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

## Performance Considerations

Streams add overhead compared to raw channels:
- Observable wrapper allocation
- Operator chain overhead
- Subscription management

**Benchmark before using in:**
- High-throughput paths (>10k events/sec)
- Low-latency requirements (<1ms)
- Memory-constrained environments

```go
func BenchmarkStreamVsChannel(b *testing.B) {
    ch := make(chan Event, 1000)

    // Fill channel
    go func() {
        for i := 0; i < b.N; i++ {
            ch <- Event{ID: i}
        }
        close(ch)
    }()

    b.Run("channel", func(b *testing.B) {
        for e := range ch {
            _ = e
        }
    })

    b.Run("ro.FromChannel", func(b *testing.B) {
        stream := ro.FromChannel(ch)
        done := make(chan struct{})
        stream.Subscribe(ro.Observer[Event]{
            OnNext:     func(e Event) { _ = e },
            OnComplete: func() { close(done) },
        })
        <-done
    })
}
```

## Anti-patterns

### 1. Using Streams for Simple Request/Response

```go
// OVERKILL
func handleRequest(r *http.Request) ro.Observable[Response] {
    return ro.Just(processRequest(r))
}

// JUST DO IT DIRECTLY
func handleRequest(r *http.Request) (*Response, error) {
    return processRequest(r)
}
```

### 2. Creating Streams for Bounded Data

```go
// WRONG: Use lo for bounded data
items := []Item{item1, item2, item3}
filtered := ro.Filter(ro.FromSlice(items), pred)

// RIGHT: Use lo
filtered := lo.Filter(items, pred)
```

### 3. Ignoring OnError

```go
// BAD: Silent failure
stream.Subscribe(ro.Observer[Event]{
    OnNext: func(e Event) { process(e) },
    // OnError not implemented - errors silently ignored
})

// GOOD: Always handle errors
stream.Subscribe(ro.Observer[Event]{
    OnNext: func(e Event) { process(e) },
    OnError: func(err error) {
        log.Error().Err(err).Msg("stream error")
    },
})
```

### 4. Not Waiting for Completion

```go
// BAD: Function returns before stream completes
func processStream(stream ro.Observable[Event]) {
    stream.Subscribe(ro.Observer[Event]{
        OnNext: func(e Event) { process(e) },
    })
    // Returns immediately, processing may not complete!
}

// GOOD: Wait for completion
func processStream(stream ro.Observable[Event]) {
    done := make(chan struct{})
    stream.Subscribe(ro.Observer[Event]{
        OnNext: func(e Event) { process(e) },
        OnComplete: func() { close(done) },
    })
    <-done  // Wait
}
```

## Comparison with Alternatives

| Approach | Pros | Cons | Use When |
|----------|------|------|----------|
| Raw channels | Simple, no deps | No operators | Simple producer/consumer |
| ro streams | Rich operators, composition | Overhead, pre-1.0 | Complex stream processing |
| lo collections | Fast, stable | Bounded data only | Transform slices/maps |
| goroutines | Flexible | Manual coordination | Fire-and-forget, parallel |

## Related Skills

- @.claude/skills/samber-ro.md - Full API reference
- @.claude/skills/samber-lo.md - Collection processing (bounded data)
