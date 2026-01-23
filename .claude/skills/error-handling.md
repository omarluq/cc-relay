# Error Handling Patterns

Patterns for error handling in Go using samber/mo Result monad and traditional (T, error) returns.

**Reference:** @.claude/skills/samber-mo.md for API details

## When to Use Which

| Pattern | Use Case | Example |
|---------|----------|---------|
| `(T, error)` | Public APIs, interop, single operations | `func Load(path string) (*Config, error)` |
| `mo.Result[T]` | Chained operations, internal logic | `validateRequest().FlatMap(process)` |
| `mo.Option[T]` | Nullable fields, optional values | `config.Timeout.OrElse(defaultTimeout)` |

### Decision Guide

**Use `(T, error)` when:**
- Function is public API (exported)
- Single operation (no chaining benefit)
- Interacting with stdlib or other libraries
- Simplicity matters more than composition

**Use `mo.Result[T]` when:**
- Multiple sequential operations that can fail
- Operations naturally chain (output of one feeds into next)
- Railway-Oriented Programming improves readability
- Internal package code (not public API)

## Pattern 1: Railway-Oriented Programming

Chain operations that can fail, short-circuiting on first error.

```go
// cc-relay example: Auth chain validation
func authenticateRequest(req *http.Request) mo.Result[*AuthContext] {
    return extractAPIKeyResult(req).
        FlatMap(validateKeyResult).
        FlatMap(buildAuthContextResult)
}

// Each step returns mo.Result
func extractAPIKeyResult(req *http.Request) mo.Result[string] {
    key := req.Header.Get("x-api-key")
    if key == "" {
        return mo.Err[string](errors.New("missing API key"))
    }
    return mo.Ok(key)
}

func validateKeyResult(key string) mo.Result[*ValidatedKey] {
    if !isValidFormat(key) {
        return mo.Err[*ValidatedKey](errors.New("invalid key format"))
    }
    return mo.Ok(&ValidatedKey{Key: key})
}

func buildAuthContextResult(vk *ValidatedKey) mo.Result[*AuthContext] {
    ctx, err := lookupUser(vk.Key)
    return mo.TupleToResult(ctx, err)
}
```

**Benefits:**
- Linear flow (no nested if-err blocks)
- Early exit on any error
- Error context preserved through chain

## Pattern 2: FlatMap Composition

Transform and chain operations.

```go
// Complex pipeline with multiple transformations
func processRequest(req *Request) mo.Result[*Response] {
    return validateRequest(req).
        FlatMap(func(r *Request) mo.Result[*KeySelection] {
            return pool.GetKeyResult(r.Context())
        }).
        FlatMap(func(k *KeySelection) mo.Result[Provider] {
            return lookupProvider(k.ProviderName)
        }).
        FlatMap(func(p Provider) mo.Result[*Response] {
            return forwardRequest(p, req)
        })
}
```

## Pattern 3: Map for Simple Transformations

Transform success value without changing error type.

```go
// Map transforms success, passes through error
func getProviderURL(name string) mo.Result[string] {
    return lookupProvider(name).Map(func(p Provider) string {
        return p.BaseURL()
    })
}

// cc-relay example: Extract key ID from selection
func getKeyID(ctx context.Context) mo.Result[string] {
    return pool.GetKeyResult(ctx).Map(func(sel KeySelection) string {
        return sel.KeyID
    })
}
```

## Pattern 4: Pattern Matching with Match

Handle both success and error cases explicitly.

```go
// cc-relay HTTP handler pattern
func handleRequest(w http.ResponseWriter, r *http.Request) {
    result := processRequest(r)

    result.Match(
        func(resp *Response) {
            w.WriteHeader(resp.StatusCode)
            json.NewEncoder(w).Encode(resp.Body)
        },
        func(err error) {
            log.Error().Err(err).Msg("request failed")
            writeErrorResponse(w, err)
        },
    )
}

// Exhaustive handling - compiler ensures both cases handled
```

## Pattern 5: OrElse for Defaults

Provide fallback value on error.

```go
// Simple default
timeout := config.GetTimeoutResult().OrElse(30 * time.Second)

// cc-relay: Default key selection on pool failure
func selectKeyWithFallback(ctx context.Context) *KeySelection {
    return pool.GetKeyResult(ctx).OrElse(&KeySelection{
        KeyID:  "default",
        APIKey: cfg.DefaultAPIKey,
    })
}
```

## Pattern 6: Converting at API Boundaries

Keep Result internal, convert at public boundaries.

```go
// Internal: Uses Result for chaining
func validateAndProcessInternal(req *Request) mo.Result[*Response] {
    return validateRequest(req).
        FlatMap(processRequest)
}

// Public API: Returns (T, error) for compatibility
func ValidateAndProcess(req *Request) (*Response, error) {
    return validateAndProcessInternal(req).Get()
}

// HTTP Handler: Converts Result to HTTP response
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    result := validateAndProcessInternal(parseRequest(r))

    // Convert to HTTP response
    resp, err := result.Get()
    if err != nil {
        writeError(w, err)
        return
    }
    writeSuccess(w, resp)
}
```

## Pattern 7: Custom Error Types

Create typed errors for better error handling.

```go
// cc-relay auth/chain.go pattern
type ValidationError struct {
    Type    Type    // APIKey, Bearer, etc.
    Message string
}

func (e *ValidationError) Error() string {
    return e.Message
}

func NewValidationError(authType Type, message string) *ValidationError {
    return &ValidationError{
        Type:    authType,
        Message: message,
    }
}

// Usage in Result
func (c *ChainAuthenticator) ValidateResult(r *http.Request) mo.Result[Result] {
    result := c.Validate(r)
    if result.Valid {
        return mo.Ok(result)
    }
    return mo.Err[Result](NewValidationError(result.Type, result.Error))
}

// Caller can type-assert for specific handling
result.Match(
    func(r Result) { /* success */ },
    func(err error) {
        if vErr, ok := err.(*ValidationError); ok {
            // Handle validation error specifically
            log.Warn().Str("type", string(vErr.Type)).Msg(vErr.Message)
        } else {
            // Handle other errors
            log.Error().Err(err).Msg("unexpected error")
        }
    },
)
```

## Pattern 8: mo.Option for Nullable Values

Handle optional/nullable values without nil checks.

```go
// cc-relay config pattern
type ServerConfig struct {
    Port         int
    ReadTimeout  mo.Option[time.Duration]
    WriteTimeout mo.Option[time.Duration]
}

// Safe access with defaults
func (c *ServerConfig) GetReadTimeout() time.Duration {
    return c.ReadTimeout.OrElse(10 * time.Second)
}

func (c *ServerConfig) GetWriteTimeout() time.Duration {
    return c.WriteTimeout.OrElse(600 * time.Second)
}

// Transform optional value
func (c *ServerConfig) GetReadTimeoutMS() mo.Option[int64] {
    return c.ReadTimeout.Map(func(d time.Duration) int64 {
        return d.Milliseconds()
    })
}
```

## cc-relay Real Examples

### KeyPool GetKeyResult (keypool/pool.go)

```go
// KeySelection contains the result of a successful key selection.
type KeySelection struct {
    KeyID  string
    APIKey string
}

// GetKeyResult selects a key using Railway-Oriented Programming.
func (p *KeyPool) GetKeyResult(ctx context.Context) mo.Result[KeySelection] {
    keyID, apiKey, err := p.GetKey(ctx)
    if err != nil {
        return mo.Err[KeySelection](err)
    }
    return mo.Ok(KeySelection{KeyID: keyID, APIKey: apiKey})
}

// UpdateKeyFromHeadersResult wraps update operation.
func (p *KeyPool) UpdateKeyFromHeadersResult(keyID string, headers http.Header) mo.Result[bool] {
    err := p.UpdateKeyFromHeaders(keyID, headers)
    if err != nil {
        return mo.Err[bool](err)
    }
    return mo.Ok(true)
}
```

### Auth Chain ValidateResult (auth/chain.go)

```go
// ValidateResult tries each authenticator and returns Result.
func (c *ChainAuthenticator) ValidateResult(r *http.Request) mo.Result[Result] {
    result := c.Validate(r)
    if result.Valid {
        return mo.Ok(result)
    }
    return mo.Err[Result](NewValidationError(result.Type, result.Error))
}
```

### Chained Usage Example

```go
// Full request pipeline using Result chaining
func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
    // Chain: authenticate -> select key -> forward -> respond
    result := chain.ValidateResult(r).
        FlatMap(func(_ Result) mo.Result[KeySelection] {
            return pool.GetKeyResult(r.Context())
        }).
        FlatMap(func(sel KeySelection) mo.Result[*Response] {
            return forwardToProvider(r, sel)
        })

    // Handle at boundary
    result.Match(
        func(resp *Response) {
            w.WriteHeader(resp.StatusCode)
            w.Write(resp.Body)
        },
        func(err error) {
            log.Error().Err(err).Msg("request failed")
            writeAPIError(w, err)
        },
    )
}
```

## Anti-patterns

### 1. Immediate Unwrapping

```go
// BAD: Defeats the purpose of Result
result := doOperation()
if result.IsError() {
    return nil, result.Error()
}
value := result.MustGet()
// ... continue

// GOOD: Chain operations
return doOperation().
    FlatMap(nextOperation).
    FlatMap(finalOperation)
```

### 2. MustGet Without Checking

```go
// BAD: Panics on error
value := result.MustGet()

// GOOD: Check first or use safe methods
if result.IsOk() {
    value := result.MustGet()
}
// OR
value := result.OrElse(defaultValue)
// OR
result.Match(handleSuccess, handleError)
```

### 3. Converting Simple Operations

```go
// BAD: Overkill for single operation
func readConfig(path string) mo.Result[[]byte] {
    return mo.TupleToResult(os.ReadFile(path))
}

// GOOD: Keep simple, convert when chaining
func loadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    return parseConfig(data)
}
```

### 4. Losing Error Context

```go
// BAD: Generic error loses context
return mo.Err[T](errors.New("failed"))

// GOOD: Wrap with context
return mo.Err[T](fmt.Errorf("operation X failed for Y: %w", originalErr))
```

### 5. Mixing Styles Inconsistently

```go
// BAD: Mixed in same package
func op1() mo.Result[T] { ... }
func op2() (T, error) { ... }  // Confusing!

// GOOD: Consistent within package, convert at boundaries
func op1() mo.Result[T] { ... }
func op2() mo.Result[T] { ... }

// Public API converts
func PublicOp() (T, error) {
    return op1().FlatMap(op2).Get()
}
```

## Error Handling Strategy Summary

| Layer | Pattern | Example |
|-------|---------|---------|
| **Public API** | `(T, error)` | `func Load(path string) (*Config, error)` |
| **Internal Logic** | `mo.Result[T]` | `validateRequest().FlatMap(process)` |
| **HTTP Handlers** | `Match` or `Get` | `result.Match(success, error)` |
| **Config Fields** | `mo.Option[T]` | `cfg.Timeout.OrElse(default)` |
| **Typed Errors** | Custom error types | `*ValidationError` |

## Related Skills

- @.claude/skills/samber-mo.md - mo API reference
- @.claude/agents/error-to-result.md - Conversion agent
