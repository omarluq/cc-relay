# Error-to-Result Refactoring Agent

Automatically convert Go (value, error) tuples to samber/mo Result[T] monads.

## Purpose

Transform traditional Go error handling into Railway-Oriented Programming (ROP) patterns using mo.Result[T], enabling cleaner error chaining and reducing nested if-err checks.

## Input

- **Go file path** with functions to convert
- **Function name(s)** to convert (optional - converts all eligible if omitted)
- Example: `internal/auth/chain.go` or `internal/auth/chain.go:Validate`

## Process

### 1. Identify Candidate Functions

Scan for functions returning `(T, error)` that:
- Have multiple sequential operations that can fail
- Are called in chains (result of one feeds into next)
- Would benefit from FlatMap composition

**Good candidates:**
```go
// Multiple chained operations
func authenticateRequest(req *http.Request) (*AuthContext, error) {
    key, err := extractAPIKey(req)
    if err != nil { return nil, err }
    validated, err := validateKey(key)
    if err != nil { return nil, err }
    ctx, err := buildAuthContext(validated)
    if err != nil { return nil, err }
    return ctx, nil
}
```

**Skip conversion for:**
- Single-operation functions (just wrapping adds overhead, no benefit)
- Public API boundaries (keep (T, error) for compatibility)
- Performance-critical hot paths (benchmark first)

### 2. Convert Return Type

Reference: @.claude/skills/samber-mo.md

**Function signature change:**
```go
// Before
func processRequest(req *Request) (*Response, error)

// After
func processRequest(req *Request) mo.Result[*Response]
```

### 3. Convert Function Body

**Pattern: Sequential operations with early return**

```go
// Before (imperative)
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
    return extractAPIKeyResult(req).
        FlatMap(validateKeyResult).
        FlatMap(buildAuthContextResult)
}
```

### 4. Create Helper Functions (if needed)

Convert existing (T, error) functions to Result-returning variants:

```go
// Original function (keep for compatibility)
func extractAPIKey(req *http.Request) (string, error) {
    key := req.Header.Get("x-api-key")
    if key == "" {
        return "", errors.New("missing API key")
    }
    return key, nil
}

// Result variant (new)
func extractAPIKeyResult(req *http.Request) mo.Result[string] {
    key := req.Header.Get("x-api-key")
    if key == "" {
        return mo.Err[string](errors.New("missing API key"))
    }
    return mo.Ok(key)
}

// OR: Wrap existing function
func extractAPIKeyResult(req *http.Request) mo.Result[string] {
    return mo.TupleToResult(extractAPIKey(req))
}
```

### 5. Update Call Sites

**Pattern: Extracting result at boundary**

```go
// At API boundary, convert back to (T, error)
func handleRequest(w http.ResponseWriter, r *http.Request) {
    result := authenticateRequest(r)

    // Option 1: Get tuple
    ctx, err := result.Get()
    if err != nil {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }

    // Option 2: Pattern match
    result.Match(
        func(ctx *AuthContext) {
            // Success path
        },
        func(err error) {
            http.Error(w, err.Error(), http.StatusUnauthorized)
        },
    )
}
```

**Pattern: Chaining at intermediate layers**

```go
// Middleware or service layer - keep as Result
func processAuthenticatedRequest(req *http.Request) mo.Result[*Response] {
    return authenticateRequest(req).
        FlatMap(authorizeRequest).
        FlatMap(executeRequest)
}
```

### 6. Add Custom Error Types (if needed)

From cc-relay auth/chain.go:

```go
// ValidationError wraps authentication failure details.
type ValidationError struct {
    Type    Type
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
```

### 7. Ensure Import Added

```go
import "github.com/samber/mo"
```

### 8. Run Tests

```bash
go test ./path/to/package/...
```

Update tests to use new API:
```go
// Before
ctx, err := authenticateRequest(req)
assert.NoError(t, err)
assert.NotNil(t, ctx)

// After
result := authenticateRequest(req)
assert.True(t, result.IsOk())
ctx := result.MustGet()
assert.NotNil(t, ctx)

// Or keep using Get()
ctx, err := result.Get()
assert.NoError(t, err)
```

## cc-relay Examples

### KeyPool GetKeyResult (keypool/pool.go)

```go
// Result variant alongside traditional API
func (p *KeyPool) GetKeyResult(ctx context.Context) mo.Result[KeySelection] {
    keyID, apiKey, err := p.GetKey(ctx)
    if err != nil {
        return mo.Err[KeySelection](err)
    }
    return mo.Ok(KeySelection{KeyID: keyID, APIKey: apiKey})
}
```

### Auth Chain ValidateResult (auth/chain.go)

```go
func (c *ChainAuthenticator) ValidateResult(r *http.Request) mo.Result[Result] {
    result := c.Validate(r)
    if result.Valid {
        return mo.Ok(result)
    }
    return mo.Err[Result](NewValidationError(result.Type, result.Error))
}
```

## Output

- Modified Go file(s) with mo.Result returns
- Helper functions for wrapping existing code
- Updated call sites
- All tests passing

## Verification Checklist

- [ ] Only functions with chaining benefit converted
- [ ] Public API boundaries preserved (keep (T, error))
- [ ] Custom error types created where needed
- [ ] Import `github.com/samber/mo` added
- [ ] Call sites updated (FlatMap chains or Get at boundaries)
- [ ] All tests pass
- [ ] Error information preserved (no silent failures)

## Anti-patterns to Avoid

### 1. Immediate Unwrapping (defeats the purpose)

```go
// DON'T do this
result := doOperation()
if !result.IsOk() {
    return result.Error()
}
value := result.MustGet()
// ... continue

// DO chain operations
return doOperation().
    FlatMap(nextOperation).
    FlatMap(finalOperation)
```

### 2. Converting Single Operations

```go
// DON'T convert simple single-op functions
func readFile(path string) mo.Result[[]byte] {
    return mo.TupleToResult(os.ReadFile(path))
}

// Unless it's part of a chain
func loadAndParseConfig(path string) mo.Result[*Config] {
    return readFileResult(path).
        FlatMap(parseConfig).
        FlatMap(validateConfig)
}
```

### 3. Losing Error Context

```go
// DON'T lose error details
return mo.Err[T](errors.New("failed"))

// DO preserve error chain
return mo.Err[T](fmt.Errorf("operation failed: %w", originalErr))
```

## Related Skills

- @.claude/skills/samber-mo.md - Full mo reference
- @.claude/skills/error-handling.md - Error handling patterns

## Example Invocation

```
/refactor error-to-result internal/auth/chain.go
```

Or specific functions:

```
/refactor error-to-result internal/keypool/pool.go:GetKey,UpdateKeyFromHeaders
```
