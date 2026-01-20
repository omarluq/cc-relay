# Testing Patterns

**Analysis Date:** 2026-01-20

## Test Framework

**Runner:**
- Go standard testing package (`testing`)
- Command: `go test ./...` (all tests)
- Verbose: `go test -v ./...` (show individual test execution)
- Specific package: `go test -v ./internal/proxy`

**Assertion Library:**
- Go standard testing package with manual assertions (`if got != want { t.Errorf(...) }`)
- No external assertion framework specified (follow Go conventions)
- Error reporting pattern: `t.Errorf("got %v, want %v", got, want)`
- Fatal assertions: `t.Fatalf()` when condition is required for continuing

**Run Commands:**
```bash
go test ./...                           # Run all tests
go test -v ./...                        # Verbose output with individual tests
go test -v ./internal/proxy             # Test specific package
go test -v ./internal/proxy -run TestProxyHandler  # Run specific test
go test -race ./...                     # Race condition detection
go test -cover ./...                    # Coverage summary
go test -coverprofile=coverage.out ./...  # Generate coverage profile
go tool cover -html=coverage.out        # View HTML coverage report
go test -bench=. ./internal/router      # Run benchmarks
go test -bench=. -benchmem ./...        # Benchmarks with memory allocation
```

## Test File Organization

**Location:**
- Co-located with source files (Go standard pattern)
- Test files in same directory as code they test
- Package name ends with `_test`: `package proxy_test` (for black-box tests) or `package proxy` (for white-box)

**Naming:**
- Test files: `*_test.go` suffix
- Test functions: `Test[FunctionName]` or `Test[FunctionName]_[Scenario]`
- Examples:
  - `TestProxyHandler` (basic test)
  - `TestProxyHandler_WithSSEStreaming` (scenario test)
  - `TestCircuitBreakerTransition_FromClosedToOpen` (state transition)
  - `TestRoundRobinStrategy_WithMultipleKeys` (parametric test)

**Structure:**
```
internal/proxy/
├── server.go
├── server_test.go           # Tests for server.go
├── sse.go
├── sse_test.go              # Tests for sse.go
└── middleware_test.go

internal/router/
├── router.go
├── router_test.go
├── strategies/
│   ├── shuffle.go
│   ├── shuffle_test.go
│   └── roundrobin_test.go
```

## Test Structure

**Suite Organization:**
```go
// Use table-driven tests for multiple scenarios
func TestProviderSelection(t *testing.T) {
    tests := []struct {
        name     string
        strategy string
        providers []*Provider
        want     string
        wantErr  bool
    }{
        {
            name:     "selects healthy provider",
            strategy: "shuffle",
            providers: []*Provider{...},
            want:     "anthropic-pool",
            wantErr:  false,
        },
        {
            name:     "fails when all providers unhealthy",
            strategy: "failover",
            providers: []*Provider{...},
            want:     "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := SelectProvider(tt.providers, tt.strategy)
            if (err != nil) != tt.wantErr {
                t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %s, want %s", got, tt.want)
            }
        })
    }
}
```

**Patterns:**
- **Setup:** Use helper functions to create test fixtures (providers, configs, keys)
  ```go
  func newTestProvider(name string, healthy bool) *Provider {
      return &Provider{
          Name:    name,
          Type:    "test",
          Enabled: true,
          Status:  conditionalStatus(healthy),
      }
  }
  ```

- **Teardown:** Use `t.Cleanup()` for resource cleanup (new in Go 1.14)
  ```go
  func TestProviderWithConnection(t *testing.T) {
      conn := setupTestConnection(t)
      t.Cleanup(func() { conn.Close() })
      // Test code using conn
  }
  ```

- **Assertion:** Manual comparison with descriptive error messages
  ```go
  if provider.Status != "healthy" {
      t.Errorf("provider status: got %q, want %q", provider.Status, "healthy")
  }
  ```

## Mocking

**Framework:**
- Go standard library only (no external mocking framework)
- Use interface-based design to enable easy mocking
- Create test implementations of interfaces in `*_test.go` files

**Patterns:**
```go
// Provider interface allows mocking
type Provider interface {
    TransformRequest(req *Request) (*http.Request, error)
    HealthCheck(ctx context.Context) error
}

// Mock implementation for testing
type mockProvider struct {
    transformFunc  func(*Request) (*http.Request, error)
    healthCheckFunc func(context.Context) error
}

func (m *mockProvider) TransformRequest(req *Request) (*http.Request, error) {
    if m.transformFunc != nil {
        return m.transformFunc(req)
    }
    return nil, nil
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
    if m.healthCheckFunc != nil {
        return m.healthCheckFunc(ctx)
    }
    return nil
}

// Usage in test
func TestRouterWithMockProvider(t *testing.T) {
    mock := &mockProvider{
        transformFunc: func(req *Request) (*http.Request, error) {
            // Simulate transformation
            return nil, errors.New("mock error")
        },
    }
    // Test router with mock
}
```

**What to Mock:**
- External service calls (provider APIs, health checks)
- I/O operations (file reads, network calls)
- Time-dependent operations (use `time.Time` injection or `context.WithTimeout`)
- Configuration loading (pass config as parameter instead of reading from disk)

**What NOT to Mock:**
- Internal business logic (route selection algorithms)
- Core data structures (circuits, rate limiters)
- Configuration parsing (use real YAML for integration tests)
- Error handling (test real error paths)

## Fixtures and Factories

**Test Data:**
```go
// Factory functions for common test objects
func newTestConfig() *config.Config {
    return &config.Config{
        Server: config.ServerConfig{Listen: "127.0.0.1:0"},
        Providers: []config.ProviderConfig{
            {
                Name: "test-provider",
                Type: "anthropic",
                Keys: []config.KeyConfig{
                    {Key: "test-key", RPMLimit: 60, TPMLimit: 100000},
                },
            },
        },
        Routing: config.RoutingConfig{Strategy: "simple-shuffle"},
    }
}

// Request/response builders for API tests
func newTestMessage(model string) *Message {
    return &Message{
        Model: model,
        Messages: []MessageBlock{
            {Role: "user", Content: "test"},
        },
    }
}
```

**Location:**
- `*_test.go` files in the same package
- Shared fixtures in `testdata/` subdirectory (YAML configs, mock responses)
- Golden files for response validation: `testdata/golden/provider_response.json`

## Coverage

**Requirements:**
- Target: 70%+ coverage for core packages
- No strict enforcement specified in SPEC, but coverage tracked per PR
- Focus on critical paths: routing, provider selection, circuit breaker state transitions

**View Coverage:**
```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View in terminal
go tool cover -func=coverage.out

# View in browser
go tool cover -html=coverage.out
```

**Coverage by Package:**
- `internal/proxy/`: High (request handling is critical)
- `internal/router/`: High (selection logic must be correct)
- `internal/providers/`: High (transformations are provider-specific)
- `internal/health/`: High (circuit breaker correctness is essential)
- `internal/config/`: Medium (mostly parsing, some error paths)
- `internal/grpc/`: Medium (gRPC boilerplate, test client calls)
- `ui/tui/`: Low (UI rendering is hard to test, focus on state logic)

## Test Types

**Unit Tests:**
- Scope: Individual functions/methods
- Approach: Table-driven tests with multiple scenarios
- Examples:
  - `TestCircuitBreaker_TransitionFromClosedToOpen` - state machine transitions
  - `TestRoundRobinRouter_DistributesRequestsEvenly` - distribution logic
  - `TestRateLimiter_EnforcesTPMLimit` - rate enforcement
  - `TestProviderTransformer_HandlesBedrockFormat` - provider-specific logic
- Dependencies: Mocked (providers, I/O)
- Execution: ~milliseconds per test

**Integration Tests:**
- Scope: Multiple components working together (router + provider, proxy + health tracker)
- Approach: Use real components but mock external services
- Examples:
  - `TestProxyFlow_EndToEnd` - request through entire proxy
  - `TestFailover_CircuitBreakerTriggersSecondProvider` - failover chains
  - `TestMultiKeyPooling_DistributesLoad` - key rotation
  - `TestSSEStreaming_PreservesEventOrder` - event sequence correctness
- Dependencies: Real internal components, mocked providers
- Execution: ~10-100ms per test

**E2E Tests (if applicable):**
- Framework: Would use local Ollama or Z.AI test endpoint
- Scope: Real proxy running, real provider calls (if available)
- Not planned for MVP (Phase 1-2), consider for Phase 3+
- Location: `tests/e2e/` directory
- Trigger: Separate CI job, requires environment setup

## Common Patterns

**Async Testing:**
```go
// Test concurrent provider calls
func TestRouter_ConcurrentSelection(t *testing.T) {
    router := NewRouter(providers)
    results := make(chan string, 100)

    for i := 0; i < 100; i++ {
        go func() {
            provider, _ := router.SelectProvider()
            results <- provider.Name
        }()
    }

    // Collect results
    distribution := make(map[string]int)
    for i := 0; i < 100; i++ {
        distribution[<-results]++
    }

    // Verify distribution is reasonable
    if distribution["provider1"] < 30 || distribution["provider1"] > 70 {
        t.Errorf("distribution skewed: %v", distribution)
    }
}
```

**Error Testing:**
```go
// Test error paths explicitly
func TestProxyHandler_InvalidRequest(t *testing.T) {
    tests := []struct {
        name      string
        req       *http.Request
        wantError string
    }{
        {
            name:      "missing model field",
            req:       newRequestWithoutModel(),
            wantError: "model is required",
        },
        {
            name:      "invalid JSON",
            req:       newRequestWithInvalidJSON(),
            wantError: "invalid JSON",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resp, err := handler.ServeHTTP(tt.req)
            if err == nil || !strings.Contains(err.Error(), tt.wantError) {
                t.Errorf("got %v, want error containing %q", err, tt.wantError)
            }
        })
    }
}
```

**Context Testing:**
```go
// Test timeout behavior
func TestProvider_RespectContextTimeout(t *testing.T) {
    provider := NewTestProvider()

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    // Start slow operation
    done := make(chan error)
    go func() {
        done <- provider.HealthCheck(ctx)
    }()

    // Should timeout
    select {
    case err := <-done:
        if err != context.DeadlineExceeded {
            t.Errorf("got %v, want context.DeadlineExceeded", err)
        }
    case <-time.After(500*time.Millisecond):
        t.Error("test timeout: healthcheck did not respect context deadline")
    }
}
```

## Benchmarking

**Run Benchmarks:**
```bash
go test -bench=. ./internal/router       # Run benchmarks
go test -bench=. -benchmem ./...         # Include memory allocations
go test -bench=. -benchtime=10s ./...    # Longer benchmark runs
```

**Benchmark Pattern:**
```go
func BenchmarkRouterSelection(b *testing.B) {
    router := newTestRouter()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        router.SelectProvider()
    }
}

func BenchmarkProviderTransform(b *testing.B) {
    provider := newTestProvider()
    req := newTestRequest()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        provider.TransformRequest(req)
    }
}
```

## Test Naming Conventions

**Pattern:** `Test[ComponentOrFunction][Scenario]` or `Test[ComponentOrFunction]_[Condition]`

**Examples:**
- `TestCircuitBreaker_TransitionToOpen` - Focus on state transition
- `TestProxyHandler_WithInvalidJSON` - Focus on input condition
- `TestRateLimiter_EnforcesLimit_WithConcurrentRequests` - Multiple conditions
- `TestSSEWriter_CompressesEvents` - Behavior/output focus

## Race Detection

**When to Use:**
- Always in CI pipeline: `go test -race ./...`
- Locally during development: `go test -race ./...`
- Especially for concurrent code: router selection, health tracking, metrics

**Fixed:** Fix all data race reports before committing

---

*Testing analysis: 2026-01-20*
