# Loop-to-Lo Refactoring Agent

Automatically convert Go for-range loops to samber/lo functional expressions.

## Purpose

Transform imperative loops into declarative functional expressions using samber/lo, improving code readability and reducing cognitive complexity.

## Input

- **Go file path** or **package path** to refactor
- Example: `internal/keypool/pool.go` or `internal/keypool/...`

## Process

### 1. Identify Loop Patterns

Scan target files for `for-range` loops and classify by pattern:

| Pattern | Loop Signature | lo Function |
|---------|----------------|-------------|
| Filter | `for _, v := range slice { if cond { result = append(result, v) } }` | `lo.Filter` |
| Map | `for _, v := range slice { result = append(result, transform(v)) }` | `lo.Map` |
| FilterMap | Filter + transform in same loop | `lo.FilterMap` |
| Reduce | `for _, v := range slice { acc = combine(acc, v) }` | `lo.Reduce` |
| Find | `for _, v := range slice { if cond { return v } }` | `lo.Find` |
| FindMin/Max | Loop tracking min/max value | `lo.MinBy` / `lo.MaxBy` |
| ForEach | Loop with side effects (no accumulator) | `lo.ForEach` |
| GroupBy | Building `map[K][]V` from slice | `lo.GroupBy` |
| Chunk | Batching slice into fixed-size groups | `lo.Chunk` |
| SliceToMap | Building `map[K]V` from slice | `lo.SliceToMap` |

### 2. Assess Suitability

**CONVERT** when:
- Loop body is pure (no side effects besides accumulator)
- Pattern matches a lo function cleanly
- Loop is not in performance-critical hot path (or benchmarks exist)

**KEEP IMPERATIVE** when:
- Loop has early exit (`break`, `return` mid-loop) that doesn't fit `Find`
- Loop modifies multiple variables with complex control flow
- Index is used for non-trivial purposes (wraparound, sliding window)
- Loop has side effects that must execute in order

### 3. Convert to lo Function

Reference: @.claude/skills/samber-lo.md

**Filter Example** (from cc-relay keypool/pool.go):

```go
// Before (imperative)
for _, key := range keys {
    if key.IsAvailable() {
        available = append(available, key)
    }
}

// After (functional)
available := lo.Filter(keys, func(key *KeyMetadata, _ int) bool {
    return key.IsAvailable()
})
```

**FilterMap Example** (from cc-relay keypool/pool.go):

```go
// Before
var resetTimes []time.Time
for _, key := range p.keys {
    key.mu.RLock()
    resetAt := key.RPMResetAt
    key.mu.RUnlock()
    if !resetAt.IsZero() {
        resetTimes = append(resetTimes, resetAt)
    }
}

// After
resetTimes := lo.FilterMap(p.keys, func(key *KeyMetadata, _ int) (time.Time, bool) {
    key.mu.RLock()
    resetAt := key.RPMResetAt
    key.mu.RUnlock()
    return resetAt, !resetAt.IsZero()
})
```

**Reduce Example** (from cc-relay auth/chain.go):

```go
// Before
var result Result
for _, auth := range c.authenticators {
    result = auth.Validate(r)
    if result.Valid {
        break
    }
}

// After
result := lo.Reduce(c.authenticators, func(acc Result, auth Authenticator, _ int) Result {
    if acc.Valid {
        return acc  // Short-circuit: already found valid
    }
    return auth.Validate(r)
}, Result{Valid: false, Type: TypeNone})
```

**MinBy Example** (from cc-relay keypool/pool.go):

```go
// Before
var earliest time.Time
for _, t := range resetTimes {
    if earliest.IsZero() || t.Before(earliest) {
        earliest = t
    }
}

// After
earliest := lo.MinBy(resetTimes, func(a, b time.Time) bool {
    return a.Before(b)
})
```

### 4. Ensure Import Added

```go
import "github.com/samber/lo"
```

### 5. Run Tests

```bash
go test ./path/to/package/...
```

All existing tests must pass. If tests fail, investigate whether:
- The conversion has a bug (fix it)
- The test is overly specific to implementation details (update test)
- The loop shouldn't have been converted (revert)

### 6. Run Benchmarks (if exist)

```bash
go test -bench=. ./path/to/package/...
```

Compare performance. If significant regression (>20% slower), consider:
- Using `lop` (parallel) for large datasets
- Keeping imperative for hot paths
- Combining operations with `lo.FilterMap` instead of chained calls

## Output

- Modified Go file(s) with lo functions
- All tests passing
- Benchmark comparison (if applicable)

## Verification Checklist

- [ ] All identified loops classified
- [ ] Only suitable loops converted
- [ ] Import `github.com/samber/lo` added
- [ ] All tests pass
- [ ] No performance regression (benchmarked if available)
- [ ] Code is more readable than before

## Anti-patterns to Avoid

### 1. Over-converting

```go
// DON'T convert simple single-item loops
for _, provider := range providers {
    if provider.Name == name {
        return provider  // Find is fine, but this is clear
    }
}
```

### 2. Nested Complexity

```go
// DON'T create deeply nested lo calls
result := lo.Map(lo.Filter(lo.Map(items, f1), pred), f2)

// DO use intermediate variables or single-pass FilterMap
```

### 3. Ignoring Index When Needed

```go
// DON'T convert if index is semantically important
for i, v := range items {
    result[i] = transform(v, items[(i+1)%len(items)])  // Uses wraparound
}
```

## Related Skills

- @.claude/skills/samber-lo.md - Full lo function reference
- @.claude/skills/collections.md - Pattern selection guide

## Example Invocation

```
/refactor loop-to-lo internal/keypool/pool.go
```

Or for entire package:

```
/refactor loop-to-lo internal/providers/...
```
