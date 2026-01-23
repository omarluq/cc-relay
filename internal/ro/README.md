# internal/ro - Reactive Streams for cc-relay

This package provides reactive stream utilities for cc-relay using [samber/ro](https://github.com/samber/ro).

## Stability Notice

**samber/ro is v0.2.0 (pre-1.0 stability)**. This means:

- API may change between minor versions
- Use for non-critical paths first
- Monitor GitHub releases for breaking changes
- Consider abstraction layer if stability is critical

## When to Use

**Use ro streams when:**

- Processing actual streams (SSE, websockets, file watching)
- Event-driven architectures
- Need operators like debounce, throttle, buffer
- Complex async coordination

**Good candidates in cc-relay:**

- SSE streaming pipeline (future)
- Config hot-reload watching (future)
- Health check monitoring
- Rate limit event processing
- Metrics aggregation

## When NOT to Use

**Avoid ro streams when:**

- Simple request/response (use standard handlers)
- Synchronous operations (overhead not justified)
- Small, bounded data (use `samber/lo` instead)
- Critical hot paths (benchmark first)

**NOT recommended for:**

- Simple HTTP handlers (overkill)
- Single-item transformations
- Synchronous config loading
- Basic CRUD operations

## Quick Start

### Create Streams

```go
import ccro "github.com/omarluq/cc-relay/internal/ro"

// From channel (most common)
events := ccro.StreamFromChannel(eventChan)

// From slice
items := ccro.StreamFromSlice([]int{1, 2, 3})

// Single value
single := ccro.Just(42)
```

### Process Streams

```go
// Map and filter
result := ccro.ProcessStream(
    source,
    func(i int) int { return i * 2 },      // mapper
    func(i int) bool { return i > 4 },     // filter
)

// Filter only
filtered := ccro.FilterStream(source, func(i int) bool {
    return i % 2 == 0
})

// Map only
mapped := ccro.MapStream(source, func(i int) string {
    return strconv.Itoa(i)
})
```

### Collect Results

```go
// Blocking collect
results, err := ccro.Collect(stream)

// With context
results, ctx, err := ccro.CollectWithContext(ctx, stream)
```

### Operators

```go
import (
    ccro "github.com/omarluq/cc-relay/internal/ro"
    "github.com/samber/ro"
)

// Log each item
stream := ro.Pipe1(source, ccro.LogEach[Event](&logger, "events"))

// Add timeout
stream := ro.Pipe1(source, ccro.WithTimeout[Event](5*time.Second))

// Handle errors
stream := ro.Pipe1(source, ccro.Catch(func(err error) ro.Observable[Event] {
    return ccro.Just(fallbackEvent)
}))

// Remove duplicates
stream := ro.Pipe1(source, ccro.DistinctValues[int]())
```

### Graceful Shutdown

```go
// Wait for shutdown signal
sig, err := ccro.WaitForShutdown(ctx)

// Or register callback
sub := ccro.OnShutdown(ctx, func(sig os.Signal) {
    log.Info().Msgf("received %v, cleaning up...", sig)
    cleanup()
})
```

### Buffering

```go
// Buffer by time
batched := ccro.BufferWithTime(events, 100*time.Millisecond)

// Buffer by count
batched := ccro.BufferWithCount(events, 10)

// Buffer by time or count (whichever comes first)
batched := ccro.BufferWithTimeOrCount(events, 10, 100*time.Millisecond)
```

## Package Structure

| File | Purpose |
|------|---------|
| `streams.go` | Core stream creation and transformation functions |
| `operators.go` | Stream operators (logging, timeout, retry, catch, distinct) |
| `shutdown.go` | Graceful shutdown signal handling |

## Future Use Cases in cc-relay

### SSE Streaming Pipeline

```go
func streamSSE(w http.ResponseWriter, upstreamResp *http.Response) error {
    events := ccro.StreamFromChannel(readSSEEvents(upstreamResp.Body))

    stream := ro.Pipe2(
        events,
        ro.Filter(func(e SSEEvent) bool { return e.Type != "" }),
        ro.Map(func(e SSEEvent) []byte { return formatSSE(e) }),
    )

    done := make(chan struct{})
    ccro.SubscribeWithCallbacks(
        stream,
        func(data []byte) {
            w.Write(data)
            w.(http.Flusher).Flush()
        },
        func(err error) {
            log.Error().Err(err).Msg("SSE error")
            close(done)
        },
        func() { close(done) },
    )

    <-done
    return nil
}
```

### Config Hot-Reload

```go
func watchConfig(path string, onReload func(*Config)) {
    changes := ccro.StreamFromChannel(watchFile(path))

    stream := ro.Pipe2(
        changes,
        ro.Debounce(500*time.Millisecond),
        ro.Map(func(_ FileEvent) *Config {
            cfg, _ := loadConfig(path)
            return cfg
        }),
    )

    ccro.SubscribeWithCallbacks(stream, onReload, nil, nil)
}
```

## Comparison with Alternatives

| Approach | Use When |
|----------|----------|
| Raw channels | Simple producer/consumer, no operators needed |
| ro streams | Complex operators (debounce, buffer, combine) |
| lo collections | Bounded data, synchronous processing |
| goroutines | Fire-and-forget, simple parallel work |

## Performance Considerations

Streams add overhead compared to raw channels:

- Observable wrapper allocation
- Operator chain overhead
- Subscription management

**Benchmark before using in:**

- High-throughput paths (>10k events/sec)
- Low-latency requirements (<1ms)
- Memory-constrained environments

## Related Documentation

- [samber/ro GitHub](https://github.com/samber/ro)
- [ro Documentation](https://ro.samber.dev/)
- [.claude/skills/samber-ro.md](/.claude/skills/samber-ro.md) - Full API reference
- [.claude/skills/streams.md](/.claude/skills/streams.md) - Stream patterns
