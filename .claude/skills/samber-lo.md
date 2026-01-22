# samber/lo - Functional Collection Utilities

A comprehensive guide to using samber/lo in cc-relay for functional collection processing.

**Version:** v1.52.0
**Import:** `github.com/samber/lo`
**Docs:** https://lo.samber.dev/

## Quick Reference

### Most Used Functions

| Function | Purpose | Example |
|----------|---------|---------|
| `Filter` | Keep elements matching predicate | Filter active keys |
| `Map` | Transform each element | Extract API keys from providers |
| `Reduce` | Aggregate to single value | Sum total RPM |
| `GroupBy` | Group by key function | Group keys by provider |
| `Find` | First element matching predicate | Find key by ID |
| `Contains` | Check if element exists | Check if provider supported |
| `Uniq` | Remove duplicates | Unique model names |
| `Chunk` | Split into batches | Batch API requests |
| `FilterMap` | Filter + map in one pass | Extract valid API keys |

### Execution Models

lo provides four execution models via sub-packages:

| Package | Import | Use When |
|---------|--------|----------|
| `lo` | `github.com/samber/lo` | Normal single-threaded operations |
| `lop` | `github.com/samber/lo/parallel` | CPU-bound operations on large datasets (>1000 items) |
| `lom` | `github.com/samber/lo/mutable` | Mutating in-place (avoid allocations) |
| `lol` | `github.com/samber/lo/lazy` | Lazy evaluation for large/infinite streams |

## cc-relay Examples

### Filter Active Keys

```go
import "github.com/samber/lo"

// Before (imperative)
func getActiveKeys(keys []*keypool.KeyMetadata) []*keypool.KeyMetadata {
    var active []*keypool.KeyMetadata
    for _, k := range keys {
        if k.IsAvailable() {
            active = append(active, k)
        }
    }
    return active
}

// After (functional)
func getActiveKeys(keys []*keypool.KeyMetadata) []*keypool.KeyMetadata {
    return lo.Filter(keys, func(k *keypool.KeyMetadata, _ int) bool {
        return k.IsAvailable()
    })
}
```

### Map Providers to Names

```go
import "github.com/samber/lo"

// Extract provider names from config
func getProviderNames(providers []config.ProviderConfig) []string {
    return lo.Map(providers, func(p config.ProviderConfig, _ int) string {
        return p.Name
    })
}
```

### Group Keys by Provider

```go
import "github.com/samber/lo"

// Group API keys by their provider name
func groupKeysByProvider(keys []*keypool.KeyMetadata) map[string][]*keypool.KeyMetadata {
    return lo.GroupBy(keys, func(k *keypool.KeyMetadata) string {
        return k.ProviderName
    })
}
```

### Calculate Total Capacity

```go
import "github.com/samber/lo"

// Sum up remaining RPM across all keys
func getTotalRemainingRPM(keys []*keypool.KeyMetadata) int {
    return lo.Reduce(keys, func(total int, k *keypool.KeyMetadata, _ int) int {
        return total + k.GetRemainingRPM()
    }, 0)
}
```

### Find Key by ID

```go
import "github.com/samber/lo"

// Find specific key in pool
func findKeyByID(keys []*keypool.KeyMetadata, id string) (*keypool.KeyMetadata, bool) {
    return lo.Find(keys, func(k *keypool.KeyMetadata) bool {
        return k.ID == id
    })
}
```

### Filter and Transform in One Pass

```go
import "github.com/samber/lo"

// Get API keys only from active providers (single pass)
func getActiveAPIKeys(providers []config.ProviderConfig) []string {
    return lo.FilterMap(providers, func(p config.ProviderConfig, _ int) (string, bool) {
        if p.Enabled && p.APIKey != "" {
            return p.APIKey, true
        }
        return "", false
    })
}
```

### Check Provider Support

```go
import "github.com/samber/lo"

// Check if a provider name is in supported list
func isProviderSupported(name string) bool {
    supported := []string{"anthropic", "zai", "ollama", "bedrock", "azure", "vertex"}
    return lo.Contains(supported, name)
}
```

### Batch Requests

```go
import "github.com/samber/lo"

// Split keys into batches for parallel health checks
func batchKeys(keys []*keypool.KeyMetadata, batchSize int) [][]*keypool.KeyMetadata {
    return lo.Chunk(keys, batchSize)
}
```

### Parallel Processing for Large Datasets

```go
import lop "github.com/samber/lo/parallel"

// Filter with parallel execution for large key pools (>1000 keys)
func getActiveKeysParallel(keys []*keypool.KeyMetadata) []*keypool.KeyMetadata {
    return lop.Filter(keys, func(k *keypool.KeyMetadata, _ int) bool {
        return k.IsAvailable()
    })
}
```

### Unique Values

```go
import "github.com/samber/lo"

// Get unique model names from multiple providers
func getUniqueModels(providers []config.ProviderConfig) []string {
    allModels := lo.FlatMap(providers, func(p config.ProviderConfig, _ int) []string {
        return p.Models
    })
    return lo.Uniq(allModels)
}
```

### Key-Value Operations (Maps)

```go
import "github.com/samber/lo"

// Convert slice to map for O(1) lookup
func keysByID(keys []*keypool.KeyMetadata) map[string]*keypool.KeyMetadata {
    return lo.SliceToMap(keys, func(k *keypool.KeyMetadata) (string, *keypool.KeyMetadata) {
        return k.ID, k
    })
}

// Get all keys from a map
func getAllKeys(m map[string]*keypool.KeyMetadata) []*keypool.KeyMetadata {
    return lo.Values(m)
}

// Get all IDs from a map
func getAllIDs(m map[string]*keypool.KeyMetadata) []string {
    return lo.Keys(m)
}
```

### Partition by Condition

```go
import "github.com/samber/lo"

// Split keys into healthy and unhealthy groups
func partitionByHealth(keys []*keypool.KeyMetadata) (healthy, unhealthy []*keypool.KeyMetadata) {
    return lo.PartitionBy(keys, func(k *keypool.KeyMetadata) bool {
        return k.IsAvailable()
    })
}
```

## When to Use

**Use lo when:**
- Processing slices/maps with filter, map, reduce operations
- Need to group, partition, or deduplicate data
- Working with multiple transformations in sequence
- Code clarity matters (declarative > imperative)

**Use parallel (lop) when:**
- Dataset has >1000 items
- Operation is CPU-bound (not I/O)
- Order of results doesn't matter (or you'll sort after)
- Each element processing is independent

**Keep imperative when:**
- Very hot path (benchmark first!)
- Need early exit from loop
- Complex control flow (break, continue with conditions)
- Single-item operations

## When NOT to Use

**Avoid lo when:**
- Simple single-element operations (just use direct access)
- Need index-based access patterns
- Loop body has side effects that must execute in order
- Performance-critical code without benchmarking first

## Performance Tips

### 1. Use FilterMap for Combined Operations

```go
// BAD: Creates intermediate slice
keys := lo.Filter(all, isActive)
apiKeys := lo.Map(keys, extractAPIKey)

// GOOD: Single pass
apiKeys := lo.FilterMap(all, func(k *Key, _ int) (string, bool) {
    if isActive(k, 0) {
        return extractAPIKey(k, 0), true
    }
    return "", false
})
```

### 2. Use Parallel for Large Datasets

```go
import lop "github.com/samber/lo/parallel"

// Use lop for datasets > 1000 items
if len(keys) > 1000 {
    result = lop.Filter(keys, predicate)
} else {
    result = lo.Filter(keys, predicate)
}
```

### 3. Avoid Chaining When Performance Matters

```go
// BAD: 3 allocations, 3 passes
result := lo.Reduce(lo.Map(lo.Filter(items, f), m), r, init)

// GOOD: 1 pass, manual composition
result := lo.Reduce(items, func(acc T, item I, _ int) T {
    if f(item, 0) {
        return r(acc, m(item, 0), 0)
    }
    return acc
}, init)
```

### 4. Benchmark Before Refactoring Hot Paths

```go
func BenchmarkFilterKeys(b *testing.B) {
    keys := generateTestKeys(10000)

    b.Run("imperative", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            filterKeysImperative(keys)
        }
    })

    b.Run("lo.Filter", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            lo.Filter(keys, isActive)
        }
    })

    b.Run("lop.Filter", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            lop.Filter(keys, isActive)
        }
    })
}
```

## Common Pitfalls

### 1. Forgetting the Index Parameter

```go
// lo callbacks always have index as second parameter
lo.Filter(keys, func(k *Key, index int) bool {
    return k.Active // Use _ if index not needed
})
```

### 2. Modifying Slice During Iteration

```go
// BAD: lo doesn't prevent this, but it's undefined behavior
lo.Filter(keys, func(k *Key, _ int) bool {
    keys = append(keys, newKey) // DON'T
    return k.Active
})
```

### 3. Using Wrong Package for Mutability

```go
// lo.Filter creates new slice
filtered := lo.Filter(keys, pred) // keys unchanged

// Use lom for in-place mutation
import lom "github.com/samber/lo/mutable"
lom.Filter(&keys, pred) // keys modified in place
```

### 4. Expecting Order Preservation with Parallel

```go
// lop.Map may return results in different order
import lop "github.com/samber/lo/parallel"

// If order matters, sort after or use sequential lo
result := lop.Map(items, transform)
sort.Slice(result, func(i, j int) bool {
    return result[i].Order < result[j].Order
})
```

## Related Skills

- [samber-mo.md](samber-mo.md) - Monads (Option, Result) for error handling
- [samber-do.md](samber-do.md) - Dependency injection
- [samber-ro.md](samber-ro.md) - Reactive streams

## References

- [Official Documentation](https://lo.samber.dev/)
- [GitHub Repository](https://github.com/samber/lo)
- [API Reference](https://pkg.go.dev/github.com/samber/lo)
