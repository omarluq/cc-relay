# Fix Test Failures Skill

**CRITICAL**: When tests fail, FIX THE CODE or the TEST, not the test configuration.

## Core Principle

❌ **NEVER** skip tests or modify test infrastructure to make them pass
✅ **ALWAYS** fix the actual code or test logic

## When Tests Fail

### Step 1: Identify the Failure

```bash
# Run tests to see failures
task test

# Or for quick feedback
task test-short

# Example output:
# --- FAIL: TestProxyHandler (0.00s)
#     proxy_test.go:42: Expected 200, got 500
```

### Step 2: Understand Why

```bash
# Run with verbose output
go test -v ./internal/proxy

# Run specific test
go test -run TestProxyHandler -v ./internal/proxy

# Check for race conditions
go test -race ./internal/proxy
```

### Step 3: Fix the Issue

Fix the code or test, **never skip or disable the test**.

## Common Test Failures

### 1. Logic Errors in Code

**Test Output:**
```
Expected: 200
Got: 500
```

❌ Wrong approach:
```go
// Skipping the test
func TestProxyHandler(t *testing.T) {
    t.Skip("TODO: fix later")  // NEVER DO THIS
}
```

✅ Right approach:
```go
// Fix the actual code
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Before (bug)
    w.WriteHeader(500)

    // After (fixed)
    if err := p.processRequest(r); err != nil {
        w.WriteHeader(500)
        return
    }
    w.WriteHeader(200)
}
```

### 2. Incorrect Test Expectations

**Test Output:**
```
Expected: []string{"a", "b"}
Got: []string{"b", "a"}
```

❌ Wrong approach:
```go
// Changing expectation without understanding
assert.Equal(t, []string{"b", "a"}, result)  // Just to make it pass
```

✅ Right approach:
```go
// If order matters, fix the code
func getItems() []string {
    items := fetchItems()
    sort.Strings(items)  // Ensure consistent order
    return items
}

// Or if order doesn't matter, fix the test
assert.ElementsMatch(t, []string{"a", "b"}, result)
```

### 3. Race Conditions

**Test Output:**
```
WARNING: DATA RACE
```

❌ Wrong approach:
```go
// Removing -race flag from tests
// NEVER DO THIS
```

✅ Right approach:
```go
// Fix the race condition in code
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}
```

### 4. Flaky Tests

**Symptom:** Test passes sometimes, fails others

❌ Wrong approach:
```go
// Retry logic in test
for i := 0; i < 3; i++ {  // NEVER DO THIS
    if test() == nil {
        return
    }
}
```

✅ Right approach:
```go
// Fix timing issues in code
func waitForReady(ctx context.Context) error {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if isReady() {
                return nil
            }
        }
    }
}
```

### 5. Mock Failures

**Test Output:**
```
Unexpected call to method X
```

❌ Wrong approach:
```go
// Removing mock expectations
// NEVER DO THIS
```

✅ Right approach:
```go
// Fix mock setup or code behavior
mockClient.EXPECT().
    SendRequest(gomock.Any()).
    Return(&Response{Status: 200}, nil).
    Times(2)  // Expect 2 calls, not 1
```

### 6. Nil Pointer Errors

**Test Output:**
```
panic: runtime error: invalid memory address
```

✅ Fix approach:
```go
// Add nil checks in code
func processConfig(cfg *Config) error {
    if cfg == nil {
        return errors.New("config cannot be nil")
    }
    // ... rest of code
}

// Or ensure proper initialization
func NewProxy(cfg *Config) *Proxy {
    if cfg == nil {
        cfg = DefaultConfig()
    }
    return &Proxy{config: cfg}
}
```

## Debugging Workflow

```bash
# 1. Run failing test with verbose
go test -v -run TestName ./package

# 2. Add debug logging if needed (temporarily)
log.Printf("DEBUG: value=%v", value)

# 3. Check test in isolation
go test -run TestName -count=1 ./package

# 4. Check for race conditions
go test -race -run TestName ./package

# 5. Fix the code

# 6. Verify fix
go test ./package

# 7. Run full suite
task test
```

## Integration Test Failures

```bash
# Run integration tests
task test-integration

# If they fail, check:
# 1. External dependencies (databases, APIs)
# 2. Test data setup
# 3. Cleanup between tests
```

Fix by:
- Ensuring proper test setup/teardown
- Using test containers for dependencies
- Isolating test data

## Test Coverage Drops

**When coverage decreases:**

❌ Wrong approach:
```yaml
# Lowering coverage threshold
# NEVER DO THIS
```

✅ Right approach:
```go
// Add tests for uncovered code
func TestNewFeature(t *testing.T) {
    result := newFeature()
    assert.NotNil(t, result)
    // Cover all branches
}
```

## Never Do This

❌ Skip failing tests with `t.Skip()`
❌ Remove test assertions to make them pass
❌ Disable race detection
❌ Increase timeouts without understanding why
❌ Comment out test code
❌ Remove tests from CI

## Always Do This

✅ Fix the underlying bug in code
✅ Fix incorrect test logic
✅ Add missing nil checks
✅ Fix race conditions properly
✅ Understand why test fails before fixing
✅ Keep tests deterministic
✅ Run tests after fixes

## Tips

1. **Read error messages**: They tell you exactly what's wrong
2. **Test in isolation**: Use `-run` to focus on one test
3. **Use verbose mode**: `go test -v` shows more detail
4. **Check for races**: Always run with `-race`
5. **Understand expectations**: Don't just change them to pass
6. **Fix root cause**: Not just the symptom
7. **Add tests**: When fixing bugs, add regression tests

## Asking for Help

When stuck:
1. Show the failing test output
2. Show the relevant code
3. Explain what you've tried
4. Ask specific questions
