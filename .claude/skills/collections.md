# Collection Processing Patterns

Patterns and best practices for functional collection processing with samber/lo in Go.

**Reference:** @.claude/skills/samber-lo.md for API details

## Function Selection Guide

### By Operation Type

| Need To | lo Function | Example |
|---------|-------------|---------|
| Keep matching items | `lo.Filter` | Active keys only |
| Transform each item | `lo.Map` | Extract API keys from configs |
| Filter + transform | `lo.FilterMap` | Active keys with their URLs |
| Aggregate to value | `lo.Reduce` | Sum of remaining RPM |
| Find first match | `lo.Find` | Key by ID |
| Find min/max | `lo.MinBy` / `lo.MaxBy` | Earliest reset time |
| Group by key | `lo.GroupBy` | Keys by provider |
| Split by condition | `lo.PartitionBy` | Healthy vs unhealthy |
| Deduplicate | `lo.Uniq` | Unique model names |
| Batch items | `lo.Chunk` | Batch for parallel processing |
| Check existence | `lo.Contains` | Is provider supported? |
| Slice to map | `lo.SliceToMap` | Keys indexed by ID |
| Flatten nested | `lo.FlatMap` | All models from all providers |
| Side effects | `lo.ForEach` | Log each item |
| Map iteration | `lo.Entries` + `lo.ForEach` | Process HTTP headers |

## Pattern 1: Filter

Keep elements matching a predicate.

```go
// cc-relay keypool/pool.go
func (p *KeyPool) GetKey(ctx context.Context) (string, string, error) {
    // Filter to only available keys
    availableKeys := lo.Filter(p.keys, func(key *KeyMetadata, _ int) bool {
        return key.IsAvailable()
    })

    if len(availableKeys) == 0 {
        return "", "", ErrAllKeysExhausted
    }
    // ... continue with selection
}
```

**When to use:**
- Need subset of slice matching condition
- Predicate is pure (no side effects)

## Pattern 2: Map

Transform each element.

```go
// cc-relay providers/base.go
func (p *BaseProvider) ListModels() []Model {
    return lo.Map(p.models, func(id string, _ int) Model {
        return Model{ID: id, Provider: p.name}
    })
}

// Transform config to key metadata
func buildKeyMetadata(configs []KeyConfig) []*KeyMetadata {
    return lo.Map(configs, func(cfg KeyConfig, _ int) *KeyMetadata {
        return NewKeyMetadata(cfg.APIKey, cfg.RPMLimit, cfg.ITPMLimit, cfg.OTPMLimit)
    })
}
```

**When to use:**
- Transform slice to different type
- One-to-one mapping

## Pattern 3: FilterMap (Combined)

Filter and transform in single pass - more efficient than chaining.

```go
// cc-relay keypool/pool.go - extract non-zero reset times
resetTimes := lo.FilterMap(p.keys, func(key *KeyMetadata, _ int) (time.Time, bool) {
    key.mu.RLock()
    resetAt := key.RPMResetAt
    key.mu.RUnlock()
    return resetAt, !resetAt.IsZero()
})

// Get active API keys from providers
func getActiveAPIKeys(providers []ProviderConfig) []string {
    return lo.FilterMap(providers, func(p ProviderConfig, _ int) (string, bool) {
        if p.Enabled && p.APIKey != "" {
            return p.APIKey, true
        }
        return "", false
    })
}
```

**When to use:**
- Need both filter and transform
- Single pass is more efficient
- Cleaner than `lo.Map(lo.Filter(...))`

## Pattern 4: Reduce

Aggregate slice to single value.

```go
// cc-relay keypool/pool.go - aggregate stats
func (p *KeyPool) GetStats() PoolStats {
    return lo.Reduce(p.keys, func(stats PoolStats, key *KeyMetadata, _ int) PoolStats {
        key.mu.RLock()
        stats.TotalRPM += key.RPMLimit
        stats.RemainingRPM += key.RPMRemaining
        key.mu.RUnlock()

        if key.IsAvailable() {
            stats.AvailableKeys++
        } else {
            stats.ExhaustedKeys++
        }
        return stats
    }, PoolStats{TotalKeys: len(p.keys)})
}

// cc-relay auth/chain.go - find first valid result
func (c *ChainAuthenticator) Validate(r *http.Request) Result {
    return lo.Reduce(c.authenticators, func(acc Result, auth Authenticator, _ int) Result {
        if acc.Valid {
            return acc  // Short-circuit on valid
        }
        return auth.Validate(r)
    }, Result{Valid: false, Type: TypeNone})
}
```

**When to use:**
- Sum, count, aggregate operations
- Building accumulator from elements
- Short-circuit iteration (check accumulator first)

## Pattern 5: MinBy / MaxBy

Find minimum or maximum by comparison.

```go
// cc-relay keypool/pool.go - find earliest reset time
func (p *KeyPool) GetEarliestResetTime() time.Duration {
    resetTimes := lo.FilterMap(p.keys, extractResetTime)

    if len(resetTimes) == 0 {
        return 60 * time.Second
    }

    // MinBy comparison: return true if a < b
    earliest := lo.MinBy(resetTimes, func(a, b time.Time) bool {
        return a.Before(b)
    })

    duration := time.Until(earliest)
    if duration < 0 {
        return 0
    }
    return duration
}

// cc-relay keypool/least_loaded.go - find key with most capacity
func (s *LeastLoadedSelector) Select(keys []*KeyMetadata) (*KeyMetadata, error) {
    available := lo.Filter(keys, func(k *KeyMetadata, _ int) bool {
        return k.IsAvailable()
    })

    if len(available) == 0 {
        return nil, ErrAllKeysExhausted
    }

    // MaxBy comparison: return true if a > b (a should replace b as max)
    return lo.MaxBy(available, func(a, b *KeyMetadata) bool {
        return a.CapacityScore() > b.CapacityScore()
    }), nil
}
```

**Comparison semantics:**
- `MinBy(a, b)` returns true if `a < b`
- `MaxBy(a, b)` returns true if `a > b` (a should replace b as new max)

## Pattern 6: GroupBy / PartitionBy

Group elements by key or split by condition.

```go
// Group keys by provider
func groupKeysByProvider(keys []*KeyMetadata) map[string][]*KeyMetadata {
    return lo.GroupBy(keys, func(k *KeyMetadata) string {
        return k.ProviderName
    })
}

// Split into healthy/unhealthy
func partitionByHealth(keys []*KeyMetadata) (healthy, unhealthy []*KeyMetadata) {
    return lo.PartitionBy(keys, func(k *KeyMetadata) bool {
        return k.IsAvailable()
    })
}
```

## Pattern 7: FlatMap

Flatten nested slices.

```go
// cc-relay proxy/handler.go - all models from all providers
func getAllModels(providers []Provider) []Model {
    return lo.FlatMap(providers, func(p Provider, _ int) []Model {
        return p.ListModels()
    })
}

// All keys from all providers
func getAllKeys(configs []ProviderConfig) []string {
    return lo.FlatMap(configs, func(p ProviderConfig, _ int) []string {
        return lo.Map(p.Keys, func(k KeyConfig, _ int) string {
            return k.Key
        })
    })
}
```

## Pattern 8: SliceToMap

Convert slice to map for O(1) lookup.

```go
// cc-relay keypool/pool.go - index keys by ID
func indexKeysById(keys []*KeyMetadata) map[string]*KeyMetadata {
    return lo.SliceToMap(keys, func(k *KeyMetadata) (string, *KeyMetadata) {
        return k.ID, k
    })
}

// Index providers by name
func indexProvidersByName(providers []Provider) map[string]Provider {
    return lo.SliceToMap(providers, func(p Provider) (string, Provider) {
        return p.Name(), p
    })
}
```

## Pattern 9: ForEach with Entries (Map Iteration)

Process map entries with functional style.

```go
// cc-relay providers/base.go - copy headers
func copyHeaders(dest, src http.Header) {
    lo.ForEach(lo.Entries(src), func(entry lo.Entry[string, []string], _ int) {
        for _, v := range entry.Value {
            dest.Add(entry.Key, v)
        }
    })
}

// Log all config values
func logConfig(cfg map[string]string) {
    lo.ForEach(lo.Entries(cfg), func(entry lo.Entry[string, string], _ int) {
        log.Info().Str(entry.Key, entry.Value).Msg("config")
    })
}
```

## Pattern 10: Parallel Processing

Use `lop` package for large datasets.

```go
import lop "github.com/samber/lo/parallel"

// Parallel filter for large key pools (>1000 keys)
func getActiveKeysParallel(keys []*KeyMetadata) []*KeyMetadata {
    if len(keys) < 1000 {
        return lo.Filter(keys, func(k *KeyMetadata, _ int) bool {
            return k.IsAvailable()
        })
    }
    return lop.Filter(keys, func(k *KeyMetadata, _ int) bool {
        return k.IsAvailable()
    })
}

// Parallel health checks
func healthCheckAll(providers []Provider) []HealthStatus {
    return lop.Map(providers, func(p Provider, _ int) HealthStatus {
        return p.HealthCheck()  // I/O bound, benefits from parallel
    })
}
```

**When to use parallel:**
- Dataset > 1000 items
- Operation is CPU or I/O bound
- Results order doesn't matter (or sort after)

## When NOT to Use lo

### 1. Simple Single-Item Operations

```go
// OVERKILL
result := lo.Find(items, func(i Item) bool { return i.ID == id })

// FINE as-is (clear and simple)
for _, item := range items {
    if item.ID == id {
        return item
    }
}
```

### 2. Index-Based Logic

```go
// DON'T convert - index is semantically important
for i, v := range items {
    result[i] = combine(v, items[(i+1)%len(items)])  // Wraparound
}
```

### 3. Complex Control Flow

```go
// DON'T convert - break/continue with conditions
for _, item := range items {
    if shouldSkip(item) {
        continue
    }
    if shouldStop(item) {
        break
    }
    // complex processing...
}
```

### 4. Side Effects with Order Dependency

```go
// DON'T convert - order matters for side effects
for _, item := range items {
    if prev != nil {
        compare(prev, item)  // Needs previous item
    }
    prev = item
}
```

### 5. Hot Paths Without Benchmarks

```go
// BENCHMARK FIRST for hot paths
func BenchmarkFilterMethods(b *testing.B) {
    keys := generateTestKeys(10000)

    b.Run("imperative", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            filterImperative(keys)
        }
    })

    b.Run("lo.Filter", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            lo.Filter(keys, isActive)
        }
    })
}
```

## Performance Considerations

### 1. Avoid Chained Allocations

```go
// BAD: 3 allocations, 3 passes
result := lo.Reduce(lo.Map(lo.Filter(items, f), m), r, init)

// GOOD: Single FilterMap or combined Reduce
result := lo.FilterMap(items, func(i Item, _ int) (T, bool) {
    if f(i, 0) {
        return m(i, 0), true
    }
    return *new(T), false
})
```

### 2. Pre-allocate When Size Known

```go
// lo.Filter doesn't know final size - may reallocate
filtered := lo.Filter(items, pred)

// If you need exact control, use imperative
result := make([]*Item, 0, estimatedSize)
for _, item := range items {
    if pred(item, 0) {
        result = append(result, item)
    }
}
```

### 3. Use FilterMap Over Map+Filter

```go
// BAD: Two passes, intermediate slice
keys := lo.Filter(items, isActive)
apiKeys := lo.Map(keys, extractKey)

// GOOD: Single pass
apiKeys := lo.FilterMap(items, func(i Item, _ int) (string, bool) {
    if isActive(i, 0) {
        return extractKey(i, 0), true
    }
    return "", false
})
```

## Common Pitfalls

### 1. Forgetting Index Parameter

```go
// lo callbacks always have index
lo.Filter(keys, func(k *Key, _ int) bool {  // _ for unused index
    return k.Active
})
```

### 2. MaxBy Comparison Direction

```go
// MaxBy: return true if 'a' should replace 'b' as new max
// This means: return true if a > b
lo.MaxBy(items, func(a, b Item) bool {
    return a.Score > b.Score  // a replaces b if a is bigger
})
```

### 3. Parallel Order Not Preserved

```go
// lop.Map may return results in different order
import lop "github.com/samber/lo/parallel"

result := lop.Map(items, transform)
// Sort if order matters
sort.Slice(result, func(i, j int) bool {
    return result[i].Order < result[j].Order
})
```

## Related Skills

- @.claude/skills/samber-lo.md - Full API reference
- @.claude/agents/loop-to-lo.md - Conversion agent
