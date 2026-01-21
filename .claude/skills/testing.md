# Testing Skill

Use this skill when working with tests in cc-relay.

## When to Use

- Running tests
- Writing new tests
- Debugging test failures
- Checking test coverage

## Quick Commands

```bash
# Run all tests
task test
go test ./...

# Quick tests (for pre-commit)
task test-short
go test -short ./...

# With coverage
task test-coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific package
go test ./internal/proxy/...

# Specific test
go test -run TestProxyHandler ./internal/proxy

# With race detection
go test -race ./...

# Verbose output
go test -v ./...

# Benchmarks
task bench
go test -bench=. -benchmem ./...

# Integration tests
task test-integration
go test -tags=integration ./...
```

## Test Organization

```
cc-relay/
├── internal/
│   ├── proxy/
│   │   ├── proxy.go
│   │   └── proxy_test.go
│   ├── router/
│   │   ├── router.go
│   │   └── router_test.go
│   └── providers/
│       ├── anthropic.go
│       └── anthropic_test.go
```

## Test Types

### 1. Unit Tests

```go
// proxy_test.go
func TestProxyHandler(t *testing.T) {
    proxy := NewProxy(config)

    req := httptest.NewRequest("POST", "/v1/messages", nil)
    rec := httptest.NewRecorder()

    proxy.ServeHTTP(rec, req)

    assert.Equal(t, 200, rec.Code)
}
```

### 2. Table-Driven Tests

```go
func TestRouter(t *testing.T) {
    tests := []struct {
        name     string
        strategy string
        want     Provider
    }{
        {"shuffle", "shuffle", provider1},
        {"round-robin", "round-robin", provider2},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := router.Select(tt.strategy)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### 3. Integration Tests

```go
//go:build integration
// +build integration

func TestFullFlow(t *testing.T) {
    // Test full request flow
}
```

### 4. Benchmarks

```go
func BenchmarkProxy(b *testing.B) {
    proxy := NewProxy(config)
    req := httptest.NewRequest("POST", "/v1/messages", nil)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        rec := httptest.NewRecorder()
        proxy.ServeHTTP(rec, req)
    }
}
```

## Test Helpers

### HTTP Testing

```go
import "net/http/httptest"

// Create test request
req := httptest.NewRequest("POST", "/endpoint", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")

// Create recorder
rec := httptest.NewRecorder()

// Test handler
handler.ServeHTTP(rec, req)

// Check response
assert.Equal(t, 200, rec.Code)
assert.Contains(t, rec.Body.String(), "expected")
```

### Mock Server

```go
// Create test server
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(200)
    w.Write([]byte(`{"result": "success"}`))
}))
defer server.Close()

// Use in test
client := NewClient(server.URL)
```

## Coverage

```bash
# Generate coverage
go test -coverprofile=coverage.out ./...

# View in browser
go tool cover -html=coverage.out

# Coverage report
go tool cover -func=coverage.out

# Coverage by package
go test -cover ./...
```

### Coverage Goals

- **Overall**: Aim for 80%+
- **Critical paths**: 90%+ (proxy, router, providers)
- **Utilities**: 70%+ acceptable
- **Generated code**: Exclude from coverage

## Test Fixtures

```go
// testdata/
// └── valid_request.json

func loadFixture(t *testing.T, name string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join("testdata", name))
    if err != nil {
        t.Fatalf("failed to load fixture: %v", err)
    }
    return data
}
```

## Parallel Tests

```go
func TestParallel(t *testing.T) {
    t.Parallel() // Run in parallel with other tests

    tests := []struct{}{...}

    for _, tt := range tests {
        tt := tt // capture
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // Run subtests in parallel
            // test code
        })
    }
}
```

## Race Detection

```bash
# Detect race conditions
go test -race ./...

# Common in:
# - Concurrent map access
# - Shared state without mutex
# - Goroutine lifecycle issues
```

## Test Debugging

```bash
# Verbose output
go test -v ./...

# Run specific test
go test -run TestName ./package

# Print test duration
go test -v -timeout 30s ./...

# Show test coverage gaps
go test -cover -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep "0.0%"
```

## CI Integration

- **Pre-commit**: Quick tests (`go test -short`)
- **Pre-push**: Full test suite with coverage
- **GitHub Actions**: Tests run on every PR

## Best Practices

1. **Name tests clearly**: `TestRouterFailover`
2. **Use table-driven tests**: For multiple scenarios
3. **Test error cases**: Not just happy path
4. **Use t.Helper()**: For test utilities
5. **Clean up resources**: Use `defer` and `t.Cleanup()`
6. **Parallel when possible**: Mark with `t.Parallel()`
7. **Test concurrency**: Use `-race` flag
8. **Mock external deps**: Don't hit real APIs
9. **Use testdata/**: For fixtures
10. **Check coverage**: Aim for 80%+

## Tips

- Run `task test-short` during development for quick feedback
- Run `task test-coverage` before PRs
- Use `go test -v` to see which tests are slow
- Add `-count=1` to disable test caching
- Use `-timeout` to prevent hanging tests
