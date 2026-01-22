# samber/mo - Monads for Go

A comprehensive guide to using samber/mo in cc-relay for functional error handling and optional values.

**Version:** v1.16.0
**Import:** `github.com/samber/mo`
**Docs:** https://github.com/samber/mo

## Quick Reference

### Core Types

| Type | Purpose | Go Equivalent |
|------|---------|---------------|
| `Result[T]` | Success or failure | `(T, error)` tuple |
| `Option[T]` | Present or absent | `*T` pointer (nullable) |
| `Either[L, R]` | One of two types | Tagged union |
| `Future[T]` | Async computation | Channel + goroutine |

### Result[T] Methods

| Method | Purpose | Example |
|--------|---------|---------|
| `mo.Ok(v)` | Create success | `mo.Ok(user)` |
| `mo.Err[T](err)` | Create failure | `mo.Err[User](err)` |
| `mo.TupleToResult` | Convert from (T, error) | `mo.TupleToResult(db.Find(id))` |
| `r.IsOk()` | Check success | `if r.IsOk() { ... }` |
| `r.IsError()` | Check failure | `if r.IsError() { ... }` |
| `r.Get()` | Unwrap (value, error) | `v, err := r.Get()` |
| `r.MustGet()` | Unwrap or panic | `v := r.MustGet()` |
| `r.OrElse(def)` | Default on error | `v := r.OrElse(defaultVal)` |
| `r.Map(f)` | Transform success | `r.Map(toDTO)` |
| `r.FlatMap(f)` | Chain operations | `r.FlatMap(validate)` |
| `r.Match(ok, err)` | Pattern match | `r.Match(onSuccess, onError)` |

### Option[T] Methods

| Method | Purpose | Example |
|--------|---------|---------|
| `mo.Some(v)` | Create present value | `mo.Some(42)` |
| `mo.None[T]()` | Create absent value | `mo.None[int]()` |
| `o.IsPresent()` | Check if present | `if o.IsPresent() { ... }` |
| `o.IsAbsent()` | Check if absent | `if o.IsAbsent() { ... }` |
| `o.Get()` | Unwrap (value, ok) | `v, ok := o.Get()` |
| `o.MustGet()` | Unwrap or panic | `v := o.MustGet()` |
| `o.OrElse(def)` | Default if absent | `v := o.OrElse(30)` |
| `o.Map(f)` | Transform if present | `o.Map(double)` |
| `o.FlatMap(f)` | Chain operations | `o.FlatMap(lookup)` |
| `o.Match(some, none)` | Pattern match | `o.Match(present, absent)` |

## cc-relay Examples

### Authentication Chain with Result

```go
import "github.com/samber/mo"

// Before (traditional Go)
func authenticateRequest(req *http.Request) (*AuthContext, error) {
    key, err := extractAPIKey(req)
    if err != nil {
        return nil, fmt.Errorf("extract key: %w", err)
    }

    validated, err := validateKey(key)
    if err != nil {
        return nil, fmt.Errorf("validate key: %w", err)
    }

    ctx, err := buildAuthContext(validated)
    if err != nil {
        return nil, fmt.Errorf("build context: %w", err)
    }

    return ctx, nil
}

// After (monadic)
func authenticateRequest(req *http.Request) mo.Result[*AuthContext] {
    return extractAPIKey(req).
        FlatMap(validateKey).
        FlatMap(buildAuthContext)
}

// Helper: convert traditional function to Result-returning
func extractAPIKey(req *http.Request) mo.Result[string] {
    key := req.Header.Get("x-api-key")
    if key == "" {
        return mo.Err[string](errors.New("missing API key"))
    }
    return mo.Ok(key)
}
```

### Config Field Defaults with Option

```go
import "github.com/samber/mo"

// Config with optional fields
type ServerConfig struct {
    Port         int
    ReadTimeout  mo.Option[time.Duration]
    WriteTimeout mo.Option[time.Duration]
    MaxConns     mo.Option[int]
}

// Use defaults for absent values
func (c *ServerConfig) GetReadTimeout() time.Duration {
    return c.ReadTimeout.OrElse(10 * time.Second)
}

func (c *ServerConfig) GetWriteTimeout() time.Duration {
    return c.WriteTimeout.OrElse(600 * time.Second)
}

func (c *ServerConfig) GetMaxConns() int {
    return c.MaxConns.OrElse(1000)
}
```

### Route Selection with Result Chain

```go
import "github.com/samber/mo"

// Chain provider selection with error handling
func selectProviderForRequest(req *Request, pool *KeyPool) mo.Result[*RouteDecision] {
    return validateRequest(req).
        FlatMap(func(r *Request) mo.Result[*KeyMetadata] {
            return selectKey(pool, r)
        }).
        FlatMap(func(k *KeyMetadata) mo.Result[Provider] {
            return lookupProvider(k.ProviderName)
        }).
        Map(func(p Provider) *RouteDecision {
            return &RouteDecision{Provider: p, Request: req}
        })
}

func selectKey(pool *KeyPool, req *Request) mo.Result[*KeyMetadata] {
    key, err := pool.GetKey()
    return mo.TupleToResult(key, err)
}

func lookupProvider(name string) mo.Result[Provider] {
    p, ok := providers[name]
    if !ok {
        return mo.Err[Provider](fmt.Errorf("unknown provider: %s", name))
    }
    return mo.Ok(p)
}
```

### Pattern Matching for Response Handling

```go
import "github.com/samber/mo"

// Handle both success and error paths
func handleProxyResult(w http.ResponseWriter, result mo.Result[*Response]) {
    result.Match(
        func(resp *Response) {
            w.WriteHeader(resp.StatusCode)
            json.NewEncoder(w).Encode(resp.Body)
        },
        func(err error) {
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(map[string]string{
                "error": err.Error(),
            })
        },
    )
}
```

### Converting Existing Functions

```go
import "github.com/samber/mo"

// Wrap existing (T, error) function
func loadConfig(path string) mo.Result[*Config] {
    cfg, err := config.Load(path)
    return mo.TupleToResult(cfg, err)
}

// Chain with other Result functions
func initializeServer() mo.Result[*Server] {
    return loadConfig("config.yaml").
        FlatMap(validateConfig).
        FlatMap(buildServer)
}
```

### Optional Key Metadata Fields

```go
import "github.com/samber/mo"

// Key with optional rate limit overrides
type KeyConfig struct {
    Key        string
    RPMLimit   mo.Option[int]
    ITPMLimit  mo.Option[int]
    OTPMLimit  mo.Option[int]
}

// Apply overrides or use provider defaults
func (k *KeyConfig) GetEffectiveRPM(providerDefault int) int {
    return k.RPMLimit.OrElse(providerDefault)
}

// Transform optional value
func (k *KeyConfig) GetRPMPerSecond() mo.Option[float64] {
    return k.RPMLimit.Map(func(rpm int) float64 {
        return float64(rpm) / 60.0
    })
}
```

### Either for Provider Response Types

```go
import "github.com/samber/mo"

// Provider can return streaming or non-streaming response
type ProviderResponse = mo.Either[*StreamingResponse, *DirectResponse]

func handleProviderResponse(resp ProviderResponse) {
    if resp.IsLeft() {
        streaming, _ := resp.Left()
        handleStreaming(streaming)
    } else {
        direct, _ := resp.Right()
        handleDirect(direct)
    }
}
```

### Future for Async Health Checks

```go
import "github.com/samber/mo"

// Async health check
func healthCheckAsync(provider Provider) mo.Future[HealthStatus] {
    return mo.NewFuture(func(resolve func(HealthStatus), reject func(error)) {
        status, err := provider.HealthCheck()
        if err != nil {
            reject(err)
        } else {
            resolve(status)
        }
    })
}

// Collect results
func checkAllProviders(providers []Provider) []mo.Future[HealthStatus] {
    futures := make([]mo.Future[HealthStatus], len(providers))
    for i, p := range providers {
        futures[i] = healthCheckAsync(p)
    }
    return futures
}
```

## When to Use

### Use Result[T] when:
- Function can fail (returns error)
- You want to chain operations that might fail
- Error handling would create deep nesting
- You want explicit error type in signature

### Use Option[T] when:
- Value may be absent (not an error)
- Config fields with defaults
- Cache lookups (miss is normal)
- Optional function parameters

### Use Either[L, R] when:
- Function returns one of two distinct types
- Response can be different formats
- Tagged union pattern needed

### Use Future[T] when:
- Async computation with result
- Parallel operations to collect
- Timeout handling needed

## When NOT to Use

**Avoid mo when:**
- Simple single operations (over-engineering)
- Team unfamiliar with monadic patterns
- Interop with libraries expecting (T, error)
- Hot paths without benchmarking

**At API boundaries:**
- Convert back to (T, error) for public APIs
- Keep mo internal to packages

## Converting Between Styles

### Result to Tuple

```go
// Convert Result back to (T, error) for external APIs
func PublicAPI() (*Response, error) {
    result := internalOperation()
    return result.Get()
}
```

### Tuple to Result

```go
// Convert (T, error) to Result for chaining
func chainOperations() mo.Result[*FinalResult] {
    return mo.TupleToResult(step1()).
        FlatMap(func(r1 *Step1Result) mo.Result[*Step2Result] {
            return mo.TupleToResult(step2(r1))
        }).
        FlatMap(func(r2 *Step2Result) mo.Result[*FinalResult] {
            return mo.TupleToResult(step3(r2))
        })
}
```

### Option to Pointer

```go
// Convert Option to pointer (for JSON, etc.)
func (c *Config) TimeoutPtr() *int {
    if c.Timeout.IsPresent() {
        v := c.Timeout.MustGet()
        return &v
    }
    return nil
}

// Convert pointer to Option
func optionFromPtr[T any](ptr *T) mo.Option[T] {
    if ptr == nil {
        return mo.None[T]()
    }
    return mo.Some(*ptr)
}
```

## Common Pitfalls

### 1. Immediate Unwrapping (Defeats the Purpose)

```go
// BAD: Immediate unwrap defeats monadic benefit
result := Validate(req)
if !result.IsOk() {
    return result.Error()
}
value := result.MustGet()
// ... continue with value

// GOOD: Chain operations
return Validate(req).
    FlatMap(Process).
    FlatMap(Store)
```

### 2. Using MustGet Without Checking

```go
// BAD: May panic
value := result.MustGet()

// GOOD: Check first or use OrElse
if result.IsOk() {
    value := result.MustGet()
}

// OR use default
value := result.OrElse(defaultValue)

// OR use pattern matching
result.Match(
    func(v T) { /* use v */ },
    func(err error) { /* handle error */ },
)
```

### 3. Mixing Styles Inconsistently

```go
// BAD: Mixed styles in same package
func op1() mo.Result[T] { ... }
func op2() (T, error) { ... }  // Different pattern!

// GOOD: Consistent within package, convert at boundaries
func op1() mo.Result[T] { ... }
func op2() mo.Result[T] { ... }

// At public API boundary
func PublicOp() (T, error) {
    return op1().FlatMap(op2).Get()
}
```

### 4. Ignoring Error in Match

```go
// BAD: Silently ignore error
result.Match(
    func(v T) { use(v) },
    func(err error) { }, // Silent discard
)

// GOOD: Always handle or log errors
result.Match(
    func(v T) { use(v) },
    func(err error) { log.Error().Err(err).Msg("operation failed") },
)
```

## Performance Considerations

### Allocation Overhead

Result and Option are value types in Go generics, but wrapping values does add minor overhead:

```go
// Benchmark to compare
func BenchmarkResultVsTuple(b *testing.B) {
    b.Run("tuple", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            v, err := tupleFunc()
            if err != nil {
                continue
            }
            _ = v
        }
    })

    b.Run("result", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            r := resultFunc()
            if r.IsError() {
                continue
            }
            _ = r.MustGet()
        }
    })
}
```

### When Performance Matters

For hot paths (>100k ops/sec), benchmark first:

```go
// If benchmarks show overhead matters
// Keep traditional style in hot paths
func hotPath(data []byte) (Result, error) {
    // Traditional error handling for performance
    if err := validate(data); err != nil {
        return Result{}, err
    }
    return process(data)
}

// Use mo in orchestration/business logic
func handleRequest(req *Request) mo.Result[*Response] {
    return validate(req).
        FlatMap(enrich).
        FlatMap(route)
}
```

## JSON Serialization

### Option with JSON

```go
import "github.com/samber/mo"

type Config struct {
    Name    string             `json:"name"`
    Timeout mo.Option[int]     `json:"timeout,omitempty"`
}

// mo.Option supports JSON marshaling:
// - Some(v) -> v
// - None -> null (or omitted with omitempty)
```

### Result with JSON

Results should typically be converted to a concrete type before JSON encoding:

```go
type APIResponse struct {
    Data  json.RawMessage `json:"data,omitempty"`
    Error string          `json:"error,omitempty"`
}

func toAPIResponse(result mo.Result[*Data]) APIResponse {
    if result.IsOk() {
        data, _ := json.Marshal(result.MustGet())
        return APIResponse{Data: data}
    }
    return APIResponse{Error: result.Error().Error()}
}
```

## Related Skills

- [samber-lo.md](samber-lo.md) - Functional collection utilities
- [samber-do.md](samber-do.md) - Dependency injection
- [samber-ro.md](samber-ro.md) - Reactive streams

## References

- [GitHub Repository](https://github.com/samber/mo)
- [API Reference](https://pkg.go.dev/github.com/samber/mo)
- [Railway Oriented Programming](https://fsharpforfunandprofit.com/rop/)
